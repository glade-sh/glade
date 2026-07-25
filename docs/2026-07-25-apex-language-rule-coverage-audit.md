# Apex Language Rule Coverage Audit

Date: 2026-07-25

## Outcome

The reserved-word fix closes the reported `currency` identifier bug, but it
does not close Apex compiler compatibility. A second, corpus-wide adversarial
pass found several validation layers that are absent or fail open.

Two scratch-org matrices produced these raw program results:

| Pass | Programs | Salesforce rejected | Glade also rejected | Salesforce rejected, Glade passed | Both accepted |
| --- | ---: | ---: | ---: | ---: | ---: |
| Initial targeted pass | 54 | 52 | 5 | 47 | 2 |
| Corpus-wide second pass | 168 | 144 | 15 | 129 | 24 |
| **Combined** | **222** | **196** | **20** | **176** | **26** |

These are program counts, not 176 unique language rules. Several programs test
different dimensions of one contract. A matched rejection also means only that
both compilers rejected the program; it does not mean Glade produced the same
diagnostic or rejected it for the same reason.

The second pass found no program that Salesforce accepted and Glade
hard-rejected. The Salesforce-accepted controls remain essential because the
documentation contains statements that the current compiler does not enforce
at compile time.

The highest-risk new finding is structural: Glade does not retain or analyze a
trigger body. An invalid assignment, unknown call, or other semantic error
inside a trigger can therefore pass `glade check` without any diagnostic.

## Source and method

The offline source corpus is:

`example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run`

The Apex Developer Guide `_version.json` reports:

- Atlas version 262.0;
- scrape time `2026-06-04T03:18:31.926555Z`;
- 546 successful pages;
- zero empty or failed pages.

The second pass searched all 546 Apex Guide pages and all 90 SOQL/SOSL Guide
pages. It reviewed compiler-enforceable statements across:

- identifiers, types, declarations, modifiers, constructors, and inheritance;
- expressions, operators, properties, collections, statements, and exceptions;
- annotations, tests, asynchronous declarations, REST, SOAP, and triggers;
- SOQL, SOSL, binds, field capabilities, and DML;
- versioned, package-history, preview, and runtime-only rules.

Every counted mismatch was compiled through the Salesforce Tooling API in a
disposable scratch org and checked with:

```bash
glade check --project <isolated-project> --json --no-progress
```

Most probes used API 66.0. Explicit version probes used API 40, 41, 42, 59, 60,
or 67 as named by the source row. Salesforce behavior is the oracle.
Documentation supplied candidates but never overrode observed compiler
behavior.

The final 168-row totals combine a completed 168-program run with an immediate
rerun of one corrected `Database.Batchable<T>` control. The corrected control
changed from a setup-related Salesforce rejection to acceptance, producing the
final 144/129/24 counts above.

## Ranked findings

### P0: Trigger bodies bypass semantic analysis

Salesforce rejected all of these while Glade reported no diagnostic:

| Probe | Salesforce |
| --- | --- |
| `Integer value = 'wrong';` in an Account trigger | `Illegal assignment from String to Integer` |
| `DefinitelyMissing.run();` in an Account trigger | `Variable does not exist: DefinitelyMissing` |
| duplicate `before insert` event | `Duplicate Trigger Usage: BEFORE INSERT` |
| trigger declared on `ApexClass` | `SObject type does not allow triggers: ApexClass` |

The code path explains the complete blind spot:

- `third_party/glade-apex-parser/parser.go` `treeSitterTrigger` keeps only the
  trigger name, object, declaration range, and events;
- `internal/apexast/model.go` has no trigger-specific body representation;
- `internal/typesys/symbols.go` carries only trigger metadata;
- `internal/sema/sema_checks.go` `checkTriggers` checks only whether the object
  name is known;
- `internal/sema/type_members.go` builds body work only from `index.Types`, not
  `index.Triggers`.

The existing trigger declaration range already spans the body, so closure does
not require a second parser. The body can be extracted with the same
`extractBodyForSema` path used for methods, then analyzed in a synthetic
trigger context. Trigger object metadata must also distinguish "known" from
"supports triggers" and eventually carry supported event capabilities.

Required proof:

- assignment, call, type-reference, query, DML, and control-flow errors inside
  triggers hard-fail;
- duplicate events hard-fail;
- known but non-triggerable objects hard-fail;
- legal `Trigger.new`, `Trigger.oldMap`, and handler calls remain valid;
- incremental indexing and watch mode recheck changed trigger bodies.

### P0: An unambiguous duplicate declaration can still pass the command

The initial pass confirmed that Salesforce rejects a class containing an inner
class and interface with the same case-insensitive name:

`Type name already in use: ProbeDuplicateTypeName.Item`

Glade emits `GLADETYPE001` but assigns `diagnostic.Warning` to every duplicate,
so the command passes. Same-owner duplicates must be errors. Cross-file
workspace ambiguity can retain warning behavior where it is intentional.

Required proof:

- same-file, same-owner type, field, property, method, and constructor identity
  rules hard-fail;
- return type does not create a valid overload;
- names compare case-insensitively;
- intentional cross-file ambiguity remains a warning.

### P1: Annotation text is not represented as a compiler contract

Salesforce rejected, and Glade accepted:

- unknown class and method annotations;
- unknown properties on `@IsTest`, `@AuraEnabled`, `@future`,
  `@RestResource`, `@InvocableMethod`, and `@InvocableVariable`;
- missing or malformed `@RestResource(urlMapping=...)`;
- invalid `@JsonAccess` values and targets;
- invalid `@NamespaceAccessible` targets and companion annotations;
- invalid test, invocable, remote, REST, and owner-level annotation contracts.

Representative Salesforce messages were:

- `Annotation does not exist: DefinitelyNotApex`;
- `No such property, DefinitelyNotApex, defined on this annotation: IsTest`;
- `HttpGet methods do not support parameters`.

`third_party/glade-apex-parser/parser.go` currently stores normalized annotation
text in the modifier string list. `internal/sema/sema.go` `modifierName` strips
everything after `(`. The argument names, values, and ranges are therefore
unavailable to semantic validation. `checkMemberAnnotations` in
`internal/sema/sema_checks.go` implements only fragments of a few contracts.

Closure requires three separate layers:

1. structured annotation AST/index data and a supported-name/property schema;
2. stable target, owner, visibility, signature, count, and companion rules;
3. API-version, package-history, and preview gates.

The shared product catalog should also replace the four hard-coded annotation
completion names in `internal/lsp/handler.go`.

### P1: Test declarations can be invalid while the test runner masks them

Salesforce rejected, and Glade accepted:

- `@IsTest` on a nested class, interface, or enum;
- a test method in a non-`@IsTest` owner;
- a nonstatic or parameterized test method;
- two `@TestSetup` methods;
- `@TestSetup` under `SeeAllData=true`;
- `SeeAllData=true` combined with `IsParallel=true`;
- method-level `critical` and `testFor` properties.

This is not only a sema gap:

- `internal/typesys/symbols.go` marks a method as a test without validating its
  owner;
- `internal/apextest/runner.go` `Discover` accepts that marker;
- `compileTestMethods` forcibly compiles every discovered method as
  `static void`, ignoring its declared return type, parameters, and modifiers;
- setup compilation accepts and executes multiple setup methods.

The runner must consume project semantic errors before discovery/execution.
Runner regressions must prove zero tests execute for an invalid test
declaration.

### P1: Declaration, constructor, inheritance, and interface contracts are broad gaps

The two passes confirmed missing checks for:

- top-level visibility, inner nesting, owner identity, and modifier
  combinations;
- static initializers in inner classes and multiple sharing modifiers;
- `abstract virtual` classes and illegal abstract/virtual/override static
  methods;
- methods or constructors over 32 parameters;
- overloads that differ only by return type and case-only duplicate members;
- `this(...)` or `super(...)` not first and missing implicit `super()`;
- class/interface target-kind mismatches;
- missing `override`, non-virtual inheritance, and transitive interface
  requirements;
- interface implementations without public/global visibility;
- erased generic interface requirements.

The generic-interface probes were direct:

```apex
public class Probe implements Iterator<String> {
    public Boolean hasNext() { return false; }
    public Integer next() { return 1; }
}
```

Salesforce required `String System.Iterator<String>.next()`. Glade accepted the
`Integer` return. The same mismatch exists for `Iterable<String>.iterator()`.
`collectRequiredMethods` and `hasConcreteMethodSignature` currently compare
erased requirements and do not validate implementation visibility.

Important oracle correction: a public static method did satisfy the tested
interface method. Do not add a blanket "interface implementations must be
instance methods" rule.

### P1: Expression, property, literal, and collection contracts are incomplete

Salesforce rejected, and Glade accepted:

- Boolean ordering;
- an incompatible cast between unrelated classes;
- an `instanceof` expression that is statically always true;
- safe navigation on a static receiver or on the left side of assignment;
- writing a get-only property and reading a set-only property;
- raw `List`/`Map` construction;
- wrong `List`/`Map` generic arity;
- nine nested collection levels;
- an unsuffixed integer outside Integer range;
- scientific notation syntax;
- unsupported user generic classes.

Some nearby programs are already hard-rejected by Glade, including `!1`,
String multiplication, incompatible null coalescing, enum method syntax, and
diamond construction. Those rows still need stable product regressions because
some are rejected indirectly rather than by the owning rule.

The expression pass must validate operand families recursively, not rely on a
later assignment or return mismatch. Property resolution must retain read/write
capability and static/instance receiver mode.

### P1: REST and SOAP exposure contracts are mostly absent

Confirmed REST mismatches include:

- `@HttpGet` parameters and duplicate methods for one verb;
- nonstatic HTTP methods;
- `Set`, `Blob`, and non-String-key `Map` parameters;
- missing leading slash, invalid wildcard, missing mapping, and mapping over
  255 characters;
- invalid `@RemoteAction` visibility/static mode.

Confirmed SOAP mismatches include:

- inner or interface webservice methods;
- nonstatic methods and non-global owners;
- `Map` parameters and `Set` returns;
- same-name webservice overloads.

Wire-type validation must use Salesforce-proven compile-time rows. Nested REST
collections and cyclic user types can be request-format/runtime concerns, so a
blanket recursive ban would be incorrect.

### P1: SOQL, SOSL, binds, and DML fail open

Salesforce rejected, and Glade accepted:

- a non-grouped selected field in an aggregate query;
- `LIMIT` on an ungrouped overall aggregate query;
- more than three `ROLLUP` fields;
- a self semi-join;
- `TYPEOF` combined with grouping;
- `ORDER BY` combined with `FOR UPDATE`;
- `FIELDS(ALL)` as an unbounded field set in Apex;
- SOSL without `RETURNING`;
- a missing or type-incompatible bind;
- `SUM(Name)`;
- upsert on non-external/non-idLookup `Name`;
- merge operands with incompatible sObject types;
- primitive and non-sObject-list DML operands.

The root causes are direct:

- `querySemanticsChecker.checkFile` in `internal/sema/sema_checks.go` silently
  continues after every `soql.Parse` or `sosl.Parse` error;
- query checking skips function expressions and does not check `HAVING`;
- inline query expressions are not resolved against the Apex body scope;
- `ir.OpDML` receives no operation-specific operand validation;
- `internal/schema/schema.go` lacks several query capability properties needed
  for complete field validation.

Parser and runtime validation must share one compiler-contract layer. Glade
should not acquire separate, divergent rules for `check` and local execution.

### P1: `Currency` is also an invalid source type

The original report used `currency` as a variable name. The second pass found
an adjacent but separate hole:

```apex
public class Probe {
    public Currency Value;
}
```

Salesforce rejected it with `Type is not visible: CURRENCY`; Glade accepted it.
`Currency` is valid schema metadata terminology but not a source-level Apex
type. The type validator must keep those contexts distinct. `AnyType` is
already rejected by Glade.

The namespace-authority probe also confirmed that a project class named
`Database` prevents fallback to `System.Database`. Salesforce rejected
`Database.query(...)` when the project class lacked that method; Glade fell
back to the platform namespace and accepted it.

### P1: Statement, switch, and exception contracts remain incomplete

The initial pass confirmed missing checks for duplicate switch values,
`when else` ordering, `break`/`continue` context, thrown value type, duplicate
or already-covered catches, and custom Exception naming. The second pass added:

- unsupported Boolean, Decimal, and Date switch selector types;
- a non-Exception catch type;
- unreachable code after `throw` at both API 40 and API 41.

Glade already matches Salesforce for non-Boolean `if`, `while`, and `for`
conditions and invalid return forms.

## Salesforce-accepted controls

These programs compiled in Salesforce and must not be turned into Glade
compiler errors solely because a documentation sentence suggests otherwise:

- local `final` reassignment and a `final` property;
- `super.method()` outside an overriding method;
- transient static fields and transient locals;
- a webservice parameter of `System.LoggingLevel`;
- a global `@IsTest` class and a value-returning `@IsTest` method;
- reading an uninitialized local;
- declaring a future method in a Batchable class and a future-to-future call;
- `WITH SECURITY_ENFORCED` at API 66 and API 67;
- the tested `List<Account> instanceof Iterable<SObject>` form at API 59 and
  API 60;
- getter mutation at API 41 and API 42;
- a static local declaration in a trigger body;
- a ContentVersion delete trigger;
- a public static interface implementation;
- duplicate SOSL `RETURNING` objects;
- grouping Account `Description`;
- both tested `Database.Batchable<T>` execute-scope variance forms;
- `System.runAs` outside a test method;
- multiple HTTP verb annotations on one method.

Some of these can still fail when invoked or executed. They are compile-time
controls, not claims that every runtime behavior is supported.

## Root cause

The evidence points to a compiler-coverage system problem, not a missing table:

1. Tree-sitter accepts many declarations that Salesforce rejects in later
   compiler validation.
2. Several AST/index representations discard information required for
   validation: trigger bodies, annotation arguments, declaration body state,
   owner identity, and generic substitutions.
3. Existing sema checks are strong where implemented but are not organized
   around complete Apex contracts.
4. Query parse failures are explicitly discarded.
5. The test runner can normalize invalid source into runnable `static void`
   methods.
6. The repository lacks a checked negative-rule ledger connecting a Salesforce
   oracle row to product ownership and regression tests.
7. Positive corpus success cannot prove compiler parity. These gaps require
   adversarial negative fixtures.

## Updated closure program

The executable follow-on plan is:

`docs/superpowers/plans/2026-07-25-apex-language-rule-compatibility.md`

It divides closure into independently landable packets:

0. checked rule ledger and scratch comparator in `glade-tools`;
1. fail-closed semantic diagnostics in `glade test`;
2. trigger-body analysis and trigger capability checks;
3. duplicate severity and declaration identity;
4. identifier shape, length, rename, and local scope;
5. declaration, signature, modifier, and constructor contracts;
6. inheritance, interface generic substitution, and namespace authority;
7. property, expression, literal, and collection contracts;
8. statement, switch, and exception contracts;
9. structured annotation representation and shared catalog;
10. stable annotation/test contracts;
11. source API, package-history, and preview gates;
12. REST and SOAP exposure contracts;
13. SOQL, SOSL, bind, field-capability, and DML contracts;
14. anonymous Apex, watch, LSP, command, and release proof.

Each packet starts with Salesforce-proven RED fixtures. Product diagnostics and
tests stay in this repository. The rule ledger and scratch runner stay in the
first-party compat plugin or its `glade-tools` source. Base Glade must not
depend on `glade-tools`.

## Release proof

For every rule moved to `supported`:

1. run its focused product regression;
2. replay the same source against a scratch org;
3. compare accept/reject behavior rather than message wording alone;
4. prove `check`, `test`, and every affected consumer fail closed;
5. run the focused package tests;
6. run `go test ./...` and `scripts/smoke.sh`;
7. run the checked scratch comparator with zero supported-row mismatches;
8. retain NU as the stronger corpus compatibility gate.

Package-history rules such as released-package `@Deprecated` behavior remain
explicit gaps until package history is available. API 67 Developer Preview
annotations such as `@IntegrationTest` and `@TearDown` remain version/feature
gated rather than being inferred from the API number.

## Reproduction summary

Second-pass scratch org:

- edition: Developer;
- requested API: 66.0;
- instance API: 67.0;
- duration: one day;
- source tracking: disabled;
- creation date: 2026-07-25.

Product probe:

```bash
go build -o /private/tmp/glade-apex-rule-audit-pass2 ./cmd/glade
node tmp/apex-rule-audit-pass2/probe.mjs \
  /private/tmp/glade-apex-rule-audit-pass2 \
  glade-language-rules-pass2-20260725
```

The temporary org, binary, probe script, result file, and credentials are
disposable evidence and must not be committed.
