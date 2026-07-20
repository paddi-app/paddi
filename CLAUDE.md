# CLAUDE.md

**Tech Stack**: `urfave/cli/v3` (CLI) · `knadh/koanf/v2` (config: flag→env→file→default) · `zalando/go-keyring` (token, `PADDI_TOKEN` override) · stdlib `net/http` (+ hand-rolled SSE) · `pkg/browser` (device-flow URL) · `text/tabwriter`/`encoding/json` (output).

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

## Docs Sync

CLI changes to commands, flags, env vars, config keys, output format, or exit codes must be mirrored in `paddi-website`'s `src/content/docs/cli.mdx` before merging (can't be the same commit — separate repos). Its sections map 1:1 to `internal/commands/*.go` command groups, plus global flags, config/env vars, auth flow, and exit codes — use that to find the matching section fast. Internal refactors and bug fixes with no user-visible effect don't need it.

The website repo is usually a sibling checkout (e.g. `../paddi-website`); if it isn't found locally, say so instead of guessing a path or skipping the update silently.
