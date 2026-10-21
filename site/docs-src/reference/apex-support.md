---
pageType: reference
canonicalTask: /guide/support-map
---

# Apex support map

Read the maintained local surface matrix, then use the capability explorer
for receiver-level behavior and named limits.

Use this page when deciding if an Apex loop belongs in Glade.
The full support map carries the current local contract.

v0.3.0 supports Apex source API 62.0-68.0. Source API below 62.0 is unsupported;
DataWeave, ConnectApi, and Slack are excluded and unsupported.

## Start here

Start with parse, check, test, SOQL, DML, triggers, and SObjects.
Those are the main local Apex paths.
Then check the detailed source for limits before moving a team workflow.

Glade runs local Apex development paths.
It does not replace Salesforce for live services, auth, or exact production governor accounting.
Use first-party plugins for generated capability reports.

## Local surface matrix

This table is included from the maintained repository compatibility source.
Its raw status labels describe that source's local scope. They are not a
platform-wide parity percentage or a verification claim for the current release.

<!--@include: ../../../docs/COMPATIBILITY.md#initial-matrix-->

## FormulaEval

`Formula.builder()` creates a local `FormulaEval.FormulaInstance` for a
supported formula and context type. An instance can evaluate a record and list
its referenced fields; template parsing and numeric-null handling are available
as builder options. Evaluation uses Glade's local formula model, so this does
not imply complete Salesforce formula parity. An expression outside the
modeled subset reports a local unsupported-feature error.

## Detailed source

[What Glade runs locally](/guide/support-map)

For compiler rules, reserved identifiers, and diagnostic propagation, see
[Apex language compatibility](/reference/apex-language-compatibility).

## Record identity and conversions

Custom metadata records preserve their owner namespace separately from the
namespace of the metadata type. `NamespacePrefix` and `QualifiedApiName` use
record ownership in the checked SOQL and identity paths. This does not imply
that every owner-qualified string is accepted by `getInstance`.

Record-list assignment and explicit casts can fail even when a list is empty;
the checked method-return conversions have different rules. See
[Apex execution and conversion details](/reference/apex-language-compatibility#execution-and-conversion-details).

## Related workflows

- [Apex runtime](/guide/modules#apex-runtime)
- [Apex tests](/guide/workflows/apex-tests)
- [Enterprise workflows](/guide/enterprise-workflows)
