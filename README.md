# crono (MVP)

A mini **cross-platform** scheduler (macOS, Windows, Linux) with a readable **DSL** for executing cron-style commands.

## Install / Compile

```bash
# With Go 1.22+
cd crono
go build ./cmd/crono
```

## DSL (MVP)
Supported examples:
```txt
job "daily_report" {
  schedule:  every weekday at 08:30 Europe/Paris
  run:       sh "echo 'Hello'"
  retry:     3 with backoff 5s..30s
  timeout:   30s
  jitter:    ±10s
  overlap:   skip
}

job "heartbeat" {
  schedule:  every 5m
  run:       sh "date"
}
```

## Commands

```
crono validate examples/jobs.crn
crono explain  examples/jobs.crn
crono next     examples/jobs.crn -n 5
crono run      examples/jobs.crn
```

## Notes
- Timezones: `Europe/Paris` is supported via the system TZ database.
- **MVP**: some options are parsed but not fully implemented (e.g. `queue`, `cancel-prev`).

## Roadmap
- Persistence (state, last run)
- `except:` holidays / dates
- HTTP jobs
- Prometheus metrics
- Compiler to cron/systemd (optional)
