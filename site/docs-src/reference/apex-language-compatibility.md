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

## Detailed source

- [Repository Apex language compatibility reference](https://github.com/glade-sh/glade/blob/main/docs/APEX_LANGUAGE_COMPATIBILITY.md)
- [What Glade runs locally](/guide/support-map)
- [Apex support map](/reference/apex-support)
