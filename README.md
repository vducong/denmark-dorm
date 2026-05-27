# KKIK waitlist scraper

Go CLI that logs into [Kollegiernes Kontor i København](https://www.kollegierneskontor.dk), reads your accommodation wishes (waitlist positions), writes a CSV sorted by your waitlist number (best first), optionally updates a Google Sheet, and emails the report.

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

sheets:
    spreadsheet_id: "your-spreadsheet-id"
    sheet_name: Sheet1
    oauth_client_file: ./client_secret.json
    oauth_token_file: ./token.json
```

Loaded with [cleanenv](https://github.com/ilyakaznacheev/cleanenv). Optional env overrides still work (e.g. `KKIK_EMAIL`, `EMAIL_TO`, `SHEETS_SPREADSHEET_ID`). Set `CONFIG_PATH` to use another file.

Leave `sheets.spreadsheet_id` empty to skip Google Sheets upload.

## Google Sheets (OAuth)

Uses your Google account (no service account). You need a one-time OAuth setup, then cron runs headless.

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
5. Set `sheets.spreadsheet_id` (and optional `sheet_name` for the tab) in `config.yaml`.
6. Authorize once:

```bash
./kkik-waitlist --auth-sheets
```

This prints a URL, listens on `http://127.0.0.1:8080/`, and saves `token.json`.

Cron and normal runs reuse `token.json` automatically.

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

## Usage

**Offline parse** (no login, uses saved HTML):

```bash
make parse
```

**Full run** (login, scrape, CSV, optional sheet, email):

```bash
make run
```

**Scrape without email** (still updates the sheet if configured):

```bash
make run-no-email
```

**Debug live HTML:**

```bash
make dump
```

| Flag                | Description                                |
| ------------------- | ------------------------------------------ |
| `--parse-only PATH` | Parse local HTML only                      |
| `--no-email`        | Skip SMTP                                  |
| `--auth-sheets`     | One-time Google OAuth; writes `token.json` |
| `--dump-html PATH`  | Save fetched HTML after live scrape        |

## CSV

Columns: `request_id,dorm,room_type,size_sqm,your_rank`

Sorted by `your_rank` ascending (lower = better position). The KKIK page shows **your** position on each list, not total queue size.

## Cron (daily example)

```bash
0 8 * * * cd /path/to/dom && ./kkik-waitlist >> /var/log/kkik-waitlist.log 2>&1
```

Non-zero exit on login, parse, sheet, or mail failure.
