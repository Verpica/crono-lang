package dsl

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Program struct {
	Jobs []Job
}

type Job struct {
	Name     string
	Schedule string // raw schedule expression
	Run      string // command to run (shell string)
	RetryN   int
	BackoffA time.Duration
	BackoffB time.Duration
	Timeout  time.Duration
	Jitter   time.Duration
	Overlap  string // "skip" | "queue" | "cancel-prev" (only skip implemented)
	Env      map[string]string
}

var (
	reJobStart   = regexp.MustCompile(`^\s*job\s+"([^"]+)"\s*{\s*$`)
	reKV         = regexp.MustCompile(`^\s*([a-zA-Z_]+)\s*:\s*(.+?)\s*$`)
	reJobEnd     = regexp.MustCompile(`^\s*}\s*$`)
	reString     = regexp.MustCompile(`^"(.*)"$`)
	reRetry      = regexp.MustCompile(`^(\d+)\s+with\s+backoff\s+([0-9smhd]+)\.\.([0-9smhd]+)$`)
	reDuration   = regexp.MustCompile(`^([0-9]+)([smhd])$`)
)

func ParseFile(path string) (*Program, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

func Parse(f *os.File) (*Program, error) {
	sc := bufio.NewScanner(f)
	var prog Program

	var cur *Job
	lineno := 0
	for sc.Scan() {
		lineno++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if cur == nil {
			m := reJobStart.FindStringSubmatch(line)
			if m != nil {
				cur = &Job{Name: m[1], RetryN: 0, BackoffA: 0, BackoffB: 0, Timeout: 0, Jitter: 0, Overlap: "skip", Env: map[string]string{}}
				continue
			}
			return nil, fmt.Errorf("line %d: expected 'job \"name\" {'", lineno)
		} else {
			if reJobEnd.MatchString(line) {
				prog.Jobs = append(prog.Jobs, *cur)
				cur = nil
				continue
			}
			kv := reKV.FindStringSubmatch(line)
			if kv == nil {
				return nil, fmt.Errorf("line %d: invalid key:value", lineno)
			}
			key := kv[1]
			val := strings.TrimSpace(kv[2])
			switch key {
			case "schedule":
				cur.Schedule = val
			case "run":
				// run: sh "echo hello" | exec "cmd"
				// For MVP, accept: sh "...."
				if strings.HasPrefix(val, "sh ") {
					s := strings.TrimSpace(strings.TrimPrefix(val, "sh "))
					m := reString.FindStringSubmatch(s)
					if m == nil {
						return nil, fmt.Errorf("line %d: run: sh \"...\"", lineno)
					}
					cur.Run = m[1]
				} else if strings.HasPrefix(val, "exec ") {
					// exec "command args"
					s := strings.TrimSpace(strings.TrimPrefix(val, "exec "))
					m := reString.FindStringSubmatch(s)
					if m == nil {
						return nil, fmt.Errorf("line %d: run: exec \"...\"", lineno)
					}
					cur.Run = m[1]
				} else {
					return nil, fmt.Errorf("line %d: unknown run (use 'sh' or 'exec')", lineno)
				}
			case "retry":
				m := reRetry.FindStringSubmatch(val)
				if m == nil {
					return nil, fmt.Errorf("line %d: retry: '<N> with backoff <a>..<b>'", lineno)
				}
				n, _ := strconv.Atoi(m[1])
				a, err := parseShortDuration(m[2])
				if err != nil {
					return nil, fmt.Errorf("line %d: %v", lineno, err)
				}
				b, err := parseShortDuration(m[3])
				if err != nil {
					return nil, fmt.Errorf("line %d: %v", lineno, err)
				}
				if a > b {
					return nil, fmt.Errorf("line %d: backoff min > max", lineno)
				}
				cur.RetryN = n
				cur.BackoffA = a
				cur.BackoffB = b
			case "timeout":
				d, err := parseShortDuration(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: %v", lineno, err)
				}
				cur.Timeout = d
			case "jitter":
				// accept "±10s" or "10s"
				val = strings.TrimPrefix(val, "±")
				d, err := parseShortDuration(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: %v", lineno, err)
				}
				cur.Jitter = d
			case "overlap":
				v := strings.TrimSpace(val)
				if v != "skip" && v != "queue" && v != "cancel-prev" {
					return nil, fmt.Errorf("line %d: overlap: 'skip' | 'queue' | 'cancel-prev'", lineno)
				}
				cur.Overlap = v
			case "env":
				// env: { KEY: "VALUE", K2: "V2" }
				if !strings.HasPrefix(val, "{") || !strings.HasSuffix(val, "}") {
					return nil, fmt.Errorf("line %d: env: { KEY: \"VALUE\" }", lineno)
				}
				content := strings.TrimSpace(val[1:len(val)-1])
				if content == "" {
					continue
				}
				parts := strings.Split(content, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					kv := strings.SplitN(p, ":", 2)
					if len(kv) != 2 {
						return nil, fmt.Errorf("line %d: env: invalid entry", lineno)
					}
					k := strings.TrimSpace(kv[0])
					v := strings.TrimSpace(kv[1])
					k = strings.Trim(k, "\"")
					v = strings.Trim(v, "\"")
					cur.Env[k] = v
				}
			default:
				return nil, fmt.Errorf("line %d: unknown key '%s'", lineno, key)
			}
		}
	}
	if cur != nil {
		return nil, errors.New("end of file during a job block")
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return &prog, nil
}

func parseShortDuration(s string) (time.Duration, error) {
	m := reDuration.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("invalid duration: %q (expected Ns|Nm|Nh|Nd)", s)
	}
	n, _ := strconv.Atoi(m[1])
	switch m[2] {
	case "s":
		return time.Duration(n) * time.Second, nil
	case "m":
		return time.Duration(n) * time.Minute, nil
	case "h":
		return time.Duration(n) * time.Hour, nil
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit")
	}
}
