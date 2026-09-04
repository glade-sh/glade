# Contributing to Glade

Start with a [workflow question or bug report](https://github.com/glade-sh/glade/issues/new/choose).
For security reports use [private reporting](https://github.com/glade-sh/glade/security/advisories/new),
not a public issue. Be specific and respectful; discuss behavior and evidence,
not the person reporting it. See [SECURITY.md](SECURITY.md) for contact availability.

## Share a useful reproduction

Include `glade version`, OS/architecture, source API version, the exact command,
expected/actual result, selected/executed test counts, and a minimal public
project or fixture. Remove credentials, private paths, private package names,
proprietary source, customer records, and unredacted support bundles.
Do not assume a closed issue is private.

Use owned fixtures, public Salesforce behavior/docs, public grammars, or
authorized black-box comparisons. Never copy Salesforce implementation code
or private customer code into a fix. Label compile-only, local runtime,
deterministic harness, and Salesforce comparison evidence separately.

## Put the change in the right repository

- **Glade:** parser/checker, runtime, CLI, test runner, schema/storage, servers,
  editor/browser interfaces, user docs, and release distribution.
- **[Glade Tools](https://github.com/glade-sh/glade-tools):** maintenance
  fixtures/scanners, capability catalogs, generated ledgers, compatibility
  reports, and first-party plugin sources.

Tools may depend on Glade; Glade must not depend on Tools internals. Regenerate
maintenance-owned reports through Tools rather than editing their output alone.
Read [AGENTS.md](AGENTS.md) and the surrounding code before changing behavior.

## Keep the fix focused

Preserve unrelated work. Add a regression that fails on the original behavior,
make the smallest shared-root-cause fix, then rerun the same test. Do not change
valid Salesforce source merely to accommodate a Glade limitation.

Run the affected package first, then the boundary checks:

```bash
go test ./internal/playground -count=1
go test ./internal/gladecli -count=1
go test ./internal/repoguard -count=1
```

Use the package you actually changed in place of `internal/playground`. For
site changes, run `npm --prefix site test`, `npm --prefix site run build`, and
the affected Playwright checks. Use the existing parser-enabled source build
and [release policy](docs/RELEASE_POLICY.md) for integrated validation; a raw
build or a small local test gate is not release/Salesforce parity proof.

Describe the change, red/green test commands, compatibility impact, and any
unverified behavior in the PR. Keep generated binaries, local evidence dumps,
private logs, and screenshots with private identifiers out of commits.
