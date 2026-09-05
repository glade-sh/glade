# Apex parser

`glade` parses Apex declarations with a tree-sitter Apex parser. The parser is a
self-contained Go module, `github.com/glade-sh/apex-parser`, vendored into this
repository at `third_party/glade-apex-parser` and wired in through a `replace`
directive in `go.mod`:

```text
require github.com/glade-sh/apex-parser v0.1.0
replace github.com/glade-sh/apex-parser => ./third_party/glade-apex-parser
```

It is a separate Go module (its own `go.mod`) behind the `internal/apexast`
adapter, but vendored in-repo so every build (local, CI, Docker, DigitalOcean) is
hermetic with no external checkout or registry. The adapter keeps the grammar
dependency isolated, so the underlying parser can change without rippling through
the codebase. If the module is ever published to `github.com/glade-sh/apex-parser`,
the `replace` can point at the published module and the vendored copy removed.

## Scope

The parser drives `internal/apexast` declaration extraction through a small,
stable API:

- `type Parser`
- `NewParser`
- `ParseFile`
- `ParseSource`

It extracts the structural model used by project loading, symbol indexing,
semantic analysis, LSP, watch, replay, and test discovery:

- classes, interfaces, enums, and triggers
- methods, constructors, fields, properties, initializers, and nested types
- modifiers, parameters, accessors, trigger object names, and trigger events
- source ranges and parser diagnostics

It does not execute Apex (the VM has its own anonymous-Apex parser) and does not
provide formatting.

## Identifier validation

The parser validates Apex declaration identifiers against Salesforce's
case-insensitive naming rules. It rejects all 121 Salesforce reserved words in
non-method source identifier contexts with `APEXPARSE002`, while preserving
Salesforce's contextual exception that permits most reserved words as method
names. It also reports `APEXPARSE003` for invalid identifier shapes and names
longer than 255 characters.

The check covers type, constructor, field, property, parameter, local,
enhanced-for, catch, trigger, and enum-constant declarations. Schema and API
references such as `Invoice__c` use their own naming contract and do not receive
source-identifier shape diagnostics.

See [Apex Language Compatibility](APEX_LANGUAGE_COMPATIBILITY.md) for the full
reserved-word table, command propagation, and the broader 400-row language-rule
evidence boundary.

## CGO requirement

Real parsing requires CGO: the parser uses a generated tree-sitter Apex grammar
written in C. With `CGO_ENABLED=1` (the default for local `go build`/`go test` and
the playground image), parsing works. With `CGO_ENABLED=0` the module still
compiles, but parsing a declaration returns diagnostic code `APEXPARSECGO` instead
of a parse result.

Build and run with CGO enabled:

```sh
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go build -o glade ./cmd/glade
```

The playground container (`Dockerfile`) builds with `CGO_ENABLED=1` and runs on a
glibc base image. `scripts/release-build.sh` also builds with CGO enabled and
checks `glade doctor --json` for `"parserOK": true` before writing a release
archive. That assertion checks parser availability, not complete project
readiness.

## Performance

Benchmark numbers on Apple M2 Pro:

```text
BenchmarkParseClass-12    ~1.5 ms/op    ~151 KB/op    ~2,170 allocs/op
BenchmarkBuildIndex-12    ~3.4 ms/op    ~455 KB/op    ~6,811 allocs/op
```

## Validation

Run the parent module suite (the nested parser module needs a separate run):

```sh
CGO_ENABLED=1 go test ./...
```

Run parser and index benchmarks:

```sh
go test ./internal/apexast ./internal/typesys -run '^$' -bench=. -benchmem
```

Run the vendored parser module's own tests:

```sh
cd third_party/glade-apex-parser
CGO_ENABLED=1 go test -count=1 ./...
CGO_ENABLED=0 go test -count=1 ./...
```

Run project-level smoke checks from `glade`:

```sh
go run ./cmd/glade check --project path/to/project --json
# Requires an installed or locally linked compat plugin.
glade plugins install @glade/compat
glade compat replay testdata/replay/selector-service-domain
scripts/smoke.sh
```

After changing the vendored parser, run its module tests above and the affected
parent packages, including `internal/apexast`. Run `go mod tidy` only when
dependencies changed, in the module whose dependencies changed.

## Notes

The parser preserves the `void(` method-name edge case with length-preserving
normalization, which keeps source offsets stable while allowing Apex methods named
`void`.

Downstream code should depend on diagnostic severity, code, file, range, and
excerpt rather than exact parser wording.
