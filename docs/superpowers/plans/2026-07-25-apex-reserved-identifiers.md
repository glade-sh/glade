# Apex Reserved Identifiers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject every Salesforce-reserved Apex word as an identifier in `glade parse`, `glade check`, and `glade test`, while preserving Salesforce's context-sensitive method-name behavior.

**Architecture:** Validate identifier declaration nodes immediately after Tree-sitter parsing. Use the 121-word API 66 reserved table verified against a disposable Salesforce scratch org, but exempt method declarations from the contextual/basic-name subset exactly as Salesforce's Jorje validator does. Preserve parsed symbols alongside reserved-name diagnostics so all consumers share one result, and make the test runner fail closed on index errors before execution.

**Tech Stack:** Go, Tree-sitter Apex AST, Glade type index, Glade test runner, Salesforce CLI API 66 behavioral probes.

---

### Task 1: Add Salesforce reserved-word parser regressions

**Files:**
- Modify: `third_party/glade-apex-parser/parser_test.go`

- [x] **Step 1: Add a failing all-keywords variable-name test**

Add the complete API 66 table and require every entry to produce an error:

```go
func TestParseRejectsEverySalesforceReservedVariableName(t *testing.T) {
    reserved := strings.Fields(`abstract activate and any array as asc autonomous
        begin bigdecimal blob boolean break bulk by byte case cast catch char class
        collect commit const continue currency date datetime decimal default delete
        desc do double else end enum exception exit export extends false final finally
        float for from global goto group having hint if implements import in inner
        insert instanceof int integer interface into join like limit list long loop
        map merge new not null nulls number object of on or outer override package
        parallel pragma private protected public retrieve return rollback select set
        short sObject sort static string super switch synchronized system testmethod
        then this throw time transaction trigger true try undelete update upsert using
        virtual void webservice when where while`)
    parser := NewParser()
    for _, word := range reserved {
        t.Run(word, func(t *testing.T) {
            source := fmt.Sprintf("class Probe { void run() { String %s = 'x'; } }", word)
            if file := parser.ParseSource("Probe.cls", source); !file.HasErrors() {
                t.Fatalf("%q was accepted as a variable name", word)
            }
        })
    }
}
```

- [x] **Step 2: Add context and diagnostic-range tests**

Cover `currency` as a field, local, parameter, enhanced-for variable, and catch variable. Assert code `APEXPARSE002`, Salesforce-compatible message `Identifier name is reserved: currency`, and a range selecting only the identifier. Keep `after`, `before`, `count`, `excludes`, `first`, `includes`, `last`, `order`, `sharing`, `with`, and `id` valid. Keep methods named `void` and `currency` valid; require an always-keyword method such as `trigger` to fail.

- [x] **Step 3: Run the parser tests and verify RED**

Run:

```bash
(cd third_party/glade-apex-parser && go test ./...)
```

Expected: FAIL because `currency` and other contextual/basic reserved names currently produce no diagnostics.

### Task 2: Implement parser-boundary validation and preserve symbols

**Files:**
- Create: `third_party/glade-apex-parser/reserved_identifiers.go`
- Modify: `third_party/glade-apex-parser/parser.go`
- Modify: `internal/typesys/symbols.go`

- [x] **Step 1: Add the case-insensitive Salesforce sets**

Define:

```go
var salesforceReservedIdentifiers = wordSet(`...all 121 API 66 reserved words...`)
var salesforceAlwaysKeywordIdentifiers = wordSet(`trigger insert update upsert delete undelete merge new for select`)
```

`isReservedIdentifier(name, method)` must reject all 121 names outside method declarations. For methods, reject only the always-keyword set and let Tree-sitter continue to reject syntax keywords. This preserves the Salesforce-proven `void(...)` method surface.

- [x] **Step 2: Walk declaration-name nodes**

Recursively inspect Tree-sitter nodes that introduce names: types, triggers, constructors, methods, fields/properties/local declarators, formal parameters, enhanced-for variables, catch variables, and enum constants. Emit:

```go
Diagnostic{
    Severity: Error,
    Code:     "APEXPARSE002",
    Message:  "Identifier name is reserved: " + name,
    File:     path,
    Range:    &identifierRange,
    Excerpt:  excerpt(source, identifierRange.Start.Line),
}
```

Append these diagnostics in both `ParseSource` and `ParseSourceAST` after the Tree-sitter syntax diagnostic.

- [x] **Step 3: Preserve declarations for reserved-name diagnostics**

In `projectSymbolFileFromPath`, return early only when a parser diagnostic other than `APEXPARSE002` exists. Continue building symbols for reserved-name files so test discovery can identify affected test methods while the index remains erroneous.

- [x] **Step 4: Run parser and type-index tests and verify GREEN**

Run:

```bash
(cd third_party/glade-apex-parser && go test ./...)
go test ./internal/typesys
```

Expected: PASS.

### Task 3: Fail tests closed and prove command behavior

**Files:**
- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/runner_test.go`
- Modify: `internal/gladecli/cli_test.go`

- [x] **Step 1: Add failing runner and CLI regressions**

Add a runner test with an indexed test class plus an `APEXPARSE002` error and require one compile error and zero executed tests. Add CLI tests using:

```apex
@IsTest
private class ReservedCurrencyTest {
    @IsTest static void failsCompilation() {
        String currency = 'USD';
        System.assertEquals('USD', currency);
    }
}
```

Require `check --json` to exit 1 with `APEXPARSE002`, and `test --json --no-cache` to exit 1 with one compile error and no pass.

- [x] **Step 2: Run focused tests and verify RED**

Run:

```bash
go test ./internal/apextest -run 'IndexDiagnostic|Reserved'
go test ./internal/gladecli -run 'Reserved'
```

Expected: FAIL because the test runner currently ignores index diagnostics.

- [x] **Step 3: Gate test execution on index errors**

At the start of `RunCasesContext`, discover cases, then convert error diagnostics to a deterministic compile error before runtime/cache compilation. If no cases were discovered, make `compileErrorRun` emit one synthetic `project compile` case so `glade test` cannot report a zero-test pass for invalid Apex.

- [x] **Step 4: Run focused and broad validation**

Run:

```bash
(cd third_party/glade-apex-parser && go test ./...)
go test ./internal/typesys
go test ./internal/apextest
go test ./internal/gladecli
go test ./...
scripts/smoke.sh
```

Then rerun the original local reproduction:

```bash
go run ./cmd/glade check --project tmp/repro --json --no-progress
go run ./cmd/glade test --project tmp/repro --class ReservedCurrencyTest --no-cache --no-progress --json
```

Expected: both commands exit 1; check reports `APEXPARSE002`; test reports compile error and does not execute the test.

## Follow-on audit

This reserved-identifier plan is complete. The corpus-wide compiler-rule audit
found additional, independently owned gaps; do not broaden this completed
packet into a sema rewrite.

Continue with:

`docs/superpowers/plans/2026-07-25-apex-language-rule-compatibility.md`

The follow-on plan preserves this packet's all-121-word coverage and adds
bounded work for trigger bodies, declaration and expression contracts,
structured annotations, REST/SOAP, tests, SOQL/SOSL/DML, and end-to-end
fail-closed behavior.
