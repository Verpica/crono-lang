package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/example/crono/internal/dsl"
	"github.com/example/crono/internal/scheduler"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "validate":
		validateCmd(os.Args[2:])
	case "next":
		nextCmd(os.Args[2:])
	case "explain":
		explainCmd(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`crono - mini scheduler DSL
Usage:
  crono run <file.crn>           # start the scheduler and execute jobs
  crono validate <file.crn>      # validate syntax
  crono next <file.crn> -n 5     # display next occurrences
  crono explain <file.crn>       # explain each job in text
`)
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	file := fs.String("f", "", ".crn file (alias: positional arg)")
	fs.Parse(args)
	var path string
	if *file != "" {
		path = *file
	} else if fs.NArg() > 0 {
		path = fs.Arg(0)
	} else {
		log.Fatal("specify a .crn file")
	}

	prog, err := dsl.ParseFile(path)
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}
	ctx := context.Background()
	engine := scheduler.NewEngine(prog)
	log.Printf("Starting scheduler (%d job[s])", len(prog.Jobs))
	if err := engine.Run(ctx); err != nil {
		log.Fatalf("Scheduler: %v", err)
	}
}

func validateCmd(args []string) {
	if len(args) == 0 {
		log.Fatal("specify a .crn file")
	}
	prog, err := dsl.ParseFile(args[0])
	if err != nil {
		log.Fatalf("Invalid: %v", err)
	}
	fmt.Printf("OK: %d job(s)\n", len(prog.Jobs))
}

func nextCmd(args []string) {
	fs := flag.NewFlagSet("next", flag.ExitOnError)
	n := fs.Int("n", 5, "number of occurrences")
	from := fs.String("from", "", "start time (RFC3339, default: now)")
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatal("specify a .crn file")
	}
	prog, err := dsl.ParseFile(fs.Arg(0))
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}
	start := time.Now()
	if *from != "" {
		if t, e := time.Parse(time.RFC3339, *from); e == nil {
			start = t
		} else {
			log.Fatalf("invalid from: %v", e)
		}
	}
	for _, j := range prog.Jobs {
		fmt.Printf("Job %q:\n", j.Name)
		t := start
		for i := 0; i < *n; i++ {
			next, err := scheduler.NextRun(j.Schedule, t)
			if err != nil {
				fmt.Printf("  error: %v\n", err)
				break
			}
			fmt.Printf("  %s\n", next.Format(time.RFC3339))
			t = next.Add(time.Second)
		}
	}
}

func explainCmd(args []string) {
	if len(args) == 0 {
		log.Fatal("specify a .crn file")
	}
	prog, err := dsl.ParseFile(args[0])
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}
	for _, j := range prog.Jobs {
		fmt.Printf("Job %q: %s\n", j.Name, scheduler.Explain(j))
	}
}
