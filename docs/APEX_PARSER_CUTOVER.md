# Apex parser cutover

`glade` now uses the local tree-sitter Apex parser module for declaration
parsing:

```text
github.com/glade-sh/apex-parser v0.1.0 => ../glade-apex-parser
```

This local cutover replaces the former direct parser dependencies:

- `github.com/octoberswimmer/apexfmt`
- `github.com/antlr4-go/antlr/v4`

The parser module lives at:

```text
/Users/matt/Dev/glade-apex-parser
```

The module is local-only for now. Do not remove the `replace` directive until
`github.com/glade-sh/apex-parser` is published or otherwise reachable by CI.

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
large-example-corpus-a:                  2424 files, 2424 matched, 0 mismatches
large-example-corpus-b:                  2963 files, 2963 matched, 0 mismatches
small-example-corpus:                     135 files, 135 matched, 0 mismatches
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
/Users/matt/Dev/glade
/Users/matt/Dev/glade-apex-parser
```

The relevant module entries are:

```text
require github.com/glade-sh/apex-parser v0.1.0
replace github.com/glade-sh/apex-parser => ../glade-apex-parser
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
cd /Users/matt/Dev/glade-apex-parser
go test -count=1 ./...
```

Run project-level smoke checks from `glade`:

```sh
go run ./cmd/glade check --project path/to/project --json
go run ./cmd/glade compat replay testdata/replay/selector-service-domain
scripts/smoke.sh
```

## CGO and local builds

Real tree-sitter parsing requires CGO because the parser uses generated C. With
`CGO_ENABLED=0`, the parser module still builds, but parsing returns diagnostic
code `APEXPARSECGO`.

For the local-only cutover, use normal local CGO-enabled builds:

```sh
go test ./...
go build -o glade ./cmd/glade
```

Remote release builds still need a separate decision. `scripts/release-build.sh`
currently forces `CGO_ENABLED=0`, which is not suitable for a production
tree-sitter parser release unless the no-CGO diagnostic fallback is acceptable.

## Rollback

No data migration is involved. To roll back:

1. Restore the old `internal/apexast/parser.go`.
2. Remove `github.com/glade-sh/apex-parser` from `go.mod`.
3. Remove the local `replace` directive.
4. Run `go mod tidy`.
5. Run `go test ./...`.

## Notes

The parser module preserves the existing `void(` method-name edge case with
length-preserving normalization. That keeps source offsets stable while allowing
Apex methods named `void`.

Parser diagnostic message text differs from ANTLR. Downstream code should depend
on severity, code, file, range, and excerpt rather than exact parser wording.
