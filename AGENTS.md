# Housing waitlist scraper (`dom` / module `housing-waitlist`)

Go CLI that crawls student-housing waitlist portals, normalizes each into a
shared row model, writes a per-source CSV sorted by waitlist position (best
first), and optionally updates a Google Sheet — one combined email covers
every source. A separate `--score-only` mode crawls each source's full public
catalog and ranks every available room, not just applied-to ones, by rent,
commute, and size. Sources are pluggable: the per-source **fetch → parse →
CSV → sheet** pipeline, commute enrichment, and scoring all run uniformly
across sources.

## Commands

- `make build` — build `./waitlist`
- `make test` — `go test ./...`
- `make run` / `make run-no-email` — run all enabled sources (needs `internal/config/config.yaml`)
- `make score` — crawl every enabled source's full catalog and write `data/candidates.csv` only (`--score-only`)
- `make dump SRC=<name>` — scrape one source to `debug.html`, skip email + sheet
- `make list-sources` — print registered sources
- `make auth-sheets` — one-time Google OAuth (writes `token.json`)

## Layout

| Path                   | Responsibility                                                                                                 |
| ---------------------- | -------------------------------------------------------------------------------------------------------------- |
| `cmd/waitlist`         | CLI: flags → runner; blank-imports `source/all`                                                                |
| `internal/model`       | shared row model (`WaitlistRow`: rank, address, commute, rent)                                                 |
| `internal/source`      | `Source` interface + optional capabilities (`AddressResolver`, `RentResolver`, `CatalogSource`) + registry     |
| `internal/source/kkik` | KKIK source (waitlist scraper, catalog crawl, rent + address lookup)                                           |
| `internal/source/sdk`  | s.dk source (waitlist scraper, catalog crawl)                                                                  |
| `internal/source/all`  | blank-imports every source for registration                                                                    |
| `internal/commute`     | pluggable commute-time estimation (Google Routes) + on-disk cache                                              |
| `internal/scoring`     | desirability/opportunity ranking used by `--score-only`                                                        |
| `internal/runner`      | per-source fetch → parse → CSV → sheet loop + combined email (`Run`); full-catalog scoring loop (`RunScoring`) |
| `internal/export`      | waitlist CSV, candidates CSV, rank diff, history, prev-rank loading                                            |
| `internal/sheets`      | Google Sheets append-by-date                                                                                   |
| `internal/mailer`      | SMTP report (to + cc)                                                                                          |
| `internal/config`      | shared (`smtp`/`google`/`commute`/`scoring`) + per-source config structs                                       |
| `data/<source>/`       | per-source CSV history                                                                                         |

## Conventions

- **Config and data are gitignored.** Only `internal/config/config.example.yaml` is committed; copy it to `config.yaml`. `data/` is not tracked.
- **Ranks are abstracted.** Each source maps its native rank (numeric for KKIK, letter grades for s.dk) to a sortable `RankOrder` (lower = better); `RankDisplay` is what the source shows. Catalog rows with no waitlist application use the sentinel `RankOrder=99`.
- **The pipeline is source-agnostic.** Adding a portal touches only `internal/source/<name>` plus a config block — never `export`, `sheets`, `mailer`, or `runner`.
- **Optional capabilities, not required.** A source only needs the base `Source` interface; implementing `AddressResolver`, `RentResolver`, and/or `CatalogSource` (`internal/source/source.go`) unlocks commute lookup, rent enrichment, and full-catalog scoring — no pipeline changes.
- **Scoring is a separate mode from ranking.** `--score-only` never writes per-source CSV history, updates Sheets, or sends email — it merges every source's rows into one ranked `data/candidates.csv`, overwriting it each run.

## Adding a source

1. Create `internal/source/<name>/` with a type implementing `source.Source` (`Descriptor`, `Fetch`, `Parse`, `RankOrder`) and a `func init()` that calls `source.Register("<name>", …)`.
2. Blank-import it from `internal/source/all/import.go`.
3. Add a `<Name>Config` struct, a `Sources` field, and a `case` in `config.Source()` / `config.SourceNames()`.
4. Optionally implement `AddressResolver`, `RentResolver`, and/or `CatalogSource` from `internal/source/source.go` to opt into commute lookup, rent enrichment, or full-catalog scoring.

See [`README.md`](README.md) for setup, full configuration, and Google Sheets OAuth. `CLAUDE.md` mirrors this file for Claude Code.

<!-- gitnexus:start -->

# GitNexus — Code Intelligence

This project is indexed by GitNexus as **dom** (860 symbols, 2488 relationships, 72 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({search_query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource                             | Use for                                  |
| ------------------------------------ | ---------------------------------------- |
| `gitnexus://repo/dom/context`        | Codebase overview, check index freshness |
| `gitnexus://repo/dom/clusters`       | All functional areas                     |
| `gitnexus://repo/dom/processes`      | All execution flows                      |
| `gitnexus://repo/dom/process/{name}` | Step-by-step execution trace             |

## CLI

| Task                                         | Read this skill file                                        |
| -------------------------------------------- | ----------------------------------------------------------- |
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md`       |
| Blast radius / "What breaks if I change X?"  | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?"             | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md`       |
| Rename / extract / split / refactor          | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md`     |
| Tools, resources, schema reference           | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md`           |
| Index, status, clean, wiki CLI commands      | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md`             |

<!-- gitnexus:end -->
