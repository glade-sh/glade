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
