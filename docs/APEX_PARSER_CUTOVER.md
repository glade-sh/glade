# Apex parser cutover

`oaer` now uses the local tree-sitter Apex parser module for declaration
parsing:

```text
github.com/open-aer/apex-parser v0.1.0 => ../oaer-apex-parser
```

This local cutover replaces the former direct parser dependencies:

- `github.com/octoberswimmer/apexfmt`
- `github.com/antlr4-go/antlr/v4`

The parser module lives at:

```text
/Users/matt/Dev/oaer-apex-parser
```

The module is local-only for now. Do not remove the `replace` directive until
`github.com/open-aer/apex-parser` is published or otherwise reachable by CI.

## Scope

The tree-sitter module replaces `internal/apexast` declaration extraction. It
preserves the existing `internal/apexast` API:

- `type Parser`
- `NewParser`
- `ParseFile`
- `ParseSource`

It extracts the structural model used by project loading, symbol indexing,
semantic analysis, LSP, watch, readiness, replay, and test discovery:

- classes, interfaces, enums, and triggers
- methods, constructors, fields, properties, initializers, and nested types
- modifiers, parameters, accessors, trigger object names, and trigger events
- source ranges and parser diagnostics

It does not replace the VM's anonymous Apex parser or provide formatting.

## Current validation baseline

The updated local parser module matches the former `apexfmt` adapter across the
checked example projects:

```text
example-projects/src-nmb-nc-develop:     2424 files, 2424 matched, 0 mismatches
example-projects/src-nmb-nu-develop:     2963 files, 2963 matched, 0 mismatches
example-projects/src-nmb-nutpl-develop:   135 files, 135 matched, 0 mismatches
```

It also matches the checked replay bundle used during cutover:

```text
testdata/replay/selector-service-domain: 2 files, 2 matched, 0 mismatches
```

Prior benchmark numbers on Apple M2 Pro:

```text
tree-sitter BenchmarkParseClass-12    ~1.5 ms/op    ~151 KB/op    ~2,170 allocs/op
tree-sitter BenchmarkBuildIndex-12    ~3.4 ms/op    ~455 KB/op    ~6,811 allocs/op
old ANTLR BenchmarkParseClass-12      ~6.5 ms/op    ~6.45 MB/op   ~81,935 allocs/op
```

## Local setup

Keep the parser module beside this repository:

```text
/Users/matt/Dev/oaer
/Users/matt/Dev/oaer-apex-parser
```

The relevant module entries are:

```text
require github.com/open-aer/apex-parser v0.1.0
replace github.com/open-aer/apex-parser => ../oaer-apex-parser
```

After changes in either repo, run:

```sh
go mod tidy
go test ./...
```

## Validation

Run the full suite:

```sh
go test ./...
```

Run parser and index benchmarks:

```sh
go test ./internal/apexast ./internal/typesys -run '^$' -bench=. -benchmem
```

Run the parser module's example-project corpus test:

```sh
cd /Users/matt/Dev/oaer-apex-parser
go test -count=1 ./...
```

Run project-level smoke checks from `oaer`:

```sh
go run ./cmd/oaer check --project example-projects/src-nmb-nutpl-develop --json
go run ./cmd/oaer compat replay testdata/replay/selector-service-domain
scripts/smoke.sh
```

## CGO and local builds

Real tree-sitter parsing requires CGO because the parser uses generated C. With
`CGO_ENABLED=0`, the parser module still builds, but parsing returns diagnostic
code `APEXPARSECGO`.

For the local-only cutover, use normal local CGO-enabled builds:

```sh
go test ./...
go build -o oaer ./cmd/oaer
```

Remote release builds still need a separate decision. `scripts/release-build.sh`
currently forces `CGO_ENABLED=0`, which is not suitable for a production
tree-sitter parser release unless the no-CGO diagnostic fallback is acceptable.

## Rollback

No data migration is involved. To roll back:

1. Restore the old `internal/apexast/parser.go`.
2. Remove `github.com/open-aer/apex-parser` from `go.mod`.
3. Remove the local `replace` directive.
4. Run `go mod tidy`.
5. Run `go test ./...`.

## Notes

The parser module preserves the existing `void(` method-name edge case with
length-preserving normalization. That keeps source offsets stable while allowing
Apex methods named `void`.

Parser diagnostic message text differs from ANTLR. Downstream code should depend
on severity, code, file, range, and excerpt rather than exact parser wording.
