# Error codes and `glade explain`

Actionable diagnostics include stable codes. Use `glade explain <code>` for the short local note.

```bash
glade explain GLADESEMA002
```

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
| `GLADESEMA002` | Unknown Apex or metadata type | `glade schema load --project .` |
| `GLADESCHEMA001` | Local metadata schema load failed | `glade schema load --project .` |

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
