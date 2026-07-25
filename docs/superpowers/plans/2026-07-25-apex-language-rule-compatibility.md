# Apex Language Rule Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development or superpowers:executing-plans to
> implement this plan packet-by-packet. Keep checkbox state current. Do not
> start a dependent packet before its prerequisite is green.

**Goal:** Make Glade reject Salesforce-invalid Apex across parsing, checking,
testing, execution, watch, and LSP surfaces without rejecting programs that the
Salesforce compiler accepts.

**Architecture:** Treat compiler compatibility as a checked set of
Salesforce-oracle contracts. Keep lexical rules at the parser boundary,
declaration and owner rules in cohesive semantic passes, and expression/query
rules in body analysis. Preserve source facts that validation needs instead of
recovering them from normalized strings. Put the rule ledger and scratch-org
runner in the first-party compat plugin sourced from `glade-tools`; product
code must not depend on that repository.

**Tech Stack:** Go, Tree-sitter Apex AST, Glade type index and semantic
analyzer, VM IR, SOQL/SOSL parsers, Salesforce Tooling API, Salesforce CLI,
Glade first-party compat plugin.

**Evidence:** `docs/2026-07-25-apex-language-rule-coverage-audit.md` records 222
raw scratch-oracle programs: 196 Salesforce rejects, 20 matching Glade rejects,
176 Salesforce-reject/Glade-pass mismatches, and 26 Salesforce/Glade accepts.
Those counts are programs, not unique rules.

---

## Non-negotiable oracle controls

Keep positive regression rows for every Salesforce-accepted program. In
particular, do not introduce blanket compiler errors for:

- local `final` reassignment or a `final` property;
- `super.method()` outside an override;
- transient static fields or transient locals;
- `System.LoggingLevel` in a webservice signature;
- global or value-returning `@IsTest`;
- a future method in a Batchable class or a future-to-future call;
- static trigger locals or ContentVersion delete triggers;
- a public static method satisfying an interface;
- either scratch-proven `Database.Batchable<T>` scope-variance form;
- `System.runAs` outside a test method;
- multiple HTTP verb annotations on one method;
- duplicate SOSL `RETURNING` objects or grouping Account `Description`;
- current `WITH SECURITY_ENFORCED`, getter-mutation, and Iterable
  `instanceof` controls at their tested API versions.

Runtime failure is not proof of a compiler rule. Documentation is not allowed
to override a current scratch-org compilation result.

## Execution order

- Packet 0 can proceed independently in `/Users/matt/Dev/glade-tools`.
- Packet 1 is a product prerequisite for every sema-owned rule.
- Packet 2 is the first product correctness priority.
- Packets 3-8 are independently landable after Packet 1.
- Packet 9 precedes Packets 10-12.
- Packet 11 precedes any version-, package-, or preview-gated enforcement.
- Packet 13 can proceed after Packet 1; reuse Packet 7 expression typing.
- Packet 14 closes each affected consumer and supplies the release gate.

---

### Task 0: Add a checked rule ledger and scratch comparator

**Repository:** `/Users/matt/Dev/glade-tools`

**Files:**

- Create: `internal/apexrules/model.go`
- Create: `internal/apexrules/catalog.go`
- Create: `internal/apexrules/tooling.go`
- Create: `internal/apexrules/glade.go`
- Create: `internal/apexrules/compare.go`
- Create: `internal/apexrules/apexrules_test.go`
- Create: `docs/fixtures/apex-language-rules.json`
- Modify: `internal/toolcli/compat_command.go`
- Create: `internal/toolcli/apex_rules_command_test.go`
- Modify: `plugins/compat/plugin.json`

- [ ] **Step 1: Define and validate the catalog schema**

Use explicit states so runtime, package-history, preview, and unresolved rows
are not silently omitted:

```go
type Rule struct {
	ID           string       `json:"id"`
	Area         string       `json:"area"`
	DocsPath     string       `json:"docsPath"`
	DocsLines    string       `json:"docsLines"`
	APIVersion   float64      `json:"apiVersion"`
	SourceKind   string       `json:"sourceKind"`
	Source       string       `json:"source"`
	Dependencies []SourceFile `json:"dependencies,omitempty"`
	Oracle       Outcome      `json:"oracle"`
	Owner        string       `json:"owner"`
	Status       string       `json:"status"`
	ProductTest  string       `json:"productTest,omitempty"`
}
```

`Validate` must reject duplicate IDs, missing evidence, unknown outcomes, and a
`supported` row without a product test.

- [ ] **Step 2: Add Salesforce and Glade runners**

`RunSalesforce` creates disposable Tooling API `ApexClass` or `ApexTrigger`
records, records accept/reject plus compiler problems, and deletes accepted
records. It must never print access tokens.

`RunGlade` creates an isolated SFDX project and invokes a supplied Glade
binary. `Compare` compares accept/reject outcome, not exact wording.

- [ ] **Step 3: Seed all audit probes and controls**

Import both audit matrices. Mark every row as one of:

- `supported`;
- `confirmed-gap`;
- `runtime-only`;
- `package-history-pending`;
- `preview-disabled`;
- `oracle-pending`.

- [ ] **Step 4: Add the compat-plugin command**

```text
glade-tools apex-rules compare \
  --catalog <path> \
  --target-org <alias> \
  --glade-bin <path> \
  --json
```

Base Glade must not gain this maintenance command or import `glade-tools`.

- [ ] **Step 5: Verify**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/apexrules ./internal/toolcli
go run ./cmd/glade-tools apex-rules validate \
  --catalog docs/fixtures/apex-language-rules.json
```

---

### Task 1: Make `glade test` fail closed on semantic diagnostics

**Files:**

- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/runner_test.go`
- Modify: `internal/gladecli/cli_test.go`

- [x] **Step 1: Add RED runner tests**

Add:

- `TestRunCasesContextRejectsSemanticDiagnosticsBeforeExecution`;
- `TestRunCasesContextSynthesizesProjectCompileCaseWhenNoTestsDiscovered`;
- a CLI test whose invalid method body produces one compile error and zero
  executed/passed tests.

- [x] **Step 2: Run the focused tests and confirm RED**

```bash
go test ./internal/apextest -run 'Semantic|Compile'
go test ./internal/gladecli -run 'Semantic.*Test|Compile.*Test'
```

- [x] **Step 3: Run semantic analysis before discovery/runtime compilation**

At the start of `RunCasesContext`, obtain the same non-performance semantic
diagnostics used by `glade check`. Convert error-severity diagnostics into
`compileErrorRun` before cache lookup or method compilation. If no test case is
discoverable, retain the synthetic `project compile` case.

Do not run a second divergent validator. If repeated analysis is measurable,
cache the result by the existing source digest identity.

- [x] **Step 4: Preserve declared test method shape**

Stop passing hard-coded `"void"` and `[]string{"static"}` to
`compileProjectMethod`. Use the indexed member type and modifiers. This must
keep the Salesforce-accepted value-returning `@IsTest` control runnable.

- [x] **Step 5: Verify GREEN**

```bash
go test ./internal/apextest
go test ./internal/gladecli -run 'Test.*Test|Semantic|Compile'
```

---

### Task 2: Analyze trigger bodies and trigger capabilities

**Files:**

- Modify: `internal/typesys/symbols.go`
- Modify: `internal/typesys/symbols_test.go`
- Modify: `internal/typesys/workspace_sources.go`
- Modify: `internal/typesys/workspace_sources_test.go`
- Modify: `internal/typesys/incremental_equivalence_test.go`
- Modify: `internal/sema/sema.go`
- Modify: `internal/sema/sema_checks.go`
- Modify: `internal/sema/type_members.go`
- Create: `internal/sema/trigger_contracts_test.go`
- Modify: `internal/schema/schema.go`
- Modify: `internal/schema/schema_test.go`
- Modify: `internal/gladecli/cli_test.go`

- [x] **Step 1: Add RED trigger regressions**

Cover:

- `Integer value = 'wrong';`;
- an unknown call and unknown local type;
- duplicate canonical event names;
- a trigger on known non-triggerable `ApexClass`;
- valid `Trigger.new`, `Trigger.oldMap`, and handler calls;
- Salesforce-accepted static trigger local and ContentVersion delete controls;
- full-build/incremental source retrieval and changed-body equivalence.

- [x] **Step 2: Preserve trigger source occurrence metadata**

Extend `TriggerSymbol` with the source metadata required by
`BuildArtifacts.SourceForTrigger` and `semaSources.normalizedForTrigger`.
`TriggerSymbol.Range` already spans the declaration; do not add a redundant
body parser.

- [x] **Step 3: Add trigger body work items**

Implement `buildSemaTriggerBodyWorkItems` and
`Analyzer.checkTriggerBodiesWithView`. Extract the body using
`extractBodyForSema` and call the existing `checkBodyText` with a synthetic
void trigger member plus the legal trigger scope.

Invoke trigger body analysis with the same type-member view and constructability
model used for method bodies.

- [x] **Step 4: Validate event identity and object capability**

Extend `checkTriggers` to reject duplicate normalized events. Add explicit
triggerability/event capability to schema objects when the describe source
provides it. Distinguish "known object" from "object supports triggers".

- [x] **Step 5: Verify**

```bash
go test ./internal/typesys -run 'Trigger|SourceForTrigger|Incremental'
go test ./internal/schema -run 'Trigger'
go test ./internal/sema -run 'TriggerBody|TriggerEvent|TriggerObject'
go test ./internal/gladecli -run 'Trigger.*Check|Trigger.*Test'
```

---

### Task 3: Make same-owner duplicates compiler errors

**Files:**

- Modify: `internal/typesys/symbols.go`
- Modify: `internal/typesys/symbols_test.go`
- Create: `internal/sema/declaration_contracts.go`
- Create: `internal/sema/declaration_contracts_test.go`
- Modify: `internal/sema/sema.go`

- [ ] **Step 1: Add RED identity tests**

Cover same-owner class/interface names, inner type equal to an ancestor,
case-insensitive duplicate fields/properties, duplicate constructors, duplicate
methods, methods differing only by return type, and case-only method names.
Keep a cross-file workspace ambiguity warning control.

- [ ] **Step 2: Preserve structural identity**

Add explicit `LocalName`, `OwnerName`, and `NestingDepth` to `TypeSymbol`.
Populate them in `typeSymbolsFromDeclaration`; do not infer ownership later by
splitting qualified names.

- [ ] **Step 3: Split duplicate severity**

Make an unambiguous same-file/same-owner collision an error. Preserve warning
severity only for intentional cross-file/workspace ambiguity.

- [ ] **Step 4: Add member identity validation**

Normalize names and parameter types case-insensitively. Ignore return type when
deciding method signature identity.

- [ ] **Step 5: Verify**

```bash
go test ./internal/typesys -run 'Duplicate|Owner|Nesting'
go test ./internal/sema -run 'DuplicateDeclaration|DuplicateMember|InnerType'
```

---

### Task 4: Complete identifier shape, length, rename, and local scope

**Files:**

- Create: `third_party/glade-apex-parser/identifier_contracts.go`
- Modify: `third_party/glade-apex-parser/parser.go`
- Modify: `third_party/glade-apex-parser/parser_test.go`
- Modify: `internal/refactor/rename.go`
- Modify: `internal/refactor/rename_test.go`
- Modify: `internal/sema/body_ir.go`
- Modify: `internal/sema/type_members.go`
- Create: `internal/sema/scope_contracts_test.go`

- [ ] **Step 1: Add RED identifier fixtures**

Cover `_value`, `value_`, `value__name`, 255/256 characters, every declaration
context, duplicate parameters, duplicate locals, parent-block redeclaration,
loop/catch collisions, and valid sibling scopes.

- [ ] **Step 2: Add one context-aware parser validator**

Keep the existing 121-word reserved table and method-name exceptions. Add the
Salesforce-proven start/end, consecutive-underscore, and length checks. Do not
apply source-identifier rules to schema/API reference names.

- [ ] **Step 3: Share validation with rename**

Reject a rename target that would produce invalid Apex before emitting edits.

- [ ] **Step 4: Add Apex local declaration rules**

Teach `collectBodyScopes` or `irSemaScope.declare` to retain declaration origin
and diagnose Salesforce-invalid redeclaration across parameter/local/nested
block/loop/catch scopes.

- [ ] **Step 5: Verify**

```bash
(cd third_party/glade-apex-parser && go test ./...)
go test ./internal/refactor -run 'Identifier|Rename'
go test ./internal/sema -run 'Identifier|DuplicateLocal|Shadow|Scope'
```

---

### Task 5: Add declaration, modifier, signature, and constructor contracts

**Files:**

- Modify: `third_party/glade-apex-parser/model.go`
- Modify: `third_party/glade-apex-parser/parser.go`
- Modify: `third_party/glade-apex-parser/parser_test.go`
- Modify: `internal/apexast/model.go`
- Modify: `internal/apexast/parser.go`
- Modify: `internal/apexast/parser_test.go`
- Modify: `internal/typesys/symbols.go`
- Modify: `internal/typesys/incremental_equivalence_test.go`
- Modify: `internal/sema/declaration_contracts.go`
- Modify: `internal/sema/declaration_contracts_test.go`
- Modify: `internal/sema/body_ir.go`

- [ ] **Step 1: Preserve body state and enum members**

Add `HasBody` to declarations/member symbols. Retain enum body members instead
of dropping them. Prove full and incremental indexes remain equivalent.

- [ ] **Step 2: Add RED declaration matrices**

Cover:

- public/global top-level visibility with the `@IsTest` exception;
- one allowed inner level and illegal deeper nesting;
- illegal `static`/`final` class forms and `abstract virtual`;
- illegal abstract/virtual/override method combinations;
- inner static initializers and mutually exclusive sharing modifiers;
- 32/33 method and constructor parameter boundaries;
- method/interface body consistency;
- accessor visibility and duplicate getters/setters;
- enum user method/constructor syntax;
- user generic class declarations;
- ambiguous `null` overload calls.

- [ ] **Step 3: Implement `checkDeclarationContracts`**

Use owner, kind, nesting, normalized modifiers, `HasBody`, and effective API
version. Keep grammar-shaped failures at the parser boundary and semantic
owner/modifier combinations in sema.

- [ ] **Step 4: Validate constructor chaining**

In body IR, require `this(...)` or `super(...)` to be the first instruction and
at most one explicit chain call. Resolve an implicit `super()` to an accessible
no-argument constructor.

- [ ] **Step 5: Diagnose ambiguous overload resolution**

When two incomparable overloads are equally applicable to `null`, emit a
compiler error. Retain a positive row where a most-specific overload exists.

- [ ] **Step 6: Verify**

```bash
(cd third_party/glade-apex-parser && go test ./... -run 'Declaration|Enum|Body')
go test ./internal/typesys -run 'Declaration|Incremental'
go test ./internal/sema -run 'DeclarationContract|Modifier|ParameterLimit|Constructor|Accessor|Overload'
```

---

### Task 6: Close inheritance, interface generic, and namespace authority gaps

**Files:**

- Modify: `internal/typesys/symbols.go`
- Modify: `internal/typesys/standard_symbols.go`
- Modify: `internal/typesys/standard_symbols_test.go`
- Modify: `internal/sema/sema.go`
- Modify: `internal/sema/body_calls.go`
- Modify: `internal/sema/namespace_precedence_test.go`
- Create: `internal/sema/inheritance_contracts_test.go`

- [ ] **Step 1: Add RED inheritance fixtures**

Cover class-implements-class, class-extends-interface,
interface-extends-class, non-virtual superclass/method, missing `override`,
visibility narrowing, transitive interface requirements, raw
`Database.Batchable`, missing public/global implementation visibility, and
wrong `Iterator<T>`/`Iterable<T>` returns.

Keep positive controls for a public static interface implementation and both
scratch-proven Batchable execute-scope variance forms.

- [ ] **Step 2: Preserve declaration-kind-aware inheritance**

Represent a class superclass, implemented interfaces, and interface parents
without conflating their target kinds.

- [ ] **Step 3: Substitute generic interface arguments**

`collectRequiredMethods` must instantiate `Iterator<T>`, `Iterable<T>`, and
other supported platform generic requirements before signature and return
comparison. `hasConcreteMethodSignature` must enforce public/global visibility
but must not reject a public static implementation that the oracle accepts.

- [ ] **Step 4: Make project namespaces authoritative**

Once `Database` resolves to a project type, a missing `Database.query` must not
fall back to `System.Database`. Apply the same authority rule in IR call return
inference, platform-call diagnostics, and ordinary method-call resolution.

- [ ] **Step 5: Verify**

```bash
go test ./internal/typesys -run 'Iterator|Iterable|Batchable'
go test ./internal/sema -run 'Inheritance|Interface|Generic|Batchable|NamespacePrecedence|PlatformFallback'
```

---

### Task 7: Add source-type, property, expression, literal, and collection contracts

**Files:**

- Create: `internal/sema/type_contracts.go`
- Create: `internal/sema/type_contracts_test.go`
- Modify: `internal/sema/body_ir.go`
- Modify: `internal/sema/body_calls.go`
- Modify: `internal/sema/type_members.go`
- Modify: `internal/vm/compiler.go`
- Modify: `internal/vm/vm_test.go`

- [ ] **Step 1: Add RED source-type and collection tests**

Cover source-level `Currency`, raw List/Map construction, wrong generic arity,
the scratch-proven collection-depth boundary, unsupported user generic types,
and an unsuffixed integer outside Integer range.

Keep schema metadata `Currency` and `AnyType` behavior as separate controls.

- [ ] **Step 2: Add recursive expression contract tests**

Cover `!1`, Boolean ordering, String multiplication, incompatible cast,
impossible and always-true `instanceof`, incompatible coalescing, safe
navigation on a static receiver, safe navigation on assignment LHS, scientific
notation, and valid numeric widening/casts.

Even matched hard rejects need owning-rule tests; do not rely on a later return
or assignment mismatch.

- [ ] **Step 3: Preserve property capabilities and receiver mode**

Member resolution must distinguish field, get-only property, set-only
property, static receiver, instance receiver, and static method context.
Assignments require write capability; reads require read capability.

- [ ] **Step 4: Implement source-type and expression validators**

Add a recursive `checkIRExpressionContract` using the existing type inference
and assignability model. Validate operand families, cast viability,
`instanceof`, common coalescing type, safe-navigation placement, and literal
range/syntax.

- [ ] **Step 5: Verify**

```bash
go test ./internal/vm -run 'Literal|Expression'
go test ./internal/sema -run 'TypeContract|Operator|Cast|Instanceof|Coalesce|SafeNavigation|PropertyCapability|Collection|NumericLiteral|StaticContext'
```

---

### Task 8: Add statement, switch, and exception contracts

**Files:**

- Modify: `internal/sema/body_ir.go`
- Modify: `internal/sema/sema_text.go`
- Create: `internal/sema/statement_contracts_test.go`

- [ ] **Step 1: Add RED/positive matrices**

Cover:

- legal and illegal `break`/`continue` nesting;
- Boolean, Decimal, Date, and legal switch selectors;
- duplicate canonical branch values and `when else` ordering;
- thrown non-Exception values;
- catch types, duplicate catches, and already-covered catch order;
- custom Exception suffix;
- unreachable instructions after unconditional termination.

- [ ] **Step 2: Thread statement context through IR checking**

Use an explicit context carrying loop and switch depth. Do not infer placement
from text after IR compilation.

- [ ] **Step 3: Use the type hierarchy for throw/catch coverage**

Validate assignability to `System.Exception` and compare catch coverage in
source order. Use existing type-distance helpers instead of name-only checks.

- [ ] **Step 4: Verify**

```bash
go test ./internal/sema -run 'StatementContext|SwitchContract|Throw|Catch|CustomException|Unreachable'
```

---

### Task 9: Preserve structured annotations and add one shared catalog

**Files:**

- Modify: `third_party/glade-apex-parser/model.go`
- Modify: `third_party/glade-apex-parser/parser.go`
- Modify: `third_party/glade-apex-parser/parser_test.go`
- Modify: `internal/apexast/model.go`
- Modify: `internal/apexast/parser.go`
- Modify: `internal/apexast/parser_test.go`
- Modify: `internal/typesys/symbols.go`
- Modify: `internal/typesys/incremental_equivalence_test.go`
- Create: `internal/apexlang/annotation_catalog.go`
- Create: `internal/apexlang/annotation_catalog_test.go`
- Modify: `internal/lsp/handler.go`
- Modify: `internal/lsp/handler_test.go`

- [ ] **Step 1: Add RED structured parsing tests**

Cover named/positional arguments, whitespace/comments, strings containing
spaces or `=`, exact ranges, unknown annotations, and unknown properties.

- [ ] **Step 2: Add annotation structures**

```go
type Annotation struct {
	Name      string
	Arguments []AnnotationArgument
	Range     diagnostic.Range
}

type AnnotationArgument struct {
	Name  string
	Value string
	Range diagnostic.Range
}
```

Add `Annotations` to declarations, parameters/accessors where supported,
`TypeSymbol`, and `MemberSymbol`. Preserve annotation strings in `Modifiers`
during migration, but all new validation must use the structured form.

- [ ] **Step 3: Add the product annotation catalog**

Catalog current Apex annotations, supported target kinds, property names/value
kinds, positional forms, API gates, and preview state. Unknown annotations
must hard-fail.

- [ ] **Step 4: Drive LSP completion from the catalog**

Replace the four hard-coded completion items. Exclude disabled preview entries.

- [ ] **Step 5: Verify**

```bash
(cd third_party/glade-apex-parser && go test ./... -run Annotation)
go test ./internal/apexast ./internal/typesys -run Annotation
go test ./internal/apexlang ./internal/lsp -run Annotation
```

---

### Task 10: Implement stable annotation and test contracts

**Files:**

- Create: `internal/sema/annotation_contracts.go`
- Create: `internal/sema/annotation_contracts_test.go`
- Modify: `internal/sema/sema.go`
- Modify: `internal/sema/sema_checks.go`
- Modify: `internal/apextest/runner_test.go`

- [ ] **Step 1: Replace fragment checks with two passes**

`checkAnnotationContracts` must perform:

1. declaration-local target/property/value validation;
2. owner-level count, companion, visibility, signature, and overload
   validation.

- [ ] **Step 2: Cover confirmed test rules**

Validate `@IsTest` owner/kind/nesting/static/no-argument rules,
`SeeAllData`/`IsParallel`, `@TestSetup` cardinality and owner state, and
method-level `critical`/`testFor`.

Do not require a void return: Salesforce accepted the value-returning test
control.

- [ ] **Step 3: Cover other confirmed annotations**

Implement:

- AuraEnabled overload prohibition;
- future parameter shapes and allowed property schema;
- InvocableMethod count, visibility, static, parameter, and return shapes;
- InvocableVariable field-only, visibility, nonstatic, nonfinal contracts;
- JsonAccess values and targets;
- NamespaceAccessible target/owner/companion rules;
- RemoteAction public/global static;
- ReadOnly allowed exposed-method targets.

Future-to-future, Batchable/future, and non-test `System.runAs` remain
compile-time controls unless a later runtime packet owns them.

- [ ] **Step 4: Prove runner fail-closed behavior**

Invalid owner, nonstatic method, parameterized method, duplicate TestSetup, and
SeeAllData/TestSetup fixtures must execute zero tests.

- [ ] **Step 5: Verify**

```bash
go test ./internal/sema -run 'Annotation|IsTest|TestSetup|Future|AuraEnabled|Invocable|JsonAccess|NamespaceAccessible|RemoteAction|ReadOnly'
go test ./internal/apextest -run 'InvalidTest|TestSetup|Compile'
```

---

### Task 11: Add effective source API, package-history, and preview gates

**Files:**

- Modify: `internal/project/project.go`
- Modify: `internal/project/project_test.go`
- Modify: `internal/typesys/symbols.go`
- Modify: `internal/typesys/symbols_test.go`
- Modify: `internal/typesys/incremental_equivalence_test.go`
- Modify: `internal/sema/declaration_contracts.go`
- Modify: `internal/sema/annotation_contracts.go`
- Create: `internal/sema/version_contracts_test.go`

- [ ] **Step 1: Resolve effective per-source API versions**

Read companion `.cls-meta.xml` and `.trigger-meta.xml`, store the effective
version on type/trigger symbols, and fall back to
`Project.SourceAPIVersion`. Include the value in incremental/cache identity.

- [ ] **Step 2: Gate confirmed versioned rules**

Use the declaration's effective version for API 65 access requirements and
other rows only after paired oracle fixtures exist.

- [ ] **Step 3: Keep unavailable evidence explicit**

Released-package `@Deprecated` behavior remains
`package-history-pending` until package artifact/history input exists.
`@IntegrationTest` and `@TearDown` remain preview-disabled unless an explicit
feature state is available; API 67 alone is insufficient.

- [ ] **Step 4: Verify**

```bash
go test ./internal/project ./internal/typesys -run 'SourceAPIVersion|APIVersion|Incremental'
go test ./internal/sema -run 'APIVersion|VersionContract|Deprecated|IntegrationTest|TearDown'
```

---

### Task 12: Add REST and SOAP exposure contracts

**Files:**

- Create: `internal/sema/web_exposure_contracts.go`
- Create: `internal/sema/web_exposure_contracts_test.go`
- Modify: `internal/sema/sema.go`

- [ ] **Step 1: Add RED REST matrices**

Cover global/top-level RestResource ownership, required URL mapping, leading
slash, 255-character limit, terminal wildcard, global/static HTTP methods,
GET/DELETE arity, one method per verb, and the scratch-proven Set/Blob/Map
signature failures.

Keep the accepted multiple-HTTP-annotations control.

- [ ] **Step 2: Add RED SOAP matrices**

Cover top-level/global owner, static method, interface/inner placement,
same-name overload prohibition, Map/Set, and every additional wire type backed
by a scratch row. Keep `System.LoggingLevel` positive.

- [ ] **Step 3: Implement one exposed-type validator**

Share type-shape traversal between REST and SOAP, but parameterize their
different allowlists. Do not ban nested REST collections or cyclic user types
based only on XML/request-time behavior.

- [ ] **Step 4: Verify**

```bash
go test ./internal/sema -run 'Webservice|RestResource|Http|RemoteAction|AuraEnabled|ExposedType'
```

---

### Task 13: Make SOQL, SOSL, binds, and DML fail closed

**Files:**

- Modify: `internal/soql/parser.go`
- Modify: `internal/soql/soql.go`
- Modify: `internal/soql/soql_test.go`
- Modify: `internal/sosl/parser.go`
- Modify: `internal/sosl/parser_test.go`
- Create: `internal/sema/query_contracts.go`
- Create: `internal/sema/query_contracts_test.go`
- Modify: `internal/sema/sema_checks.go`
- Modify: `internal/sema/body_ir.go`
- Modify: `internal/schema/schema.go`
- Modify: `internal/schema/schema_test.go`

- [ ] **Step 1: Add RED query-shape fixtures**

Cover the confirmed rows:

- non-grouped aggregate select field;
- aggregate `LIMIT` without `GROUP BY`;
- ROLLUP over three fields;
- self semi-join;
- TYPEOF plus grouping;
- ORDER BY plus FOR UPDATE;
- unbounded FIELDS(ALL) in Apex;
- SOSL without RETURNING.

Keep accepted duplicate SOSL RETURNING objects and Account Description grouping
as positive controls. Add an owning-rule test for aggregate-in-WHERE even
though the current fixture already hard-rejects indirectly.

- [ ] **Step 2: Stop discarding parser errors**

Convert `soql.Parse`/`sosl.Parse` failures in
`querySemanticsChecker.checkFile` into stable diagnostics. Before enabling
fail-closed behavior, run the positive SOQL/SOSL corpus so valid unsupported
syntax is not misclassified.

- [ ] **Step 3: Add query-clause contracts**

Preserve the ordered query structure needed to validate aggregate/grouping,
HAVING, semi-join, TYPEOF, locking, and FIELDS combinations. Reuse this
validator from local execution instead of maintaining a second rule set.

- [ ] **Step 4: Resolve bind expressions against Apex scope**

Walk inline query and SOSL binds, diagnose missing variables, and apply
field/clause-compatible type checks. Cover Name/Integer, LIMIT/OFFSET numeric,
and SOSL numeric bind rows.

- [ ] **Step 5: Add field capability metadata only from evidence**

Carry filterable/groupable/sortable/aggregate capabilities when describe data
provides them. Never infer a capability from a field name.

- [ ] **Step 6: Add `checkIRDMLContract`**

For `ir.OpDML`, require operation-specific sObject or sObject-collection
operands. Validate upsert external-ID/idLookup keys and compatible merge
master/duplicate types before runtime.

- [ ] **Step 7: Verify**

```bash
go test ./internal/soql ./internal/sosl
go test ./internal/schema -run 'Capability|ExternalID|IDLookup'
go test ./internal/sema -run 'QueryContract|QueryParse|SOQL|SOSL|Bind|DMLContract'
```

---

### Task 14: Close anonymous Apex, watch, LSP, CLI, and release proof

**Files:**

- Modify: `internal/sema/sema.go`
- Create: `internal/sema/anonymous.go`
- Create: `internal/sema/anonymous_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/watch/refgraph_test.go`
- Modify: `internal/lsp/handler_test.go`
- Modify: `internal/refactor/rename_test.go`

- [ ] **Step 1: Add shared anonymous semantic analysis**

Implement `sema.AnalyzeAnonymous(index, source)` using a synthetic static void
context and the project type/member model. Translate wrapper offsets back to
the original source.

- [ ] **Step 2: Gate execute-anonymous before VM execution**

When a project is supplied, load its index and run the shared anonymous
analysis before `vm.CompileAnonymous`/execution.

- [ ] **Step 3: Add representative consumer proofs**

Use at least one rule owned by each layer:

- parser: reserved/invalid identifier;
- index/declaration: same-owner duplicate;
- body sema: invalid operator/property;
- trigger: assignment error;
- annotation: unknown property;
- query/DML: missing bind or invalid DML operand.

Prove the affected behavior in `parse`, `check`, `test`, `exec`, watch rebuild,
LSP published diagnostics, and rename/completion where applicable.

- [ ] **Step 4: Run focused product gates**

```bash
cd /Users/matt/Dev/glade
(cd third_party/glade-apex-parser && go test ./...)
go test ./internal/apexast
go test ./internal/typesys
go test ./internal/sema
go test ./internal/apextest
go test ./internal/gladecli
go test ./internal/lsp
go test ./internal/watch
go test ./internal/refactor
```

- [ ] **Step 5: Run broad product gates**

```bash
go test ./...
scripts/smoke.sh
```

- [ ] **Step 6: Replay the checked oracle**

```bash
go build -o /private/tmp/glade-apex-language-candidate ./cmd/glade

cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools apex-rules compare \
  --catalog docs/fixtures/apex-language-rules.json \
  --target-org <scratch-alias> \
  --glade-bin /private/tmp/glade-apex-language-candidate \
  --json
```

Require zero accept/reject mismatches for rows marked `supported`.

- [ ] **Step 7: Run the stronger compatibility gate**

Run NU after the final high-risk semantic packet and again before release. Do
not trade Salesforce correctness for corpus pass rate or performance.

---

## Completion criteria

This program is complete only when:

- every confirmed audit row exists in the checked ledger;
- every `supported` row names a product regression;
- the scratch comparator has zero supported-row mismatches;
- `check`, `test`, and affected consumers fail closed;
- all Salesforce-accepted controls remain accepted;
- package-history, runtime-only, preview, and oracle-pending rows remain
  explicitly classified;
- focused tests, `go test ./...`, `scripts/smoke.sh`, and NU are green.
