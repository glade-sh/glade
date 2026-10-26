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
| `excludedClasses` | array | Invalid classes, their dependents, and excluded triggers for `glade test`. |

## Test exclusions

`summary.excludedClasses` counts exclusions, including non-test classes and
triggers. Each `excludedClasses` entry has `className`, `file`, `line`,
`dependsOn`, `root`, and `message`. A root with multiple errors may include
`problems`, an array of line/message pairs. Excluded tests appear as
`compile_error` test rows. Any exclusion makes the command exit `1`.

## Doctor JSON

`glade doctor --json` has its own schema version `1.1`, with fields such as
`parserOK`, `parserStatus`, and toolchain/project status. It does not use the
example envelope above. Inspect command-specific schemas before parsing them.

## Doctor object

In v0.3.0, `glade doctor --json` uses a flattened `1.1`
object because its fields predate the shared envelope. Version `1.1` makes the
Apex readiness scope explicit and treats LWC-toolchain and local-data health as
independent workflow advisories. Older v0.2.15 binaries emit schema `1.0`;
consumers must select fields according to the installed schema version.
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
| `sourceApiInCheckedWindow` | boolean | The default is in the 62.0 through 68.0 checked window. This does not mean Salesforce was queried during this run. |
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

When source uses undeclared org features or project permission-set assignments,
the optional `orgSetup` array contains setup advice. It is omitted when there
are no suggestions. Each entry has:

| Field | Meaning |
| --- | --- |
| `kind` | `feature` or `permissionSet`. |
| `feature` | Feature name for a feature suggestion; otherwise omitted. |
| `names` | Permission-set API names for a permission-set suggestion; otherwise omitted. |
| `reason` | Source evidence that prompted the suggestion. |
| `files` | Up to three project-relative files containing evidence; omitted when empty. |
| `yaml` | Suggested `glade.yml` block, retaining existing configured entries. |

Suggestions do not enable features or assign permissions, and do not change
doctor's `status` or `exitCode`. See [setup suggestions](/reference/config#setup-suggestions).

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
