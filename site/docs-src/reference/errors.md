---
pageType: reference
canonicalTask: /help/troubleshooting
---

# Error codes

Actionable diagnostics include stable codes. Use `glade explain <code>` for
codes registered with the installed binary. The parser codes documented below
do not currently have `glade explain` entries.

```bash
glade explain GLADESEMA002
```

If it reports an unknown code, use the detailed guide on this page and rerun
the named local command.

```text
GLADESEMA002 - unknown type

Why:
  Glade found an Apex type reference that is not present in local Apex,
  schema, or platform symbols.

Try:
  glade schema load --project .
  glade check --project .
```

## Common codes

| Code | Meaning | First command |
| --- | --- | --- |
| `APEXPARSE002` | Reserved Apex source identifier | `glade check --project .` |
| `APEXPARSE003` | Invalid Apex source identifier shape or length | `glade check --project .` |
| `GLADESEMA002` | Unknown Apex or metadata type | `glade schema load --project .` |
| `GLADESCHEMA001` | Local metadata schema load failed | `glade schema load --project .` |
| `GLADEAUTO001` | Unsupported workflow behavior (warning) | `glade check --project .` |
| `GLADEAUTO002` | Unsupported Flow behavior (warning) | `glade check --project .` |
| `GLADEAUTO003` | Automation file could not load and was skipped (warning) | `glade check --project .` |
| `GLADEAUTO004` | Invalid supported Flow definition (error) | `glade check --project .` |

`APEXPARSE002` and `APEXPARSE003` are emitted by project parsing but are not
currently accepted by `glade explain`. Use the guidance below and rerun
`glade check`.

## APEXPARSE002

Glade found a case-insensitive Salesforce reserved word in a source identifier
context. Rename the declaration and rerun:

```bash
glade check --project .
```

Salesforce permits most reserved words as method names. See
[Apex language compatibility](/reference/apex-language-compatibility) before
renaming a method.

## APEXPARSE003

Glade found an Apex source identifier with an invalid shape or length. Apex
source identifiers must start with an ASCII letter, use only ASCII letters,
digits, and underscores, contain no consecutive or trailing underscores, and
be no longer than 255 characters.

Schema and API names such as `Invoice__c` have a separate contract.

## GLADESEMA002

Glade found an Apex type reference that is not present in local Apex, schema, or platform symbols.

First run:

```bash
glade schema load --project .
glade check --project .
```

## GLADESCHEMA001

Glade could not load local Salesforce metadata for the project.

First run:

```bash
glade schema load --project .
glade check --project .
```

Check output prints the code beside the diagnostic:

```text
force-app/main/default/classes/RefinementService.cls:2:3
error GLADESEMA002 method "latestInvoice" references unknown type "Invoice__c"
```

## Automation diagnostics

Warnings identify the workflow, Flow, or file that needs attention. A
`GLADEAUTO003` file is skipped while the other automation files still load.
`GLADEAUTO004` is an error, including invalid references within the supported
record-triggered Flow subset. See [local Flow execution](/guide/automation#local-flow-execution).

## Recursive collection display

Displaying a List, Set, or Map that contains itself directly or through other
collections raises `UnsupportedFeature` with a message beginning
`Glade: cannot display a List, Set or Map that contains itself`.
This is a local crash guard, with native behavior uncaptured. It is not a
Salesforce exception contract and has no stable diagnostic code.
