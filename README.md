# Housing waitlist scraper

Go CLI that crawls student-housing waitlist portals, normalizes each into a
common row model, writes a per-source CSV sorted by your waitlist position (best
first), optionally updates a Google Sheet, and emails one combined report
covering every source. A separate scoring mode crawls each source's full
public catalog — not just rooms you've applied to — and ranks every available
room by commute time, rent, and size.

Sources are pluggable. Built in today:
[Kollegiernes Kontor i København](https://www.kollegierneskontor.dk) (`kkik`)
and [s.dk](https://s.dk) (`sdk`). Adding another portal is a self-contained
package plus a config block — the fetch/CSV/sheet/email pipeline is
source-agnostic.

## Requirements

- Go 1.26+
- Chromium (used by chromedp; installed automatically on first run or via system Chrome)
- (optional) a Google Maps Platform API key with the **Routes API** enabled, for commute-time estimation

## Setup

```bash
cp internal/config/config.example.yaml internal/config/config.yaml
# Edit internal/config/config.yaml with your source credentials and SMTP settings
make build
```

Shortcuts: `make help` lists targets (`make run`, `make run-no-email`,
`make score`, `make dump`, `make list-sources`, `make test`).

## Configuration

Copy [`internal/config/config.example.yaml`](internal/config/config.example.yaml)
to `internal/config/config.yaml` and fill in your values — it's the source of
truth for every field, default, and comment. `smtp`, `google`, `commute`, and
`scoring` are shared by every source; each source has its own block under
`sources`.

| Key                            | Configures                                                                                              |
| ------------------------------ | ------------------------------------------------------------------------------------------------------- |
| `smtp`                         | Transport / sender / recipient — one combined email to `to` (+ optional `cc`)                           |
| `google`                       | Shared OAuth identity for Sheets                                                                        |
| `commute`                      | Optional commute-time estimation: provider, API key, destinations, cache path                           |
| `scoring`                      | Optional `--score-only` ranking: budget gate, commute/rent bands, weights, output path                  |
| `sources.kkik` / `sources.sdk` | Per-source: `enabled`, `steps.email`/`sheet`, `login`, `headless`, `timeout_sec`, `sheet.*`, `data_dir` |

Loaded with [cleanenv](https://github.com/ilyakaznacheev/cleanenv). Set `CONFIG_PATH` to use another file.

### Sources and steps

| Setting       | Default | What it does                                                          |
| ------------- | ------- | --------------------------------------------------------------------- |
| `enabled`     | `false` | Include this source when no `--source` flag is given                  |
| (crawl)       | always  | Live scrape + write timestamped CSV under `data_dir`                  |
| `steps.email` | `true`  | Include this source's section in the combined email (needs `smtp.to`) |
| `steps.sheet` | `true`  | Upload rows to Google Sheets (needs `sheet.spreadsheet_id`)           |

Each source is scraped independently — **fetch → parse → CSV → sheet** — and its
CSV and Google Sheet stay per source. Email is the exception: every
email-enabled source contributes a section, and **one combined email** (with each
source's CSV attached) is sent to `smtp.to` (and `smtp.cc`, if set) after all
sources succeed. If any source fails, the run aborts and no email is sent.

## Usage

```bash
make run                 # run all enabled sources
./waitlist               # same
./waitlist --source kkik # run a specific source (even if enabled: false)
./waitlist --list-sources
make run-no-email        # all enabled sources, skip email
make score               # crawl every enabled source's full catalog, write data/candidates.csv only
make dump SRC=kkik       # scrape one source → debug.html, skip email + sheet
```

| Flag               | Description                                                                                           |
| ------------------ | ----------------------------------------------------------------------------------------------------- |
| `--source a,b`     | Comma-separated sources to run (default: all enabled)                                                 |
| `--list-sources`   | Print registered sources and exit                                                                     |
| `--no-email`       | Skip the email step for this run                                                                      |
| `--no-sheet`       | Skip the Google Sheets step for this run                                                              |
| `--score-only`     | Crawl full catalogs and write the scored candidates CSV only (skips per-source CSV, sheet, and email) |
| `--dump-html PATH` | Save fetched HTML (requires a single `--source`)                                                      |
| `--auth-sheets`    | One-time Google OAuth; writes `token.json` (shared)                                                   |

## CSV

Per source, written to `data/<source>/<timestamp>_waitlist.csv`:

```
request_id,dorm,url,room_type,size_sqm,<commute columns>,your_rank,diff
```

Sorted by rank order (lower = better). `your_rank` is the rank as the source
displays it; `diff` compares against the latest previous CSV (`+N` = improved).
A source's rank may be numeric (KKIK) or otherwise (e.g. s.dk's letter grades)
— each source maps its rank to a sortable order so sorting and diffing work
uniformly. Commute columns (three per destination — see
[Commute time estimation](#commute-time-estimation)) are only present when
`commute.enabled: true`.

## Dorm scoring

`--score-only` (or `make score`) crawls each source's full public catalog —
via the optional `CatalogSource` capability, see
[Adding a source](#adding-a-source) — and scores every listed room, including
buildings you haven't applied to. Rows from all enabled sources merge into one
ranked list, **overwriting** `scoring.output_path` (default
`./data/candidates.csv`) each run. It never writes per-source CSV history,
updates Sheets, or sends email.

Every room gets two 0–100 scores:

- **Desirability** — weighted blend of commute, size, and rent, each
  normalized to `[0,1]` (fixed bands for commute/rent via
  `commute_best_min`/`worst_min`, `max_rent`/`rent_floor`; relative min-max
  for size). Weights: `scoring.weights` (default 0.4/0.3/0.3).
- **Opportunity** — desirability blended with waitlist rank
  (`scoring.rank_weight`), rank normalized per source since a KKIK numeric
  position and an s.dk letter grade aren't comparable. Catalog-only rows
  (no rank) score the same as desirability.

`scoring.max_rent` (DKK/mo) is a hard gate — rooms priced above it are dropped
before scoring; `0` disables it. `scoring.sort_by` picks which score orders
the CSV (`desirability` default, or `opportunity`).

Output columns (`data/candidates.csv`):

```
source,dorm,room_type,size_sqm,rent,<commute columns>,your_rank,desirability,opportunity,url
```

## Commute time estimation

When `commute.enabled: true`, every dorm is checked against each destination
in `commute.destinations` and three numbers are computed: transit arriving by
`arrive_by`, transit leaving at `depart_at`, and walking time. Results land in
both the waitlist CSV/Sheet (inserted right after `size_sqm`) and the scoring
CSV, as three columns per destination: `<name>_transit_morning_min`,
`<name>_transit_evening_min`, `<name>_walk_min`.

Only `provider: google` (Maps Routes API) is implemented; `internal/commute`
is pluggable — add an adapter and a case in `NewRouter` for another backend.

Results are cached to `commute.cache_path` (default
`./data/commute_cache.json`, always on once commute is enabled), keyed by
`(origin, destination, time-of-day)` — delete the file to force a recompute
after changing addresses or times. The same cache also remembers addresses a
source's `AddressResolver` discovers (e.g. KKIK, whose rows carry no street
address), so each is looked up at most once.

Origin address precedence per row: the row's own parsed address (e.g. s.dk) →
a `commute.dorm_addresses` override → the source's `AddressResolver` lookup.

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

The tab keeps fixed columns (`request_id`, `dorm`, `url`, `room_type`,
`size_sqm`, commute columns when `commute.enabled`, `latest_diff`) plus one
**ddmmyy** column per calendar day (e.g. `260526` for 26 May 2026). Commute
columns sit right after `size_sqm` — three per destination (transit-morning,
transit-evening, walk). Each run appends a new day on the right, or updates
that day's column if you run again the same day. Ranks are backfilled from
`data/<source>/*_waitlist.csv`. **`latest_diff`** shows change vs the previous
day's rank (`+N` = improved). Older sheets predating `url`/commute columns
still parse fine — only `request_id`, `dorm`, `room_type`, `size_sqm` are
required.

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

Optionally implement one of the capability interfaces in
[`internal/source/source.go`](internal/source/source.go) to opt into more
behavior without touching the pipeline:

| Interface         | Unlocks                                                                                  |
| ----------------- | ---------------------------------------------------------------------------------------- |
| `AddressResolver` | Commute lookup for sources whose rows carry no parsed street address (KKIK)              |
| `RentResolver`    | Rent enrichment during scoring for rows with no parsed rent                              |
| `CatalogSource`   | Full public-catalog crawl during scoring, not just the applicant's waitlist (KKIK, s.dk) |

## Architecture

| Path                      | Responsibility                                                                                |
| ------------------------- | --------------------------------------------------------------------------------------------- |
| `cmd/waitlist`            | CLI: flags → runner; blank-imports `source/all`                                               |
| `internal/model`          | shared row model (`WaitlistRow`: rank, address, commute, rent)                                |
| `internal/source`         | `Source` interface + optional capabilities + registry                                         |
| `internal/source/kkik`    | KKIK source (waitlist scraper, catalog crawl, rent + address lookup)                          |
| `internal/source/sdk`     | s.dk source (waitlist scraper, catalog crawl)                                                 |
| `internal/source/all`     | blank-imports every source for registration                                                   |
| `internal/commute`        | pluggable commute-time estimation (Google Routes) + on-disk cache                             |
| `internal/scoring`        | desirability/opportunity ranking used by `--score-only`                                       |
| `internal/runner`         | per-source fetch→parse→CSV→sheet→email loop (`Run`); full-catalog scoring loop (`RunScoring`) |
| `internal/export`         | waitlist CSV, candidates CSV, diff, history, prev-rank loading                                |
| `internal/sheets`         | Google Sheets append-by-date                                                                  |
| `internal/mailer`         | SMTP report (to + cc)                                                                         |
| `internal/config`         | shared (`smtp`/`google`/`commute`/`scoring`) + per-source structs                             |
| `data/<source>/`          | per-source CSV history                                                                        |
| `data/candidates.csv`     | merged scoring output (`--score-only`)                                                        |
| `data/commute_cache.json` | commute + address-lookup cache                                                                |

## Cron (daily example)

```bash
0 8 * * * cd /path/to/dom && ./waitlist >> /var/log/housing-waitlist.log 2>&1
```

Non-zero exit on login, parse, sheet, or mail failure. `make score` is a
separate, side-effect-free crawl (no CSV history, sheet, or email) — cron it
on its own schedule if you want a standalone `candidates.csv` refresh.
