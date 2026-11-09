# AI contributor setup

The end-to-end process (stages, roles, review and release rules, templates) is
in [WORKFLOW.md](WORKFLOW.md). This file covers client setup and the validation
commands per surface.

This guide is for developing Glade itself. To use an agent to develop Apex in
a Salesforce DX project, use the [AI-assisted Apex guide](../site/docs-src/guide/ai-assisted-apex.md).

## Instructions and models

[AGENTS.md](../AGENTS.md) is the shared contributor contract. Codex is the
primary documented client; the repository does not require a particular model,
provider, plugin, or personal skill collection. Other clients should load the
same file through their supported instruction mechanism. Add a small pointer
only when a client needs one; keep the actual rules in one place.

Keep repository facts, constraints, and verification commands in `AGENTS.md`.
Keep model selection, reasoning effort, authentication, permissions, and
machine-specific tools in personal settings. The repository needs no Codex
configuration override today. Add `.codex/config.toml` only for a demonstrated
project requirement.

Codex discovers instructions from its home directory and the repository path to
the working directory. `AGENTS.override.md` takes precedence over `AGENTS.md`
in the same directory. Linked guides are references to read when needed, not
automatically expanded instructions. Start a new task after changing startup
instructions and verify which files it loaded. See the official
[AGENTS.md guide](https://learn.chatgpt.com/docs/agent-configuration/agents-md).

Pick the model in the client; the repository does not depend on one. State
the client and model in your report so results can be reproduced. Model and
reasoning defaults belong in user configuration; provider endpoints and auth
also belong there. Pass the model explicitly when a client's default may
differ, and inspect the effective task settings when an app selection differs
from a CLI default. See
[Codex configuration](https://learn.chatgpt.com/docs/config-file/config-basic).

Give any agent a clear outcome, the relevant files, the constraints in
`AGENTS.md`, and the completion checks from the validation table. Say when to
proceed on reasonable assumptions and when independent subtasks may run in
parallel. Audit overlapping skills before adding more instructions: broad
triggers, conflicting approval rules, unavailable tool names, and mandatory
ceremonies can all change the result.

Use task-specific skills for recurring procedures. Routine repository work
already has shell commands and tests; it does not require a new MCP server,
orchestration layer, or model-specific prompt copy. Keep skills narrow and
use the tools actually exposed by the active client.

## Local prerequisites

Run from the checkout being changed. Use Go `1.26.10` (the version in `go.mod`),
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

For LWC Go browser integration, install dependencies in `third_party/lwc` and
`lwcruntime` and install Chromium using the latter's Playwright version, then run:

```bash
GLADE_LWC_BROWSER=1 CGO_ENABLED=1 go test -p=1 -count=1 -timeout=25m \
  -run '^(TestBrowserRuntimeSuite|TestGeneratedPhase3BaseComponentsRunInBrowser)$' \
  ./internal/lwcruntime ./internal/lwcbrowser
```

`TestL08SalesforceConformance` in `internal/server` also requires this browser
environment. Ordinary Go runs skip it with an explicit reason; set
`GLADE_LWC_BROWSER=1` to require it (`GLADE_L08_CAPTURE` also requires it).
An enabled run fails immediately when Node, the compiler, Playwright or Chromium
is unavailable. A passing ordinary Go run does not establish L08 browser parity.
The required Browser workflow runs L08 and rejects missing, skipped or failing
results. It also runs after changes confined to L08 fixtures or its browser
harness. The ordinary CI server lane routes L08 to that authority. Provisioned
race jobs and full server routes (`core`, `test`, `race`, `local-release`, and
`local-release-lane server-playground` in `scripts/ci-go-test.sh`) enable browser
coverage and fail when prerequisites are missing; they do not accept an L08 skip.
Use `npm ci --prefix third_party/lwc` and `npm ci --prefix lwcruntime`, then
install Chromium with the Playwright version in `lwcruntime`. The compiler's
postinstall prepares the pinned `@lwc/shared` API identities. Check the selected
installation without modifying it with
`node third_party/lwc/scripts/prepare-lwc-shared.mjs --check <third_party/lwc-root>`.
An older API 67 preparation cannot replay the retained API 68 observations;
those historical rows do not extend the product's API 59–67 support contract.
For an archived checkout, point `GLADE_HOME` at a prepared toolchain and set
`GLADE_LWC_PLAYWRIGHT_MODULE` to the absolute Playwright package directory.
`GLADE_LWC_BROWSER_EXECUTABLE` optionally selects an installed Chromium binary.

Visualforce's V09, V09 clicked-field, V12, V13 and V15 Salesforce
conformance tests also require Node, Playwright and Chromium. CI installs the
locked `lwcruntime` dependencies and its Chromium before the Visualforce
shards. A tracked-files-only archive omits `node_modules`, so a runner without
an externally provisioned Playwright package cannot execute these browser
checks. Treat them as CI-only on such runners; missing prerequisites are not a
conformance pass. To run them on a provisioned checkout, install both dependency
trees and Chromium as in CI, then use:

```bash
CGO_ENABLED=1 go test -count=1 -p=1 -timeout=0 \
  -run '^TestV(09|12|13|15).*SalesforceConformance$' ./internal/visualforce
```

An archived checkout needs `GLADE_LWC_PLAYWRIGHT_MODULE` set to the absolute
path of a compatible installed Playwright package and a usable Chromium. On
Linux, Chromium may also need system dependencies; CI uses Playwright's
`install --with-deps chromium`.

For a checkout with dependencies installed, run:

```bash
GLADE_HOME="$PWD" GLADE_LWC_BROWSER=1 CGO_ENABLED=1 go test -p=1 -count=1 -timeout=0 \
  -run '^TestL08SalesforceConformance$' ./internal/server
```

LWC family conformance runs one API at a time by default. Set
`GLADE_LWC_BROWSER_API_PARALLEL=2` and pass `-parallel=2` to opt in to two
isolated API observations. The mode requires at least 16 GiB of physical host
memory; smaller hosts or an unknown memory size stay sequential even with an
explicit request. Higher requested counts are capped at two. API 68 and
intermediate version checks retain their existing coverage. Observation windows,
within-API page histories, comparison rules and TSV row order are unchanged. Each family remains sequential with other families.

For a small native-backed project check, run
`go test ./internal/lwc/compile -run '^TestBrowserAPIProjectMode$' -count=1 -parallel=2`
with the API-pass variable set to `1` and then `2`. Set
`GLADE_LWC_BROWSER_API_REPORT` to separate TSV paths and compare them byte for
byte. Measure aggregate RSS across the Go, Node and Chromium process tree;
a single process's maximum RSS does not measure the concurrent host cost.
A small-project result does not establish equivalence or a performance gain
for all families: repeat each family's conformance test and compare its report
before enabling concurrency in a lane planner.

Use [the CI workflows](../.github/workflows) and package scripts as the current
command authority.

## Validation ladder

Climb only as far as the change needs. Each step runs once per exact tree.

| Step | Command | When |
| --- | --- | --- |
| Focused | `go test ./internal/<pkg> -run '^(TestA\|TestB)$'` | While iterating. Rerun a failure alone before anything broader. |
| Package | `go test ./internal/<pkg>` for each changed package, plus `./internal/gladecli ./internal/repoguard` | Once before review. |
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
Actions, with the same shard counts: apextest 12, vm 4, gladecli 4, sema 2,
LWC compile 8 and Visualforce 4. It needs Go with CGO, Node 22, actionlint,
the CodeQL CLI and Go query packs, and Playwright Chromium. A missing tool is
`SKIP-ENV`, which exits nonzero. Shards run serially unless `--jobs N` is set.

## Hosted CI

GitHub Actions is the hosted authority. Linux jobs use Ubuntu 24.04 for
compatibility with the pinned Playwright browser dependencies. Pull requests and
pushes to `main` run these workflows:

| Workflow | Triggers | What it gates |
| --- | --- | --- |
| `CI` (`ci.yml`) | PR, push to `main`, Monday schedule, dispatch | The `Required CI` job, the only ruleset-required check. It succeeds only when every dependency succeeds: workflow-lint, site, test-binaries, vet, the sharded apextest, vm, gladecli and sema jobs and their duration-history jobs, node-integration, nested-sources, server-and-playground, the `test` matrix (remaining Go packages plus repoguard, LWC compile and Visualforce shards), smoke-runtime and smoke-distribution. The schedule adds `Required Scheduled CI` (sema-full and sema-equivalence). |
| `Browser` (`browser.yml`) | PR, push to `main`, daily, dispatch | LWC browser and L08 conformance. Missing or skipped browser results fail. |
| `Race` (`race.yml`) | Successful CI workflow run for a push to `main` (`workflow_run`), Monday schedule, dispatch | The main-branch run follows CI instead of competing with it for the 20 concurrent hosted runners, and runs the concurrency selection in `scripts/ci-race-push.json`. Schedule and dispatch run the full race matrix, with apextest in three parts. Not a PR check. |
| `Security` (`security.yml`) | PR, push to `main`, `v*` tags, Monday schedule, dispatch | govulncheck, gosec, npm audit, dependency review and OpenSSF Scorecard. The fast CodeQL profile runs on PRs and main pushes; full runs on the schedule, tags and dispatch. |
| `Repeat timing-sensitive tests` (`repeat.yml`) | Daily, dispatch | Reruns `scripts/ci-repeat-tests.txt` on Linux and macOS. |
| `Release` (`release.yml`) | `v*` tags, dispatch | See [DISTRIBUTION_WORKFLOW.md](DISTRIBUTION_WORKFLOW.md). |

CodeQL has two profiles in one reusable workflow (`codeql.yml`). The fast
profile runs in Security outside `Required CI`. The full profile runs in Security
and as a release tag dependency. Analysis has a hard 300-second budget. When
the budget expires, both profiles warn and upload no SARIF.
Large changes regularly hit this. An incomplete scan is not evidence that there
are no findings. See [CODEQL_WORKFLOW.md](CODEQL_WORKFLOW.md) and
[CODEQL_BUDGET.md](CODEQL_BUDGET.md).

Release tags need an exact-SHA `Required CI` success from a push to `main`.
`scripts/verify-required-ci.sh OWNER/REPO SHA` finds it; PR and manual runs do
not count. Browser, Race and Security are operator checks for the release candidate. The
release workflow independently runs the full CodeQL scan on the tag and fails
on findings or errors; a timeout there is a known-issue warning.

## Check that the setup helps

Two habits remove most review rounds. Review your own diff with the reviewer's
criteria before asking for review: observed evidence for each behavior change,
a fail-before test, route twins, and no loosened assertion without a citation.
Then rerun your new tests from a clean export of HEAD
(`git archive HEAD | tar -x -C <dir>`), so untracked files cannot hide a
failure.

In a fresh task, ask the agent to identify its instruction files, the product
boundary, and the checks for the files you intend to change. It should find
these answers without reading every document or installing extra tooling.

Compare candidate instruction or model changes on the same representative
tasks: one runtime regression, one parser change, and one documentation edit.
Keep the starting commit and acceptance checks fixed. Record correctness,
unnecessary questions, unrelated edits, skipped gates, elapsed time, and usage
when available. More reasoning or more instructions is not proof of a better
setup; keep changes that improve observed results.
