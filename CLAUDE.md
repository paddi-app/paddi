# CLAUDE.md

**Tech Stack**: `urfave/cli/v3` (CLI) · `knadh/koanf/v2` (config: flag→env→file→default) · `zalando/go-keyring` (token, `PADDI_TOKEN` override) · stdlib `net/http` · `pkg/browser` (device-flow URL) · `go-runewidth`/`encoding/json` (output).

## Working Principles

These four rules override everything else below. Align before acting.

### Rule 1 — Think Before Coding

- No silent assumptions: write out the premises you're assuming.
- Surface trade-offs; don't pick one path and bury head.
- Ask when unsure. Don't guess.
- Push back when you see a simpler approach, even if the user specified a complex one.

### Rule 2 — Simplicity First

- Minimum code to solve the current problem. No speculative features.
- Don't abstract one-shot logic.
- If a senior engineer would say "this is too complex", simplify.

### Rule 3 — Surgical Changes

- Touch only what's necessary. Don't "improve" adjacent code, comments, or formatting.
- Don't refactor what isn't broken.
- Follow existing style. Don't impose personal preference.

### Rule 4 — Goal-Driven Execution

- Define success criteria before acting: what should be verifiable on completion?
- The user's "step description" is not the goal — the goal is the outcome; steps are means.
- Iterate until verification passes. Don't ship after one pass.

## Architecture

- **`main.go`** — thin entrypoint: build root command, `Run(ctx, os.Args)`, map exit code.
- **`internal/commands`** — CLI layer (one file per command group): parse flags/args, build the API client, call it, render via `output`. No HTTP or business logic.
- **`internal/api`** — the only place that talks HTTP to the backend: typed methods returning typed structs + typed errors, built from `baseURL` + `token` + `*http.Client`.
- **`internal/config`** — koanf load/save, XDG paths, precedence; local-machine state only.
- **`internal/credentials`** — keyring get/set/delete + `PADDI_TOKEN` override; local-machine state only.
- **`internal/output`** — pure rendering (table / json / markdown); no I/O beyond the writer it is handed.

## CLI Reference

### Commands

- `auth` — authenticate with Paddi
  - `login` — log in via browser device flow
  - `logout` — revoke session and clear local credentials
  - `status` — show logged-in user and current context
- `workspace` — manage workspace context
  - `list` — list my workspaces
  - `use [id]` — set current workspace (interactive picker if `id` omitted)
- `project` — manage project context
  - `list` — list projects in the current workspace
  - `use [id]` — set current project (interactive picker if `id` omitted)
- `spec` — work with specs
  - `list` — list specs in the current project
  - `info <id>` — print spec metadata (no markdown content)
  - `view <id>` — print spec markdown content
  - `download <id> [-o path]` — write spec markdown to a local file
  - `lock <id>` — lock a spec to prevent further edits
- `request` — work with feedback requests
  - `list` — list requests, sorted by RIGE score
  - `view <id>` — show analysis, score, and solution paths
  - `regenerate <id> [-e expectation]` — regenerate solution paths
  - `draft <id> -f answers.json` — answer solution paths and trigger spec generation
- `capture` — feed raw feedback into Paddi
  - `create -m <msg> | -f <file> | -` — create a capture from a message, file, or stdin
- `source` — manage data sources
  - `list` — list data sources in the current project
  - `index <id>` — trigger re-indexing of a source

### Global Flags

`--json`, `--quiet`/`-q`, `--project`, `--api-base` (root command).

### Config & Auth

- Config file: `$XDG_CONFIG_HOME/paddi/config.toml` (or `PADDI_CONFIG` override); precedence is flag > env > file > default.
- Env overrides: `PADDI_API_BASE`, `PADDI_PROJECT`, `PADDI_TOKEN` (bypasses the keyring and disables auto-refresh).
- Tokens live in the OS keyring (service `paddi-cli`); access token auto-refreshes once on a 401 via `Client.Refresh`.

### Exit Codes

`0` ok · `1` user error · `2` auth (401/403 or not logged in) · `3` server (5xx or network error) — mapped in `main.go`.

## Docs Sync

CLI changes to commands, flags, env vars, config keys, output format, or exit codes must be mirrored in `paddi-website`'s `src/content/docs/cli.mdx` before merging (can't be the same commit — separate repos). Its sections map 1:1 to `internal/commands/*.go` command groups, plus global flags, config/env vars, auth flow, and exit codes — use that to find the matching section fast. Internal refactors and bug fixes with no user-visible effect don't need it.

The website repo is usually a sibling checkout (e.g. `../paddi-website`); if it isn't found locally, say so instead of guessing a path or skipping the update silently.
