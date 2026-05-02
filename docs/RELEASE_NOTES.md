# Release Notes

## Unreleased

Compatibility status:

- MVP readiness: not ready.
- Required MVP capabilities are still partial or unsupported.
- See [`COMPATIBILITY_DASHBOARD.md`](COMPATIBILITY_DASHBOARD.md) and
  [`KNOWN_GAPS.md`](KNOWN_GAPS.md) for generated status.

Release engineering:

- Added tag-driven release artifact builds for macOS, Linux, and Windows.
- Added `SHA256SUMS.txt` checksum generation for release artifacts.
- Added manual, CI, and future Homebrew installation guidance.
- Added binary smoke coverage for parser, exec, test, db, server, LSP
  diagnostics, profile, and compatibility commands.

Upgrade notes:

- No migration is required for this unreleased preview state.
- Persistent database and fixture formats are still preview interfaces.
