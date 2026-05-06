# Copilot Instructions

Follow `AGENTS.md` at the repository root. It is the source of truth for AI and
agent development guidance in this repo.

Current development focus is Section 8 Local API Server parity and integration
gates. Do not duplicate deep core stdlib work from the separate stdlib
worktrees. Do not pull in the parser proof-of-concept unless the task is the
parser cutover documented in `docs/APEX_PARSER_CUTOVER.md`.

For server/API work, use the shared runtime stack already in the repo:
`storage`, `dml`, `soql`, `vm`, `apextest`, `compat`, and `capability`. Add
focused tests and black-box compatibility fixtures before changing capability
status or checking off parity tasks.

Keep generated docs in sync after capability changes:

```bash
go run ./cmd/oaer compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --output docs/KNOWN_GAPS.md
go run ./cmd/oaer compat stdlib --output docs/STDLIB_COVERAGE.md
```

Do not stage or commit the built `oaer` binary unless explicitly requested.
