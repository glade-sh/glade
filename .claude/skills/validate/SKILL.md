---
name: validate
description: Use to choose checks for a changed surface in glade - Go packages, the parser module, site, playground frontend, LWC compiler, VS Code extension, release scripts - and to check toolchain prerequisites before running them.
---

# Choose checks for a surface

Start with the smallest gate that covers the change. Broaden only when the
change crosses a package boundary. Run each step once per exact tree.

## Prerequisites

Use the Go version in `go.mod`, a C compiler and `CGO_ENABLED=1`.

    go version
    CGO_ENABLED=1 go build -o ./bin/glade ./cmd/glade
    ./bin/glade doctor --json

Read `parserOK` in the doctor output. A no-CGO build does not prove that
declaration parsing works. For JavaScript surfaces use the Node version named
in the matching CI workflow and `npm ci --prefix <dir>`.

## Surface table

| Changed surface | Checks |
| --- | --- |
| CLI, repository conventions | `go test ./internal/gladecli ./internal/repoguard` |
| Parser, semantics | Affected package tests. For `third_party/glade-apex-parser`, run `go test ./...` inside that module with `CGO_ENABLED=1` and `CGO_ENABLED=0`. |
| Runtime and data | Affected package tests. `scripts/smoke-runtime.sh ./bin/glade` on a fresh binary when integration moves. |
| Site content | `npm test --prefix site`. For rendered output: `npm run release:check --prefix site`, `npm run check:built --prefix site`, then `CI=1 GLADE_SITE_PREBUILT=1 npm run test:browser --prefix site`. |
| Playground frontend | `npm test --prefix internal/playground/web`, `npm run build --prefix internal/playground/web`, then affected Go playground tests. Review the `internal/playground/static` diff. |
| LWC compiler, Node integration | `npm ci --prefix third_party/lwc`, then `scripts/ci-go-test.sh node-integration`. |
| LWC browser runtime, Visualforce browser conformance | Use the `browser-suites` skill. |
| VS Code extension | `npm test --prefix contrib/vscode-glade`, then `npm run package --prefix contrib/vscode-glade`. |
| Release and distribution | Use the `release-cut` skill. |

## Ladder

| Step | Command | When |
| --- | --- | --- |
| Focused | `go test ./internal/<pkg> -run '^(TestA\|TestB)$'` | While iterating. |
| Package | `go test -timeout=0 ./internal/<pkg>` and `./internal/repoguard` | Once, before review. |
| Local release | `CGO_ENABLED=1 scripts/ci-go-test.sh local-release` | Changes that cross packages, or touch the parser, `sema`, `vm`, `storage`, `soql` or `dml`. |
| Required CI, locally | `scripts/ci-local-all` | Only when hosted CI is unavailable. |

Never run the whole `internal/apextest` package locally; CI shards it. Use
`GLADE_APEXTEST_SHARD=i/n` to reproduce one shard. Rerun a failing test alone
first, then on `origin/main`.

## Reporting

A test that needs Node, Playwright, Chromium or `jq` and did not run is "not
run", never "passed". Name the missing tool. Details: `docs/AI_SETUP.md`.
