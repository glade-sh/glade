# AI contributor setup

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

For Codex, select Astra or another available model in the client. Model and
reasoning defaults belong in user configuration; provider endpoints and auth
also belong there. Inspect the effective task settings when an app selection
differs from a CLI default. Use the current client's supported values rather
than copying context limits or model-specific settings from another client.
See [Codex configuration](https://learn.chatgpt.com/docs/config-file/config-basic).

For Astra, give a clear outcome, relevant context, constraints, and completion
checks. State when to proceed with reasonable assumptions and when independent
subtasks should run in parallel. Audit overlapping skills before adding more
instructions: broad triggers, conflicting approval rules, unavailable tool
names, and mandatory ceremonies can all change the result. These priorities
follow [OpenAI's Astra guidance](https://developers.openai.com/api/docs/guides/latest-model#prompting-best-practices).

Use task-specific skills for recurring procedures. Routine repository work
already has shell commands and tests; it does not require a new MCP server,
orchestration layer, or model-specific prompt copy. Keep skills narrow and
use the tools actually exposed by the active client.

## Local prerequisites

Run from the checkout being changed. Use the Go version in `go.mod`, a C
compiler, and CGO for normal builds and tests:

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

Use [the CI workflows](../.github/workflows) and package scripts as the current
command authority. `scripts/ci-go-test.sh local-release` runs the broad Go lanes;
avoid running it concurrently with another broad suite on the same host.

## Check that the setup helps

In a fresh task, ask the agent to identify its instruction files, the product
boundary, and the checks for the files you intend to change. It should find
these answers without reading every document or installing extra tooling.

Compare candidate instruction or model changes on the same representative
tasks: one runtime regression, one parser change, and one documentation edit.
Keep the starting commit and acceptance checks fixed. Record correctness,
unnecessary questions, unrelated edits, skipped gates, elapsed time, and usage
when available. More reasoning or more instructions is not proof of a better
setup; keep changes that improve observed results.
