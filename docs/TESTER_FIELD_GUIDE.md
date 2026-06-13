# Tester Field Guide

Use this guide when you hand Glade to Salesforce engineers for a small pilot.
It starts from an existing SFDX project and keeps the first loop local. No org
login, scratch org, source push, or metadata deploy is required for the core
commands.

Glade does not replace Salesforce. It gives a fast local loop before Salesforce
enters the path. Check the support map before treating any platform service,
live auth flow, Visualforce rendering path, or broad API surface as supported.

## 10-Minute Setup

Install and prove the parser:

```bash
curl -fsSL https://glade.sh/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
glade version
glade doctor
```

`glade doctor` must print:

```text
Glade doctor

Ready.
```

Open an SFDX project:

```bash
cd path/to/sfdx-project
test -f sfdx-project.json
glade init --project . --yes
glade config validate --project .
glade config show --project .
```

Run the first local checks:

```bash
glade check --project .
glade test --project . --json
```

If a tester uses VS Code, install the bundled extension after the binary works:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

Open the SFDX project root in VS Code. The extension adds the Glade Activity
Bar, local Apex tests in Test Explorer, local CodeLens actions, DAP debug
launches, and named SQLite-backed data environments.

## Daily Local Loop

| Job | Command |
| --- | --- |
| Check source after a pull | `glade check --project .` |
| Run one test class | `glade test --project . --class AccountServiceTest --json` |
| Run one test method | `glade test --project . --class AccountServiceTest --method testCreatesAccount --json` |
| Run tests affected by a branch | `glade test changed --project . --since origin/main --json --no-progress` |
| Rerun last failures | `glade test failed --project .` |
| Let Glade pick the next loop | `glade test --project . --wizard` |
| Keep repeated CLI runs warm | `glade test serve --project .` |
| Keep one watch loop warm | `glade test --project . --daemon --watch` |
| Execute a quick Apex probe | `glade exec --project . "System.debug('local');"` |
| Open the local playground | `glade playground --project . --open` |

Start with one focused class or method when bringing up a large project. Move
to `glade test changed` after the first known-good run. Use the whole suite
before trusting a release branch.

Clear the startup cache after branch switches, Glade upgrades, or stale-looking
results:

```bash
glade test clear-cache --project .
glade test --project . --no-cache --class AccountServiceTest --json
```

## AI-Assisted Work

Glade gives AI tools small, checked work packets instead of broad guesses. The
useful pattern is: inspect the project, run a local gate, fix the smallest
thing, then rerun the same gate.

Give an AI coding agent this contract from the SFDX project root:

```text
Use Glade for local Salesforce checks.
Do not connect to a Salesforce org for the first pass.
Run:
  glade doctor
  glade config validate --project .
  glade check --project . --format json --output reports/glade-check.json --no-progress
  glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
If a command fails, quote the exact diagnostic, fix only the relevant source,
and rerun the same command before claiming success.
Check the Glade support map before treating unsupported platform services as bugs.
```

For a review or refactor prompt, ask the agent to produce a proof artifact:

```bash
mkdir -p reports
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
glade report refactor-proof --project . --since origin/main --fail-on-api-break --format json > reports/glade-refactor-proof.json
```

The proof report records the git diff, parse and semantic status, graph impact,
affected-test selection, optional trace summary, and public or global API
surface warnings.

## CI Pattern

Use a full git fetch when CI needs `origin/main` for affected-test selection:

```yaml
name: glade
on: [pull_request]
jobs:
  glade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: curl -fsSL https://glade.sh/install.sh | sh
      - run: echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: glade doctor
      - run: mkdir -p reports
      - run: glade check --project . --format sarif --output reports/glade-check.sarif
      - run: glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
      - run: glade test --project . --junit reports/glade-junit.xml
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: glade-results
          path: |
            reports/
```

For richer saved run artifacts:

```bash
glade dev test --project . --all --out .glade/runs
glade report github latest --runs-dir .glade/runs
glade report export latest --runs-dir .glade/runs --format html --output reports/glade-report.html
```

## Useful Opportunities

| Opportunity | Command |
| --- | --- |
| Explain a saved Salesforce debug log against local source | `glade debug explain --log apex.log --project .` |
| Generate a starter local repro test from a log | `glade debug repro --log apex.log --project . > ReproTest.cls` |
| Run a Salesforce-shaped local API | `glade server --project . --addr 127.0.0.1:8080` |
| Seed or inspect a local SQLite org state | `glade db seed --db .glade/org.sqlite --project . fixtures/dev.json` |
| Map a large Apex project | `glade inspect graph --project . --json` |
| Create an assessment report | `glade report assess --project . --format html --out reports/glade-assessment.html` |
| Review conservative dead-code candidates | `glade report cruft --project . --format html --out reports/glade-cruft.html` |
| Compare managed-package artifacts | `glade package diff old.glade.json new.glade.json --json` |

Advisory performance scans and compatibility ledgers are plugin commands. The
default public plugin registry is preview. Use them only when you have a live
registry, a direct archive, or a locally linked first-party plugin:

```bash
glade plugins available
glade plugins install @glade/performance
glade performance scan --project . --json > reports/glade-performance.json
```

## What To Send Back

Useful pilot feedback includes:

- `glade version`
- full `glade doctor` output
- the exact command that failed
- JSON output from `glade check` or `glade test`, when available
- the smallest Apex class, trigger, metadata file, fixture, or debug log that
  shows the problem
- whether Salesforce accepts or rejects the same code, when known
- which workflow the tester was trying: local loop, VS Code, AI, CI, report, or
  plugin

## Next Docs

- Install: [INSTALL.md](INSTALL.md)
- Local testing: [LOCAL_TESTING.md](LOCAL_TESTING.md)
- VS Code: [EDITOR.md](EDITOR.md)
- CI artifacts: [CI_ARTIFACTS.md](CI_ARTIFACTS.md)
- Enterprise reports: [ENTERPRISE_WORKFLOWS.md](ENTERPRISE_WORKFLOWS.md)
- Plugins: [PLUGINS.md](PLUGINS.md)
- Support map: <https://glade.sh/guide/support-map>
