# Salesforce Agent Surface Todo

> **For agentic workers:** Start with the setup items in order. Setup is green only after the expanded local reference shelf passes the zero-gap coverage gate. After that, claim one area packet at a time. Do not start broad Salesforce runtime work without a packet, a baseline ledger, and a ratchet target.

**Goal:** Make Salesforce feature breadth and depth work repeatable for AI agents.

**Operating Shape:** Agents enter through the generated surface ledger and an area packet. Each agent claims one area, explains the rows for that area, fixes identity defects before runtime defects, adds fixture evidence before capability claims, and records before/after counts. Older oracle, docs inventory, reconcile, coverage, and probe commands stay available as lower-level tools; they are not the front door for new breadth work.

**Primary Inputs:** Salesforce docs workspace copy, Tooling completions snapshot, Glade standard symbols, capability rows, compat fixtures, oracle evidence, focused Go tests.

---

## Surface Universe Todo

- [x] **A1. Treat current scrape coverage as verified scope, not all Salesforce**

  The current checked workspace docs scrape still covers these six legacy baseline docsets:

  ```text
  apex=2233 manifest entries
  visualforce=162 manifest entries
  lightning-aura=157 manifest entries
  lwc=23 manifest entries
  rest-api=309 manifest entries
  tooling-api=339 manifest entries
  ```

  `go run ./cmd/glade compat docs-inventory --source "$GLADE_SALESFORCE_DOCS_SOURCE"` currently reports:

  ```text
  documents=3224
  members=5177
  namespaces=89
  ```

  The scraper has now been widened so the next scrape can include first-class Atlas references and modern docs-site verticals. The old six-docset scrape is no longer described as every public Salesforce developer surface.

- [x] **A2. Add a source-universe registry**

  Create one checked registry that lists every known Salesforce source family and its scrape status. Seed it from the scraper source priority list:

  ```text
  apex                         scraper-supported, baseline-present
  apex-guide                   scraper-supported
  soql-sosl                    scraper-supported
  object-reference             scraper-supported
  field-reference              scraper-supported
  visualforce                  scraper-supported, baseline-present
  lightning                    scraper-supported
  lwc                          scraper-supported, baseline-present
  rest-api                     scraper-supported, baseline-present
  tooling-api                  scraper-supported, baseline-present
  metadata-api                 scraper-supported
  soap-api                     scraper-supported
  bulk-api                     scraper-supported
  ui-api                       scraper-supported
  platform-events              scraper-supported
  streaming-api                scraper-supported
  connect-rest-api             scraper-supported
  service-connector-api-reference scraper-supported
  limits-reference             scraper-supported
  cli-reference                scraper-supported
  analytics-cli-reference      scraper-supported
  commerce-cli-reference       scraper-supported
  site-references              scraper-supported-from-sitemap
  pub-sub-api                  covered-through-site-references
  graphql-api                  covered-through-site-references
  agentforce                   covered-through-site-references
  marketing-cloud-ampscript    covered-through-site-references
  sf-connect-amazon-rds        covered-through-site-references
  ```

  Done. `compat surface sources` now reports covered, missing, and partial
  docsets from the checked source-universe registry.

- [x] **A2a. Promote the expanded scraper output into the docs workspace**

  Run the expanded scrape into a fresh directory first:

  ```bash
  cd "example-projects/Salesforce Docs Scraper/salesforce-scraper"
  python3 scrape.py --reference-sources
  python3 scrape.py \
    --docsets apex apex-guide object-reference field-reference soql-sosl visualforce lightning lwc rest-api tooling-api metadata-api soap-api bulk-api ui-api platform-events streaming-api connect-rest-api service-connector-api-reference limits-reference cli-reference analytics-cli-reference commerce-cli-reference \
    --version latest \
    --concurrency 2 \
    --request-delay 0.5 \
    --output /tmp/salesforce-docs-expanded
  python3 scrape.py \
    --docsets site-references \
    --site-metadata ../sitemap.json \
    --concurrency 2 \
    --output /tmp/salesforce-docs-expanded
  python3 scrape.py \
    --reference-coverage \
    --site-metadata ../sitemap.json \
    --output /tmp/salesforce-docs-expanded
  ```

  Merge only after checking manifest counts, `_catalog.md`, and spot pages for Apex Reference Guide, Object Reference for the Salesforce Platform, Salesforce Field Reference Guide, AMPscript, Agentforce, GraphQL, Pub/Sub API, and Amazon RDS Connect.

  Done when `REFERENCE_COVERAGE.md` reports `Atlas docsets missing: 0`,
  `Atlas docsets partial: 0`, `lwc` as `present`, and
  `Missing local markdown: 0` for the expanded docs source, or the plan records
  why a docset stayed external.

  Current default workspace source:

  ```text
  example-projects/Salesforce Docs Scraper/salesforce-docs
  manifest=15634
  Atlas docsets missing=0
  Atlas docsets partial=0
  Missing local markdown=0
  ```

  The previous six-docset shelf is preserved at
  `example-projects/Salesforce Docs Scraper/salesforce-docs-legacy-six-docset`.

- [x] **A2b. Add a repeatable all-reference coverage audit**

  Current command:

  ```bash
  cd "example-projects/Salesforce Docs Scraper/salesforce-scraper"
  python3 scrape.py \
    --reference-coverage \
    --site-metadata ../sitemap.json \
    --output ../salesforce-docs
  ```

  Current measured workspace result:

  ```text
  atlasDocsetsPinned=21
  atlasDocsetsMissing=17
  atlasDocsetsPartial=0
  siteReferenceProjects=55
  siteReferenceHrefs=211
  plannedSiteMarkdown=648
  missingLocalMarkdown=648
  ```

  That proves the current checked workspace docs source is still the legacy baseline, not the expanded complete local reference shelf.

- [x] **A2c. Add direct static-source capture for docs-site API references**

  The scraper should keep Playwright for session setup and rendered markdown
  pages, but some modern reference pages publish static source payloads. Network
  inspection found examples such as:

  ```text
  /static/genai/agentforce/models-api/models.yaml.amf.json
  /static/genai/agentforce/agent-api/agents.yaml.amf.json
  /static/graphqlapi/graphql/graphql/graphql.yaml
  ```

  Use `sourceFilename`, `amfFilename`, and `renderWith` from `sitemap.json` to
  attempt a direct static fetch first for OpenAPI, RAML, AMF, and YAML
  references. Fall back to rendered-page markdown when the static path cannot be
  derived or returns a non-200 response.

  Initial derivation rule from the current sitemap:

  ```text
  source=/dist/repos/{repo}/content/en-us/.../{sourceFilename}
  static=/static/{repo-with-hyphens-removed}/{projectId}/{reference.id}/{sourceFilename}
  amf=/static/{repo-with-hyphens-removed}/{projectId}/{reference.id}/{amfFilename}
  ```

  Fetch static first for `rest-raml/mulesoft`, `rest-oa3/mulesoft`,
  `rest-oa2/mulesoft`, and `rest-oa3/redoc`. Do not rely on this blindly:
  Agentforce, GraphQL, and B2C examples fit the pattern, but live static URLs can
  still return 403/404 and RAML can require includes. Store static payloads under
  `_sources/` and keep markdown as the agent-facing index.

  Done. The expanded docs shelf under the default workspace source now includes
  static source metadata in site-reference manifests, and `compat surface
  sources` verifies the `site-references` family before feature packets start.
  Future scraper work should keep `_references.json` `sourceFetch` values as the
  static/rendered/metadata audit trail.

- [x] **A3. Add a docs completeness audit command**

  Target command shape:

  ```bash
  go run ./cmd/glade compat surface sources \
    --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
    --check docs/generated/salesforce/SURFACE_SOURCES.md
  ```

  The command should verify required files, product folders, manifest counts, source families not yet scraped, and `site-references/_catalog.md` when modern vertical references are present.

  Done. Current command:

  ```bash
  go run ./cmd/glade compat surface sources \
    --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
    --check docs/generated/salesforce/SURFACE_SOURCES.md
  ```

- [x] **A4. Add missing docset capture rules**

  If the audit finds a missing source family, choose one action:

  ```text
  add scraper support
  copy improved docs into the workspace docs source
  mark the family out-of-current-runtime-scope
  add a small public-doc-backed fixture for a narrow behavior
  ```

  Done. `compat surface sources --output` writes explicit
  `missing-source-family:*` rows and lists the allowed actions; `--check`
  compares the checked report without modifying it.

- [x] **A4a. Split reference inventory from behavior evidence**

  Use reference docs as the source of surface shape. Use guide pages and focused fixtures as the source of behavior, limits, exception text, transaction semantics, and test behavior.

  Done. Ledger rows now carry:

  ```text
  shapeSource=reference
  behaviorSource=guide-or-fixture
  implementationDecision=runtime|server|metadata|explicit-unsupported|external-product
  ```

- [x] **A5. Separate "Salesforce public surface" from "Glade implementation target"**

  Done. Each surface row carries both fields:

  ```text
  salesforceSurfaceFamily
  gladeImplementationTarget
  ```

  Example:

  ```text
  salesforceSurfaceFamily=pub-sub-api
  gladeImplementationTarget=server-or-explicit-unsupported

  salesforceSurfaceFamily=apex
  gladeImplementationTarget=runtime
  ```

  Done when the ledger can list a public Salesforce surface even when Glade chooses explicit unsupported behavior for now.

## Setup Todo

- [x] **1. Pin the docs input path**

  Use the workspace docs copy as the default docs source:

  ```bash
  export GLADE_SALESFORCE_DOCS_SOURCE="$PWD/example-projects/Salesforce Docs Scraper/salesforce-docs"
  test -d "$GLADE_SALESFORCE_DOCS_SOURCE"
  ```

  Fallback only if the workspace copy is missing:

  ```bash
  export GLADE_SALESFORCE_DOCS_SOURCE="/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper (1)/salesforce-docs"
  ```

  Done when the path points to the expanded local reference shelf and
  `REFERENCE_COVERAGE.md` reports:

  ```text
  Atlas docsets missing: 0
  Atlas docsets partial: 0
  lwc: present
  Missing local markdown: 0
  ```

- [x] **2. Add a docs-source health check**

  Add a small check that reports page counts by product folder and verifies:

  ```text
  salesforce-docs/manifest.json exists
  salesforce-docs/search-index.json exists
  apex/apex_class_System_FeatureManagement.md exists
  apex/apex_methods_system_database.md exists
  ```

  Done when an agent can run one command and know whether to use the scrape, re-scrape, or stop.

  Current command:

  ```bash
  go run ./cmd/glade compat surface sources --docs "$GLADE_SALESFORCE_DOCS_SOURCE"
  ```

- [x] **3. Keep raw docs out of git unless deliberately imported**

  The workspace copy under `example-projects/Salesforce Docs Scraper/salesforce-docs` is the default input. Default rule: use that workspace copy, and check in only generated summaries or small fixtures unless the docs copy is deliberately promoted into version control.

  Done. `docs/ADDING_A_PLATFORM_API.md` names the external workspace docs input
  and the checked outputs/fixtures agents may commit.

- [x] **4. Create a baseline surface refresh command**

  Standard command:

  ```bash
  tmp="$(mktemp -d)"
  go run ./cmd/glade compat surface refresh \
    --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
    --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
    --out "$tmp"
  ```

  Record the baseline counts from stdout. The recent measured baseline was:

  ```text
  implemented=64 partial=33 passive=24515 explicitUnsupported=617
  missingShape=33070 missingEvidence=26843 staleGlade=30347
  ```

  Done when the command is documented as the first step for every packet.

  Current expanded-source baseline:

  ```text
  implemented=72 partial=33 passive=44590 explicitUnsupported=634
  missingShape=16697 missingBehavior=0 missingEvidence=33401
  parser=0 docsOrgMismatch=0 staleGlade=3692 passiveServiceRisk=0
  ```

## Ledger Truth Todo

- [x] **5. Normalize Apex method IDs across docs, org, Glade, and evidence**

  Fix the current false split:

  ```text
  apex:System.FeatureManagement.checkPackageBooleanValue(apiName)
  apex:System.FeatureManagement.checkPackageBooleanValue(String)

  apex:System.Database.executeBatch(batchClassObject,scope)
  apex:System.Database.executeBatch(Object,Integer)
  ```

  Done. Fresh ledger rows now join:

  ```text
  apex:System.FeatureManagement.checkPackageBooleanValue(String)
  apex:System.Database.executeBatch(Object)
  apex:System.Database.executeBatch(Object,Integer)
  ```

- [x] **6. Normalize docs parameter names to parameter types**

  Use docs method declarations when present, not the link text alone:

  ```text
  public static Boolean checkPackageBooleanValue(String apiName)
  public static ID executeBatch(Object batchClassObject, Integer scope)
  ```

  Done. Docs rows read `#### Signature` declarations and re-key method rows by
  parameter type, including overloaded methods.

- [x] **7. Normalize property rows and generated getter rows**

  Join documented properties to generated Glade getter behavior where that is the same Salesforce surface.

  Done. Rows like these no longer produce one missing row and one stale row:

  ```text
  apex:ApexPages.Component.childComponents
  apex:ApexPages.Component.childComponents()
  ```

- [x] **8. Add ledger identity tests**

  Add focused tests under `internal/surfaceledger` for:

  ```text
  FeatureManagement.checkPermission(apiName) -> FeatureManagement.checkPermission(String)
  Database.executeBatch(batchClassObject,scope) -> Database.executeBatch(Object,Integer)
  ApexPages.Component.childComponents -> ApexPages.Component.childComponents()
  ```

  Done. `go test ./internal/surfaceledger` covers typed docs parameters,
  Tooling `APEX_OBJECT` normalization, and property IDs without call parens.

## Agent Packet Todo

- [x] **9. Add an area registry**

  Define the initial areas and row filters:

  ```text
  Ledger.Identity
  Core.Runtime.System.FeatureManagement
  Core.Runtime.Database.Batchable
  Core.Runtime.SystemAndStdlib
  Query.Runtime.SOQLSOSL
  Data.Reference.ObjectsFields
  Data.Runtime.SchemaDescribe
  Data.Runtime.SOQL
  Data.Runtime.DML
  Tests.AsyncAndIsolation
  UI.ApexPagesControllers
  UI.VisualforceComponents
  UI.LWCModules
  UI.AuraComponents
  UI.UIAPI
  Server.RESTResources
  Server.ToolingObjects
  Integration.GraphQL
  Integration.PubSub
  Integration.BulkAPI
  Integration.MetadataAPI
  Integration.SOAPAPI
  Integration.StreamingAPI
  Integration.SalesforceConnect.AmazonRDS
  Platform.Events
  AI.Agentforce
  External.MarketingCloud.AMPscript
  External.MarketingCloud.Handlebars
  ConnectApi.PassiveDTOs
  ```

  Done. `internal/surfaceledger.AreaRegistry()` names these areas and
  `compat surface packet --area <name>` resolves them.

- [x] **10. Add the packet markdown template**

  Each packet must contain:

  ```text
  Area
  Baseline command
  Ledger row filter
  Rows to explain first
  dependsOn
  mayRunInParallelWith
  sharedFiles
  exclusiveFiles
  Allowed files
  Blocked files
  Required fixtures
  Focused tests
  Done criteria
  Ratchet target
  Area ratchet command
  Handoff format
  ```

  Done. `surfaceledger.PacketMarkdown` emits the named area, baseline command,
  row filter, dependency rules, file boundaries, fixtures, tests, done criteria,
  ratchet target, area ratchet command, and handoff format.

- [x] **11. Add a packet generator command**

  Target command shape:

  ```bash
  go run ./cmd/glade compat surface packet \
    --ledger "$tmp/SURFACE_LEDGER.json" \
    --area FeatureManagement \
    --out docs/agent-packets/salesforce/FeatureManagement.md
  ```

  Done. Current command:

  ```bash
  go run ./cmd/glade compat surface packet \
    --ledger "$tmp/SURFACE_LEDGER.json" \
    --area Core.Runtime.System.FeatureManagement \
    --out docs/agent-packets/salesforce/FeatureManagement.md
  ```

- [x] **12. Add ratchet output**

  Each packet should close with before/after counts:

  ```text
  area=FeatureManagement
  before missingShape=6 missingEvidence=7 failure=0
  after  missingShape=0 missingEvidence=0 failure=0
  ```

  Done. Packets include a ratchet target and area ratchet command; closeout
  item 24 still owns the final before/after reporting discipline.

## Vertical Slice Separation Todo

- [x] **B1. Define packet ownership boundaries**

  A packet owns one Salesforce concern and one Glade implementation target:

  ```text
  Core.Runtime.*        -> internal/vm, internal/typesys, internal/capability, docs/fixtures
  Query.Runtime.*       -> internal/soql, internal/sema, internal/vm, docs/fixtures
  Data.Reference.*      -> internal/schema, internal/storage metadata overlays, docs/fixtures
  Data.Runtime.*        -> internal/schema, internal/sobject, internal/soql, internal/dml, internal/storage, internal/vm
  Tests.*               -> internal/apextest, internal/vm async/test support, docs/fixtures
  UI.*                  -> internal/visualforce, internal/vm ApexPages/PageReference, component docs
  Server.*              -> internal/server, internal/storage, REST/Tooling fixtures
  Integration.*         -> server/API shape or explicit unsupported rows
  Platform.Events       -> event metadata, publish semantics, async hooks, or explicit unsupported rows
  AI.*                  -> Apex/REST/Metadata rows if they affect Glade, otherwise external-product rows
  External.*            -> external-product rows kept visible without forcing runtime implementation
  ConnectApi.*          -> passive DTO shape separated from active service behavior
  ```

  Done when packet generation includes the owner, allowed files, blocked files,
  `dependsOn`, `mayRunInParallelWith`, `sharedFiles`, and `exclusiveFiles`.

- [x] **B2. Split shape, behavior, and evidence tasks inside every packet**

  Every packet must preserve this order:

  ```text
  shape -> behavior -> evidence -> capability/docs -> refresh/check
  ```

  Done when agents do not claim support from type shape alone.

- [x] **B3. Keep shared identity work out of feature packets**

  `Ledger.Identity` owns canonical IDs, source-family classification, property/getter joins, and docs parameter typing. Feature packets may add examples, but they must not create one-off ID rules.

  Done. `internal/surfaceledger` has shared tests for typed Apex signatures,
  namespace normalization, generic parameter normalization, property/getter
  joins, Schema namespace inference, zero-arg evidence inference, hidden
  character stripping, and acronym casing.

- [x] **B4. Keep active services separate from passive DTO breadth**

  Some Salesforce namespaces are broad DTO forests. Treat them as two packet types:

  ```text
  PassiveDTO packet: constructors, fields/properties, enum constants, default local shape.
  ActiveService packet: call methods, side effects, HTTP/server behavior, explicit unsupported diagnostics.
  ```

  Done. Packet rows and generated packet templates carry
  `gladeImplementationTarget`, and the registry keeps `ConnectApi.PassiveDTOs`
  separate from active server/runtime service packets.

  Done when ConnectApi, GraphQL, Pub/Sub, Metadata, Bulk, and Tooling service calls cannot hide inside passive DTO counts.

- [x] **B5. Add parallel-work safety rules**

  Parallel packets may share immutable docs snapshots, Tooling snapshots, type shape, and generated reports. They must not share mutable runtime behavior without a declared dependency.

  Done when each packet lists:

  ```text
  dependsOn
  mayRunInParallelWith
  sharedFiles
  exclusiveFiles
  areaRatchetCommand
  ```

## Existing Tool Cleanup Todo

- [x] **13. Make `compat surface refresh` the documented front door**

  Update the runbooks so broad Salesforce feature work starts here:

  ```bash
  go run ./cmd/glade compat surface refresh \
    --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
    --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
    --out "$tmp"
  ```

  Done. `docs/ADDING_A_PLATFORM_API.md`, `docs/README.md`, and this plan now
  point to `compat surface sources`, `compat surface refresh`, and
  `compat surface packet` as the front-door sequence.

- [x] **14. Mark lower-level surface commands as debugging tools**

  Keep these commands, but document them as inspection tools used after `refresh`:

  ```text
  compat surface docs
  compat surface org
  compat surface glade
  compat surface evidence
  compat surface ledger
  compat surface gaps
  compat surface explain
  compat surface check
  ```

  Done. `docs/ADDING_A_PLATFORM_API.md` now names these as inspection commands
  used after `refresh`.

- [x] **15. Fold docs inventory, catalog, reconcile, and doc-contracts under the ledger**

  Keep these commands for parser and docs debugging:

  ```text
  compat docs-inventory
  compat catalog
  compat reconcile
  compat doc-contracts
  ```

  They should feed the ledger or explain a ledger row. They should not create a separate agent work queue.

  Done. `docs/ADDING_A_PLATFORM_API.md` now says to start with surface refresh
  and use docs inventory/reconcile only when the docs/catalog join is suspect or
  Apex rows need inspection.

- [x] **16. Make oracle a packet evidence tool**

  Keep `compat oracle`, but route it from ledger rows or packet rows:

  ```bash
  go run ./cmd/glade compat oracle inventory \
    --ledger "$tmp/SURFACE_LEDGER.json" \
    --gap-class missing-evidence \
    --limit 25 \
    --output "$tmp/ORACLE_INVENTORY.json"
  ```

  Done. `compat oracle inventory --ledger --gap-class --limit` already exists,
  and `docs/generated/apex-oracle/README.md` now starts from the ledger.

- [x] **17. Rewrite generated oracle README**

  Update `docs/generated/apex-oracle/README.md` so its regenerate path is ledger-first, not stub-first:

  ```text
  surface refresh -> oracle inventory --ledger -> domains -> plan -> generate/run/diff/promote
  ```

  Done. The old `--stubs <stub-root>` path is labeled legacy.

- [x] **18. Update Salesforce coverage next gates**

  `internal/capability/salesforce_coverage.go` currently points next gates at docs inventory, catalog, and coverage. Change those next gates to point at:

  ```text
  compat surface refresh
  compat surface packet
  compat surface check
  ```

  Done. `internal/capability/salesforce_coverage.go` now emits next gates for
  `compat surface refresh`, `compat surface packet`, and `compat surface check`.

- [x] **19. Reclassify probe and stub discovery outputs**

  Treat these as lab artifacts:

  ```text
  docs/generated/apex-oracle/PROBE_MANIFEST*.json
  docs/generated/apex-oracle/WORK_QUEUE*.json
  docs/generated/STUB_DISCOVERY*.json
  docs/generated/STUB_CONTRACT_PROBE_MANIFEST.json
  ```

  Done. `docs/generated/apex-oracle/README.md` and
  `docs/BEHAVIORAL_STUB_SUPPORT_PLAN.md` now classify these as lab evidence or
  packet evidence inputs, not the broad Salesforce implementation backlog.

## First Area Packets

- [x] **20. Packet: Ledger.Identity**

  Goal: remove false split rows before runtime work.

  Required validation:

  ```bash
  go test ./internal/surfaceledger
  go run ./cmd/glade compat surface refresh \
    --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
    --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
    --out "$(mktemp -d)"
  ```

  Done. FeatureManagement and Database batch examples no longer appear as
  paired missing/stale rows in a fresh surface refresh.

- [x] **21. Packet: FeatureManagement**

  Goal: finish shape, behavior, evidence, capability rows, and docs for supported FeatureManagement methods.

  Rows must include:

  ```text
  changeProtection
  checkPermission
  checkPackageBooleanValue
  setPackageBooleanValue
  checkPackageIntegerValue
  setPackageIntegerValue
  checkPackageDateValue
  setPackageDateValue
  ```

  Done. The supported rows listed above are implemented with
  `docs/fixtures/core-feature-management.json` fixture evidence. Constructor and
  inherited `clone()` rows remain outside this packet's listed method scope.

- [x] **22. Packet: Database.Batchable**

  Goal: finish ledger shape and evidence for the supported batch lifecycle.

  Rows must include:

  ```text
  Database.executeBatch
  Database.Batchable.start
  Database.Batchable.execute
  Database.Batchable.finish
  Database.BatchableContext.getJobId
  Database.BatchableContext.getChildJobId
  AsyncApexJob state from local test drain
  ```

  Done. `docs/fixtures/async-batchable-lifecycle.json` covers both
  `Database.executeBatch` overloads plus `Database.Batchable.start`,
  `execute`, `finish`, `Database.BatchableContext.getJobId`, and
  `getChildJobId`. Ledger identity now joins docs `List<sObject>`, Tooling
  `List<ANY>`, and Glade `List<Object>` as one `List<Object>` row. A fresh
  surface refresh shows the lifecycle rows implemented with fixture evidence
  and no false missing-shape split for the batch interface methods.

- [x] **23. Packet: Schema.Describe**

  Goal: expand data-runtime breadth through describe surfaces used by local tests.

  Start with rows in `Schema`, `SObjectType`, field describe, record type describe, and child relationship describe.

  Done. The first packet is narrowed to local metadata-backed describe behavior:
  `Schema.getGlobalDescribe`, `Schema.describeSObjects(List<String>)`,
  `SObjectType.getDescribe`, object labels/fields/key prefix, field label/type/
  relationship/picklist values, record type names, and child relationship names/
  cascade flags. Existing fixtures now pin canonical `surfaceId` values and a
  fresh surface refresh shows those rows implemented or passive with fixture
  evidence. The documented `Schema.describeSObjects(List<String>,Object)`
  overload remains a real runtime gap and stays outside this first packet.

- [x] **23A. Packet: Core.Runtime.SystemAndStdlib family chunk**

  Reviewed 2026-06-05. The completed chunk added exact shape/evidence for a
  broad set of already-modeled local stdlib rows: `AccessLevel` explicit
  unsupported behavior, `ApexPages` message/current-page/test-callback rows,
  `Test.createStub*` and SOQL stub rows, `Assert` overload evidence, exact
  `Limits` getter evidence, exact common `String` evidence, and exact
  `Boolean.valueOf(String|Object)` evidence.

  Review fixes: added exact `ApexPages.addMessages(Exception|Object)` and
  `Boolean.valueOf(String|Object)` shape/evidence so supported VM behavior no
  longer appears as top missing-shape or missing-evidence rows.

  Fresh refresh:

  ```text
  implemented=55122 partial=31 passive=46986 explicitUnsupported=692
  gaps: missingShape=12239 missingBehavior=0 missingEvidence=7635
  failures: parser=0 docsOrgMismatch=0 staleGlade=0 passiveServiceRisk=0
  ```

  Remaining top rows are real missing-shape families such as `Answers`,
  `Approval`, `BusinessHours`, `DMLOptions`, and broad System reference pages.

- [x] **23B. Packet: Data.Reference.ObjectsFields generated shape chunk**

  Reviewed 2026-06-05. Generated standard SObject and field rows now enter the
  surface ledger as `gladeShape=generated` from
  `standard-sobject-generated-shape`, with sema/SOQL sentinel coverage for
  generated standard objects and fields.

  Review fix: merged docs-backed object rows were still classified as
  `missing-evidence` because docs kept `shapeSource=reference`. Classification
  now also honors the merged generated-shape source list. That moved the
  generated object rows into `implemented` without requiring fixture evidence
  for every reference-backed field.

  Fresh packet top rows are now true `missing-shape` gaps, starting with
  entitlement- or org-feature-backed objects such as `AIFeatureExtractor`,
  `AIPredictionEvent`, and `AccountForecast`. Use the max scratch-org probe
  config before claiming those rows.

## Agent Closeout Todo

- [x] **24. Require a standard validation block**

  Every packet closeout must report:

  ```text
  focused tests run
  fixture command run
  surface refresh run
  area ratchet command run
  before counts
  after counts
  next top row
  ```

  Include `go test ./internal/repoguard` after code changes.

  Done. `compat surface packet` now emits this block, and
  `docs/ADDING_A_PLATFORM_API.md` carries the same closeout rule.

- [x] **25. Add docs for re-scrape or docs repair**

  If a docs row is missing or malformed, the agent must choose one path:

  ```text
  re-scrape docs
  copy improved docs into the external docs source
  patch the docs parser to read existing docs correctly
  add a small checked fixture if public docs are ambiguous
  ```

  Done. The packet template and platform API runbook tell agents to choose one
  docs repair path before runtime work and not invent runtime behavior to cover
  docs defects.

- [x] **26. Add a reviewer checklist**

  Reviewers check:

  ```text
  no corpus-specific runtime hacks
  public Salesforce behavior cited by docs or fixture
  shape and behavior are not claimed without evidence
  packet area did not expand during work
  generated docs are updated when capability changes
  ```

  Done. The packet template and runbook include the reviewer checklist.

- [x] **27. Start breadth work only after packets are ready**

  First breadth agents should run in this order:

  ```text
  Ledger.Identity
  FeatureManagement
  Database.Batchable
  Schema.Describe
  ApexPages.Controllers
  REST.Resources
  Tooling.Objects
  ```

  Done. The packet template and runbook pin the breadth work order after
  packet readiness.
