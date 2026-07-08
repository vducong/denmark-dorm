# Housing waitlist scraper (`dom` / module `housing-waitlist`)

Go CLI that crawls student-housing waitlist portals, normalizes each into a
shared row model, writes a per-source CSV sorted by waitlist position (best
first), and optionally updates a Google Sheet. One combined email covers every
source. Sources are pluggable; the per-source **fetch → parse → CSV → sheet**
pipeline is source-agnostic, and email is aggregated across sources into a single
digest.

## Commands

- `make build` — build `./waitlist`
- `make test` — `go test ./...`
- `make run` / `make run-no-email` — run all enabled sources (needs `internal/config/config.yaml`)
- `make dump SRC=<name>` — scrape one source to `debug.html`, skip email + sheet
- `make list-sources` — print registered sources
- `make auth-sheets` — one-time Google OAuth (writes `token.json`)

## Layout

| Path | Responsibility |
| ---- | -------------- |
| `cmd/waitlist` | CLI: flags → runner; blank-imports `source/all` |
| `internal/model` | shared row model (`RankDisplay` + `RankOrder`) |
| `internal/source` | `Source` interface + registry |
| `internal/source/kkik` | KKIK source (scraper + parser + selectors) |
| `internal/source/sdk` | s.dk source |
| `internal/source/all` | blank-imports every source for registration |
| `internal/runner` | per-source fetch → parse → CSV → sheet loop, then one combined email |
| `internal/export` | CSV, rank diff, history, prev-rank loading |
| `internal/sheets` | Google Sheets append-by-date |
| `internal/mailer` | SMTP report |
| `internal/config` | shared (`smtp`/`google`) + per-source config structs |
| `data/<source>/` | per-source CSV history |

## Conventions

- **Config and data are gitignored.** Only `internal/config/config.example.yaml` is committed; copy it to `config.yaml`. `data/` is not tracked.
- **Ranks are abstracted.** Each source maps its native rank (numeric for KKIK, possibly non-numeric elsewhere) to a sortable `RankOrder` (lower = better) so sorting and diffing work uniformly; `RankDisplay` is what the source shows.
- **The pipeline is source-agnostic.** Adding a portal touches only `internal/source/<name>` plus a config block — never `export`, `sheets`, `mailer`, or `runner`.

## Adding a source

1. Create `internal/source/<name>/` with a type implementing `source.Source` (`Descriptor`, `Fetch`, `Parse`, `RankOrder`) and a `func init()` that calls `source.Register("<name>", …)`.
2. Blank-import it from `internal/source/all/import.go`.
3. Add a `<Name>Config` struct, a `Sources` field, and a `case` in `config.Source()` / `config.SourceNames()`.

See [`README.md`](README.md) for setup, full configuration, and Google Sheets OAuth. `CLAUDE.md` carries the same guidance for Claude Code, plus its GitNexus tooling notes.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **dom** (685 symbols, 1815 relationships, 56 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/dom/context` | Codebase overview, check index freshness |
| `gitnexus://repo/dom/clusters` | All functional areas |
| `gitnexus://repo/dom/processes` | All execution flows |
| `gitnexus://repo/dom/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
