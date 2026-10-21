---
pageType: reference
canonicalTask: /reference/apex-language-compatibility
---

# Apex language compatibility

Glade checks Apex source against a Salesforce-backed compiler contract before
local execution. The current catalog contains 400 checked language-rule rows:
121 reserved-identifier rows and 279 additional accept or reject controls. In
total, 51 rows are accept controls and 349 are rejection controls.

This is a checked compatibility set. It is not a claim that every Apex language
or hosted platform behavior runs locally.

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

Both `currency` and `CuRrEnCy` are therefore invalid variable names. The check
also covers type, constructor, field, property, parameter, enhanced-for, catch,
trigger, and enum-constant declaration names.

Salesforce permits most reserved words as method names. Glade preserves those
method names, including `currency()` and `void()`. Grammar keywords that
Salesforce always reserves remain invalid as methods: `trigger`, `insert`,
`update`, `upsert`, `delete`, `undelete`, `merge`, `new`, `for`, and `select`.

Schema and API references such as `Invoice__c` and `Amount__c` use their own
naming contract.

## Identifier shape

Apex source identifiers must start with an ASCII letter, contain only ASCII
letters, digits, and underscores, contain no consecutive or trailing
underscores, and be no longer than 255 characters.

The declaration parser reports:

- `APEXPARSE002` for a reserved source identifier; and
- `APEXPARSE003` for an invalid identifier shape or length.

Those diagnostics preserve the original spelling and exact identifier range
through `glade parse`, project indexing, `glade check`, `glade test`, and editor
diagnostics. Tests with an invalid identifier are reported as compile errors
and are not executed.

Anonymous `glade exec` applies the same checks before execution but reports
`GLADESEMA_ANONYMOUS_PARSE`, with the spelling and byte position. Apex source
rename operations reject an invalid target before creating edits and return an
`Invalid identifier` error rather than an `APEXPARSE` code.

## Broader checked rules

The remaining catalog rows cover annotation placement and arguments,
declaration and generic type contracts, inheritance and visibility, variable
scope and statements, trigger and REST declarations, SOQL/SOSL clauses and bind
types, namespace precedence, and source API-version gates.

Every supported row identifies its Salesforce evidence, expected compiler
outcome, owning Glade subsystem, exact product regression test, and pinned Glade
commit. Maintainers can replay every row against a Salesforce scratch org and
the pinned Glade binary.

Salesforce remains the validation gate for behavior outside this checked set and
for hosted services, deployment, metadata, permissions, and exact production
runtime behavior.

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

## Detailed source

- [Repository Apex language compatibility reference](https://github.com/glade-sh/glade/blob/main/docs/APEX_LANGUAGE_COMPATIBILITY.md)
- [What Glade runs locally](/guide/support-map)
- [Apex support map](/reference/apex-support)
