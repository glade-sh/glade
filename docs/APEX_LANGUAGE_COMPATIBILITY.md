# Apex Language Compatibility

Glade checks Apex source against a Salesforce-backed language contract before
local execution. This reference describes the compiler rules that are checked
independently of the VM and standard-library support maps.

The current evidence catalog contains 400 checked language-rule rows:

- 121 Salesforce reserved words, each checked as an identifier rejection.
- 279 additional accept and reject controls for annotations, declarations,
  types, inheritance, statements, triggers, queries, and API-version behavior.
- 51 accept controls and 349 rejection controls in total.

Every supported catalog row names its Salesforce evidence, expected compiler
outcome, owning Glade subsystem, exact product regression test, and the full
Glade commit used for comparison. The catalog and scratch org comparator live
in the first-party `glade-tools` maintenance repository. The Glade product does
not depend on that repository at runtime.

## Reserved identifiers

Apex identifiers are case-insensitive. Glade rejects all 121 Salesforce
reserved words in non-method source declaration identifier contexts:

```text
abstract activate and any array as asc autonomous begin bigdecimal blob
boolean break bulk by byte case cast catch char class collect commit const
continue currency date datetime decimal default delete desc do double else
end enum exception exit export extends false final finally float for from
global goto group having hint if implements import in inner insert instanceof
int integer interface into join like limit list long loop map merge new not
null nulls number object of on or outer override package parallel pragma
private protected public retrieve return rollback select set short sobject
sort static string super switch synchronized system testmethod then this
throw time transaction trigger true try undelete update upsert using virtual
void webservice when where while
```

For example, both `currency` and `CuRrEnCy` are rejected as local variable
names:

```apex
String currency = 'USD';
```

The same check covers type, constructor, field, property, parameter, local,
enhanced-for, catch, trigger, and enum-constant declaration names.

Salesforce permits most reserved words as method names. Glade preserves that
contextual exception. For example, `currency()` and `void()` are valid method
names. Grammar keywords that Salesforce does not permit as method names remain
rejected, including `trigger`, `insert`, `update`, `upsert`, `delete`,
`undelete`, `merge`, `new`, `for`, and `select`.

Schema and API names use their own naming contract. Glade does not apply Apex
source-identifier shape rules to references such as `Invoice__c` or
`Amount__c`.

## Identifier shape

Apex source identifiers must:

- start with an ASCII letter;
- contain only ASCII letters, digits, and underscores;
- contain no consecutive underscores;
- not end with an underscore; and
- contain no more than 255 characters.

Glade applies these rules to Apex declarations and rename targets. Schema
renames retain their Salesforce suffix rules instead.

## Diagnostics and command behavior

The declaration parser reports:

- `APEXPARSE002` for a reserved source identifier; and
- `APEXPARSE003` for an invalid identifier shape or length.

Those diagnostics include the original spelling and exact identifier range.
They flow through:

- `glade parse`;
- `glade check`;
- `glade test`, where the affected test is a compile error and is not run;
- project indexing; and
- LSP publish-diagnostics for open editor buffers.

Anonymous `glade exec`, with or without a project, applies the same identifier
rules before execution. Anonymous parse failures use
`GLADESEMA_ANONYMOUS_PARSE` and preserve the identifier spelling and byte
position.

`glade refactor rename` applies the same reserved-word and shape checks to Apex
source targets before it creates or writes edits. Rename validation returns an
`Invalid identifier` error rather than an `APEXPARSE` diagnostic. Schema rename
targets continue to use their separate suffix contract.

## Other checked language rules

The same catalog tracks Salesforce accept and reject behavior for:

- annotation targets, arguments, duplicates, and method signatures;
- class, interface, enum, constructor, property, and generic type contracts;
- inheritance, visibility, overrides, interface implementation, and namespace
  precedence;
- variable scope, control flow, DML, switch, catch, trigger, REST, and web
  service declarations;
- SOQL and SOSL syntax, clauses, fields, aggregate use, and bind types; and
- source API-version gates and selected platform or SObject member rules.

These rows are a checked compatibility set, not a claim that every Apex
language or hosted platform behavior is implemented.

## Execution and conversion details

The checked runtime cases also cover these distinctions:

- A classic `for` initializer and an enhanced-for variable are scoped to the
  loop; sibling loops may reuse their names.
- User-defined `equals` and `hashCode` methods participate in the checked
  method-dispatch and collection contracts. A method named `equals` is not
  automatically a built-in collection operation.
- A typed null keeps its declared type for overload selection. An untyped
  `null` does not select an overload by an imagined runtime value.
- A final collection reference can still have mutable elements. Incrementing
  a supported numeric element does not reassign the final variable.
- Generic-interface and List-to-SObject casts preserve the captured runtime
  type checks. A cast does not turn every list into a compatible record list.
- Static SOQL binds cover checked parenthesized expressions, indexed values,
  and lexical bindings. Colons inside strings are not bind expressions.

For non-query lists, assigning or explicitly casting a `List<SObject>` view
of one concrete record list to an unrelated concrete record-list type raises
`TypeException`, including for an empty list. The checked method-return path
allows that conversion and preserves the original runtime list type. Query
results use their own conversion rules; do not generalize this return rule to
assignments, arguments, or every list cast.

### Legacy queried-record reads

For source API 43.0 and earlier, assigning to a queried root record enables
captured legacy behavior where omitted scalar or parent fields can read as
null. Even a same-value assignment counts; DML alone does not. Trigger records,
parent/subquery projections, and child collections have separate rules.
API 44.0 and later retain missing-field errors for these reads.
This compatibility detail does not extend the release support window below
Apex 62.0.

## Validation boundary

Routine Glade CI runs product regression tests. The first-party maintenance CI
validates the catalog, its product-test pointers, and its exact Glade commit
pin. Maintainers can also run all catalog programs against a Salesforce scratch
org and the pinned Glade binary to detect either product mismatch or stale
Salesforce expectations.

Salesforce remains the validation gate for Apex behavior outside the checked
catalog and for hosted services, deployment, metadata, permissions, and exact
production runtime behavior.
