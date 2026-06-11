# Housing waitlist scraper

Go CLI that crawls student-housing waitlist portals, normalizes each into a
common row model, writes a per-source CSV sorted by your waitlist position (best
first), optionally updates a Google Sheet, and emails the report.

Sources are pluggable. The built-in source is
[Kollegiernes Kontor i København](https://www.kollegierneskontor.dk) (`kkik`).
Adding another portal is a self-contained package plus a config block — the
fetch/CSV/sheet/email pipeline is source-agnostic.

## Requirements

- Go 1.22+
- Chromium (used by chromedp; installed automatically on first run or via system Chrome)

## Setup

```bash
cp internal/config/config.example.yaml internal/config/config.yaml
# Edit internal/config/config.yaml with your source credentials and SMTP settings
make build
```

Shortcuts: `make help` lists targets (`make run`, `make run-no-email`,
`make dump`, `make list-sources`, `make test`).

## Configuration

[`internal/config/config.yaml`](internal/config/config.yaml) (not committed —
copy from [`internal/config/config.example.yaml`](internal/config/config.example.yaml)).
`smtp` and `google` are shared by every source; each source has its own block
under `sources`.

```yaml
smtp:            # shared mail transport / sender
  from: you@example.com
  host: smtp.gmail.com
  port: 587
  user: you@example.com
  password: your-gmail-app-password

google:          # shared Google OAuth identity for Sheets
  oauth_client_file: ./internal/config/client_secret.json
  oauth_token_file: ./internal/config/token.json

sources:
  kkik:
    enabled: true
    steps:                 # crawl always runs; email/sheet default to true
      email: true
      sheet: true
    login:
      email: you@example.com
      password: your-password
    headless: true
    timeout_sec: 120
    sheet:
      spreadsheet_id: "your-spreadsheet-id"
      sheet_name: Sheet1
    email:
      to: you@example.com
    data_dir: ./data/kkik  # CSV history dir (defaults to ./data/<source>)
```

Loaded with [cleanenv](https://github.com/ilyakaznacheev/cleanenv). Secrets can
come from the environment instead of YAML: `SMTP_HOST`, `SMTP_PORT`,
`SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`, `GOOGLE_OAUTH_CLIENT_FILE`,
`GOOGLE_OAUTH_TOKEN_FILE`, and per-source `KKIK_EMAIL`, `KKIK_PASSWORD`,
`KKIK_TIMEOUT_SEC`, `KKIK_DEBUG_DIR`. Set `CONFIG_PATH` to use another file.

### Sources and steps

| Setting          | Default | What it does                                                       |
| ---------------- | ------- | ------------------------------------------------------------------ |
| `enabled`        | `false` | Include this source when no `--source` flag is given               |
| (crawl)          | always  | Live scrape + write timestamped CSV under `data_dir`               |
| `steps.email`    | `true`  | Send SMTP report with CSV attachment (needs `email.to`)            |
| `steps.sheet`    | `true`  | Upload rows to Google Sheets (needs `sheet.spreadsheet_id`)        |

Each run executes the pipeline per source: **fetch → parse → CSV → sheet →
email**. Outputs are per source and never merged.

## Usage

```bash
make run                 # run all enabled sources
./waitlist               # same
./waitlist --source kkik # run a specific source (even if enabled: false)
./waitlist --list-sources
make run-no-email        # all enabled sources, skip email
make dump SRC=kkik       # scrape one source → debug.html, skip email + sheet
```

| Flag               | Description                                                  |
| ------------------ | ----------------------------------------------------------- |
| `--source a,b`     | Comma-separated sources to run (default: all enabled)       |
| `--list-sources`   | Print registered sources and exit                           |
| `--no-email`       | Skip the email step for this run                            |
| `--no-sheet`       | Skip the Google Sheets step for this run                    |
| `--dump-html PATH` | Save fetched HTML (requires a single `--source`)            |
| `--auth-sheets`    | One-time Google OAuth; writes `token.json` (shared)         |

## CSV

Per source, written to `data/<source>/<timestamp>_waitlist.csv`:

```
request_id,dorm,room_type,size_sqm,your_rank,diff
```

Sorted by rank order (lower = better). `your_rank` is the rank as the source
displays it; `diff` compares against the latest previous CSV (`+N` = improved).
A source's rank may be numeric (KKIK) or otherwise (e.g. letters) — each source
maps its rank to a sortable order so sorting and diffing work uniformly.

## Google Sheets (OAuth)

Uses your Google account (no service account). One-time OAuth setup, then cron
runs headless. The OAuth identity in `google:` is shared across all sources;
each source targets its own `sheet.spreadsheet_id` / `sheet_name`.

1. In [Google Cloud Console](https://console.cloud.google.com/), create or select a project and enable **Google Sheets API**.
2. Configure **Google Auth Platform** (Google renamed “OAuth consent screen” — the old menu item often redirects to Overview):
    - Open the **☰ menu** → **Google Auth platform** (not “APIs & Services → OAuth consent screen”).
    - If Overview says **Get started**, complete the wizard: app name, **External** user type, your email, agree to policy → **Create**.
    - **Audience** — add **Test users** (your Gmail); keep **Publishing status** on **Testing**.
    - **Data access** — **Add or remove scopes** → Google Sheets API → `.../auth/spreadsheets` → **Save**.
    - Direct links (replace `YOUR_PROJECT_ID` with `project_id` from `client_secret.json`, e.g. `j4f-xd`):
        - [Overview](https://console.cloud.google.com/auth/overview?project=YOUR_PROJECT_ID)
        - [Audience (test users)](https://console.cloud.google.com/auth/audience?project=YOUR_PROJECT_ID)
        - [Data access (scopes)](https://console.cloud.google.com/auth/scopes?project=YOUR_PROJECT_ID)
3. **Google Auth platform → Clients** (or **APIs & Services → Credentials**) → **Create credentials** → **OAuth client ID** → **Desktop app** → download JSON as `client_secret.json`.
4. Create or open a spreadsheet you own. Copy the ID from the URL:
   `https://docs.google.com/spreadsheets/d/<SPREADSHEET_ID>/edit`
5. Set the source's `sheet.spreadsheet_id` (and optional `sheet_name` for the tab) in `config.yaml`.
6. Authorize once:

```bash
./waitlist --auth-sheets
```

This prints a URL, listens on `http://127.0.0.1:8080/`, and saves `token.json`.
Cron and normal runs reuse `token.json` automatically.

### Sheet layout

The tab keeps fixed columns (`request_id`, `dorm`, `room_type`, `size_sqm`,
`latest_diff`) plus one **ddmmyy** column per calendar day (e.g. `260526` for 26
May 2026). Each run appends a new day on the right, or updates that day’s column
if you run again the same day. Ranks are backfilled from
`data/<source>/*_waitlist.csv`. **`latest_diff`** (after `size_sqm`) shows change
vs the previous day’s rank (`+N` = improved).

### Why there is no “redirect URI” field

If your OAuth client type is **Desktop app** (your `client_secret.json` has an `"installed"` section), Google **does not show** authorized redirect URIs in the console. That is normal: Desktop apps use the [loopback flow](https://developers.google.com/identity/protocols/oauth2/native-app) (`http://127.0.0.1:PORT`). You do not need to add URIs manually.

To **see and edit** redirect URIs, you would need a separate **Web application** client (Credentials → Create credentials → OAuth client ID → **Web application** → **Authorized redirect URIs**). This project is set up for **Desktop** + loopback on port 8080; stick with Desktop unless you have a specific reason to switch.

### OAuth troubleshooting

**Error 403: `access_denied`** (on Google’s sign-in page, before the app gets a code):

1. **Google Auth platform → Audience → Test users** — add the exact Gmail you use in the browser. Status must be **Testing** (not Production unless verified).
2. **Google Auth platform → Data access** — scope `https://www.googleapis.com/auth/spreadsheets` must be listed.
3. **Unverified app** — **Advanced** → **Go to … (unsafe)** on the warning screen.
4. **Work/school account** — try personal `@gmail.com` or ask an admin.

**“OAuth consent screen” redirects to Overview** — use **Google Auth platform** in the ☰ menu and open **Audience** / **Data access** from the left sidebar (or the direct links above). Try an incognito window if the UI loops.

**Error 400: `redirect_uri_mismatch`** — With a Desktop client, use the built-in auth (`make auth-sheets`); do not create a Web client unless you also change `client_secret.json` and add matching URIs there.

## Adding a source

1. Create `internal/source/<name>/` with a type implementing `source.Source`
   (`Descriptor`, `Fetch`, `Parse`, `RankOrder`) and a `func init()` that calls
   `source.Register("<name>", …)`.
2. Blank-import it from [`internal/source/all`](internal/source/all/import.go).
3. Add a `<Name>Config` struct + a `Sources` field + a `case` in
   `config.Source()` / `config.SourceNames()`.

No changes to `export`, `sheets`, `mailer`, or `runner` are needed.

## Architecture

```
cmd/waitlist          CLI: flags → runner; blank-imports source/all
internal/
  model               shared row model (RankDisplay + RankOrder)
  source              Source interface + registry
  source/kkik         KKIK source (scraper + parser + selectors)
  source/all          blank-imports every source for registration
  runner              per-source fetch→parse→CSV→sheet→email loop
  export              CSV, diff, history, prev-rank loading
  sheets              Google Sheets append-by-date
  mailer              SMTP report
  config              shared (smtp/google) + per-source structs
data/<source>/        per-source CSV history
```

## Cron (daily example)

```bash
0 8 * * * cd /path/to/dom && ./waitlist >> /var/log/housing-waitlist.log 2>&1
```

Non-zero exit on login, parse, sheet, or mail failure.
