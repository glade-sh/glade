# Security & Trust

Glade is a local binary for Salesforce project work. The trust story is a set
of checked facts, not one badge.

## What runs in CI

Report vulnerabilities through [Glade private reporting](https://github.com/glade-sh/glade/security/advisories/new)
or [Tools private reporting](https://github.com/glade-sh/glade-tools/security/advisories/new),
not public issues. If GitHub reporting is unavailable, use
[security@glade.sh](mailto:security@glade.sh).

- `govulncheck` checks reachable vulnerabilities in Go dependencies and the Go
  standard library.
- CodeQL runs the Go security-extended query suite with event-specific query
  filters. Both initialization profiles declare generated files in `paths-ignore`; the
  fast profile also declares `*_test.go` and `testdata`. Pull requests use only the fast profile. Full scans run weekly,
  on release tags, and on manual Security dispatch. Analysis has a 300-second
  deadline, with query evaluation capped at 240 seconds. Timeouts warn (a known
  issue for full scans); analysis errors and unsuppressed alerts fail. `Required CI` does not
  aggregate CodeQL; release tags require a successful full scan.
  The emergency job limit is 15 minutes. Go uses a manual build;
  [GitHub documents](https://docs.github.com/en/code-security/reference/code-scanning/troubleshoot-analysis-errors/alerts-in-generated-code)
  that compiled Go analysis follows the build, so this path setting alone does
  not prove those paths are excluded from the database or that analysis will
  finish within the budget. On large changes the scan often times out; a
  timed-out scan is incomplete coverage, not a clean result.
  Both profiles exclude `go/cookie-httponly-not-set` and
  `go/cookie-secure-not-set`: they never finished in run 39616762308. The only
  cookie sink is the Visualforce Apex Cookie emulation in
  `internal/server/visualforce.go`, which writes the flags the Apex program
  set. A server test pins those flags, and a source scan fails if another
  non-test file sets a cookie. The [release budget and cookie assessment](CODEQL.md#cookie-sink-assessment)
  record measured full-profile times, waiver limits, and the release check command.
- `gosec` uploads SARIF for source-pattern review.
- `npm audit --omit=dev --audit-level=high` checks packaged JavaScript
  production dependencies in the LWC toolchain and VS Code extension.
- GitHub Dependency Review blocks pull requests that add high-severity
  vulnerable dependencies.
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/glade-sh/glade)
  publishes repository-posture results.

## Release proof

Use the [Security & trust guide](https://glade.sh/guide/security-trust#release-proof)
to choose a platform archive, read its SBOM, and find its CI and Salesforce
receipts. The guide lists the assets and explains what each check verifies.

From a reviewed checkout, verify the current stable download with:

```bash
sh scripts/verify-release-download.sh
```

The helper requires `curl`, `jq`, `gh`, `awk`, `mktemp`, `uname`, and either
`shasum` or `sha256sum`. If GitHub CLI asks for authentication to fetch an
attestation, run `gh auth login`. The helper stops at the first failed check and
keeps the downloaded files for inspection.

### Historical example

The v0.2.15 binaries embed commit `82b8495972715a27aec8f864bc4ab39e7fa03974`
with `vcs.modified=false`.

**Inventory scope:** the published v0.2.15 per-archive SBOMs inventory 128–129
packaged components: 32–33 Go modules and 96 LWC/Babel or VSIX npm
dependencies. The macOS archives contain 33 Go modules; the Linux archives
contain 32. The extension includes dependency notices and an inventory bound
to its bundle hash.
The archive also includes notices for the Go distribution, linked Go modules,
and the vendored Apex parser.

## Laptop behavior

Supported local checks do not require a Salesforce org login. Glade reads a
Salesforce DX project from disk, writes optional local state, and runs the
local Apex runtime on the machine where the command starts.

## Network access

Glade uses the network for install, update, plugin registry/archive downloads,
and optional toolchain install paths. Local check, test, parse, exec, SOQL, DML,
and local API work do not call a hosted Glade service.

## Plugins

Plugins are local executables installed or linked by the user. Trust labels in
the registry are informational except where CI blocks community or unlisted
installs without an explicit approval flag. Review community and unlisted plugin
source before installing them.

`glade plugins link --exec <path>` trusts that executable immediately. Use it
only for plugin binaries you built or reviewed.

Plugin subprocesses run as the current OS user. Glade passes a minimal
environment to plugin commands: basic process variables such as `PATH`, `HOME`,
and temp directories, plus `GLADE_*` variables. It does not pass common ambient
Salesforce, GitHub, AWS, or other shell tokens by default.

The plugin registry index is a JSON document fetched over HTTPS. The index is
not separately signed. Archive integrity comes from the SHA256 listed in the
index or supplied on the command line.

## Local storage

Glade can write:

- `glade.yml` project config.
- `.glade/` run artifacts and project state, including private local caches
  under `.glade/test/` and `.glade/semantic/`.
  The test daemon directory must be owned by you and not group- or
  world-writable; otherwise Glade falls back to the per-user cache directory
  (see `docs/TEST_STARTUP_CACHE.md`).
- SQLite files named by `--db`.
- User-level plugin, editor, and LWC toolchain data under the OS user data
  directory.

## Review packet for architects

Give reviewers:

- [Security policy](../SECURITY.md).
- [Install guide](INSTALL.md).
- [Release policy](RELEASE_POLICY.md).
- Latest GitHub release assets, checksums, SBOMs, and attestations.
- The Security workflow badge, CodeQL/gosec code scanning status, and published
  OpenSSF Scorecard results.
