# Capability Work Queue

This queue turns Salesforce surface expansion into small checked slices. Each row names the public surface, the implementation entry point, and the proof hook that should move before a status changes in `internal/capability`.

| Surface | Next capability slice | Status source | Fixture or probe | Implementation entry point | Next check |
| --- | --- | --- | --- | --- | --- |
| ConnectApi | Endpoint extension lifecycle and organization settings calls | `internal/capability` | Add or extend `compat` fixture for endpoint extension before/after hooks | `internal/vm`, `internal/typesys` | `go test ./internal/vm ./internal/capability` |
| Metadata | Read-only custom metadata and Metadata API describe shapes | `internal/capability` | Add metadata fixture covering records, labels, and static resources | `internal/schema`, `internal/storage`, `internal/vm` | `go run ./cmd/glade compat mvp --json` |
| Reports | Report describe/run result DTOs used by tests | `internal/capability` | Add report fixture with grouped and tabular result expectations | `internal/vm`, `internal/compat` | `go test ./internal/vm ./internal/compat` |
| ApexPages | Standard controller, PageReference, and message behavior without rendering | `internal/capability` | Add Visualforce controller compatibility fixture | `internal/vm`, `internal/apextest` | `go test ./internal/vm ./internal/apextest` |
| Tooling | Tooling query and metadata read endpoints in the local server | `internal/capability` | Add server fixture for Tooling API query and sObject describes | `internal/server`, `internal/storage` | `go test ./internal/server` |
| Bulk | Bulk API job, batch, and result lifecycles backed by local storage | `internal/capability` | Add server fixture for create job, upload batch, close job, fetch result | `internal/server`, `internal/storage` | `go test ./internal/server ./internal/storage` |
| Composite | Composite tree and batch transaction behavior | `internal/capability` | Add REST fixture for rollback and partial success cases | `internal/server`, `internal/dml` | `go test ./internal/server ./internal/dml` |
| DML automation | Workflow and Flow side effects inside test transaction rollback | `internal/capability` | Add local Apex test fixture with declarative side effect records | `internal/dml`, `internal/storage`, `internal/apextest` | `go test ./internal/dml ./internal/apextest` |
| SOQL/SOSL | Query frontier slices for relationship, aggregate, and search behavior | `internal/capability` | Add focused SOQL/SOSL fixtures before widening parser/runtime support | `internal/soql`, `internal/vm` | `go test ./internal/soql ./internal/vm` |
| Platform Cache | Org/session cache semantics visible to Apex tests | `internal/capability` | Add cache compatibility fixture for put, get, remove, and partition names | `internal/vm`, `internal/storage` | `go test ./internal/vm ./internal/storage` |

Before moving a row from queued work into `supported`, add compatibility coverage first, update generated docs, and keep any unsupported behavior explicit.
