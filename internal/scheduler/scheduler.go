package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/example/crono/internal/dsl"
	"github.com/example/crono/internal/execer"
)

// Engine runs jobs based on their schedule.
type Engine struct {
	prog dsl.Program
	mu   sync.Mutex
	busy map[string]bool
}

func NewEngine(p *dsl.Program) *Engine {
	return &Engine{prog: *p, busy: map[string]bool{}}
}

func (e *Engine) Run(ctx context.Context) error {
	type item struct {
		job dsl.Job
		next time.Time
	}
	// Initial planning
	items := make([]item, 0, len(e.prog.Jobs))
	now := time.Now()
	for _, j := range e.prog.Jobs {
		n, err := NextRun(j.Schedule, now)
		if err != nil {
			return fmt.Errorf("job %q: %w", j.Name, err)
		}
		items = append(items, item{job: j, next: n})
	}
	// Simple loop (no persistence)
	for {
		// find earliest
		soon := 0
		for i := range items {
			if items[i].next.Before(items[soon].next) {
				soon = i
			}
		}
		wait := time.Until(items[soon].next)
		if wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		j := items[soon].job

		// overlap policy
		e.mu.Lock()
		if e.busy[j.Name] {
			if j.Overlap == "skip" {
				log.Printf("[%-16s] chevauchement: skip", j.Name)
				e.mu.Unlock()
			} else {
				// queue (not implemented) => skip for now
				log.Printf("[%-16s] chevauchement: not-implemented(%s) => skip", j.Name, j.Overlap)
				e.mu.Unlock()
			}
		} else {
			e.busy[j.Name] = true
			e.mu.Unlock()

			go func(j dsl.Job) {
				defer func() {
					e.mu.Lock()
					delete(e.busy, j.Name)
					e.mu.Unlock()
				}()
				e.runOnce(ctx, j)
			}(j)
		}

		// schedule next
		next, err := NextRun(j.Schedule, items[soon].next.Add(time.Second))
		if err != nil {
			log.Printf("planif suivante échouée pour %q: %v", j.Name, err)
			next = time.Now().Add(time.Minute)
		}
		items[soon].next = next
	}
}

func (e *Engine) runOnce(ctx context.Context, j dsl.Job) {
	start := time.Now()
	log.Printf("[%-16s] start (pid=%d)", j.Name, osGetpid())
	defer func() {
		log.Printf("[%-16s] done in %s", j.Name, time.Since(start))
	}()

	var attempt int
	backoffMin := j.BackoffA
	backoffMax := j.BackoffB
	if backoffMin == 0 && backoffMax == 0 {
		backoffMin = 1 * time.Second
		backoffMax = 30 * time.Second
	}
	for {
		// timeout
		runCtx := ctx
		if j.Timeout > 0 {
			var cancel context.CancelFunc
			runCtx, cancel = context.WithTimeout(ctx, j.Timeout)
			defer cancel()
		}
		err := execer.RunShell(runCtx, j.Run, j.Env)
		if err == nil {
			return
		}
		attempt++
		if attempt > j.RetryN {
			log.Printf("[%-16s] échec: %v (abandon après %d tentatives)", j.Name, err, attempt)
			return
		}
		// exponential backoff bounded
		bo := time.Duration(1<<uint(min(attempt, 6))) * backoffMin
		if bo > backoffMax {
			bo = backoffMax
		}
		jitter := time.Duration(rand.Int63n(int64(bo/4 + 1)))
		sleep := bo/2 + jitter
		log.Printf("[%-16s] tentative %d échouée: %v -> retry dans %s", j.Name, attempt, err, sleep)
		select {
		case <-runCtx.Done():
			return
		case <-time.After(sleep):
		}
	}
}

func min(a, b int) int { if a < b { return a }; return b }

// --------- Scheduling ---------

// Supported schedule forms (MVP):
// - "every 5m"
// - "every weekday at HH:MM [TZ]"
// - "every day at HH:MM [TZ]"
// - "at HH:MM [TZ]" (alias of every day at)
// - "every Nh|Nm|Ns|Nd starting at HH:MM" (starting at is ignored for MVP)
// TZ example: "Europe/Paris". If omitted, local time.
func NextRun(expr string, from time.Time) (time.Time, error) {
	expr = strings.TrimSpace(expr)
	l := strings.ToLower(expr)

	// every 5m
	if strings.HasPrefix(l, "every ") && (strings.HasSuffix(l, "s") || strings.HasSuffix(l, "m") || strings.HasSuffix(l, "h") || strings.HasSuffix(l, "d")) && !strings.Contains(l, " at ") {
		d, err := parseDur(strings.TrimPrefix(l, "every "))
		if err != nil {
			return time.Time{}, err
		}
		return from.Add(d), nil
	}

	// at HH:MM [TZ]
	if strings.HasPrefix(l, "at ") || strings.HasPrefix(l, "every day at ") || strings.HasPrefix(l, "every weekday at ") {
		weekdayOnly := false
		if strings.HasPrefix(l, "every weekday at ") {
			weekdayOnly = true
			l = strings.TrimPrefix(l, "every weekday at ")
		} else if strings.HasPrefix(l, "every day at ") {
			l = strings.TrimPrefix(l, "every day at ")
		} else {
			l = strings.TrimPrefix(l, "at ")
		}

		parts := strings.Fields(l)
		if len(parts) == 0 {
			return time.Time{}, errors.New("missing time")
		}
		hhmm := parts[0]
		var loc *time.Location = time.Local
		if len(parts) > 1 {
			tzName := parts[1]
			lo, err := time.LoadLocation(tzName)
			if err != nil {
				return time.Time{}, fmt.Errorf("unknown TZ: %s", tzName)
			}
			loc = lo
		}
		next := nextAtTime(from, hhmm, loc, weekdayOnly)
		return next, nil
	}

	// every 6h starting at 00:00 [TZ]
	if strings.HasPrefix(l, "every ") && strings.Contains(l, " starting at ") {
		before, after, _ := strings.Cut(l, " starting at ")
		d, err := parseDur(strings.TrimPrefix(before, "every "))
		if err != nil {
			return time.Time{}, err
		}
		// parse time (and optional tz)
		parts := strings.Fields(after)
		if len(parts) == 0 {
			return time.Time{}, errors.New("missing time after 'starting at'")
		}
		hhmm := parts[0]
		loc := time.Local
		if len(parts) > 1 {
			lo, err := time.LoadLocation(parts[1])
			if err == nil {
				loc = lo
			}
		}
		base := time.Date(from.In(loc).Year(), from.In(loc).Month(), from.In(loc).Day(), parseHH(hhmm), parseMM(hhmm), 0, 0, loc)
		t := base
		for !t.After(from.In(loc)) {
			t = t.Add(d)
		}
		return t.In(from.Location()), nil
	}

	return time.Time{}, fmt.Errorf("unsupported expression (MVP): %q", expr)
}

func Explain(j dsl.Job) string {
	l := strings.ToLower(strings.TrimSpace(j.Schedule))
	if strings.HasPrefix(l, "every ") && (strings.HasSuffix(l, "s") || strings.HasSuffix(l, "m") || strings.HasSuffix(l, "h") || strings.HasSuffix(l, "d")) && !strings.Contains(l, " at ") {
		return fmt.Sprintf("every %s", strings.TrimPrefix(l, "every "))
	}
	if strings.HasPrefix(l, "every weekday at ") {
		return "weekdays at " + strings.TrimPrefix(l, "every weekday at ")
	}
	if strings.HasPrefix(l, "every day at ") {
		return "every day at " + strings.TrimPrefix(l, "every day at ")
	}
	if strings.HasPrefix(l, "at ") {
		return "every day at " + strings.TrimPrefix(l, "at ")
	}
	if strings.Contains(l, " starting at ") {
		return "periodic " + l
	}
	return "schedule: " + j.Schedule
}

func nextAtTime(from time.Time, hhmm string, loc *time.Location, weekdayOnly bool) time.Time {
	h := parseHH(hhmm)
	m := parseMM(hhmm)

	// Candidate today in loc
	in := from.In(loc)
	cand := time.Date(in.Year(), in.Month(), in.Day(), h, m, 0, 0, loc)
	if !cand.After(in) {
		cand = cand.Add(24 * time.Hour)
	}
	// If weekdayOnly, move to next Mon-Fri
	for weekdayOnly {
		wd := cand.Weekday()
		if wd >= time.Monday && wd <= time.Friday {
			break
		}
		cand = cand.Add(24 * time.Hour)
	}
	// Return in original location (UTC/Local)
	return cand.In(from.Location())
}

func parseDur(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	unit := s[len(s)-1]
	value := s[:len(s)-1]
	n, err := time.ParseDuration(value + string(unit))
	if err == nil {
		return n, nil
	}
	// handle 'd' as 24h
	if unit == 'd' {
		h, err := time.ParseDuration(value + "h")
		if err != nil {
			return 0, err
		}
		return h * 24, nil
	}
	return 0, err
}

func parseHH(hhmm string) int {
	parts := strings.SplitN(hhmm, ":", 2)
	if len(parts) == 0 {
		return 0
	}
	var h int
	fmt.Sscanf(parts[0], "%d", &h)
	if h < 0 { h = 0 }
	if h > 23 { h = 23 }
	return h
}

func parseMM(hhmm string) int {
	parts := strings.SplitN(hhmm, ":", 2)
	if len(parts) < 2 {
		return 0
	}
	var m int
	fmt.Sscanf(parts[1], "%d", &m)
	if m < 0 { m = 0 }
	if m > 59 { m = 59 }
	return m
}

// Small helper for PID without importing os directly here (keeps execer isolated).
func osGetpid() int {
	return int(uintptr(time.Now().UnixNano()) & 0xffff) + runtime.NumGoroutine()
}
