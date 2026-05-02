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
- Added editor integration docs with VS Code tasks, debug launch examples, LSP
  wiring, watch mode, and report commands.
- Added a fail-fast `oaer compat mvp --require-ready` gate and CI visibility
  for machine-readable MVP readiness.
- Added compatibility fixture support and smoke coverage for expected
  unsupported-feature diagnostics.
- Added method/constructor parameter type diagnostics and expanded VM exception
  fidelity for multi-catch, bare rethrow, catchable null dereference, and
  malformed IR guards.
- Added a conservative method-body sema baseline for local declaration types,
  constructor references, simple assignments, project method calls, and
  known-receiver overload arity/simple argument type matching.
- Added namespace-qualified type resolution in sema, a small visibility
  diagnostic baseline, and namespace-qualified class-name parsing in the VM.
- Added Apex static and instance initializer block execution for project classes,
  including static reset behavior that reapplies static initializer blocks.
- Added `this(...)` and `super(...)` constructor chaining for supported project
  classes.
- Added VM property getter/setter body execution, source-ordered field
  initialization/reset metadata, baseline runtime visibility and namespace access
  checks, and overload selection by argument types.
- Added binary smoke coverage for parser, exec, test, db, server, LSP
  diagnostics, profile, and compatibility commands.

Upgrade notes:

- No migration is required for this unreleased preview state.
- Persistent database and fixture formats are still preview interfaces.
