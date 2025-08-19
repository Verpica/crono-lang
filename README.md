# Crono 🕐

**Crono** is a modern cross-platform task scheduler with a readable and intuitive DSL (Domain Specific Language) for managing your cron jobs elegantly.

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)](https://github.com/Verpica/crono)

## 🚀 Features

- ✨ **Modern and readable DSL** : Intuitive syntax to define your tasks
- 🌍 **Timezone support** : Native timezone management
- 🔄 **Retry handling** : Automatic retry with configurable backoff  
- ⏱️ **Jitter and timeout** : Advanced execution control
- 📝 **Validation and explanation** : Built-in debugging tools
- 🖥️ **Cross-platform** : Compatible with macOS, Windows and Linux

## 📦 Installation

### Prerequisites
- Go 1.22 or higher

### Build from source
```bash
git clone https://github.com/Verpica/crono-lang.git
cd crono
go build -o crono ./cmd/crono
```

### Direct installation
```bash
go install github.com/Verpica/crono-lang/cmd/crono@latest
```

## 📖 Usage Guide

### DSL Syntax

Crono uses a simple and expressive DSL to define your scheduled tasks:

```crono
job "daily_report" {
  schedule:  every weekday at 08:30 Europe/Paris
  run:       sh "echo 'Generating daily report'"
  retry:     3 with backoff 5s..30s
  timeout:   30s
  jitter:    ±10s
  overlap:   skip
}

job "backup_database" {
  schedule:  every day at 02:00
  run:       sh "/scripts/backup.sh"
  timeout:   1h
  overlap:   skip
}

job "health_check" {
  schedule:  every 5m
  run:       sh "curl -f http://localhost:8080/health"
  retry:     2 with backoff 1s..5s
  timeout:   10s
}
```

### Supported schedule expressions

| Expression | Description |
|------------|-------------|
| `every 5m` | Every 5 minutes |
| `every 1h` | Every hour |
| `every day at 14:30` | Every day at 2:30 PM |
| `every weekday at 09:00` | Weekdays at 9:00 AM |
| `every monday at 08:00` | Every Monday at 8:00 AM |
| `every week` | Every week |

### Advanced options

- **`retry`** : Number of attempts with backoff strategy
- **`timeout`** : Command execution timeout
- **`jitter`** : Random variation to avoid load spikes
- **`overlap`** : Behavior on overlap (`skip` | `queue` | `cancel-prev`)

## 🔧 CLI Commands

### Execution
```bash
crono run examples/jobs.crn
```

### Validation
```bash
crono validate examples/jobs.crn
```

### Preview next executions
```bash
crono next examples/jobs.crn -n 10
```

### Detailed explanation
```bash
crono explain examples/jobs.crn
```

## 📁 Project Structure

```
crono/
├── cmd/crono/          # Application entry point
├── internal/
│   ├── dsl/           # DSL parser
│   ├── execer/        # Command executor
│   └── scheduler/     # Scheduling engine
├── examples/          # Example .crn files
├── go.mod
└── README.md
```

## 🌟 Usage Examples

### Automatic backup
```crono
job "backup_daily" {
  schedule:  every day at 03:00
  run:       sh "tar -czf /backups/backup-$(date +%Y%m%d).tar.gz /data"
  timeout:   2h
  overlap:   skip
}
```

### System monitoring
```crono
job "disk_check" {
  schedule:  every 30m
  run:       sh "df -h | awk '$5 > 80 { print $0 }' | mail -s 'Disk Alert' admin@example.com"
  timeout:   30s
}
```

### Log cleanup
```crono
job "cleanup_logs" {
  schedule:  every sunday at 01:00
  run:       sh "find /var/log -name '*.log' -mtime +30 -delete"
  timeout:   10m
}
```

## 🗺️ Roadmap

- [ ] **State persistence** : Save last execution states
- [ ] **Exclusions** : Support for `except:` holidays/dates
- [ ] **HTTP jobs** : HTTP request support
- [ ] **Prometheus metrics** : Advanced monitoring
- [ ] **cron/systemd export** : Compile to native formats
- [ ] **Web interface** : Management dashboard
- [ ] **Notifications** : Email/Slack/Discord alerts
- [ ] **Conditions** : Conditional execution

## 🤝 Contributing

Contributions are welcome! Feel free to:

1. Fork the project
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by the best cron and scheduling tools
- Thanks to the Go community for excellent libraries

---

**Crono** - Schedule with style 🕐✨
