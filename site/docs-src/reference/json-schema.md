# JSON Envelope Reference

Priority commands write a versioned JSON envelope.

```json
{
  "schemaVersion": "1.0",
  "command": "check",
  "status": "passed",
  "exitCode": 0,
  "project": {},
  "summary": {},
  "diagnostics": [],
  "artifacts": [],
  "timings": {},
  "suggestions": [],
  "tests": [],
  "data": {}
}
```

## Fields

| Field | Type | Notes |
| --- | --- | --- |
| `schemaVersion` | string | Envelope version. Current value is `1.0`. |
| `command` | string | Command that produced the output. |
| `status` | string | `passed`, `failed`, `partial`, or `unsupported` where supported. |
| `exitCode` | number | Process exit status. |
| `project` | object | Project root and package information when known. |
| `summary` | object | Command-specific counts. |
| `diagnostics` | array | Stable diagnostic objects. |
| `artifacts` | array | Files written by the command. |
| `timings` | object | Timings when reported. |
| `suggestions` | array | Next commands to run. |
| `tests` | array | Flat test rows for `glade test`. |
| `data` | object | Command-specific detail during migration. |

## Diagnostic object

```json
{
  "code": "GLADESEMA002",
  "severity": "error",
  "message": "method \"latestInvoice\" references unknown type \"Invoice__c\"",
  "file": "force-app/main/default/classes/RefinementService.cls",
  "line": 2,
  "column": 3,
  "why": "The Apex type reference is not present in local Apex, schema, or platform symbols.",
  "try": [
    "glade schema load --project .",
    "glade check --project ."
  ],
  "docs": "https://glade.sh/reference/errors#gladesema002"
}
```
