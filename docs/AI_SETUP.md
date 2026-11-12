# AI contributor setup

The end-to-end process (stages, roles, review and release rules, templates) is
in [WORKFLOW.md](WORKFLOW.md). This file covers instruction loading and the
validation commands per surface.

This guide is for developing Glade itself. To use an agent to develop Apex in
a Salesforce DX project, use the [AI-assisted Apex guide](../site/docs-src/guide/ai-assisted-apex.md).

## Instructions

[AGENTS.md](../AGENTS.md) is the shared contributor contract. The repository
does not require a particular client, model, provider or plugin. Task procedures
live in `.claude/skills/<name>/SKILL.md`; a client that does not load them
automatically should open the one whose description matches the task. Keep
repository rules in `AGENTS.md`. Keep model choice, authentication, permissions
and machine-specific tools in personal settings. The repository needs no Codex
configuration override today.

Linked guides are references to read when needed, not automatically expanded
instructions. State the client and model in a report so results can be
reproduced. Give any agent a clear outcome, the relevant files, the constraints
in `AGENTS.md` and the completion checks from the table below.

## Local prerequisites

Run from the checkout being changed. Use Go `1.26.11` (the version in `go.mod`),
a C compiler, and CGO for normal builds and tests:

```bash
go version
go env CGO_ENABLED CC
CGO_ENABLED=1 go build -o ./bin/glade ./cmd/glade
./bin/glade doctor --json
```

Inspect `parserOK` in doctor output. A version command or successful no-CGO
build does not prove declaration parsing works. Doctor is project-aware; other
findings depend on the selected project. See [APEX_PARSER.md](APEX_PARSER.md).

For JavaScript surfaces, use the Node version in the corresponding CI workflow
and `npm ci --prefix <directory>` with that directory's lockfile when dependencies
are missing or changed. Browser tests also need their package's Playwright
browser installation. Do not treat a dependency-driven skip as a passing test.

These suites need such tools on the host: `internal/gladecli` and
`internal/server` (Node for the LWC shell and toolchain tests),
`internal/visualforce` and `internal/lwcbrowser` (Playwright and Chromium), and
`./scripts` (a real git checkout, `jq` and Node). Run them where the dependency
exists, or report the result as environmental and name the missing tool.

## Validation by surface

Choose the smallest relevant gate, then broaden when the change crosses a
boundary. Replace placeholders with actual package or test names. Go commands
below assume `CGO_ENABLED=1` unless a no-CGO test is explicitly requested.

| Changed surface | Source and checks |
| --- | --- |
| CLI and repository conventions | `internal/gladecli`, `internal/repoguard`; `go test ./internal/gladecli ./internal/repoguard` |
| Parser and semantics | `internal/apexast`, `internal/sema`; run affected package tests. For `third_party/glade-apex-parser`, run `go test ./...` inside that module with both `CGO_ENABLED=1` and `CGO_ENABLED=0`. Root tests do not include it. |
| Runtime and data | `internal/vm`, `internal/apextest`, `internal/soql`, `internal/dml`, `internal/storage`, `internal/server`; run affected package tests and `scripts/smoke-runtime.sh ./bin/glade` against a freshly built binary when integration moves. |
| Site content | `site/docs-src`, `site/.vitepress`; `npm test --prefix site`. For rendered behavior, use the sequence below. |
| Playground frontend | `internal/playground/web`; `npm test --prefix internal/playground/web`, then `npm run build --prefix internal/playground/web`. Review the generated `internal/playground/static` diff and run affected Go playground tests. |
| LWC compiler and Node integration | Install dependencies in `third_party/lwc`; `scripts/ci-go-test.sh node-integration` checks that required integration tests actually execute. |
| LWC browser runtime | Install dependencies in `third_party/lwc` and `lwcruntime`, plus the latter's Chromium. `npm test --prefix lwcruntime` builds the runtime and runs its browser suite. For Go integration coverage, use the explicit browser command below. Review generated `internal/lwcruntime/embed/glade.out.js`. |
| VS Code extension | `contrib/vscode-glade`; `npm test --prefix contrib/vscode-glade`, then `npm run package --prefix contrib/vscode-glade`. |
| Release and distribution | Follow [DISTRIBUTION_WORKFLOW.md](DISTRIBUTION_WORKFLOW.md) and [RELEASE_POLICY.md](RELEASE_POLICY.md); `scripts/release-check.sh` coordinates existing site, Go, and distribution gates. |

For site rendering, build the current source before checking output:

```bash
npm run release:check --prefix site
npm run check:built --prefix site
CI=1 GLADE_SITE_PREBUILT=1 npm run test:browser --prefix site
```

`CI=1` prevents Playwright from silently reusing an unrelated preview on port
4173. If that port is occupied, identify the owner; do not terminate another
task's server. `release:check` alone does not run browser or built-output checks.
See [site/README.md](../site/README.md) for preview and deployment details.

## Browser suites

A skipped browser test is not a pass. LWC runtime, `TestL08SalesforceConformance`
in `internal/server` and the Visualforce V09, V12, V13 and V15 conformance tests
need Node, Playwright and Chromium. Ordinary Go runs skip them with an explicit
reason. Set `GLADE_LWC_BROWSER=1` to require the browser environment; an enabled
run fails at once when a prerequisite is missing. The required `Browser`
workflow runs L08 and rejects missing, skipped or failing results.

Install the dependencies and Chromium as CI does:

```bash
npm ci --prefix third_party/lwc
npm ci --prefix lwcruntime
./lwcruntime/node_modules/.bin/playwright install chromium
```

Then run:

```bash
GLADE_LWC_BROWSER=1 CGO_ENABLED=1 go test -p=1 -count=1 -timeout=25m \
  -run '^(TestBrowserRuntimeSuite|TestGeneratedPhase3BaseComponentsRunInBrowser)$' \
  ./internal/lwcruntime ./internal/lwcbrowser
GLADE_HOME="$PWD" GLADE_LWC_BROWSER=1 CGO_ENABLED=1 go test -p=1 -count=1 -timeout=0 \
  -run '^TestL08SalesforceConformance$' ./internal/server
CGO_ENABLED=1 go test -count=1 -p=1 -timeout=0 \
  -run '^TestV(09|12|13|15).*SalesforceConformance$' ./internal/visualforce
```

In an archived checkout without `node_modules`, set
`GLADE_LWC_PLAYWRIGHT_MODULE` to the absolute Playwright package directory and
`GLADE_HOME` to a prepared toolchain. `GLADE_LWC_BROWSER_EXECUTABLE` selects an
installed Chromium. On Linux, CI installs Chromium with
`scripts/ci-install-playwright.sh`, which bounds each `--with-deps` attempt and
falls back to Chromium alone if it launches.
`GLADE_LWC_BROWSER_API_PARALLEL=2` with `-parallel=2` runs two API observations
at once and needs at least 16 GiB of host memory. The `browser-suites` skill
carries the same commands. Use [the CI workflows](../.github/workflows) and
package scripts as the current command authority.

## Validation ladder

Climb only as far as the change needs. Each step runs once per exact tree.

| Step | Command | When |
| --- | --- | --- |
| Focused | `go test ./internal/<pkg> -run '^(TestA\|TestB)$'` | While iterating. Rerun a failure alone before anything broader. |
| Package | `go test ./internal/<pkg>` for each changed package, plus `./internal/gladecli ./internal/repoguard`; for apextest use focused `-run` tests | Once before review; CI runs the 14 apextest shards. |
| Full local release | `scripts/ci-go-test.sh local-release` | Changes that cross packages or touch the parser, `sema`, `vm`, `storage`, `soql` or `dml`; release candidates. |
| Distribution smoke | `scripts/release-check.sh` (adds `git diff --check`, the site release proof and `scripts/smoke.sh`) | Release candidates, from a clean checkout. |
| Required CI, locally | `scripts/ci-local-all` | Before opening or updating a release PR. |

`local-release` runs its lanes one at a time: `vet`, `guard` (repoguard),
`cli-ui` (gladecli), `sema`, `apex` (two apextest shards), `server-playground`
and `remaining`. `scripts/ci-go-test.sh local-release-lane NAME` reruns one
lane. `LOCAL_GO_TEST_JOBS` accepts `auto` or a positive integer, but lanes do
not overlap. Raw events and a validated `package-summary.json` per lane go
under `ci-artifacts/local-release/`. Avoid running it alongside another broad
suite on the same host.

`scripts/ci-local-all` runs the jobs behind `Required CI` without GitHub
Actions, with the same shard counts: apextest 14, vm 3, gladecli 2, sema 2,
LWC compile 8 and Visualforce 4. It needs Go with CGO, Node 22, actionlint,
the CodeQL CLI and Go query packs, and Playwright Chromium. A missing tool is
`SKIP-ENV`, which exits nonzero. Shards run serially unless `--jobs N` is set.

## Hosted CI

GitHub Actions is the hosted authority. Linux jobs use Ubuntu 24.04 for
compatibility with the pinned Playwright browser dependencies. Pull requests and
pushes to `main` run these workflows:

| Workflow | Triggers | What it gates |
| --- | --- | --- |
| `CI` (`ci.yml`) | PR, merge queue, push to `main`, Monday schedule, dispatch | The `Required CI` job, the only ruleset-required check. It succeeds only when every dependency succeeds: workflow-lint, site, test-binaries and test-binaries-apextest (separate producers, so the apextest shards do not wait on the other three binaries), vet, the sharded apextest, vm, gladecli and sema jobs and their duration-history jobs, node-integration, nested-sources, server-and-playground, the `test` job (remaining Go packages plus repoguard) and the `lwc-vf` matrix (LWC compile and Visualforce shards), smoke-runtime, smoke-distribution and leak-scan (`scripts/leak-scan.sh` over the added lines of the change: personal and host paths, private temp paths, and the entries of the `LEAK_SCAN_DENYLIST` repository secret, with `.github/leak-scan-allow.txt` exempting named files from the denylist). The schedule adds `Required Scheduled CI` (sema-full and sema-equivalence). |
| `Browser` (`browser.yml`) | PR, push to `main`, daily, dispatch | LWC browser and L08 conformance. Missing or skipped browser results fail. |
| `Race` (`race.yml`) | Nightly schedule, dispatch | The full race matrix, with apextest in three parts. It does not run on pushes or pull requests and is not a PR check. |
| `Security` (`security.yml`) | PR, push to `main`, `v*` tags, Monday schedule, dispatch | govulncheck, gosec, npm audit, dependency review and OpenSSF Scorecard. The fast CodeQL profile runs on PRs and main pushes; full runs on the schedule, tags and dispatch. |
| `Repeat timing-sensitive tests` (`repeat.yml`) | Daily, dispatch | Reruns `scripts/ci-repeat-tests.txt` on Linux and macOS. |
| `Release` (`release.yml`) | `v*` tags, dispatch | See [DISTRIBUTION_WORKFLOW.md](DISTRIBUTION_WORKFLOW.md). |

CodeQL has two profiles in one reusable workflow (`codeql.yml`). The fast
profile runs in Security outside `Required CI`. The full profile runs in Security
and as a release tag dependency. Analysis has a hard 300-second budget. When
the budget expires, both profiles warn and upload no SARIF.
Large changes regularly hit this. An incomplete scan is not evidence that there
are no findings. See [CODEQL.md](CODEQL.md).

Release tags need an exact-SHA `Required CI` success from a push to `main`.
`scripts/verify-required-ci.sh OWNER/REPO SHA` finds it; PR and manual runs do
not count. Browser, Race and Security are operator checks for the release candidate. The
release workflow independently runs the full CodeQL scan on the tag and fails
on findings or errors; a timeout there is a known-issue warning.

## Workflow lint

CI's `workflow-lint` job installs actionlint **1.7.12** from the upstream
[release](https://github.com/rhysd/actionlint/releases/tag/v1.7.12).
`scripts/install-actionlint.sh` pins the Linux amd64 archive SHA-256 to
`8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8`.
It verifies the downloaded archive before extraction or execution. The CI job's
checkout and Go setup actions use full commit pins with version comments.

With actionlint on PATH, run from the repository root:

```sh
scripts/ci-actionlint.sh
scripts/ci-go-test.sh lane workflow-lint
```

The wrapper checks every `.github/workflows/*.yml` and `*.yaml`, including when
called from another directory. Explicit file arguments are resolved relative to
the caller. It fails if actionlint is missing or discovery finds no workflows.
Tests skip real-actionlint cases with an explicit message only when the tool is
absent; CI always installs it first. Tests never download anything. The corrupt
archive test stubs curl and verifies that extraction cannot run on bad bytes.
Optional ShellCheck and Pyflakes integrations are disabled to keep this check
focused on workflow syntax, expressions, contexts and action configuration.
No actionlint errors are ignored.

The CI job uses the lane and resource wrappers, uploads raw test events and
resource usage even on failure, and is a dependency of `Required CI`. The lane
runs only the offline actionlint tests, with a three-minute timeout. Its raw
events are written to `ci-artifacts/go-test/test-workflow-lint.json`.

For a local macOS install, download the matching archive from that release and
verify its digest against the upstream `actionlint_1.7.12_checksums.txt` before
extracting. The Darwin arm64 archive SHA-256 is
`aba9ced2dee8d27fecca3dc7feb1a7f9a52caefa1eb46f3271ea66b6e0e6953f`.

To update the pin, choose an exact upstream release, review its release notes,
download the Linux amd64 archive and published checksums, and independently
compute the archive's SHA-256. Update `version` and `sha256` together in the
installer and update this document. Repeat the commands above with the new
binary; the job-env fixture must be rejected and the step-env fixture accepted.
Review any real-workflow errors before committing the pin change. Keep action
references pinned to full commits with version comments when updating the job.

## Before asking for review

Review your own diff with the reviewer's criteria: observed evidence for each
behavior change, a fail-before test, route twins, and no loosened assertion
without a citation. Rerun your new tests from a clean export of HEAD
(`git archive HEAD | tar -x -C <dir>`) so untracked files cannot hide a
failure. The `pr-ready` skill lists the steps.

Compare candidate instruction or model changes on the same representative
tasks (a runtime regression, a parser change, a documentation edit) with the
starting commit and acceptance checks fixed. Record correctness, unnecessary
questions, skipped gates, elapsed time and usage. Keep changes that improve
observed results.
