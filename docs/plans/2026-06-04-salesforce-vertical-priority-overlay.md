# Salesforce Vertical Priority Overlay

Date: 2026-06-04

This plan orders Salesforce vertical work by real use first. The surface ledger
still owns breadth. Agent packets still own file boundaries. This overlay tells
agents which packet to take next when the goal is more working depth, not a new
alphabetical notch.

## Inputs

- Local corpus scan: `example-projects`, excluding `node_modules`, `.git`, and
  Salesforce Docs Scraper paths.
- Runtime source scan: `.cls`, `.trigger`, and `.apex`, with comments stripped.
- UI scan: Apex, Visualforce, Aura/LWC, and metadata-shaped source files.
- Latest clean surface ledger:
  `/var/folders/04/ymtl0lts3nz2w85kg8ty_2rr0000gn/T/tmp.EXX48VyyNz/SURFACE_LEDGER.json`.
- Outside public Apex sample: `apex-enterprise-patterns/fflib-apex-common`,
  `trailheadapps/apex-recipes`, and `jongpie/NebulaLogger`.

## Priority Rule

Use this order when choosing work:

1. Example-project runtime blockers.
2. Example-project file/project spread.
3. Ledger gap pressure.
4. Public Apex practice.
5. Alphabetical order, only as a tie breaker.

Passive DTO breadth does not outrank active runtime use. A namespace with a large
passive forest needs shape work, but it should not consume the first runtime
squads unless the corpus uses it.

## Corpus Signals

Real-project runtime scan:

| Surface | Files | Projects | Hits |
| --- | ---: | ---: | ---: |
| Schema describe | 4,288 | 8 | 36,819 |
| Async/Test | 2,639 | 9 | 33,607 |
| SOQL/DML | 2,059 | 8 | 17,333 |
| Database.* | 1,042 | 9 | 5,191 |
| JSON | 883 | 7 | 2,876 |
| HTTP/callouts | 416 | 7 | 2,398 |
| Auth/Site/Network/UserInfo | 392 | 7 | 968 |
| Metadata | 66 | 5 | 531 |
| Messaging | 51 | 7 | 283 |
| Cache | 22 | 6 | 99 |
| ConnectApi | 13 | 4 | 65 |
| Approval | 0 | 0 | 0 |

UI scan:

| Surface | Files |
| --- | ---: |
| LWC/Aura | 4,521 |
| UI controller usage | 2,126 |
| ApexPages | 758 |
| PageReference | 608 |
| Visualforce | 600 |
| StandardController | 217 |

Top real-project spread:

| Project | Signal |
| --- | --- |
| `src-nmb-nu-develop` | Broadest runtime usage; all major runtime surfaces. |
| `src-nmb-nc-develop` | Heavy UI/controller and runtime usage. |
| `nams-workspace` | Broad runtime, namespaced setup, metadata, endpoint, and UI contracts. |
| `sf-cred-pkg-develop` | Data, JSON, HTTP/callout, namespaces, and service models. |
| `NPSP-rel-3.237` | Data, metadata, builders, services, and FeatureManagement usage. |
| `apex-recipes-main` | Small focused checks for async, callouts, cache, and ConnectApi. |
| `src-nmb-nutpl-develop` | Fast green sentinel for VM and mock-framework regressions. |

## Work Order

### P0: Data Runtime And Describe

Goal: deepen the most common runtime path before broadening passive surfaces.

Packet areas:

- `Data.Runtime.*`
- `Data.Reference.*`
- `Query.Runtime.*`
- `Core.Runtime.SchemaDescribe`
- `Core.Runtime.Database`

Depth targets:

- `Schema.describe` globals, tokens, field maps, field result methods, and
  standard/custom object shape.
- SOQL projection, relationship paths, query locators, aggregate/ordering edges,
  and bind behavior used by selectors.
- DML validation, defaults, writeability, trigger ordering, rollback, setup data,
  and object-specific policy hooks.
- `Database.*` overloads tied to real use, especially query, DML result arrays,
  savepoints, and batch entry points.

Primary sentinels:

- `src-nmb-nu-develop`
- `src-nmb-nc-develop`
- `NPSP-rel-3.237`
- `sf-cred-pkg-develop`

Done means one failing corpus frontier moves, not just a ledger row changing
bucket.

### P1: Async And Test Semantics

Goal: make async behavior Salesforce-shaped under tests.

Packet areas:

- `Tests.Async`
- `Core.Runtime.DatabaseBatchable`
- `Core.Runtime.SystemQueueable`
- `Core.Runtime.SystemSchedulable`

Depth targets:

- `Database.executeBatch` overloads, scope validation, `QueryLocator`, chunking,
  `finish`, `Database.Stateful`, and job IDs.
- `Queueable`, chaining, `System.enqueueJob`, callout marker interfaces, and
  context objects.
- `@future`, schedulable entry points, and `Test.startTest` / `Test.stopTest`
  drain behavior.
- Async transaction isolation, limits, static state, and rollback behavior.

Primary sentinels:

- `src-nmb-nu-develop`
- `src-nmb-nc-develop`
- `NPSP-rel-3.237`
- `apex-recipes-main`
- `src-nmb-nutpl-develop`

Public Apex sample supports this priority: Apex Recipes and NebulaLogger both
show common `Queueable`, callout, and batchable patterns.

### P2: Visualforce, ApexPages, And Controller State

Goal: support controller-heavy projects before broad UI bundle inventory.

Packet areas:

- `UI.ApexPagesControllers`
- `UI.VisualforcePageReference`
- `UI.LWCAuraImports`, after controller state is stable.

Depth targets:

- `PageReference` URL, parameters, redirects, content behavior, and current page
  state.
- `ApexPages` messages, severities, current page, standard controller state,
  standard set controller pagination, selected records, and extension wiring.
- Visualforce page metadata enough for tests, without rendering.
- LWC/Aura Apex import discovery only where it affects Apex reachability or
  test setup.

Primary sentinels:

- `src-nmb-nc-develop`
- `src-nmb-nu-develop`
- `NPSP-rel-3.237`
- `nams-workspace`

LWC/Aura has the largest UI file count, but controller state has the better
runtime bite. Take the board with the knot showing.

### P3: HTTP, JSON, Named Credentials, And Integration Tests

Goal: deepen the integration path that example projects test directly.

Packet areas:

- `Integration.HTTPCallouts`
- `Integration.NamedCredentials`
- `Core.Runtime.JSON`
- `Server.RESTResources`, only when Apex tests need it.

Depth targets:

- `HttpRequest`, `HttpResponse`, `Http`, mock dispatch, callout limits, endpoint
  resolution, and named credential URL behavior.
- JSON serialize/deserialize edges, typed collections, untyped maps/lists,
  parser/generator behavior, and date/time shape.
- REST resource annotations and `RestContext` where local server or Apex tests
  exercise them.

Primary sentinels:

- `sf-cred-pkg-develop`
- `nams-workspace`
- `src-nmb-nu-develop`
- `apex-recipes-main`

### P4: User, Auth, Site, Network, Feature Management

Goal: implement identity and org-context features that real tests branch on.

Packet areas:

- `Ledger.Identity`
- `Core.Runtime.UserInfo`
- `Core.Runtime.FeatureManagement`
- `Core.Runtime.AuthSiteNetwork`

Depth targets:

- `UserInfo` methods, user/profile/permission setup data, and `System.runAs`.
- `FeatureManagement.checkPermission` and custom permission/feature parameter
  lookup.
- `Site`, `Network`, and the narrow Auth methods seen in real projects.

Primary sentinels:

- `NPSP-rel-3.237`
- `src-nmb-nu-develop`
- `src-nmb-nc-develop`
- `sf-cred-pkg-develop`

FeatureManagement is only 55 files in the broad scan, but it appears in real
NPSP and NU code. It should beat unused broad reference rows.

### P5: Metadata And Messaging

Goal: add useful side-effect and metadata depth after the main runtime path is
moving.

Packet areas:

- `Metadata.API.Active`
- `Messaging.Email`
- `Data.Reference.CustomMetadata`

Depth targets:

- Custom metadata visibility and access patterns.
- `Messaging.SingleEmailMessage`, send capture, limits, and rollback.
- Metadata service shape only where tests compile or branch on it.

Primary sentinels:

- `src-nmb-nu-develop`
- `nams-workspace`
- `NPSP-rel-3.237`
- `sf-cred-pkg-develop`

### P6: Passive Breadth And External Products

Goal: keep reference breadth visible without stealing runtime squads.

Packet areas:

- `ConnectApi.PassiveDTOs`
- `Slack.*`
- `Commerce.*`
- `Tooling.Objects`
- `REST.Resources`
- AI, Agentforce, Marketing, Pub/Sub, GraphQL, and external product references.

Depth targets:

- Constructors, properties, enum constants, DTO fields, and explicit unsupported
  diagnostics.
- Active service methods only when corpus usage or a requested server API needs
  them.
- Ledger identity cleanup for `unknown` runtime/catalog rows.

Primary sentinels:

- `apex-recipes-main`, for the small real `ConnectApi` surface.
- Dedicated fixtures, not enterprise runtime gates, for passive DTO breadth.

ConnectApi has the largest ledger pressure:

| Namespace | Pressure | Gap | Unsupported | Passive |
| --- | ---: | ---: | ---: | ---: |
| ConnectApi | 24,988 | 24,519 | 469 | 29,108 |
| System | 3,580 | 3,562 | 18 | 645 |
| Schema | 1,504 | 1,504 | 0 | 181 |
| Metadata | 756 | 756 | 0 | 320 |
| Tooling | 327 | 327 | 0 | 0 |
| REST | 309 | 309 | 0 | 0 |

That is coverage debt. It is not the first runtime frontier.

## Agent Handoff Template

Each agent gets one vertical packet and one corpus sentinel:

```text
Goal: move <project> through the next measured blocker in <vertical>.
Packet: <docs/agent-packets/salesforce/...md>
Allowed files: packet-owned files only.
Do first: add or update a focused fixture proving the Salesforce behavior.
Then: implement the narrow runtime/server/type shape.
Validate: focused Go tests plus the smallest local-test command that hits the
blocker.
Report: before/after blocker count, changed files, validation, remaining risk.
```

Do not give an agent a namespace alone. Give it a corpus blocker, a packet, and
a ratchet command. Otherwise it will sand a board that does not go into the
cabin.

## Next Packets To Generate

Generate packets in this order:

1. `Data.Reference.SchemaDescribe`
2. `Data.Runtime.SOQLDML`
3. `Core.Runtime.Database`
4. `Tests.Async`
5. `UI.ApexPagesControllers`
6. `Integration.HTTPCallouts`
7. `Core.Runtime.JSON`
8. `Core.Runtime.UserInfoFeatureManagement`
9. `Messaging.Email`
10. `ConnectApi.PassiveDTOs`

Keep `Approval.Process` deferred until a real project uses it or a user asks for
that vertical. The local real-project scan found no non-stub usage.

