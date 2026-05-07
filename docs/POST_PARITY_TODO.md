# Post-Parity Todo

Status date: 2026-05-06.

This is the follow-on work after `docs/FEATURE_PARITY_TODO.md` is complete.
It assumes `oaer` can already parse, check, and run real Apex tests with
credible SObject, SOQL, DML, trigger, async, limit, fixture, server, LSP, DAP,
and compatibility behavior.

The squad-oriented implementation plan for full local Apex test execution lives
in `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`.

Every item in Part I is intended to be additive to the initial parity todo. If a
topic overlaps a parity area, this document only tracks the next layer needed by
legacy projects. For example, parity covers HTTP callout mocks; this document
adds named credential and remote site metadata resolution. Parity covers custom
metadata symbols and storage; this document adds legacy `.md` source loading,
large metadata fixtures, and Apex `Metadata.*` deployment behavior.

When auditing or planning behavior, prefer public Salesforce documentation,
public grammars, owned fixtures, and black-box compatibility tests. A local
public-docs mirror may be available at:
`/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs`.
Use it as a reference index for Salesforce behavior and API shapes; do not copy
documentation text into source or treat it as an implementation substitute for
compatibility fixtures.

Post-parity starts with one job: run local Apex tests for large old projects.
That means supporting enough of the older Salesforce surfaces that tests can
load metadata, execute Apex controllers and services, and observe the same data
side effects they would see on platform. Full local running of Visualforce,
Aura, LWC, Experience Cloud, and other UI surfaces comes later and has a clear
line below.

The first post-parity layer includes legacy UI controller contracts, Workflow
Rules, autolaunched or record-triggered Flow, Process Builder-style automation,
labels, email templates, site/community context, org presentation metadata, and
selected platform namespaces. These are not required for the core Apex runtime
to be useful, but they block projects that mix modern Apex tests with legacy UI
and declarative automation.

Current checked status:

- The server-example support gate is green:
  `pass=101 fail=0 unsupported=0 missing=0`.
- The broader post-parity readiness inventory is green for the checked
  `example-projects` corpus. A May 6, 2026
  `oaer compat post-parity --json` run reported 51,075 files scanned, 0
  findings, 0 test-blocking findings, and 0 surfaces. This means the known
  scanner/test-readiness blockers for standard schema references, labels,
  static resources, content assets, endpoint metadata, custom metadata type
  references, legacy presentation metadata, Visualforce controller/page and
  component contracts, Aura/LWC Apex discovery, Workflow save-order metadata,
  and modeled Flow record-lookup/record-create shapes have moved out of the
  post-parity blocker frontier.
- The zero-blocker inventory is not a blanket full-Salesforce claim. The owned
  local-test corpus is green, including local `Schema.describeTabs()` coverage,
  while runtime depth for UI rendering, full metadata mutation, advanced Flow
  interviews, and local UI/API serving remains tracked below.
- Treat this document as the source for broad local-test support beyond the
  green server-example harness.

The motivating audit targets are anonymized large old projects:

- Corpus A: roughly 2,899 Apex classes, 65
  triggers, 1,390 test-bearing classes, 141 Visualforce pages, 38 Aura
  components, 101 Workflow metadata files, 6 Flow metadata files, 208
  validation rules, 159 custom objects, 1,917 custom fields, 187 layouts, 81
  tabs, 29 web links, 21 permission sets, 8 profiles, 7 remote site settings,
  and 1 named credential.
- Corpus B: roughly 2,419 Apex classes, 5 triggers,
  1,205 test-bearing classes, 89 Visualforce pages, 133 Visualforce
  components, 13 Aura components, 38 LWC bundles, 65 legacy `.object` files,
  1,140 legacy `.md` custom metadata records, 25 layouts, 11 tabs, 2
  permission sets, 1 profile, 2 named credentials, 1 remote site setting, 1
  Workflow file, 1 Flow file, 3 static-resource files, and 2 content asset
  files.
- Across both projects there is heavy usage of `Label`, `ApexPages`,
  `PageReference`, `Page.*`, `System.Callable`, `Test.createStub`,
  `Metadata.*`, `Cache.*`, `Site.*`, `ConnectApi.*`, `URLFOR($Resource...)`,
  `$Site.Template`, HTTP callouts, email APIs, custom metadata, and
  file/document SObjects.

## Suggested Completion Order

1. Test-impact inventory: make the general project gap scanner report unsupported
   metadata and platform APIs that can block `oaer test`, especially
   Visualforce/Aura/LWC Apex controllers, Workflow, Flow, Process Builder,
   labels, email templates, site context, metadata APIs, endpoint metadata,
   static resources, content assets, and legacy Metadata API source files.
2. Read-only metadata ingestion for tests: load every metadata type needed for
   Apex symbol resolution, describe results, label values, controller
   references, automation rules, and local test fixtures.
3. Legacy UI controller test support: make Visualforce controllers,
   controller extensions, Aura-enabled Apex methods, and LWC-used Apex methods
   callable from tests without rendering or serving the UI.
4. Declarative test side effects: execute Workflow Rule field updates, email
   alerts, record-triggered or autolaunched Flow, Process Builder-style
   automation, and `@InvocableMethod` paths that fire during DML in tests.
5. Test-visible platform namespace behavior: implement deterministic local
   behavior for project-visible `Site`, `Cache`, `ConnectApi`, `Metadata`,
   `System.Callable`, `Test.createStub`, named credential, and remote site APIs.
6. Enterprise test release gates: add black-box fixtures and stress tests based
   on large old projects, then publish post-parity dashboards and gap reports
   focused on what still blocks `oaer test`.
7. Local UI/API running: after local tests pass, add optional Visualforce,
   Aura, LWC, Experience Cloud, email rendering, and local server execution
   surfaces for interactive or integration-style running.

## Critical Path For Full Local Test Running

This is the shortest path to running broad legacy-project Apex tests after the
initial parity todo is complete. It is based on `oaer inspect gaps` output from
the anonymized large-project corpus.

The highest-count blockers are not the same as the safest implementation order.
Load and resolve metadata first. Execute side effects only after the metadata
and controller contracts have a solid place to stand.

Current inventory status: the checked post-parity scan has no remaining
scanner/test-readiness blockers. Unchecked items below remain as runtime-depth,
behavioral fidelity, trace/debug visibility, release-hardening, or larger owned
fixture work, not as known blockers in the current `example-projects` inventory.

1. Keep the project gap scanner current.
   - [x] Detect the major unsupported surfaces in both audited legacy projects.
   - [x] Report blockers by capability, stage, file, line, symbol, examples, and
     top-blocker count.
   - [ ] Cross-check scanner capability names and API-shape assumptions against
     the local Salesforce docs mirror where available.
   - [ ] Add scanner baselines that keep the top blockers stable as project
     support improves.
   - [ ] Wire scanner output into a local-test readiness view.
2. Load the metadata needed before tests can resolve code.
   - [ ] Legacy Metadata API source format: `.object`, `.md`, `.labels`,
     `.layout`, `.profile`, `.permissionset`, `.tab`, `.workflow`, `.flow`,
     `.resource`, `.namedCredential`, and `.remoteSite`.
   - [ ] Legacy custom metadata records and large custom metadata fixture sets.
   - [ ] Custom labels and translations.
   - [ ] Static resources, content assets, and deterministic resource URLs.
   - [ ] UI presentation metadata needed by describe/controller tests: layouts,
     tabs, profiles, permission sets, web links, quick actions, value sets, and
     flexipages.
   - [ ] Named credential and remote site metadata as endpoint configuration, not
     callout execution.
3. Resolve test-facing UI controller contracts without running a browser.
   - [ ] Visualforce page metadata, `Page.*`, `PageReference`, page parameters,
     `ApexPages` messages, standard controllers, controller extensions, and
     component attribute bindings.
   - [ ] Aura bundle-to-Apex controller discovery.
   - [ ] LWC `@salesforce/apex`, `@wire`, `@salesforce/label`,
     `@salesforce/resourceUrl`, `@salesforce/schema`, and local `c/...` import
     discovery.
   - [ ] Wrapper serialization shapes used by Aura/LWC controller tests.
4. Add platform context and platform API contracts used by tests.
   - [ ] `System.Callable`, `System.StubProvider`, and `Test.createStub`.
   - [ ] Site, Network, Community, and guest/current-site context, including
     `$Site.Template`.
   - [ ] `Auth.*` namespace methods used by tests.
   - [ ] `ConnectApi.Organization.getSettings()` and Platform Cache basics.
   - [ ] Endpoint resolution from named credentials and remote site settings.
5. Add data and messaging side effects beyond core SObject/DML/SOQL parity.
   - [ ] `Attachment`, `Document`, `ContentVersion`, `ContentDocument`, and
     `ContentDocumentLink` binary-body behavior.
   - [ ] Email templates, merge context, captured email side effects, and email
     limit accounting.
6. Execute declarative automation in the test transaction.
   - [ ] Workflow Rule criteria, field updates, email alerts, recursive
     save-order behavior, and rollback.
   - [ ] Record-triggered/autolaunched Flow and Process Builder-style metadata
     that mutates records or calls `@InvocableMethod`.
   - [ ] Trace events for Workflow/Flow decisions and side effects.
7. Prove the whole local-test path with enterprise fixtures.
   - [ ] Fixture projects modeled after both audited legacy projects.
   - [ ] Compatibility tests for trigger/service/domain tests that depend on
     Visualforce controllers, custom metadata, labels, resources, sites,
     platform APIs, files, email, Workflow, and Flow.
   - [ ] A readiness gate for claiming "legacy-project-test-ready."

Current scanner top blockers from the broad post-parity inventory:

| Rank | Blocker |
| --- | --- |
| 1 | None. The May 6, 2026 checked scan reports 0 findings, 0 test-blocking findings, and 0 surfaces across 51,075 files. |

Recently cleared blocker families:

| Cleared surface | Result |
| --- | --- |
| Aura/LWC Apex discovery | No current post-parity findings. |
| UI and org presentation metadata | No current post-parity findings. |
| Visualforce controller/page/component contracts | No current post-parity findings. |
| Workflow rule save-order metadata | No current post-parity findings. |
| Custom label and translation resolution | No current post-parity findings. |
| Flow and Process Builder save-order metadata | No current post-parity findings. |

## Local Test Running Boundary

Everything before this boundary should serve local test execution first.

- [ ] A Visualforce page is in the test-running lane when Apex tests reference
  `Page.SomePage`, `PageReference`, page parameters, `ApexPages` messages,
  standard controllers, or controller extensions.
- [ ] Aura and LWC are in the test-running lane when Apex tests call the same
  `@AuraEnabled` methods, wrapper serializers, exceptions, or permissions that
  UI actions use.
- [ ] Workflow Rules are in the test-running lane when DML can trigger field
  updates, email alerts, recursive save-order behavior, or email limits.
- [ ] Flow and Process Builder are in the test-running lane when record-triggered
  or autolaunched automation can run from DML, call `@InvocableMethod`, mutate
  records, send email, or affect rollback.
- [ ] Labels, email templates, profiles, permission sets, layouts, global value
  sets, sites, named credentials, and remote site settings are in the
  test-running lane when Apex tests or automation resolve them.
- [ ] Static resources and content assets are in the test-running lane when
  Visualforce `URLFOR`, LWC `@salesforce/resourceUrl`, or controller tests assert
  generated URLs.

Below the boundary, local running means serving or rendering UI/API surfaces:
Visualforce markup rendering, Aura/LWC action endpoints and browser-like
lifecycle, Experience Cloud routing, email body rendering for users, and
interactive local server behavior. Those should not block the first
post-parity goal unless a test fixture needs the same behavior.

## Additive Scope Over Initial Parity

Part I should not duplicate `docs/FEATURE_PARITY_TODO.md`. Treat these as the
rules for deciding whether a task belongs here:

- [ ] Keep Apex parser, sema, VM, core SObject, SOQL, DML, trigger, async,
  limits, test isolation, storage, server, LSP, DAP, and trace baselines in the
  initial parity todo.
- [ ] Put work here only when a legacy project needs another metadata or platform
  layer on top of those baselines to run local tests.
- [ ] For callouts, keep request/response mocks in parity; put named
  credentials, remote site allowlists, and endpoint metadata resolution here.
- [ ] For permissions, keep `runAs`, basic user context, and permission
  enforcement in parity; put profile/permission-set metadata ingestion for
  page/class/tab/layout access here.
- [ ] For custom metadata, keep namespace-aware symbols, SObject access, SOQL,
  and storage basics in parity; put legacy `.md` source loading, large fixture
  sets, community configuration records, and Apex `Metadata.*` deploy/mutation
  behavior here.
- [ ] For files, keep generic SObject storage and ordinary DML/SOQL behavior in
  parity; put `Attachment`, `Document`, `ContentVersion`, `ContentDocument`, and
  binary-body side effects here.
- [ ] For trace/profile/DAP, keep baseline VM/SOQL/DML/trigger tracing in parity;
  put Workflow, Flow, label, resource, page-controller, metadata deploy, cache,
  and endpoint trace events here.
- [ ] For UI, keep only Apex-side APIs that parity already promised; put
  Visualforce page metadata, controller harnesses, Aura/LWC import graphs, and
  UI-controller test support here.

## Part I: Local Test Running First

This part is the first post-parity milestone. It exists to make `oaer test`
work for large old projects. Work here may load UI metadata and execute UI
controllers, but it should not require rendering pages, serving Aura/LWC, or
simulating a browser.

## Post-Parity Local Test Gate

- [ ] Add a post-parity capability area separate from the MVP parity gate.
- [x] Add `oaer compat post-parity` or an equivalent dashboard view that reports
  local-test-impacting legacy UI, declarative automation, metadata, and platform
  namespace support.
- [ ] Keep post-parity failures non-blocking for MVP release promotion, but
  blocking for claiming large legacy project test compatibility.
- [ ] Require every supported post-parity feature to have a black-box fixture or
  owned project fixture.
- [ ] Treat silent wrong behavior in post-parity surfaces as a blocker once that
  surface is marked supported.
- [ ] Prefer explicit unsupported diagnostics over partial no-op behavior for
  metadata-driven side effects.
- [ ] Include project-impact counts in gap reports so large-project blockers can
  be ranked by blast radius.

## 1. Project Audit And Gap Reporting

First implementation: `oaer inspect gaps [--project <root>] [--json]` scans
projects read-only and reports unsupported or not-yet-aligned surfaces by
capability, stage, metadata type, file, line, symbol, examples, and top
blockers. The scanner is general-purpose so it can evaluate any Salesforce
project against `oaer` support, not just post-parity work.

- [x] Add a project gap scanner for unsupported or not-yet-aligned surfaces:
  - [x] Visualforce pages, components, controllers, extensions, `Page.*`
    references, and `$Label`/`$ObjectType` expressions.
  - [x] Aura bundles, `@AuraEnabled` controller methods, component attributes,
    events, design files, and JavaScript controller/helper references.
  - [x] Workflow Rules, field updates, email alerts, outbound messages, tasks,
    and rule criteria.
  - [x] Flow metadata, process types, invocable actions, variables, decisions,
    assignments, and record operations.
  - [x] Custom labels, translations, email templates, layouts, tabs, web links,
    quick actions, global value sets, profiles, permission sets, sites,
    networks, remote site settings, named credentials, static resources, and
    content assets.
  - [x] Legacy Metadata API-format source files such as `.object`, `.md`,
    `.resource`, `.labels`, `.layout`, `.profile`, `.permissionset`, `.tab`,
    `.workflow`, `.flow`, and `.namedCredential`.
  - [x] Platform namespaces and APIs used by Apex: `Metadata`, `ConnectApi`,
    `Cache`, `Site`, `System.Callable`, `Test.createStub`, `Auth`, and
    endpoint configuration.
- [x] Emit stable unsupported-feature diagnostics with file, line, metadata type,
  symbol, and suggested capability ID.
- [x] Add JSON output for scanners so editor, CI, and dashboard tooling can rank
  gaps.
- [x] Add a "top blockers" report that combines unsupported feature count,
  affected files, metadata types, and examples.
- [x] Add scanner fixtures modeled after the anonymized large-project corpus
  surfaces without requiring
  proprietary behavior as an implementation source.
- [x] Add a text summary generated from project scan results.
- [x] Add gap output that separates load, resolve, and execute blockers; reserve
  render blockers for Part II local-running scans.

## 2. Visualforce Test Controller Support

The core parity todo covers `ApexPages`, `URL`, and `PageReference` basics.
Large old projects first need the Visualforce controller model around those
APIs so Apex tests can set page state, construct controllers, and assert
redirects/messages. Rendering markup is a later local-running concern.

- [ ] Parse and index Visualforce `.page` and `.component` metadata:
  - [ ] Page attributes such as `controller`, `standardController`,
    `extensions`, `action`, `showHeader`, `sidebar`, `applyHtmlTag`, and
    `renderAs`.
  - [ ] Component definitions, attributes, assign-to bindings, and nested custom
    component references.
  - [ ] Merge expressions in attributes and text nodes.
  - [ ] `$Label`, `$ObjectType`, `$CurrentPage`, `$User`, `$Setup`, and simple
    global merge references.
- [ ] Resolve `Page.SomePage` references to local page metadata.
- [ ] Implement `PageReference` behavior needed by controllers:
  - [ ] URL construction, `getUrl`, `setRedirect`, `getRedirect`, `getParameters`,
    headers, cookies, and request body stubs.
  - [ ] `ApexPages.currentPage()` isolation per test and per server request.
  - [ ] Current-page parameter setup and mutation in tests.
- [ ] Implement Visualforce controller construction:
  - [ ] Custom controllers with default constructors.
  - [ ] Standard controllers for SObjects and record IDs.
  - [ ] Controller extensions, including constructor overloads that accept
    `ApexPages.StandardController` or `StandardSetController`.
  - [ ] Action method invocation during page load.
- [ ] Implement `ApexPages` message behavior:
  - [ ] `ApexPages.Message`, `Severity`, `addMessage`, `getMessages`,
    `hasMessages`, and per-request/test isolation.
  - [ ] Message ordering and duplicate behavior close enough for controller
    tests.
- [ ] Implement a non-rendering Visualforce test harness:
  - [ ] Load a page by name.
  - [ ] Construct its controller and extensions.
  - [ ] Bind page parameters.
  - [ ] Invoke page action methods.
  - [ ] Validate redirect targets and messages.
- [ ] Defer full Visualforce rendering until the local-test boundary is crossed:
  - [ ] Render simple text, output panels, repeats, page blocks, forms, command
    buttons, and custom component inclusions.
  - [ ] Evaluate merge expressions against controller properties and SObjects.
  - [ ] Preserve deterministic output for snapshots.
- [ ] Add explicit unsupported diagnostics for Visualforce features not planned
  for local execution, such as full browser lifecycle, JavaScript remoting,
  view state serialization, and PDF rendering if not implemented.
- [ ] Add compatibility fixtures for:
  - [ ] Standard controller page with extension.
  - [ ] Page action redirect.
  - [ ] Page parameters and `Page.*` references.
  - [ ] Custom component attribute binding.
  - [ ] `$Label` and `$ObjectType` merge expressions.
  - [ ] `ApexPages` messages in tests.

## 3. Aura And LWC Apex Controller Test Support

Post-parity support should focus first on the server-side Apex contract, not a
complete browser runtime. Aura and LWC matter to local tests when their Apex
controllers, serializers, and exceptions are exercised directly.

- [ ] Parse and index Aura bundle metadata:
  - [ ] `.cmp`, `.app`, `.evt`, `.design`, `.auradoc`, controller `.js`, helper
    `.js`, renderer `.js`, style, and SVG files.
  - [ ] Component attributes, controller action references, event registrations,
    and design-time attributes.
- [ ] Resolve `@AuraEnabled` Apex methods to Aura action descriptors.
- [ ] Resolve LWC-used Apex methods through the same `@AuraEnabled` metadata
  and access checks.
- [ ] Parse LWC JavaScript imports enough to build a local test-impact graph:
  - [ ] `@salesforce/apex/Class.method`.
  - [ ] `@salesforce/label/...`.
  - [ ] `@salesforce/resourceUrl/...`.
  - [ ] `@salesforce/schema/...`.
  - [ ] `lightning/navigation` current page references.
  - [ ] `lightning/uiRecordApi` and `lightning/uiObjectInfoApi` imports.
  - [ ] Local `c/...` shared modules and pubsub helpers.
- [ ] Model LWC wire usage as metadata for test impact:
  - [ ] Wired Apex method references.
  - [ ] Reactive parameter names.
  - [ ] Current page reference dependencies.
  - [ ] Labels and static resources used by wired controllers.
- [ ] Implement Aura/LWC-style Apex invocation for local tests:
  - [ ] JSON argument decoding into Apex primitives, collections, SObjects, and
    wrapper classes.
  - [ ] Return-value encoding for primitives, collections, SObjects, wrapper
    classes, and errors.
  - [ ] Static method dispatch with sharing/user context.
  - [ ] Deterministic handling for cacheable methods.
- [ ] Implement `AuraHandledException` shape and message propagation.
- [ ] Defer local server endpoint support for Aura and LWC action calls until
  after controller tests pass, unless a fixture needs endpoint behavior.
- [ ] Add unsupported diagnostics for client-only Aura and LWC features not
  executed locally.
- [ ] Add compatibility fixtures for:
  - [ ] `@AuraEnabled` method discovery.
  - [ ] LWC `@salesforce/apex` import discovery.
  - [ ] LWC `@wire` Apex method discovery.
  - [ ] Wrapper argument and return serialization.
  - [ ] `AuraHandledException`.
  - [ ] Component-to-controller action mapping.
  - [ ] Cacheable method metadata.
  - [ ] LWC Apex import-to-method mapping where source metadata is present.

## 4. Workflow Rules And Test-Critical Declarative Side Effects

The core parity todo covers DML, triggers, validation, and metadata-backed
schema behavior. Workflow Rules add another old-platform execution layer after
DML, and they belong in the test-running lane when they change records, send
emails, or cause recursive save-order behavior.

- [ ] Load Workflow metadata files and model:
  - [ ] Rules, criteria formulas, active flags, actions, and evaluation order.
  - [ ] Field updates, email alerts, tasks, outbound messages, and flow actions.
  - [ ] Referenced templates, recipients, and target fields.
- [ ] Implement formula evaluation needed for Workflow criteria and field update
  formulas.
- [ ] Execute Workflow Rule field updates during DML in Salesforce-like order:
  - [ ] After before/after triggers at the correct point in the save order.
  - [ ] With recursive update behavior where field updates cause another pass.
  - [ ] With trigger re-entry behavior and recursion guards matching supported
    local limits.
  - [ ] With all-or-none rollback across DML, triggers, validation, and workflow
    side effects.
- [ ] Implement Workflow email alert capture:
  - [ ] Resolve email templates and recipients.
  - [ ] Record deterministic email-send side effects for tests.
  - [ ] Count email limits.
- [ ] Add unsupported diagnostics for outbound messages, tasks, and flow actions
  until implemented.
- [ ] Add fixture coverage for:
  - [ ] Rule criteria true/false paths.
  - [ ] Field update side effects visible to Apex after DML.
  - [ ] Workflow-triggered second update pass.
  - [ ] Workflow rollback on DML failure.
  - [ ] Email alert capture.

## 5. Flow, Process Builder, And Invocable Execution

Post-parity does not need a full Flow runtime at first. It does need enough to
avoid silent gaps where old projects use Flow or Process Builder as part of the
save order or call Apex invocable methods during local tests.

- [ ] Load Flow metadata and index process types, versions, status, variables,
  record triggers, and invocable action references.
- [ ] Add stable diagnostics for unsupported Flow elements.
- [ ] Implement minimal autolaunched Flow execution:
  - [ ] Variables, constants, assignments, decisions, simple formulas, and
    subflow diagnostics.
  - [ ] Record create/update/delete/get elements backed by local storage.
  - [ ] Apex action calls into `@InvocableMethod` with `@InvocableVariable`
    argument and result mapping.
  - [ ] Fault paths where representable.
- [ ] Model Flow execution in DML save order where record-triggered flows are
  enabled, including rollback and limit accounting.
- [ ] Implement Process Builder-style flow metadata enough to call invocable
  Apex and update records from local test DML.
- [ ] Add deterministic Flow interview IDs and trace events.
- [ ] Add compatibility fixtures for:
  - [ ] Invocable method metadata and argument mapping.
  - [ ] Record-triggered flow update.
  - [ ] Flow action that calls Apex.
  - [ ] Flow rollback with DML transaction failure.
  - [ ] Unsupported element diagnostics.

## 6. Custom Labels, Translations, And Localization

Large projects often compare exact label text in tests. Labels also appear in
Visualforce, Aura, and LWC metadata/source.

- [ ] Load `CustomLabels.labels-meta.xml` and translation metadata.
- [ ] Load legacy Metadata API-format `.labels` files.
- [ ] Resolve Apex `Label.Name` and `System.Label.Name`.
- [ ] Resolve Visualforce `$Label.Name`, `$Label.namespace.Name`, and site label
  references where metadata is available.
- [ ] Resolve LWC `@salesforce/label/...` imports to the same label table.
- [ ] Support namespace-token label references for package-style projects.
- [ ] Implement deterministic locale selection for tests, server requests, and
  `System.runAs` user context.
- [ ] Implement fallback behavior for missing translations.
- [ ] Preserve label values through JSON, Aura, Visualforce, and exception
  messages.
- [ ] Emit stable diagnostics for missing labels and ambiguous names.
- [ ] Add compatibility fixtures for:
  - [ ] Apex label access.
  - [ ] Namespaced label access.
  - [ ] Visualforce label merge expressions.
  - [ ] LWC label import resolution.
  - [ ] Translation fallback.
  - [ ] Exact assertion text in tests.

## 7. Email Templates And Messaging Side Effects

The core parity todo covers `Messaging` basics. Post-parity should add metadata
templates and declarative email senders.

- [ ] Load email template metadata and bodies.
- [ ] Support text, HTML, and custom template metadata enough for local tests.
- [ ] Implement merge rendering for common fields:
  - [ ] Recipient fields.
  - [ ] Related-record fields.
  - [ ] Organization and user fields.
  - [ ] Custom labels.
- [ ] Connect templates to `Messaging.SingleEmailMessage`.
- [ ] Connect templates to Workflow email alerts.
- [ ] Capture email side effects in test-visible local state.
- [ ] Add email limit accounting for template and workflow sends.
- [ ] Add unsupported diagnostics for advanced template features not implemented.
- [ ] Add compatibility fixtures for:
  - [ ] Template lookup by ID/name.
  - [ ] Template merge with target object and related object.
  - [ ] Workflow email alert rendering.
  - [ ] Test isolation of captured emails.

## 8. Apex Metadata API And Local Metadata Mutation

Some old admin/configuration tools use the Apex `Metadata` namespace to create
or update custom metadata. This extends the initial parity custom-metadata
baseline with legacy source formats and local metadata mutation.

- [ ] Load legacy `.md` custom metadata records in addition to SFDX
  `customMetadata/*.md-meta.xml` or equivalent source shapes.
- [ ] Preserve custom metadata record developer names, protected flags,
  namespace tokens, field values, and relationship references.
- [ ] Make custom metadata records visible to tests through:
  - [ ] Static SOQL and dynamic SOQL.
  - [ ] Typed `__mdt` SObject construction and field access.
  - [ ] Describe and field metadata.
  - [ ] LWC/Aura controller methods that query configuration records.
- [ ] Implement Apex `Metadata` namespace models used by enterprise projects:
  - [ ] `Metadata.DeployContainer`.
  - [ ] `Metadata.CustomMetadata`.
  - [ ] `Metadata.CustomMetadataValue`.
  - [ ] Deploy callback interfaces and result shapes.
- [ ] Implement local enqueue behavior for metadata deployments:
  - [ ] Deterministic deployment IDs.
  - [ ] Synchronous or queued execution mode.
  - [ ] Test-visible callback invocation.
  - [ ] Error results for invalid metadata.
- [ ] Support local custom metadata mutation where safe:
  - [ ] Insert/update custom metadata records in the in-memory metadata model.
  - [ ] Reflect changes into describe, SOQL, and SObject access.
  - [ ] Roll back metadata mutation during tests unless a supported mode opts
    into persistence.
- [ ] Add diagnostics for metadata types not supported by local deployment.
- [ ] Add compatibility fixtures for:
  - [ ] Custom metadata deploy success.
  - [ ] Deploy error result shape.
  - [ ] Callback invocation.
  - [ ] Visibility of deployed metadata to subsequent Apex.

## 9. Sites, Communities, Networks, And Guest Context

The server can model authenticated users after parity. Post-parity adds the
site/community identity and metadata that legacy pages and jobs may ask for.

- [ ] Load site and network metadata.
- [ ] Load Site and Network records from fixtures or deterministic platform data
  so selector classes can query them during tests.
- [ ] Load community custom metadata records that configure page, navigation,
  data source, card, button, and redirect behavior.
- [ ] Implement `Site` API basics:
  - [ ] `Site.getAdminEmail`.
  - [ ] Current site name/domain/path where metadata exists.
  - [ ] Guest user lookup where a local guest user fixture exists.
- [ ] Resolve Visualforce `$Site.Template` and related site merge fields enough
  for page/controller tests.
- [ ] Support `Page.*` URL generation for community pages with current-site
  context where tests assert URLs.
- [ ] Model community/network context for Visualforce page execution and server
  requests.
- [ ] Add deterministic defaults when no current site is active.
- [ ] Add unsupported diagnostics for advanced Experience Cloud behavior.
- [ ] Add compatibility fixtures for:
  - [ ] Site admin email.
  - [ ] Community landing page controller.
  - [ ] Community custom metadata selector.
  - [ ] `$Site.Template` Visualforce reference.
  - [ ] Guest user context with labels and page parameters.

## 10. Platform Cache And ConnectApi Organization Settings

Dependency-injection libraries may use platform cache as a storage backend and
`ConnectApi` for org identity.

- [ ] Implement local Platform Cache models:
  - [ ] Org partitions and session partitions.
  - [ ] `Cache.Org.getPartition`.
  - [ ] `put`, `get`, `remove`, key index behavior, TTL, and visibility flags.
  - [ ] Test isolation and reset behavior.
- [ ] Add deterministic cache size and eviction diagnostics.
- [ ] Implement `ConnectApi.Organization.getSettings()` for org ID and other
  commonly accessed fields.
- [ ] Add unsupported diagnostics for broader `ConnectApi` resources.
- [ ] Add compatibility fixtures for:
  - [ ] Force-DI-style org partition use.
  - [ ] Cache TTL and reset behavior.
  - [ ] `ConnectApi.Organization.getSettings().orgId`.

## 11. System.Callable, Stub API, And ApexMocks Compatibility

Core interface dispatch is not enough for projects that use platform-provided
dynamic call and mocking contracts.

- [ ] Treat `System.Callable` as a built-in interface with the correct method
  signature and dispatch behavior.
- [ ] Support unqualified `Callable` and `System.Callable` consistently.
- [ ] Preserve `instanceof Callable` behavior across dynamically instantiated
  classes.
- [ ] Implement `call(String action, Map<String, Object> args)` dispatch with
  Apex-compatible argument and return handling.
- [ ] Implement `System.StubProvider` and `Test.createStub`:
  - [ ] Stub creation for interfaces and virtual classes where supported.
  - [ ] Method interception through `handleMethodCall`.
  - [ ] Argument list, method name, return type, and exception behavior.
  - [ ] Test-only isolation and unsupported diagnostics for unsupported targets.
- [ ] Add focused compatibility fixtures for fflib ApexMocks patterns:
  - [ ] Interface mock creation.
  - [ ] Method call interception.
  - [ ] Return value stubbing.
  - [ ] Exception stubbing.
  - [ ] Verification-style method call capture if feasible.
- [ ] Add fixtures for project state-machine patterns using `Callable`.

## 12. Endpoint Configuration, Named Credentials, And Remote Sites

HTTP callout mocks cover the request/response layer. Large projects also depend
on endpoint metadata and endpoint naming conventions. This extends the initial
parity HTTP/callout work; it does not replace it.

- [ ] Load Named Credential metadata.
- [ ] Load Remote Site Settings metadata.
- [ ] Resolve `callout:Name` endpoint prefixes to deterministic local endpoint
  records.
- [ ] Enforce or report remote-site allowlist behavior in strict mode.
- [ ] Model endpoint URL, label, auth protocol, and principal metadata enough for
  tests and diagnostics.
- [ ] Keep secrets out of local fixture exports and logs.
- [ ] Add configurable endpoint replacement for local integration tests.
- [ ] Add compatibility fixtures for:
  - [ ] Named credential endpoint resolution.
  - [ ] Remote-site allowed/blocked behavior.
  - [ ] Callout mock matching after endpoint resolution.
  - [ ] Missing endpoint diagnostics.

## 13. UI And Org Presentation Metadata

This metadata may not execute Apex directly, but old projects reference it from
Visualforce, describe-like APIs, package checks, and local server responses.
This extends the initial parity user/permission/schema work with old-project UI
metadata ingestion.

- [ ] Load layouts and expose layout-adjacent metadata where local APIs need it:
  - [ ] Sections, fields, buttons, related lists, and page assignments.
- [ ] Load tabs and custom applications where present.
- [ ] Load web links and quick actions.
- [ ] Load global value sets and connect them to picklist field describe results.
- [ ] Load profiles and permission sets beyond core permission checks:
  - [ ] Tab visibility.
  - [ ] Object and field permissions.
  - [ ] Apex class/page access.
  - [ ] User permissions.
- [ ] Add local server resources for metadata-backed UI discovery where useful.
- [ ] Add diagnostics for metadata that is loaded but not executable.
- [ ] Add compatibility fixtures for:
  - [ ] Global value set describe behavior.
  - [ ] Profile/permission-set class and page access.
  - [ ] Layout field discovery.
  - [ ] Tab and web-link metadata loading.

## 14. Static Resources, Content Assets, And URLFOR

Static resources and content assets are in the test-running lane when tests or
controller code assert generated URLs, when Visualforce pages use `URLFOR`, or
when LWC imports `@salesforce/resourceUrl/...`.

- [ ] Load legacy `.resource` static resources and companion metadata.
- [ ] Load SFDX static resource metadata where present.
- [ ] Load content asset metadata and binary asset files.
- [ ] Resolve Visualforce `$Resource.Name` and `URLFOR($Resource.Name, path)`.
- [ ] Resolve LWC `@salesforce/resourceUrl/Name` imports.
- [ ] Preserve MIME type, cache control, and relative path information where
  metadata provides it.
- [ ] Add deterministic local resource URLs for tests, Visualforce controller
  harnesses, and local-running server routes.
- [ ] Add fixture import/export support for binary resource bodies without
  leaking local file paths.
- [ ] Add compatibility fixtures for:
  - [ ] Visualforce `URLFOR($Resource...)`.
  - [ ] LWC `@salesforce/resourceUrl` import.
  - [ ] Content asset lookup.
  - [ ] Missing resource diagnostics.

## 15. Files, Attachments, Documents, And Binary Content

Generic SObject storage can hold these records, but project tests may expect
Salesforce-specific file relationships and body behavior. This extends the
initial parity SObject/DML/SOQL storage baseline with file-specific side effects.

- [ ] Model legacy `Attachment` and `Document` basics:
  - [ ] Body blob storage.
  - [ ] Parent linkage.
  - [ ] Name/content type fields.
- [ ] Model Salesforce Files basics:
  - [ ] `ContentVersion`.
  - [ ] `ContentDocument`.
  - [ ] `ContentDocumentLink`.
  - [ ] Latest-version behavior.
  - [ ] Version data blob handling.
- [ ] Support SOQL relationships and common filters for file/document records.
- [ ] Support DML side effects that create linked document records where needed.
- [ ] Add fixture import/export support for binary body fields without leaking
  local file paths.
- [ ] Add compatibility fixtures for:
  - [ ] Insert/query `Attachment`.
  - [ ] Insert/query `ContentVersion`.
  - [ ] Link file to record.
  - [ ] Blob/base64 round-trip.

## 16. Reports, Dashboards, And Analytics Metadata

These usually do not affect Apex test execution, but large project compatibility
reports should load and account for them.

- [ ] Load report metadata enough for project scanning and diagnostics.
- [ ] Load dashboard metadata enough for project scanning and diagnostics.
- [ ] Add unsupported diagnostics for executing reports locally unless a local
  report runner is implemented.
- [ ] Expose report/dashboard counts in post-parity dashboards.
- [ ] Add fixtures that prove report/dashboard metadata does not break project
  loading.

## 17. Packaging, Source Layout, And Legacy Project Hygiene

Large old repositories often mix package directories, deprecated source, docs
samples, extensions, unpackaged metadata, and generated files.

- [ ] Expand project discovery for multiple package directories and unpackaged
  source roots.
- [ ] Support both SFDX source format and legacy Metadata API source format in
  the same repository.
- [ ] Add source-root classification for production, deprecated, sample, test,
  extension, and unpackaged metadata.
- [ ] Allow compatibility scans to include or exclude deprecated/sample roots.
- [ ] Add namespace replacement awareness for projects that carry scripts such as
  `replace-namespace.sh`.
- [ ] Add diagnostics for duplicate metadata names across package roots.
- [ ] Add stable ordering when multiple roots define the same type or metadata
  component.
- [ ] Add large-repo performance budgets for indexing mixed metadata.
- [ ] Add fixtures for:
  - [ ] Multiple SFDX package directories.
  - [ ] Legacy Metadata API `src/` roots.
  - [ ] Unpackaged metadata.
  - [ ] Deprecated source included/excluded by config.
  - [ ] Duplicate type/metadata diagnostics.

## 18. Local Test Trace, Profile, And Debug Visibility

Local test running needs trace events so failures can be understood without
guessing which metadata layer fired. Add these before local UI/API running so
old-project test failures show their save-order and metadata causes. This
extends the initial parity trace/profile/DAP baseline with post-parity metadata
layers.

- [ ] Add trace events for:
  - [ ] Visualforce page/controller construction and action calls.
  - [ ] Aura action dispatch.
  - [ ] Workflow rule evaluation and action execution.
  - [ ] Flow interview start/end, decisions, assignments, and Apex actions.
  - [ ] Label resolution.
  - [ ] Static resource and content asset URL resolution.
  - [ ] Email capture and template rendering.
  - [ ] Metadata deploy enqueue/result.
  - [ ] Cache operations.
  - [ ] Named credential and remote-site endpoint resolution.
- [ ] Add profile aggregation for post-parity surfaces.
- [ ] Add DAP scope rendering for page parameters, `ApexPages` messages, Flow
  variables, cache state, and captured emails where practical.
- [ ] Add post-parity trace fixtures with stable JSON output.

## 19. Local Test Compatibility Fixtures And Release Claims

Post-parity support should be earned the same way core parity support is earned:
with fixtures and dashboards.

- [ ] Add owned legacy enterprise fixture projects modeled after the audited
  features in the anonymized large-project corpus.
- [ ] Add fixture categories:
  - [ ] Visualforce-heavy.
  - [ ] Aura-controller-heavy.
  - [ ] Workflow-field-update-heavy.
  - [ ] Flow-and-invocable-heavy.
  - [ ] Label-and-translation-heavy.
  - [ ] Static-resource-and-content-asset-heavy.
  - [ ] Site/community-heavy.
  - [ ] Metadata-API-heavy.
  - [ ] Named-credential/callout-heavy.
  - [ ] Email-template-heavy.
  - [ ] Files-and-attachments-heavy.
- [x] Add a post-parity readiness command that reports:
  - [x] Supported, partial, stub, unsupported, and unknown post-parity
    capabilities.
  - [x] Affected files/classes/tests per gap.
  - [x] Suggested next capability to implement.
  - [x] Generated docs drift.
- [ ] Add docs describing release language:
  - [ ] MVP-ready.
  - [ ] Apex-parity-ready.
  - [ ] Legacy-project-test-ready.
  - [ ] Declarative-automation-test-ready.
  - [ ] Visualforce/Aura/LWC-controller-test-ready.
- [ ] Add CI jobs that keep post-parity docs and dashboards in sync without
  blocking MVP releases until the project opts in.

## Part II: After Local Tests Pass

This part starts after large legacy projects can run their Apex tests locally.
It is for running or serving UI/API surfaces, rendering user-facing artifacts,
and simulating browser or Experience Cloud behavior. Nothing in this part
should block the first post-parity goal unless a local test fixture proves the
same behavior is needed for `oaer test`.

## 20. Local Running Of Legacy UI And API Surfaces

This section is for running or serving legacy UI/API surfaces, not for the first
goal of getting `oaer test` green on old projects.

- [ ] Add Visualforce page resource stubs:
  - [ ] Resolve page URL.
  - [ ] Construct controller.
  - [ ] Run page action.
  - [ ] Return redirect/message/rendered output where supported.
- [ ] Add Aura action endpoint support where needed by local-running fixtures.
- [ ] Add LWC Apex endpoint support where needed by local-running fixtures.
- [ ] Add optional Visualforce rendering beyond controller/action execution.
- [ ] Add optional Aura/LWC browser-lifecycle simulation only after endpoint
  contracts are stable.
- [ ] Add metadata resource discovery for labels, layouts, tabs, named
  credentials, remote sites, and sites.
- [ ] Add deterministic auth/user/site context for legacy server requests.
- [ ] Add server reset support for page state, messages, cache, emails, workflow
  side effects, and endpoint overrides.
- [ ] Add black-box server fixtures for:
  - [ ] Visualforce controller action.
  - [ ] Aura action dispatch.
  - [ ] LWC Apex method dispatch.
  - [ ] Label lookup.
  - [ ] Named credential callout with mock.
  - [ ] Cache reset.

## Beyond Post-Parity

These are useful after large legacy project compatibility is credible.

- [ ] Optional full Visualforce renderer with snapshot testing.
- [ ] Optional Aura client simulation for component integration tests.
- [ ] Optional Flow visual debugger and replay.
- [ ] Optional report runner for selected report types.
- [ ] Optional metadata deploy simulator for more metadata types.
- [ ] Optional site/community HTTP routing simulator.
- [ ] Optional anonymized export of legacy metadata bundles for bug reports.
- [ ] Optional org-shape packs that provide standard profiles, licenses,
  permission sets, communities, and email deliverability settings.
- [ ] Optional compatibility scoring by Salesforce API version and metadata
  surface.
- [ ] Optional migration hints that identify legacy metadata blocking local
  execution.
