# Dogfood Checklist

Use this when handing a release candidate or local build to a real Salesforce
project. Run commands from the SFDX project root unless noted.

For the broader pilot handoff, including VS Code, AI, CI, and report workflows,
start with [TESTER_FIELD_GUIDE.md](TESTER_FIELD_GUIDE.md).

## 1. Build The Candidate

For pre-tag dogfood, build from the current checkout:

```bash
cd path/to/glade
go build -o glade ./cmd/glade
mkdir -p ~/.local/bin
install -m 0755 glade ~/.local/bin/glade
./glade version
glade version
```

After a tag is published, the installer path can prove the public artifact:

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
```

## 2. Doctor

```bash
glade doctor
```

The parser line must read `parser: ok (tree-sitter)`. If it says
`parser: UNAVAILABLE`, rebuild with CGO enabled and a C compiler present.

## 3. First Project Check

```bash
cd path/to/sfdx-project
glade doctor
glade check --project .
glade test --project .
```

No Salesforce org login is needed. The project should have `sfdx-project.json`
at its root.

## 4. Focused Class Or Method

Day-to-day test runs use exact class and method selectors:

```bash
glade test --project . --class AccountServiceTest --json
glade test --project . --class AccountServiceTest --method testCreatesAccount --json
```

If the public plugin registry is not live for this machine, install a direct
archive or link the local `glade-tools` compat plugin before running plugin
commands:

```bash
glade plugins install @glade/compat
glade compat local-tests --project . --class AccountServiceTest --json
glade compat local-tests --project . --class AccountServiceTest --method testCreatesAccount --json
```

## 5. Watch Once

Use one watch cycle for editor hooks or a quick file-watch smoke check:

```bash
glade test --project . --watch-once
```

On large projects, also smoke the warm path:

```bash
glade test serve --project .
glade test --project . --class <OneTestClass>
```

The second command should auto-connect and skip a full cold startup when
`.glade/test/startup.gob` or the serve socket is warm. See
[TEST_STARTUP_CACHE.md](TEST_STARTUP_CACHE.md) if warm behavior looks stale.

## 6. Capture JSON

```bash
mkdir -p reports
glade check --project . --json > reports/glade-check.json
glade test --project . --json > reports/glade-test.json
# Requires a live plugin registry, custom registry, direct archive, or linked plugin.
glade plugins install @glade/compat
glade compat local-tests --project . --parallel auto --json > reports/glade-local-tests.json
```

Use `glade test changed --project . --since origin/main --json` when the report
should cover only affected tests.

## 6b. Performance Scanner

```bash
# Requires a live plugin registry, custom registry, direct archive, or linked plugin.
glade plugins install @glade/performance
glade performance scan --project . --json > reports/glade-performance.json
glade performance scan --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

The short aliases `compat` and `performance` resolve to `@glade/compat` and
`@glade/performance`.

Use the source-only report as a map of entry points and hard static patterns.
Use the trace-backed report to rank actual bottlenecks by elapsed spans and row
counts.

## 7. File A Useful Issue

Include:

- `glade version`
- full `glade doctor` output
- the exact command that failed
- the JSON report, if one was captured
- the smallest Apex class, trigger, metadata file, or fixture that shows the
  problem
- whether Salesforce itself accepts or rejects the same code, when known
