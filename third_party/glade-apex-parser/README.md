# apex-parser

Tree-sitter based Salesforce Apex declaration parser for `glade`.

This module provides declaration extraction for `glade`. It vendors the generated Apex parser from `aheber/tree-sitter-sfapex` and uses the official `github.com/tree-sitter/go-tree-sitter` Go binding.

## Scope

The parser extracts structural declarations:

- classes, interfaces, enums, and triggers
- methods, constructors, fields, properties, initializers, and nested types
- modifiers, parameters, accessors, ranges, trigger object names, and trigger events

It does not execute Apex and does not replace `glade`'s VM parser.

The parser preserves one current `glade` edge case: Apex methods named `void`.
Bare calls such as `return void(requests);` are normalized before parsing with
a same-length sentinel so source offsets remain stable.

## Package

The public Go package name is `apexast` to match `glade`'s current internal package vocabulary.

```go
import apexast "github.com/glade-sh/apex-parser"

parser := apexast.NewParser()
file := parser.ParseSource("Hello.cls", "public class Hello {}")
```

## Upstream grammar

Vendored from:

`aheber/tree-sitter-sfapex` commit `94f1e55064e1136738c30ea9a326e63406107a35`

## Benchmark

On an Apple M2 Pro, the synthetic 40-method class benchmark measured about 1.5 ms/op with roughly 136 KB/op and 2,127 allocs/op.

Against `glade/example-projects`, the parser handled 3,099 Apex files with zero failures in about 3.5 seconds, or about 1.13 ms/file.

## Build requirements

Real parsing requires CGO because this module uses the generated tree-sitter Apex parser written in C. With `CGO_ENABLED=0`, the module still builds, but parsing returns a diagnostic with code `APEXPARSECGO`.

## Release checks

```sh
scripts/check.sh
```

The CI workflow runs CGO tests on Linux and macOS and verifies the no-CGO fallback on Linux.
