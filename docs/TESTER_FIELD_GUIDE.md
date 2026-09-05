# Tester Field Guide

Use this guide when you hand Glade to Salesforce engineers for a small pilot.
It starts from an existing Salesforce DX project and keeps the first loop local. No org
login, scratch org, source push, or metadata deploy is required for the core
commands.

Glade does not replace Salesforce. It gives a fast local loop before Salesforce
enters the path. Check the support map before treating any platform service,
live auth flow, exact hosted Visualforce behavior, or broad API surface as
supported.

## 10-Minute Setup

Install and verify the binary:

```bash
curl -fsSL https://glade.sh/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
glade version
```

Open a Salesforce DX project:

```bash
cd path/to/sfdx-project
test -f sfdx-project.json
test -f glade.yml || glade init --project . --yes
glade config validate --project .
glade config show --project .
glade doctor --project .
```

`glade doctor --project .` must print status rows and end with `Ready.`.

Run the first local checks:

```bash
glade check --project .
glade test --project . --class RefinementServiceTest --json
```

Substitute an existing test class from your project. Confirm the executed
count is nonzero; a process that selects zero tests is not first-run proof.

If a tester uses VS Code, install the bundled extension after the binary works:

```bash
glade editor doctor vscode
glade editor install vscode --force
code --list-extensions --show-versions
```

Open the Salesforce DX project root in VS Code. The extension adds Glade Home, the Glade
Activity Bar, local Apex tests in Test Explorer, local CodeLens actions, DAP
debug launches, named SQLite-backed data environments, Apex & SOQL scratch
buffers, and plugin actions.

## Daily Local Loop

| Job | Command |
| --- | --- |
| Check source after a pull | `glade check --project .` |
| Run one test class | `glade test --project . --class RefinementServiceTest --json` |
| Run one test method | `glade test --project . --class RefinementServiceTest --method testRefinesFileRow --json` |
| Run tests affected by a branch | `glade test changed --project . --since <base-ref> --json --no-progress` |
| Rerun last failures | `glade test failed --project .` |
| Let Glade pick the next loop | `glade test --project . --wizard` |
| Keep repeated CLI runs warm | `glade test serve --project .` |
| Keep one watch loop warm | `glade test --project . --daemon --watch` |
| Execute a quick Apex probe | `glade exec --project . "System.debug('local');"` |
| Serve Visualforce preview pages locally | `glade dev vf --project . --addr 127.0.0.1:8080` |
| Serve the LWC preview shell locally | `glade dev lwc --project . --open` |
| Open the local playground | `glade playground --project . --open` |

Replace `<base-ref>` with the intended existing comparison ref and verify it
resolves in the checkout.

Start with one focused class or method when bringing up a large project. Move
to `glade test changed` after the first known-good run. Use the whole suite
before trusting a release branch.

Clear the project-local startup and semantic caches after branch switches,
Glade upgrades, or stale-looking results. `--no-cache` bypasses both for one
run:

```bash
glade test clear-cache --project .
glade test --project . --no-cache --class RefinementServiceTest --json
```

## AI-Assisted Work

Glade gives AI tools small, checked work packets instead of broad guesses. The
useful pattern is: inspect the project, run a local gate, fix the smallest
thing, then rerun the same gate.

Give an AI coding agent this contract from the Salesforce DX project root:

```text
Use Glade for local Salesforce checks.
Do not connect to a Salesforce org for the first pass.
For a behavior change, write a focused test and verify its expected failure.
For a behavior-preserving refactor, establish a passing baseline first.
Choose the intended existing comparison ref, verify it resolves, and replace
<base-ref> below.
Run:
  mkdir -p reports
  glade doctor --project .
  glade config validate --project .
  glade check --project . --format json --output reports/glade-check.json --no-progress
  glade test changed --project . --since <base-ref> --json --no-progress > reports/glade-test-changed.json
If a command fails, quote the exact diagnostic and classify the mismatch.
Do not rewrite valid Salesforce behavior to satisfy a Glade limitation.
Fix only the relevant source and rerun the same command before claiming success.
Use authorized Salesforce validation when the path requires it.
Read the saved JSON and report diagnostics plus total, passed, failed, errors,
skipped, and unsupported test counts. An empty affected selection can exit 0;
run an explicit relevant test or suite before claiming test execution evidence.
Unsupported test outcomes exit 1 and remain unvalidated behavior.
Check the Glade support map before treating unsupported platform services as bugs.
```

For a review or refactor prompt, ask the agent to produce a proof artifact:

```bash
mkdir -p reports
glade report refactor-proof --project . --since <base-ref> --format html --out reports/glade-refactor-proof.html
glade report refactor-proof --project . --since <base-ref> --fail-on-api-break --format json > reports/glade-refactor-proof.json
```

The proof report records the git diff, parse and semantic status, graph impact,
affected-test selection, optional trace summary, and public or global API
surface warnings. It does not execute tests. Run the focused and affected
tests separately and retain their results with the report.

## CI Pattern

Start with the canonical [advisory pilot](https://glade.sh/guide/ci-artifacts#advisory-pilot).
It pins the release, preserves assessment failures and artifacts, and uses a
full git fetch for affected-test selection. Setup failures still fail the job.
Do not make it a required merge gate until the team chooses the separately
documented enforcing contract and validates representative results.

For richer saved run artifacts:

```bash
glade dev test --project . --all --out .glade/runs
glade report github latest --runs-dir .glade/runs
glade report export latest --runs-dir .glade/runs --format html --output reports/glade-report.html
```

## Useful Opportunities

Create `reports/` before running commands that write report files.

| Opportunity | Command |
| --- | --- |
| Explain a saved Salesforce debug log against local source | `glade debug explain --log apex.log --project .` |
| Generate a starter local repro test from a log | `glade debug repro --log apex.log --project . > ReproTest.cls` |
| Render local Visualforce preview pages | `glade dev vf --project . --addr 127.0.0.1:8080` |
| Render local LWC preview routes | `glade dev lwc --project . --context accountRecord --open` |
| Run a Salesforce-shaped local API | `glade server --project . --addr 127.0.0.1:8080` |
| Seed or inspect a local SQLite org state | `glade db seed --db .glade/org.sqlite --project . fixtures/dev.json` |
| Map a large Apex project | `glade inspect graph --project . --json` |
| Create an assessment report | `glade report assess --project . --format html --out reports/glade-assessment.html` |
| Review conservative dead-code candidates | `glade report cruft --project . --format html --out reports/glade-cruft.html` |
| Compare managed-package artifacts | `glade package diff old.glade.json new.glade.json --json` |

Advisory performance scans and compatibility ledgers are plugin commands. The
default public plugin registry serves the three first-party packages. Direct
archives and locally linked executables remain available for offline, private,
and development use:

```bash
glade plugins available
glade plugins install @glade/performance
glade performance scan --project . --json > reports/glade-performance.json
```

## What To Send Back

Useful pilot feedback includes:

- `glade version`
- full `glade doctor --project .` output
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
