# Apex parser cutover

This document describes how to replace `oaer`'s current ANTLR/apexfmt parser with the tree-sitter parser module built at:

`/Users/matt/Dev/oaer-apex-parser`

The parser module is tagged locally as:

`github.com/open-aer/apex-parser v0.1.0`

It replaces these direct parser dependencies:

- `github.com/octoberswimmer/apexfmt`
- `github.com/antlr4-go/antlr/v4`

## Current proof point

The integration proof lives in:

`/Users/matt/Dev/oaer-parser-poc`

The useful files there are:

- `go.mod`
- `go.sum`
- `internal/apexast/parser.go`

That worktree proves the new module can sit behind the existing `internal/apexast` API without changing callers.

Validated there:

```text
go test ./...
go test ./internal/apexast ./internal/typesys -run '^$' -bench=. -benchmem
```

The example corpus at `example-projects/` also parsed cleanly:

```text
files=3099 failed=0
```

## Prerequisites

1. Publish or otherwise make `github.com/open-aer/apex-parser v0.1.0` reachable.
2. Confirm CGO is acceptable for release builds. The parser uses generated tree-sitter C.
3. Keep `CGO_ENABLED=0` builds only if the no-CGO fallback diagnostic is acceptable.

The parser module builds without CGO, but real parsing requires CGO. With `CGO_ENABLED=0`, parsing returns diagnostic code `APEXPARSECGO`.

## Cutover steps

Start from a clean `oaer` branch or worktree.

```sh
cd /Users/matt/Dev/oaer
git switch main
git pull
git switch -c replace-apex-parser
```

Add the parser module:

```sh
go get github.com/open-aer/apex-parser@v0.1.0
```

If the module is not published yet, use a local replace while testing:

```sh
go mod edit -replace github.com/open-aer/apex-parser=../oaer-apex-parser
go get github.com/open-aer/apex-parser@v0.1.0
```

Replace `internal/apexast/parser.go` with the adapter from:

```text
/Users/matt/Dev/oaer-parser-poc/internal/apexast/parser.go
```

That adapter keeps the existing API:

- `type Parser`
- `NewParser`
- `ParseFile`
- `ParseSource`

It converts the external parser module's model types into `oaer/internal/apexast` and `oaer/internal/diagnostic` types.

Then tidy dependencies:

```sh
go mod tidy
```

Confirm the old parser dependencies are gone from live code:

```sh
rg 'github.com/antlr4-go/antlr|github.com/octoberswimmer/apexfmt' go.mod internal
```

Expected result: no matches in `go.mod` or production Go files.

Historical docs may still mention `apexfmt`; those are not blockers.

## Validation

Run the full suite:

```sh
go test ./...
```

Run parser and index benchmarks:

```sh
go test ./internal/apexast ./internal/typesys -run '^$' -bench=. -benchmem
```

Run the example corpus check. A quick temporary checker can use `internal/apexast.NewParser()` and walk:

```text
example-projects/**/*.cls
example-projects/**/*.trigger
```

Expected result:

```text
files=3099 failed=0
```

Prior POC numbers on Apple M2 Pro:

```text
BenchmarkParseClass-12    ~1.5 ms/op    ~151 KB/op    ~2,170 allocs/op
BenchmarkBuildIndex-12    ~3.4 ms/op    ~455 KB/op    ~6,811 allocs/op
```

The current ANTLR parser was about:

```text
BenchmarkParseClass-12    ~6.5 ms/op    ~6.45 MB/op    ~81,935 allocs/op
```

## Release build checks

Because the parser uses CGO, verify supported release targets early:

```sh
CGO_ENABLED=1 go test ./...
```

For no-CGO fallback behavior:

```sh
CGO_ENABLED=0 go test ./...
```

For cross-compile compile-only checks from macOS:

```sh
tmp=$(mktemp -d)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -o "$tmp/oaer-linux-amd64.test" ./cmd/oaer
rm -rf "$tmp"
```

Do not assume cross-compiled CGO builds work without a target C toolchain.

## Expected code change shape

The main `oaer` cutover should be small:

```text
go.mod
go.sum
internal/apexast/parser.go
```

The old `internal/apexast` model, source helpers, walk helpers, and downstream callers should stay in place.

## Rollback

If release builds or compatibility checks fail:

1. Restore the old `internal/apexast/parser.go`.
2. Remove `github.com/open-aer/apex-parser` from `go.mod`.
3. Remove any local `replace` directive.
4. Run `go mod tidy`.
5. Re-run `go test ./...`.

No data migration is involved.

## Notes

The tree-sitter parser preserves the existing `void(` method-name edge case with length-preserving normalization. That keeps source offsets stable while allowing Apex methods named `void`.

The new parser's diagnostic message text does not match ANTLR exactly. Downstream code should depend on severity, file, range, and excerpt rather than exact parser wording.
