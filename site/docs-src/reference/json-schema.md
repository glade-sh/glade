---
pageType: reference
canonicalTask: /guide/automation
---

# JSON envelope reference

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

## Doctor object

`glade doctor --json` uses a flattened `1.1` object because its fields predate
the shared envelope. Version `1.1` makes the Apex readiness scope explicit and
treats LWC-toolchain and local-data health as independent workflow advisories.
Important fields are:

| Field | Type | Notes |
| --- | --- | --- |
| `readinessScope` | string | `apex`; `status` and `exitCode` describe Apex check/test prerequisites. |
| `apexReady` | boolean | Whether the project, configuration, and parser are ready for Apex check/test. |
| `projectOK` | boolean | The resolved local project could be loaded. |
| `projectRoot` | string | Resolved absolute project root when known. |
| `configOK` / `configMissing` | boolean | Distinguish an invalid config from an absent one. |
| `configStatus` | string | Parse or validation detail for an invalid config. |
| `sourceApiVersion` | string | Project Apex API default; individual class metadata can override it. |
| `sourceApiInCheckedWindow` | boolean | The default is in the current `65.0`, `66.0`, `67.0` checked window. This does not mean Salesforce was queried during this run. |
| `parserOK` | boolean | The binary passed a real Apex parser self-check. |
| `toolchainOK` | boolean | LWC compiler/runtime assets were found. This is advisory for Apex check and test. |
| `localData` | object | Project-local DB state and schema binding when present; a failed state is advisory for Apex and blocks DB-backed confidence. |
| `salesforceBoundary` | string | States that doctor did not contact Salesforce. |
| `suggestions` | array | Project-scoped next commands after a ready result. |
| `recovery` | array | Project-scoped recovery steps after a failed result. |
| `advisories` | array | Non-blocking historical-version or LWC-toolchain guidance. |

For recognized `doctor` flags, setup failures such as a missing project path or
malformed `glade.yml` still emit the doctor JSON object with `status: "failed"`
and the matching nonzero `exitCode`.

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
