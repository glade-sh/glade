# Glade Org SF Target Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan also requires parallel subagents after Task 0. Do not run this as one long inline edit.

**Goal:** Let Salesforce CLI commands target a named Glade org alias so data and Apex requests mutate and read a selected local Glade SQLite org database.

**Architecture:** `glade org` is a product CLI wrapper around the existing local server, SQLite org store, and Apex VM. It creates a stable local instance URL, registers that URL as an `sf` target org alias through `sf org login access-token`, and adds the missing Salesforce-shaped protocol adapters that common `sf` and manifest import data commands use. The server stays local-only and does not implement live Salesforce org lifecycle.

**Tech Stack:** Go 1.26, existing `internal/gladecli`, existing `internal/server`, existing `internal/storage` DML engine, existing Apex VM execute-anonymous path, Salesforce CLI `sf`, jsforce route compatibility, XML SOAP parsing with Go `encoding/xml`, CSV parsing with Go `encoding/csv`.

---

## Current Ground

The current server already has these pieces:

- `/services/oauth2/userinfo` and `/id/{org}/{user}` local identity stubs.
- REST discovery at `/services/data/`.
- REST SObject describe, CRUD, external-id routes, query, queryAll, query locators.
- Tooling `executeAnonymous` through `/services/data/vXX.X/tooling/executeAnonymous`.
- Composite SObject, Batch, and Tree routes.
- Bulk API v2 query routes.
- SQLite persistence through `glade server --db <path>` and `glade db`.

The missing pieces for the target behavior:

- `glade org` lifecycle and `sf` auth registration.
- Stable instance URL handling for `sf -u <alias>`.
- SOAP Apex `/services/Soap/s/<version>/<orgId>` for `sf apex run`.
- Partner SOAP `/services/Soap/u/<version>` for manifest import tools that call `describeSObjects` and `upsert`.
- Bulk API v1 ingest `/services/async/<version>/job...` for old jsforce bulk import mode.
- End-to-end proof with a real `sf` target alias and a manifest-based manifest.

## Supported Scope

Supported when this plan is complete:

- `glade org create <alias> --project . --db .glade/orgs/<alias>.sqlite --addr 127.0.0.1:<port>`.
- `glade org start <alias>` starts a local Salesforce-shaped HTTP server for that org.
- `glade org auth <alias>` registers the running local server as an `sf` target alias.
- `sf data create record -o <alias> ...` inserts into the Glade DB.
- `sf data query -o <alias> ...` queries the Glade DB.
- `sf apex run -o <alias> -f file.apex` executes through the Glade VM against the Glade DB.
- manifest import tool non-bulk data imports that use query variables, ordered records, external-id upsert, relationship external-id references, and ApexScript cleaners.
- manifest import tool bulk import mode for CSV insert and upsert jobs used by jsforce 1.x Bulk API v1.
- Persistence across server restart when the same `--db` path is used.

Not supported:

- Creating, deleting, or managing real Salesforce scratch orgs.
- Dev Hub flows.
- Browser, device, JWT, refresh-token, or live OAuth flows.
- Real token validation. Local bearer tokens only select local user context when they match a local User id.
- Source deploy, retrieve, push, pull, package install, or package version commands.
- `sf apex run test`; use `glade test` for local tests.
- Full Partner SOAP, Enterprise SOAP, Metadata SOAP, Chatter, Streaming, Pub/Sub, GraphQL, ConnectApi, or hosted Salesforce services.
- Full Bulk API v1 query/export, delete, hardDelete, PK chunking, parallel execution, retry behavior, or server-side batch scheduling.
- Exact Salesforce sharing, CRUD/FLS, mixed-DML, duplicate rules, assignment rules, validation-rule parity beyond what Glade already models, or exact governor accounting beyond selected Glade limit modes.
- Arbitrary manifest-based `.js` files that call unmodeled Salesforce APIs.
- Exact managed-package internals unless represented in project metadata, schema, fixtures, or Glade runtime support.

## Execution Model

Task 0 is a lead task. It prepares the branch and assigns non-overlapping work. After Task 0, dispatch parallel subagents:

- Agent A: `glade org` CLI lifecycle and `sf` auth registration.
- Agent B: SOAP Apex executeAnonymous.
- Agent C: Partner SOAP describe/upsert.
- Agent D: Bulk API v1 ingest.
- Agent E: manifest import compatibility fixtures, smoke script, and docs.

Agents B, C, and D will all touch `internal/server/server.go` unless the lead creates route seams first. Task 0 includes that seam. Each agent should keep new protocol code in its own file and leave final route conflict resolution to the lead integrator.

## File Structure

Create:

- `internal/gladecli/org_command.go`: Parses `glade org` subcommands, reads and writes org config, shells out to `sf` for auth, and starts the existing local server with stored settings.
- `internal/gladecli/org_command_test.go`: Focused CLI tests for create/list/status/auth command behavior.
- `internal/server/soap_common.go`: SOAP envelope parsing, response writing, and Salesforce fault helpers shared by SOAP Apex and Partner SOAP.
- `internal/server/soap_apex.go`: Minimal `/services/Soap/s/<version>/<orgId>` executeAnonymous adapter.
- `internal/server/soap_partner.go`: Minimal `/services/Soap/u/<version>` Partner API adapter for `describeSObjects` and `upsert`.
- `internal/server/bulk_v1.go`: Minimal Bulk API v1 ingest job and batch adapter for jsforce 1.x.
- `internal/server/bulk_v1_test.go`: Bulk API v1 focused tests.
- `internal/server/soap_apex_test.go`: SOAP Apex focused tests.
- `internal/server/soap_partner_test.go`: Partner SOAP focused tests.
- `scripts/smoke_glade_org_sf.sh`: Optional local smoke proof. Skips with a clear message when `sf` is missing.
- `testdata/local-tests/glade-org-data-import/insertOrder.json`: manifest-based manifest fixture.
- `testdata/local-tests/glade-org-data-import/accounts.json`: Data file using external-id upsert.
- `testdata/local-tests/glade-org-data-import/contacts.json`: Data file using relationship external-id references.
- `testdata/local-tests/glade-org-data-import/cleaners.json`: Data file with an ApexScript cleaner.
- `site/docs-src/guide/glade-orgs.md`: Product docs for local `sf` target orgs.

Modify:

- `internal/gladecli/cli.go`: Add top-level `org` command dispatch.
- `internal/cliui/help.go`: Add `glade org` to command help and examples.
- `internal/gladecli/cli_test.go`: Add command discovery and help assertions.
- `internal/server/server.go`: Route `/services/Soap/s`, `/services/Soap/u`, and `/services/async` through narrow handlers.
- `internal/server/server_test.go`: Add high-level integration tests only if focused files cannot cover the case.
- `internal/server/describe_payloads.go`: Reuse or expose describe field helpers for Partner SOAP.
- `site/docs-src/guide/cli-reference.md`: Add `glade org` command reference.
- `site/docs-src/guide/local-api-server.md`: Update supported route table.
- `site/docs-src/index.md`: Mention `glade org` as the `sf` target path, not as a real scratch org.

## Task 0: Lead Setup and Route Seams

**Files:**

- Modify: `internal/server/server.go`
- Create: `internal/server/soap_common.go`

- [ ] **Step 1: Create an isolated worktree and branch**

Run:

```bash
git status --short
git worktree add ../glade-org-sf-target -b codex/glade-org-sf-target
cd ../glade-org-sf-target
```

Expected: worktree created. Existing unrelated changes in `/Users/matt/Dev/glade` remain untouched.

- [ ] **Step 2: Record current command surfaces**

Run:

```bash
go test ./internal/gladecli ./internal/server
sf --version || true
sf org login access-token --help | sed -n '1,80p' || true
sf data create record --help | sed -n '1,80p' || true
sf apex run --help | sed -n '1,80p' || true
```

Expected: Go tests pass before work starts. `sf` output is captured if installed. Lack of `sf` is not a blocker for unit work.

- [ ] **Step 3: Add server route seams**

In `internal/server/server.go`, route SOAP and async before the `/services/data` gate:

```go
if len(parts) >= 3 && parts[0] == "services" && parts[1] == "Soap" {
    s.handleSOAP(w, r, parts[2:])
    return
}
if len(parts) >= 2 && parts[0] == "services" && parts[1] == "async" {
    s.handleBulkV1(w, r, parts[2:])
    return
}
```

Create `internal/server/soap_common.go` with stubs:

```go
package server

import "net/http"

func (s *Server) handleSOAP(w http.ResponseWriter, r *http.Request, parts []string) {
    writeSalesforceError(w, errUnsupportedFeature, "SOAP API routes are not implemented in the local server")
}

func (s *Server) handleBulkV1(w http.ResponseWriter, r *http.Request, parts []string) {
    writeSalesforceError(w, errUnsupportedFeature, "Bulk API v1 routes are not implemented in the local server")
}
```

Expected: All existing tests still pass. Agents B, C, and D replace these stubs in focused files.

- [ ] **Step 4: Dispatch parallel subagents**

Use one fresh subagent per task. Give each subagent the task section below, the supported scope, and the current ground section. Tell each subagent to return a summary, tests run, and files touched. Do not let subagents commit. The lead integrates and commits.

## Task 1: Agent A - `glade org` CLI Lifecycle

**Files:**

- Create: `internal/gladecli/org_command.go`
- Create: `internal/gladecli/org_command_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Write failing tests for org create/list/status**

Create `internal/gladecli/org_command_test.go` with tests named:

- `TestRunOrgCreateWritesConfigAndInitializesDB`
- `TestRunOrgListShowsConfiguredOrg`
- `TestRunOrgStatusReportsStoppedWhenServerIsNotRunning`

Use a temp project directory with `sfdx-project.json`. Use a temp `.glade/orgs` root by `chdir` into the temp project. Assert that `glade org create my-glade-org --project . --db .glade/orgs/my-glade-org.sqlite --addr 127.0.0.1:17911` writes `.glade/orgs/my-glade-org/org.json` with these fields:

```json
{
  "alias": "my-glade-org",
  "project": ".",
  "db": ".glade/orgs/my-glade-org.sqlite",
  "addr": "127.0.0.1:17911",
  "instanceUrl": "http://127.0.0.1:17911",
  "orgId": "00D000000000001",
  "userId": "005000000000001"
}
```

Expected failure before implementation: unknown `org` command or missing config file.

- [ ] **Step 2: Implement config and subcommand parser**

Implement these subcommands:

```text
glade org create <alias> [--project <root>] [--db <path>] [--addr <host:port>] [--json]
glade org list [--json]
glade org status <alias> [--json]
glade org start <alias>
glade org auth <alias> [--sf-config-dir <path>] [--print]
```

Rules:

- Default project is `.`.
- Default db is `.glade/orgs/<alias>.sqlite`.
- Default addr is `127.0.0.1:17911` for the first org, then scan upward until an unused configured port is found.
- Config file is `.glade/orgs/<alias>/org.json`.
- `create` must initialize the SQLite org by using the same `openDBStore` path used by `glade server --db`.
- `start` must call the same server path as `runServer` with the stored project, db, and addr.
- `status` does a GET to `<instanceUrl>/services/oauth2/userinfo`.

- [ ] **Step 3: Implement `glade org auth`**

`auth` requires the server to be running. It should call:

```bash
SF_ACCESS_TOKEN="${orgId}!glade-local-${alias}" sf org login access-token --instance-url "${instanceUrl}" --alias "${alias}" --no-prompt
```

If `--sf-config-dir <path>` is provided, set `SF_CONFIG_DIR` in the child process. If `--print` is provided, print the exact environment and command without running it.

Expected command output:

```text
Glade org auth

Alias     my-glade-org
Instance  http://127.0.0.1:17911
Target    sf -u my-glade-org
```

- [ ] **Step 4: Add command dispatch and help**

Modify `internal/gladecli/cli.go` to dispatch `org`. Modify `internal/cliui/help.go` to include:

```text
org        Create, start, inspect, and auth local Glade org targets for sf.
```

Add examples:

```text
glade org create my-glade-org --project . --db .glade/orgs/my-glade-org.sqlite --addr 127.0.0.1:17911
glade org start my-glade-org
glade org auth my-glade-org
```

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/gladecli
```

Expected: PASS.

## Task 2: Agent B - SOAP Apex for `sf apex run`

**Files:**

- Create: `internal/server/soap_common.go`
- Create: `internal/server/soap_apex.go`
- Create: `internal/server/soap_apex_test.go`
- Modify: `internal/server/server.go`

- [ ] **Step 1: Write failing SOAP Apex tests**

Create tests:

- `TestSOAPApexExecuteAnonymousInsertsIntoLocalOrg`
- `TestSOAPApexExecuteAnonymousCompileFailureReturnsSOAPResult`
- `TestSOAPApexRejectsUnknownMethod`

The success test sends a POST to:

```text
/services/Soap/s/65.0/00D000000000001
```

with a SOAP body containing:

```xml
<executeAnonymous xmlns="urn:partner.soap.sforce.com">
  <String>insert new Account(Name = 'SOAP Apex'); System.debug('ok');</String>
</executeAnonymous>
```

Assert HTTP 200, `compiled=true`, `success=true`, and the Account row exists in `org.Objects["Account"].Records`.

- [ ] **Step 2: Implement SOAP envelope helpers**

In `soap_common.go`, implement:

- Request body read with max size of 10 MB.
- Envelope method extraction.
- SOAP fault writer for unsupported method and malformed XML.
- XML escaping through `encoding/xml`.

Do not build SOAP with string concatenation for untrusted text.

- [ ] **Step 3: Implement `/services/Soap/s` executeAnonymous**

In `soap_apex.go`, implement:

- Route key `parts[0] == "s"`.
- Only POST is allowed.
- Extract API version and org id from the path.
- Extract Apex body from `executeAnonymous`.
- Reuse the same local execution path used by Tooling `executeAnonymous`.
- Commit the org only on successful transaction, matching current Tooling behavior.
- Return Salesforce-shaped SOAP result fields: `compiled`, `success`, `line`, `column`, `compileProblem`, `exceptionMessage`, `exceptionStackTrace`.
- Include `DebuggingInfo/debugLog` when a log is available.

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/server -run 'TestSOAPApex'
```

Expected: PASS.

## Task 3: Agent C - Partner SOAP for manifest import Upsert

**Files:**

- Create: `internal/server/soap_partner.go`
- Create: `internal/server/soap_partner_test.go`
- Modify: `internal/server/describe_payloads.go`

- [ ] **Step 1: Write failing Partner SOAP tests**

Create tests:

- `TestPartnerSOAPDescribeSObjectsReturnsImportToolFields`
- `TestPartnerSOAPUpsertCreatesAndUpdatesByExternalID`
- `TestPartnerSOAPUpsertResolvesRelationshipExternalID`
- `TestPartnerSOAPUpsertXsiNilClearsField`
- `TestPartnerSOAPUpsertPartialFailureReturnsRowResults`

The relationship test should seed an Account with `External_Id__c = "acct-1"` and then upsert a Contact through:

```xml
<sObjects xsi:type="sf:Contact">
  <type>Contact</type>
  <LastName>Trail</LastName>
  <Account>
    <type>Account</type>
    <External_Id__c>acct-1</External_Id__c>
  </Account>
  <External_Id__c>contact-1</External_Id__c>
</sObjects>
```

Assert the Contact `AccountId` points at the Account id.

- [ ] **Step 2: Implement `describeSObjects`**

Support only:

```xml
<describeSObjects>
  <sObjectType>Account</sObjectType>
</describeSObjects>
```

Return field data needed by a manifest import tool:

- `name`
- `label`
- `type`
- `createable`
- `updateable`
- `nillable`
- `externalId`
- `unique`
- `relationshipName`
- `referenceTo`

Return object data needed by a manifest import tool mapping:

- `name`
- `label`
- `custom`
- `customSetting`
- `keyPrefix`
- `fields`
- `recordTypeInfos`

- [ ] **Step 3: Implement Partner `upsert`**

Support:

```xml
<upsert>
  <externalIDFieldName>External_Id__c</externalIDFieldName>
  <sObjects>...</sObjects>
</upsert>
```

Rules:

- Accept one or many `sObjects`.
- Object type comes from `type`, `xsi:type`, or the first known typed element.
- Resolve nested relationship references by matching SOAP element name to `Field.RelationshipName`.
- Resolve nested external id fields against target object records.
- Convert `xsi:nil="true"` to explicit null.
- Call existing DML `UpsertWithExternalID`.
- Return one result per row with `created`, `success`, `id`, and `errors`.
- Do not implement Partner `login`, `query`, `delete`, or `update` in this task.

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/server -run 'TestPartnerSOAP'
```

Expected: PASS.

## Task 4: Agent D - Bulk API v1 Ingest for jsforce 1.x

**Files:**

- Create: `internal/server/bulk_v1.go`
- Create: `internal/server/bulk_v1_test.go`
- Modify: `internal/server/server.go`

- [ ] **Step 1: Write failing Bulk v1 tests**

Create tests:

- `TestBulkV1InsertCSVCreatesRecords`
- `TestBulkV1UpsertCSVUpdatesByExternalID`
- `TestBulkV1BatchStatusAndResultCSV`
- `TestBulkV1RejectsUnsupportedOperations`

The insert test sequence:

```text
POST /services/async/65.0/job
POST /services/async/65.0/job/<jobId>/batch
GET  /services/async/65.0/job/<jobId>/batch/<batchId>
GET  /services/async/65.0/job/<jobId>/batch/<batchId>/result
POST /services/async/65.0/job/<jobId>
```

Use CSV:

```csv
Name,External_Id__c
Bulk One,bulk-1
Bulk Two,bulk-2
```

- [ ] **Step 2: Implement job creation**

Parse XML job info:

```xml
<jobInfo xmlns="http://www.force.com/2009/06/asyncapi/dataload">
  <operation>upsert</operation>
  <object>Account</object>
  <externalIdFieldName>External_Id__c</externalIdFieldName>
  <contentType>CSV</contentType>
</jobInfo>
```

Support operations:

- `insert`
- `upsert`

Reject:

- `query`
- `delete`
- `hardDelete`
- `update`

Store jobs in memory on `Server`. Job ids should use a deterministic local prefix such as `750`.

- [ ] **Step 3: Implement CSV batch upload and result CSV**

For `POST /job/<jobId>/batch`:

- Parse CSV headers.
- Convert blank strings to empty values, not explicit null.
- Run DML insert or upsert.
- Store row results in job memory.
- Return batch XML with state `Completed`.

For `GET /job/<jobId>/batch/<batchId>/result`, return CSV:

```csv
Id,Success,Created,Error
001000000000001,true,true,
001000000000002,true,true,
```

For failures, put the DML error message in `Error`.

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/server -run 'TestBulkV1'
```

Expected: PASS.

## Task 5: Agent E - Manifest Import Compatibility Fixtures and Smoke Proof

**Files:**

- Create: `testdata/local-tests/glade-org-data-import/insertOrder.json`
- Create: `testdata/local-tests/glade-org-data-import/accounts.json`
- Create: `testdata/local-tests/glade-org-data-import/contacts.json`
- Create: `testdata/local-tests/glade-org-data-import/cleaners.json`
- Create: `scripts/smoke_glade_org_sf.sh`
- Create or modify: `internal/server/third_party_import_compat_test.go`

- [ ] **Step 1: Add manifest-based data files**

Create `insertOrder.json`:

```json
{
  "manifest": true,
  "order": [
    "accounts.json",
    "contacts.json",
    "cleaners.json"
  ]
}
```

Create `accounts.json`:

```json
{
  "extId": "External_Id__c",
  "queries": [],
  "records": {
    "Account": [
      {
        "Name": "Glade Local Account",
        "External_Id__c": "acct-1"
      }
    ]
  },
  "cleaners": []
}
```

Create `contacts.json`:

```json
{
  "extId": "External_Id__c",
  "queries": [
    {
      "variable": "Account",
      "query": "SELECT Id, External_Id__c FROM Account WHERE External_Id__c = 'acct-1'"
    }
  ],
  "records": {
    "Contact": [
      {
        "LastName": "Local Contact",
        "Account.External_Id__c": "acct-1",
        "External_Id__c": "contact-1"
      }
    ]
  },
  "cleaners": []
}
```

Create `cleaners.json`:

```json
{
  "extId": "External_Id__c",
  "queries": [],
  "records": {},
  "cleaners": [
    {
      "type": "ApexScript",
      "body": [
        "insert new Account(Name = 'Cleaner Apex', External_Id__c = 'cleaner-1');"
      ]
    }
  ]
}
```

- [ ] **Step 2: Add server compatibility test**

Write `TestThirdPartyImportRouteSetSupportsImportShape` in `internal/server/third_party_import_compat_test.go`. It should not shell out to npm. It should exercise the exact server routes the package uses:

- REST `GET /query?q=...`
- Partner SOAP `describeSObjects`
- Partner SOAP `upsert`
- Tooling `GET /tooling/executeAnonymous?anonymousBody=...`
- Bulk v1 insert or upsert if Task 4 is present

Assert final Account and Contact rows exist in the local org.

- [ ] **Step 3: Add optional smoke script**

Create `scripts/smoke_glade_org_sf.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

if ! command -v sf >/dev/null 2>&1; then
  echo "sf is not installed; skipping smoke"
  exit 0
fi

tmp="$(mktemp -d)"
cleanup() {
  if [[ -n "${server_pid:-}" ]]; then
    kill "$server_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

export SF_CONFIG_DIR="$tmp/sf"
alias_name="my-glade-org"
db="$tmp/${alias_name}.sqlite"
addr="127.0.0.1:17911"

go run ./cmd/glade org create "$alias_name" --project . --db "$db" --addr "$addr"
go run ./cmd/glade org start "$alias_name" >"$tmp/server.log" 2>&1 &
server_pid="$!"

for i in {1..50}; do
  if curl -fsS "http://$addr/services/oauth2/userinfo" >/dev/null; then
    break
  fi
  sleep 0.1
done

go run ./cmd/glade org auth "$alias_name" --sf-config-dir "$SF_CONFIG_DIR"
sf data create record -o "$alias_name" -s Account -v "Name='SF Smoke' External_Id__c='sf-smoke'"
sf data query -o "$alias_name" -q "SELECT Id, Name FROM Account WHERE External_Id__c = 'sf-smoke'" --json
printf "insert new Account(Name = 'Apex Smoke', External_Id__c = 'apex-smoke');\n" > "$tmp/smoke.apex"
sf apex run -o "$alias_name" -f "$tmp/smoke.apex"
go run ./cmd/glade db query --db "$db" --project . --json "SELECT Id, Name FROM Account WHERE External_Id__c IN ('sf-smoke','apex-smoke')"
```

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/server -run 'TestThirdPartyImport|TestPartnerSOAP|TestSOAPApex|TestBulkV1'
```

Expected: PASS.

## Task 6: Agent E - Docs and Support Matrix

**Files:**

- Create: `site/docs-src/guide/glade-orgs.md`
- Modify: `site/docs-src/guide/cli-reference.md`
- Modify: `site/docs-src/guide/local-api-server.md`
- Modify: `site/docs-src/index.md`

- [ ] **Step 1: Write guide page**

Create `site/docs-src/guide/glade-orgs.md` with these sections:

- `Create a local org`
- `Start the local org`
- `Register it with sf`
- `Run sf data commands`
- `Run sf apex`
- `Run manifest-based data import`
- `Supported locally`
- `Not supported locally`

Use this example:

```bash
glade org create my-glade-org --project . --db .glade/orgs/my-glade-org.sqlite --addr 127.0.0.1:17911
glade org start my-glade-org
glade org auth my-glade-org
sf data create record -o my-glade-org -s Account -v "Name='Local'"
sf apex run -o my-glade-org -f scripts/seed.apex
sf data import tree -p ./data/insertOrder.json -o my-glade-org
```

State that `-u` and `-o` both depend on how the installed `sf` command defines its target-org flag.

- [ ] **Step 2: Update local API support table**

In `site/docs-src/guide/local-api-server.md`, add rows:

```markdown
| SOAP Apex executeAnonymous | `/services/Soap/s/vXX.X/<OrgId>` | supported for `sf apex run` |
| Partner SOAP describe/upsert | `/services/Soap/u/vXX.X` | supported baseline for local data import tools |
| Bulk API v1 ingest | `/services/async/vXX.X/job...` | supported insert/upsert CSV baseline |
```

- [ ] **Step 3: Update CLI reference**

In `site/docs-src/guide/cli-reference.md`, add `glade org` syntax and examples. Keep it product-facing. Do not call these real scratch orgs.

- [ ] **Step 4: Run docs check**

Run:

```bash
npm --prefix site test
```

Expected: PASS. If dependencies are missing, run `npm --prefix site ci` once, then rerun.

## Task 7: Lead Integration and Full Verification

**Files:**

- Review all files touched by Agents A through E.

- [ ] **Step 1: Inspect parallel results**

Run:

```bash
git status --short
git diff --check
```

Expected: no whitespace errors. Confirm no agent edited unrelated files.

- [ ] **Step 2: Run focused Go suites**

Run:

```bash
go test ./internal/gladecli ./internal/server
```

Expected: PASS.

- [ ] **Step 3: Run broader Go suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run optional sf smoke**

Run:

```bash
bash scripts/smoke_glade_org_sf.sh
```

Expected when `sf` is installed: it creates a local org, registers an `sf` alias, inserts one Account with `sf data`, inserts one Account with `sf apex run`, and verifies both rows through `glade db query`.

Expected when `sf` is missing: script prints `sf is not installed; skipping smoke` and exits 0.

- [ ] **Step 5: Manual manifest import proof**

Run the local import smoke when the installed `sf` CLI supports tree import:

```bash
tmp="$(mktemp -d)"
export SF_CONFIG_DIR="$tmp/sf"
go run ./cmd/glade org create my-glade-org --project . --db "$tmp/my-glade-org.sqlite" --addr 127.0.0.1:17911
go run ./cmd/glade org start my-glade-org &
server_pid="$!"
trap 'kill "$server_pid" >/dev/null 2>&1 || true; rm -rf "$tmp"' EXIT
sleep 1
go run ./cmd/glade org auth my-glade-org --sf-config-dir "$SF_CONFIG_DIR"
sf data import tree -p ./testdata/local-tests/glade-org-data-import/insertOrder.json -o my-glade-org
go run ./cmd/glade db query --db "$tmp/my-glade-org.sqlite" --project . --json "SELECT Id, Name FROM Account"
```

Expected: the imported Account rows appear in the query output. If the installed `sf` CLI lacks tree import, record that the route-level manifest import compatibility test passed and the CLI smoke was not available.

- [ ] **Step 6: Commit**

After all required verification passes:

```bash
git add internal/gladecli internal/server scripts testdata/local-tests/glade-org-data-import site/docs-src docs/superpowers/plans/2026-06-16-glade-org-sf-target.md
git commit -m "feat: add local glade org sf targets"
```

Expected: one commit with product code, tests, fixtures, docs, and this plan.

## Agent Prompt Templates

Use these exact prompts when dispatching subagents after Task 0.

### Agent A Prompt

Implement Task 1 from `docs/superpowers/plans/2026-06-16-glade-org-sf-target.md`. Scope: `internal/gladecli` and `internal/cliui/help.go` only. Do not touch `internal/server`. Build `glade org create/list/status/start/auth` as specified. Return files touched, tests run, and any blockers. Do not commit.

### Agent B Prompt

Implement Task 2 from `docs/superpowers/plans/2026-06-16-glade-org-sf-target.md`. Scope: SOAP Apex executeAnonymous. Keep shared SOAP helpers small. Do not implement Partner SOAP or Bulk API v1. Return files touched, tests run, and any blockers. Do not commit.

### Agent C Prompt

Implement Task 3 from `docs/superpowers/plans/2026-06-16-glade-org-sf-target.md`. Scope: Partner SOAP `describeSObjects` and `upsert`. Do not implement Apex SOAP or Bulk API v1. Reuse existing DML engine and describe data. Return files touched, tests run, and any blockers. Do not commit.

### Agent D Prompt

Implement Task 4 from `docs/superpowers/plans/2026-06-16-glade-org-sf-target.md`. Scope: Bulk API v1 ingest only. Support CSV insert and upsert. Reject unsupported operations with Salesforce-shaped errors. Do not implement Bulk query/export. Return files touched, tests run, and any blockers. Do not commit.

### Agent E Prompt

Implement Tasks 5 and 6 from `docs/superpowers/plans/2026-06-16-glade-org-sf-target.md`. Scope: fixtures, smoke script, and docs. You may add route-level compatibility tests, but do not implement protocol handlers. Return files touched, tests run, and any blockers. Do not commit.

## Final Acceptance

The branch is ready when these commands pass:

```bash
go test ./internal/gladecli ./internal/server
go test ./...
npm --prefix site test
bash scripts/smoke_glade_org_sf.sh
```

And this user path works when tree import is available:

```bash
sf data import tree -p ./testdata/local-tests/glade-org-data-import/insertOrder.json -o my-glade-org
```

The rows must land in the SQLite database named by `glade org create`.
