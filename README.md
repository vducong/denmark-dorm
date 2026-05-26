# KKIK waitlist scraper

Go CLI that logs into [Kollegiernes Kontor i København](https://www.kollegierneskontor.dk), reads your accommodation wishes (waitlist positions), writes a CSV sorted by your waitlist number (best first), and emails the report.

## Requirements

- Go 1.22+
- Chromium (used by chromedp; installed automatically on first run or via system Chrome)

## Setup

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your KKIK and SMTP credentials
make build
```

Shortcuts: `make help` lists targets (`make parse`, `make run`, `make run-no-email`, `make dump`, `make test`).

## Configuration

[`config.yaml`](config.yaml) (not committed — copy from [`config.example.yaml`](config.example.yaml)):

```yaml
kkik:
  email: you@example.com
  password: your-password
  headless: true
  timeout_sec: 120

email:
  to: you@example.com
  from: you@example.com
  smtp_host: smtp.gmail.com
  smtp_port: 587
  smtp_user: you@example.com
  smtp_password: your-gmail-app-password
```

Loaded with [cleanenv](https://github.com/ilyakaznacheev/cleanenv). Optional env overrides still work (e.g. `KKIK_EMAIL`, `EMAIL_TO`). Set `CONFIG_PATH` to use another file.

## Usage

**Offline parse** (no login, uses saved HTML):

```bash
make parse
```

**Full run** (login, scrape, CSV, email):

```bash
make run
```

**Scrape without email:**

```bash
make run-no-email
```

**Debug live HTML:**

```bash
make dump
```

| Flag                | Description                         |
| ------------------- | ----------------------------------- |
| `--parse-only PATH` | Parse local HTML only               |
| `--no-email`        | Skip SMTP                           |
| `--dump-html PATH`  | Save fetched HTML after live scrape |

## CSV

Columns: `request_id,dorm,room_type,size_sqm,your_rank`

Sorted by `your_rank` ascending (lower = better position). The KKIK page shows **your** position on each list, not total queue size.

## Cron (daily example)

```bash
0 8 * * * cd /path/to/dom && ./kkik-waitlist >> /var/log/kkik-waitlist.log 2>&1
```

Non-zero exit on login, parse, or mail failure.
