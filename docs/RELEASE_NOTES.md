# Release Notes

## Unreleased

- First-run guidance initializes the bundled Playground sample before `doctor`.
  Runtime smoke initializes it before `check` and `test`, then requires a named,
  nonzero test result.
- Release and security documentation now reflects the published v0.2.14
  product, the v0.2.13 first-party plugins, platform-specific SBOM counts, and
  the install footprint.
- Release builds stage the shared payload outside the Git worktree and fail if
  the candidate is dirty, preserving the clean embedded revision required by
  the Salesforce correctness gate.
- `golang.org/x/text`, `nanoid`, and `postcss` receive targeted security updates.

## v0.2.14 - 2026-09-04

v0.2.14 focuses on a reliable first local-Apex result, clearer product
boundaries, safer public feedback, and broader distribution dependency
inventory.

- The local Playground rejects source errors before cache reuse or execution.
  Invalid source cannot report Pass or change the local database. Warning-only
  diagnostics remain visible without becoming errors.
- The bundled `refinement-service` example avoids the reserved `number`
  identifier and includes `RefinementServiceTest.createsAndLabelsFileRow`.
  Shared runtime smoke now loads the example and requires a named, nonzero
  passing test. The published v0.2.13 sample does not include this fix.
- Onboarding distinguishes a real local sample from the website's illustrative
  replay. It explains PATH repair, source versus endpoint API versions, installed
  editor verification, advisory CI, and useful privacy-safe feedback.
- The support explorer includes every standard-library ledger row, with filter
  totals separate from curated autocomplete entries. Descriptions distinguish
  local behavior from hosted Salesforce services.
- Website light-theme terminal contrast, highlighting warnings, anonymous-Apex
  navigation, and legacy `/docs` redirects are corrected.
- Repository contribution and issue-report guidance now includes private-source
  precautions and direct private vulnerability reporting. Product source and
  distributions carry the canonical Apache License 2.0 and project notice.
- ZIP report export confines reads to the selected saved run, rejects linked
  entries, and publishes the archive without overwriting hard-linked source
  files. This change does not cover HTML export or every report reader.
- Local HTTP servers have a ten-second request-header timeout. VF/LWC previews
  require the existing explicit opt-in before binding beyond loopback.
- Bundled JavaScript dependencies receive targeted security updates. New
  archive inventory includes packaged LWC/Babel and VSIX dependencies; the
  extension includes bundled dependency notices. Published v0.2.13 inventory
  remains Go-only. Scanner triage is not a zero-findings security certification.
- Release archives include deterministic notice evidence for the Go distribution,
  linked Go modules, and the vendored parser, bound to the built binary hash.
  System/CGO notice sufficiency remains an owner/legal review boundary.

Upgrade with `glade update`, then run `glade version` and
`glade doctor --project .` inside your project. No Windows archive or
historical-version parity is introduced. Packaged targets remain macOS/Linux
AMD64 and ARM64; the checked source window remains 65.0, 66.0, and 67.0.

## v0.2.13 - 2026-09-03

- Updated site and release documentation consistency after the `v0.2.12` line:
  - Hardened immutable release publishing behavior and checks.
  - Synced the site release manifest and manual Pages build metadata.
  - Finalized release-readiness wording and consistency across docs/UI surfaces.

## v0.2.12 - 2026-09-02

Tagged v0.2.12 validation:

- The [tagged product validation](PRIVATE_CORPUS_ASSURANCE.md#tagged-v0212-product-validation)
  passed `private-corpus-001` check exit 0/0
  diagnostics and tests 12,315/12,315 with 0 failed/compile/runtime/unsupported;
  `private-corpus-002` check exit 0/0 diagnostics and tests 782/782 with the
  same zero failures.
- The public corpus covered 86 projects with 40 expected/40 observed
  diagnostics, zero missing/unexpected/unclassified, and an exact identity
  multiset match. Public diagnostics are the known baseline, not passes.
- The Salesforce tagged-release gate recorded 475/475 pass, zero
  fail/inconclusive, and cleanup PASS. This final tagged validation is separate
  from the published v0.2.11 surface snapshot and does not claim blanket
  Salesforce parity.

Salesforce release upgrades:

- Added moving-correctness support for source API 65.0, 66.0, and 67.0 while
  retaining endpoint API 60.0, 65.0, 66.0, and 67.0. The default remains 65.0;
  malformed or future Apex project versions and unsupported Execute Anonymous,
  LWC, or endpoint versions fail rather than silently falling back.
- Preserved valid historical Apex project versions through project loading,
  per-file analysis, `glade check`, `glade test`, and package artifacts without
  adding them to the checked 65.0-67.0 parity window. Historical versions
  receive no parity credit. Execute Anonymous and LWC bundle versions remain
  limited to the checked window.
- Made Apex semantic availability, LWC module availability, REST and Tooling
  routing, anonymous execution, DAP, playground caching, and org bindings keep
  their explicit source or endpoint API version.
- Added generated availability tables from the checked Salesforce release
  contract, including API-versioned Tooling discovery and explicit unsupported
  boundaries for `System.IntegrationTest` and `PlatformEventMigration`.
- Added release-bound behavior for API 66 complex LWC expressions, invocable
  parameter constructor visibility, and Automated Process `WITH USER_MODE`,
  plus API 67 `<details name>`, elastic async-limit keys, sharing defaults, and
  database user-mode defaults.
- Kept hosted-only Named Credentials identity-provider changes, Invocable Apex
  REST actions, and OpenAPI specification generation as tested, explicit
  non-parity instead of claiming local runtime support.
- Bound each release inventory to an exporter-generated source receipt that
  preserves the exact Atlas family hashes and the `latest` current-only LWC
  source/filter provenance.
- The Salesforce promotion gate now checks generated drift, installs the real
  LWC compiler, rejects binaries not built from the named clean commits,
  records candidate hashes and the complete Go test event log, and closes exact
  surface, behavior, source, endpoint, org-profile, release-note, and
  no-fallback denominators before promotion.
- Standard SObject instance-field assignments no longer produce a static-field
  diagnostic.
- Project-referenced numeric fields no longer depend on whether an integer or
  decimal literal was observed first.
- Custom platform-event `EventUuid` and `ReplayId` fields now resolve for Apex
  access and SOQL queries.

Local checks and releases:

- Local-test progress event rendering is serialized so concurrent workers
  cannot corrupt terminal or NDJSON output.

Distribution and upgrades:

- The release workflow publishes parser-capable macOS and Linux archives for
  AMD64 and ARM64, `SHA256SUMS.txt`, CycloneDX SBOMs, and provenance
  attestations. Verify the checksum and run `glade doctor` as described in the
  [distribution workflow](https://github.com/glade-sh/glade/blob/v0.2.12/docs/DISTRIBUTION_WORKFLOW.md).
- No migration is required for documented CLI flags, database schemas, or server
  API behavior. The default local API version remains `v65.0`.
- Known hosted gaps remain explicit in
  [KNOWN_GAPS.md](https://github.com/glade-sh/glade/blob/v0.2.12/docs/KNOWN_GAPS.md).
  Local deterministic mocks and explicit non-parity outcomes are not hosted
  Salesforce support.

## v0.2.11 - 2026-08-12

Glade v0.2.11 expands checked Salesforce compatibility, publishes exact-candidate
private-corpus assurance, and makes local correctness and release checks faster
without broadening the documented support boundary.

Salesforce compatibility and private-corpus assurance:

- Expanded API 67-backed compiler, semantic, schema, standard-library, and VM
  contracts used by the frozen two-repository private corpus. The product
  default local API version remains `v65.0`; API 67 is the comparison contract,
  not a silent default-version change.
- Replayed every required project check and real local test shard against the
  exact candidate. Both neutral repositories passed all eight authoritative
  replay records.
- Reconciled 321 observed usage keys with zero unknown usage. The sealed result
  contains 184 required surfaces: 178 compile-ready and test-ready,
  54 runtime-parity-ready from fresh Salesforce proof,
  107 explicit zero-credit non-parity outcomes, and six hosted-deferred surfaces.
- Added a self-contained [private-corpus assurance explorer](https://glade.sh/private-corpus-assurance.html)
  for filtering the sealed outcome by namespace, repository, disposition,
  evidence, exclusion, and text. These overlapping, surface-specific outcomes
  are not a claim of blanket Salesforce parity.

Performance and correctness:

- Added an exact, checksummed semantic result cache for `glade check` and the
  local test semantic gate. Reuse is tied to immutable project source,
  companion metadata, schema, dependency, analyzer, platform, and option
  identity. Mismatched or corrupt entries fail closed.
- Bounded semantic results retained in a warm process and collapsed duplicate
  analysis work. `--no-cache` bypasses both disk and memory semantic reuse when
  a cache-independent run is required.
- Reduced repeated source reads, temporary allocations, startup-cache
  validation work, payload copies, and cache write I/O. Generation checks
  prevent a changed source or metadata snapshot from being published or
  returned as current.
- Expanded opt-in performance telemetry for cache provenance, source reads,
  allocation and garbage-collection cost, execution policy, and slow tests.
  These optimizations preserve Salesforce correctness and do not broaden the
  documented compatibility surface.

Local checks and releases:

- Split the local Go release proof into fail-closed package lanes with validated
  machine-readable results. The default remains serial for predictable memory
  use; parallel overlap requires an explicit maintainer choice.
- Made the site release proof run verification, unit tests, and the production
  build exactly once while checking that its source inputs stay unchanged.
- Added a measurement wrapper that records release check time, memory, file I/O,
  toolchain, commit, and caller-declared cache state without changing the
  correctness gate or priming caches.

Distribution and upgrades:

- The release workflow publishes parser-capable macOS and Linux archives for
  AMD64 and ARM64, `SHA256SUMS.txt`, CycloneDX SBOMs, and provenance
  attestations. Verify the checksum and run `glade doctor` as described in the
  [distribution workflow](https://github.com/glade-sh/glade/blob/v0.2.11/docs/DISTRIBUTION_WORKFLOW.md).
- No migration is required for documented use. The default local API version
  remains `v65.0`, and semantic cache entries are disposable and fail closed
  when their source, schema, dependency, analyzer, platform, or option identity
  changes.
- Known hosted gaps remain explicit in
  [KNOWN_GAPS.md](https://github.com/glade-sh/glade/blob/v0.2.11/docs/KNOWN_GAPS.md)
  and in the assurance snapshot. Local deterministic mocks, zero-credit
  non-parity outcomes, and hosted-deferred surfaces are not hosted Salesforce
  support.

## v0.2.10 - 2026-07-27

Glade v0.2.10 aligns local Apex language checks with Salesforce compiler
behavior for reserved and malformed identifiers, expands the checked compiler
rule set, and makes the compatibility gate faster and fail-closed.

Apex language compatibility:

- Added case-insensitive rejection for all 121 Salesforce reserved words in
  non-method Apex source identifier contexts, including `currency`, while
  preserving Salesforce's contextual method-name exceptions.
- Added checked identifier shape and length rules. Project parser diagnostics
  flow through `parse`, `check`, `test`, and LSP; anonymous `exec` fails before
  execution with its anonymous-parse diagnostic; and Apex rename rejects an
  invalid target before creating edits.
- Expanded compiler checks for annotations, declarations, types, inheritance,
  statements, triggers, web exposure, SOQL/SOSL, and source API-version
  behavior.
- Added a 400-row Salesforce language-rule evidence catalog. Every supported
  row points to its Salesforce evidence, expected outcome, owning subsystem,
  product regression test, and exact Glade commit.

Glade Tools and CI:

- Kept the routine Glade Tools pull-request gate bounded to five minutes. It
  validates the exact Glade commit pin, catalog structure, product-test
  pointers, and repository tests without waiting for an authenticated scratch
  org comparison.
- Fixed Darwin process-group cleanup in the Glade Tools comparison harness so a
  transient permission result keeps polling until cleanup is confirmed or the
  existing deadline expires.

## v0.2.9 - 2026-07-25

Glade v0.2.9 prepares a faster large local Salesforce test path while
tightening security and release safety. The changes preserve documented CLI and
test behavior; Salesforce remains the validation gate for supported local work.

Performance and compatibility:

- A controlled comparison used identical normalized 4,565-test legs. Each leg
  reported 4,561 passes and the same four 60-second cap timeouts. The optimized
  path reported 11.45% lower duration, 11.35% lower wall time, 8.00% lower
  maximum RSS, and 9.00% lower peak footprint.
- The exhaustive optimized 11,526-test corpus reported zero failures,
  unsupported results, load errors, compile errors, and internal errors.
- Test selection and result parity were preserved. The daemon, startup-cache,
  semantic, and local execution improvements underlying this comparison were
  already merged.
- This release preserves compatibility: it changes no documented CLI flag,
  fixture format, persistent database schema, or supported Apex behavior.

Security and release trust:

- Added filesystem/root confinement for plugin archives, playground workspaces,
  and static-resource reads, plus safe JSON serialization for LWC diagnostic
  data.
- Tightened CI attribution so required checks identify the exact commit they
  validated.
- The release workflow creates an absent release once. Existing release metadata,
  title, and body are reused without editing; a new uniquely named asset may be
  added, while a duplicate asset name fails instead of replacing published bytes.

## v0.2.8 - 2026-07-04

Glade v0.2.8 ships the local terminal UI, richer debug-log tooling, local data
import and record-management workflows, the LWC Workbench Console, and release
trust hardening after v0.2.7.

Terminal UI and local data workflows:

- Added `glade tui`, `glade test --ui`, and `glade db --ui` as terminal-first
  boards for project, test, data, and plugin workflows.
- Added `glade db import sf` so small Salesforce CLI org slices can seed local
  SQLite data by object, field list, raw SOQL, or importable-object discovery.
- Added `glade db ui`, a browser record manager for local SQLite data, with
  object-aware field controls, lookup filters, create/edit/delete/undelete
  actions, and project-schema checks before a DB is reused.
- Refreshed VS Code commands so Glade Home can launch the project, tests, data,
  and plugin TUI views from the current workspace.

LWC Workbench Console:

- Made `glade dev lwc` open the Workbench Console at `/` and `/lwc`, with
  Component Lab as the first workspace, route discovery, persistent builder
  context, debug panes, and runtime event capture.
- Added local builder search endpoints backed by project schema and optional
  SQLite data, so record-page setup can find objects and records without a
  hosted org.
- Improved the console layout, mobile viewport mode, diagnostics, and local
  route memory while keeping hosted Lightning parity limits explicit.

Apex debug log replay and editor analysis:

- Added Apex log parsing improvements, `.apexlog` language support, and smart
  editor analysis for navigation, folding, symbols, hovers, diagnostics, and
  source links.
- Added `glade debug replay` for dry-run anonymous Apex replay from
  source-backed log frames and tightened replay for execute-anonymous text log
  exports.
- Added `glade debug editor` and VS Code Apex Log commands for source-backed
  log inspection without requiring hosted replay debugging.

Docs and site:

- Reorganized the public site into workflow, module, and reference entry
  points, with screenshot-backed help articles for first checks, test runs,
  debug logs, CI setup, local data, and Salesforce data import.
- Refreshed public example naming around `refinement-service`, `FileRow`, and
  `refinement-local`, and removed stale maintainer-tool pages from the public
  site.
- Updated CLI, editor, local testing, local data, security, and install docs to
  match the current command surface.

Runtime, semantic checks, and performance:

- Fixed static collection alias propagation in the Apex VM, preserving shared
  list, set, and map values through static paths.
- Reduced same-ref collection alias rescans in local test runs, fixing the
  large-package data-mapping timeout while preserving nested map/list alias
  behavior.
- Reduced no-setup local test suite cost by reusing journal-backed org state for
  single-method test classes instead of cloning the runtime org for every test.
- Reduced root-scope check and standard object loading cost with lazy standard
  object name loading and tighter project-root handling.
- Reduced `glade check` semantic-analysis allocations for platform static field
  lookups, method-body cast/`instanceof` checks, and project-referenced schema
  inference scans.
- Reused content-addressed test startup-cache payloads on identical rewrites, so
  metadata/header refreshes do not replace unchanged runtime payload files.
- Hardened daemon-backed affected-test runs so warm `--daemon --changed-since`
  selections run all selected affected classes without expanding back to the
  full suite.
- Reduced internal sema package test wall time by running independent
  analysis-focused tests in parallel while keeping cache and allocation guards
  sequential.
- Added test-runner and code-intelligence hardening for selected-test routing,
  startup cache behavior, affected-test analysis, watch classification, SOQL,
  DML, Visualforce, and local server request paths.

Security and release trust:

- Added public security and trust docs, a repository security policy, checked
  release workflow wording, SBOM generation, and best-effort archive
  attestation steps.
- Kept release hosting product-facing through `glade.sh` and
  `downloads.glade.sh`, with plugin distribution staying on `plugins.glade.sh`.
- Hardened the VS Code extension packaging dependency graph by pinning patched
  transitive `form-data` and `undici` versions used by `@vscode/vsce`.

Verification and performance:

- Added focused tests for Apex log parsing, debug editor contracts, debug replay,
  terminal UI models, Salesforce data import, DB manager behavior, LWC Workbench
  Console behavior, VM aliasing, extension commands, and docs/site information
  architecture.
- Added release-readiness coverage for the new security workflow, release
  archive checks, smoke coverage, repo guards, site tests, and extension
  packaging.

## v0.2.7 - 2026-06-24

Glade v0.2.7 ships the release-prep fixes after v0.2.6.

Project configuration and release prep:

- Fixed project dependency discovery so case-variant paths that resolve to the
  same physical SFDX project are not loaded as separate managed-package source
  dependencies. This removes false duplicate-symbol diagnostics on
  case-insensitive filesystems while preserving real duplicate class detection.
- Refreshed the release-facing docs and site copy for project configuration,
  including namespace remaps, source-backed managed package dependencies,
  captured package artifacts, and package shims.

## v0.2.6 - 2026-06-24

Glade v0.2.6 ships the public-corpus semantic-check closure and a targeted
local-test throughput cut after v0.2.5.

Semantic checks:

- Added checked public-corpus gates for relationship fields, schema inference,
  standard symbols, static access, test-helper annotations, and type
  compatibility.
- Broadened schema inference, platform signature lookup, field-token handling,
  constructor/method checking, namespace-aware SObject member access, and
  project-owned diagnostic filtering for large Salesforce source trees.
- Expanded standard symbol coverage for common platform namespace spellings and
  added focused tests so public-project semantic regressions stay fenced.

Runtime and performance:

- Narrowed bulk DML SObject alias indexing to the record refs being written,
  while preserving Salesforce-style alias merge behavior and fallback
  boundaries.
- Reduced wall time on profiled large-package test methods without changing
  Salesforce DML, trigger, or alias semantics.

## v0.2.5 - 2026-06-23

Glade v0.2.5 ships the configuration, package-contract, and Salesforce surface
coverage changes after v0.2.4.

Project configuration and packages:

- Added `project.namespaceRemaps` so source-backed package dependencies can keep
  production namespace tokens while running locally under a different dependency
  namespace.
- Added managed-package artifact dependencies and `packageShims` so a project
  can compile against captured package contracts while supplying local test or
  runtime bodies only where needed.
- Documented `glade package capture` as the product bridge to the
  `@glade/orgpackage` plugin, keeping live org capture in first-party tooling
  while base Glade owns artifact loading, validation, diffing, and runtime use.

Semantic checks and runtime coverage:

- Expanded schema inference for package-style workspaces, relationship fields,
  standard SObjects, change events, namespace-token fields, and external managed
  package shapes.
- Closed the checked Apex docs surface contracts by adding generated contract
  data and platform symbol coverage for documented `System` and `Schema` type
  spellings.
- Broadened platform signatures, inherited member lookup, nested type
  resolution, package access checks, and SObject field/member handling for large
  namespaced source trees.
- Added Approval process runtime coverage and refreshed generated editor
  support data for the expanded platform surface.

Release engineering:

- Added release index generation and hardened release workflow publishing for
  `index.json`, per-version manifests, checksums, and latest manifests.
- Moved release asset handoff through GitHub Release assets so artifact storage
  quota does not block tagged releases.
- Kept support-map and configuration site docs aligned with the current
  product/plugin boundary.
- Reduced redundant generated sema coverage test work so the full Go suite stays
  under the default package timeout on release hardware.

## v0.2.4 - 2026-06-21

Glade v0.2.4 ships the semantic-check and release-hardening fixes after
v0.2.3.

Semantic checks:

- Explicit `System.*` names now beat local shadows for generated platform
  types, including platform constructors and methods.
- Standard namespace aliases now come from the generated platform symbol set,
  so newly indexed `System` types do not need one-off sema entries.
- Dynamic `Database.query(...)` and `Database.queryWithBinds(...)` results can
  flow through list indexing and `List<Object>` assignment where Apex permits
  it.
- Namespaced SObject fields and duplicate schema-object metadata are merged
  before query checks, reducing false missing-field diagnostics in package-style
  workspaces.
- Source-backed managed-package member types keep their namespace when they
  cross into a consumer project.
- Same-namespace source dependencies resolve inheritance and override checks
  without collapsing into local short names.
- Common SObject relationship fields such as `CreatedBy` and `LastModifiedBy`
  resolve through standard user/profile/license chains.
- Custom metadata `Label`, `PermissionSet.Assignments`, Visualforce component
  assignability, SObject DML options, String locale overloads, nested enum
  paths, block-scoped returns, and cast-style returns have focused coverage.
- Nested classes inside `@IsTest` classes now inherit test visibility for
  `@TestVisible` access.

Release engineering:

- Hardened the release rails for both `glade` and first-party plugins.
- Added checks to keep private corpus package markers out of product code and
  tests.
- Kept product downloads on `downloads.glade.sh` and first-party plugin
  archives on `plugins.glade.sh`.

Verification:

- `scripts/release-check.sh` passed in `glade` and `glade-tools` before
  tagging.
- The release candidate passed checks on three large namespaced SFDX workspaces
  with one performance warning and no semantic errors.
- A source-backed dependency workspace now reports warning-only diagnostics
  where referenced fields are absent from the checked source metadata.

## v0.2.3 - 2026-06-21

Glade v0.2.3 ships the latest fixes after v0.2.2.

Issue closeout:

- Ships semantic-check fixes for large source-backed SFDX workspaces after
  `v0.2.2`.
- `glade check --project .` now exits 0 for a namespaced SFDX workspace with
  5,005 Apex types, 65 triggers, 174 objects, and one performance warning.
- `glade check --project .` now exits 0 for a source-backed dependency
  workspace with 3,696 Apex types, 79 triggers, 254 objects, and dependency
  diagnostics downgraded to warnings.
- A separate package workspace still reports three real duplicate top-level Apex
  classes inside its configured `sfdx-project.json` package root.

Semantic checks:

- Source-backed managed-package dependency source is indexed for consumer
  symbols without leaking dependency-source diagnostics into consumer project
  errors.
- Source-backed dependency uncertainty is reported as warnings so large
  source-backed workspaces can still finish a check when the diagnostics are not
  project-owned errors.
- SObject field-token property chains such as
  `Schema.SObjectType.Account.fields.Name.label` infer `String`.
- Inner classes can resolve outer static helper methods before falling back to
  inherited `Object` methods.
- Duplicate top-level class bodies keep their own members during body checks, so
  duplicate-symbol reporting does not cascade into false missing-method errors.
- SOQL aggregate foreach literals infer `AggregateResult` element types.

## v0.2.2 - 2026-06-21

Issue closeout:

- Fixes #1: `glade check --project .` no longer reports the false first-check
  diagnostics in `cesarParra/expression`; the release candidate reports
  `No diagnostics found`.
- Fixes #2: `glade test --project .` now runs the same project with 713
  selected tests, 713 passed, and 0 failed.

Support status:

- The published `glade` CLI is focused on local Apex parsing, checking,
  execution, testing, Visualforce page rendering, storage, server, editor,
  profile, and playground flows.
- Added `glade plugins` with executable plugin install, link, list, doctor,
  lock, restore, and command dispatch.
- Maintenance scanners, compatibility harnesses, generated support ledgers, and
  advisory performance scans now ship through first-party plugins. The first
  plugins are `compat` and `performance`.
- See the public support map and [`STDLIB_COVERAGE.md`](STDLIB_COVERAGE.md)
  for checked support status.

Release engineering:

- Fixed `lightning/uiRecordApi` local create/update/delete record helpers to use
  the same DML engine as local Apex, preserving ID sequences, required-field
  validation, audit fields, explicit nulls, and soft deletes.
- Fixed SOQL parsing for backslash-escaped string literals produced by
  `String.escapeSingleQuotes`.
- Hardened Visualforce page/component metadata indexing for quoted attributes
  that contain `>` and hardened Lightning Out discovery for nested
  `$Lightning.createComponent()` attribute objects.
- Added ConnectApi runtime support for ChatterFeeds (postFeedElement, postFeedElementBatch,
  updateComment, getComment), ChatterUsers (setPhoto, getReputation), CommerceCart
  (getCartSummary, addItemToCart, addItemsToCart, getCartItems, getProduct,
  getProductPrice/getProductPrices), Topics (getTopicSuggestions), and Wave
  (executeQuery), with compatibility fixtures for each surface.
- Added explicit-unsupported compatibility fixtures for Platform Events
  metadata/tooling surfaces, Integration GraphQL/PubSub/SalesforceConnect Amazon
  RDS APIs, External Marketing Cloud AMPscript/Handlebars engines, and AI
  Agentforce product surfaces, moving 62 rows from missing-shape to documented
  explicit-unsupported.
- Reclassified PlatformEvent Flow references and missing label sources as
  non-test-blocking, clearing the last test-blocking post-parity inventory
  findings.
- Fixed ConnectApi.UserProfiles.setPhoto overload dispatch and added
  Communities.getCommunity evidence coverage.
- Added `glade playground`, a local Apex playground web UI with workspace files,
  execute-anonymous runs, cached results, logs, variables, limits, trace output,
  diagnostics, org diffs, reset/seed controls, and a slick developer-focused
  browser shell.
- Added richer playground examples and `--project-ref name=path` so local SFDX
  folders can appear in the playground selector and load into scratch.
- Added tag-driven parser-capable release artifact builds for macOS and Linux
  host architectures, with Windows held until a CGO-capable Windows release
  runner is wired.
- Added support-map labels for server examples, Apex parity, legacy
  local tests, declarative automation, local Visualforce page rendering, and
  Aura/LWC runtime shims.
- Added a generated SObject stub field overlay for broad standard-object field
  coverage from public Apex stub shape data, plus runtime support for
  `Schema.SObjectField.label`, `UserInfo.getOrganizationName`,
  `Date.daysInMonth`, and the `America/Panama` timezone.
- Added CI release-hardening gates for the checked local-test corpus,
  post-parity inventory, UI controller discovery, and generated stdlib coverage
  drift.
- Expanded the checked local-test corpus to 13 projects with `metadata-deploy`
  and `named-credential-callouts` fixtures covering local `Metadata.CustomObject`
  and `Metadata.CustomField` deployment plus named-credential/remote-site
  callout matching.
- Added Flow routed-decision/default-branch execution, nested decision target
  traversal, variable-backed decision criteria, richer Flow assignment traces,
  and comparison criteria for local declarative automation.
- Added Visualforce `ApexPages.StandardController` action trace events for
  `save`, `quickSave`, `delete`, `view`, `edit`, `cancel`, and `reset`.
- Added local Apex Metadata API deployment of custom objects and custom fields,
  including schema mutation and deploy-result component success details.
- Added failed `Metadata.DeployResult` records for invalid supported metadata
  deployments without partial org schema mutation.
- Added a stable post-parity trace fixture for Flow, Visualforce controller, and
  Metadata deploy trace events.
- Added `SHA256SUMS.txt` checksum generation for release artifacts.
- Added manual, CI, and future Homebrew installation guidance.
- Added editor integration docs with VS Code tasks, debug launch examples, LSP
  wiring, watch mode, and report commands.
- Added a fail-fast support gate in the maintenance tools.
- Added compatibility fixture support and smoke coverage for expected
  unsupported-feature diagnostics.
- Added typed `UnsupportedFeature` VM errors for unimplemented stdlib/platform
  calls while preserving fixture-compatible message text.
- Moved standard-library coverage generation behind the first-party compat
  plugin, with generated `docs/STDLIB_COVERAGE.md` coverage for supported and
  partial standard-library/platform APIs.
- Tuned SQLite fixture persistence with transaction-scoped prepared inserts,
  storage pragmas, and large-fixture save/load coverage.
- Strengthened server transaction boundaries so mutating REST, fixture/reset,
  composite, and Tooling executeAnonymous requests commit cloned org state only
  after successful execution and persistence, with serialized request handling
  to avoid concurrent lost updates.
- Completed fixture alias resolution with object-qualified aliases,
  relationship target validation, and ambiguity checks for relationship-heavy
  seed data.
- Expanded deterministic platform seed data with Organization, UserRole,
  enriched User/PermissionSet metadata, and RecordType records from local object
  metadata.
- Added scoped Glade reset endpoints for deterministic data, user/platform,
  limits, and async reset requests while preserving full-reset compatibility.
- Documented persistent server database setup, fixture/reset lifecycle commands,
  and operational checks for saved mutations and rollback-on-failure behavior.
- Expanded DB lifecycle compatibility coverage to re-import exported fixtures and
  assert restored record, user, profile, and Account counts.
- Added `check` compatibility fixture execution, schema-aware check fixtures,
  and enterprise-style multi-class selector/service/domain fixtures covering
  parse/index/check behavior.
- Added server black-box compatibility fixtures for version/resource discovery,
  OAuth userinfo/id stubs, REST-shaped SObject CRUD/describe/recent/query/
  queryAll, Tooling `executeAnonymous` success/failure/rollback and unsupported
  Tooling object errors, Composite sObject reference IDs/partial success/
  all-or-none rollback, explicit unsupported Composite batch responses,
  Salesforce-shaped errors, Glade fixture seed/export/reset, and SQLite
  persistence.
- Added compat plugin replay support for deterministic directory replay bundles,
  ordered in-process compat steps, JSON/text gate reports, checked expected
  outputs, path-escape validation, and redacted artifact export.
- Added compat plugin project-scan reporting to classify local project
  blockers by parser, project, schema, sema, stdlib, SOQL, DML, trigger, limit,
  storage, server, and unknown categories without mutating source or database
  state.
- Added bounded replay smoke bundles for selector/service/domain and
  server-backed REST integration gates under `testdata/replay`.
- Uses a local tree-sitter Apex declaration parser module through
  `internal/apexast`.
- Added enterprise trigger-heavy, describe-heavy, namespace-heavy, and
  package-style compatibility fixtures, with SFDX namespace/package-directory
  support in schema-aware check fixtures.
- Added bounded stress tests for large type-index builds, SQLite fixture
  round-trips, bulk DML partial results, and describe-heavy VM execution.
- Added VM debug pause hooks and DAP breakpoint execution plumbing so DAP can
  stop live execution at stable statement source locations with stack and locals.
- Added live DAP session controls for continue, pause, disconnect, and
  stack-depth step-in/step-over/step-out behavior.
- Expanded DAP scopes and variable rendering with Locals, Statics, and Trigger
  scopes plus object, SObject, exception, and nested collection children.
- Added paused-context DAP watch expression evaluation for locals, object fields,
  static fields, Trigger values, list/set indexes, map keys, and nested paths.
- Added LSP `didOpen`/`didChange`/`didClose` document overlays with incremental
  text edits and publishDiagnostics updates for open-buffer parse diagnostics.
- Added LSP semantic tokens, definition, references, prepare-rename/rename
  workspace edits, and richer completion for Apex members, schema fields, and
  keywords.
- Aligned LSP diagnostics with the shared `glade check` diagnostic model,
  restored project diagnostics when edited buffers close, and added test-result
  diagnostics from failure stack frames.
- Added `glade test --watch --watch-backend auto|native|poll` with `fsnotify`
  native watching, polling fallback, backend reporting, and run IDs in watch
  JSON events.
- Added Apex-only incremental watch re-indexing and dependency-graph
  affected-test selection before falling back to all tests for broad changes.
- Added cancellable watch reruns by threading context cancellation through the
  Apex test runner and VM instruction loop, with stale run-result suppression.
- Stabilized watch newline-delimited JSON events with `schemaVersion: 1`,
  persistent run IDs, and stable `testClasses` array output for run-start events.
- Expanded native trace/profile events across describe, callout, email, async,
  trigger, and final limit-summary activity, with profile attribution for the
  added platform/resource counters.
- Expanded `glade profile analyze` native JSON, Markdown, and pprof-compatible
  reports with hot-event, category, runtime-section, and governor/resource
  summary views so local runtime analysis no longer depends on apexrr-style
  external reporting.
- Added parser diagnostic-count fixtures plus type-index and sema panic recovery
  diagnostics for malformed project inputs.
- Added method/constructor parameter type diagnostics and expanded VM exception
  fidelity for multi-catch, bare rethrow, catchable null dereference, and
  malformed IR guards.
- Completed the exception hierarchy baseline with ordered catch blocks,
  `System.*Exception` name normalization, original-stack rethrow preservation,
  and `getTypeName`, `getLineNumber`, and `getStackTraceString`.
- Added a conservative method-body sema baseline for local declaration types,
  constructor references, simple assignments, project method calls, and
  known-receiver overload arity/simple argument type matching.
- Extended method-body sema with duplicate-local diagnostics, unknown call
  argument variable diagnostics, inherited/interface/`super` method lookup, and
  private/protected method-call visibility diagnostics through inheritance
  chains with token-level ranges for body diagnostics, including `@TestVisible`
  method access from test classes.
- Extended sema visibility diagnostics to known user-object field reads, and
  resolved namespace-token schema aliases such as `pkg__Thing__c` for namespaced
  project metadata.
- Added namespace-token custom object and field alias resolution through VM
  SObject construction, direct field access, `get`/`put`, DML validation, and
  SOQL projection/where clauses.
- Added SObject field-shape helpers for `put` previous-value returns, `isSet`,
  `clear`, and `getPopulatedFieldsAsMap` with explicit-null fields.
- Added common SObject system fields after DML and SOQL projection, including
  created/modified timestamps, user IDs, owner ID, system modstamp, and delete
  state.
- Added Metadata API picklist value loading and baseline
  `Schema.SObjectField.getDescribe().getPicklistValues()` support.
- Added Metadata API record type loading and baseline
  `Schema.DescribeSObjectResult` record type describe maps/lists with common
  `Schema.RecordTypeInfo` methods and deterministic local `012` IDs.
- Added `SObjectType.getDescribe`, `DescribeSObjectResult.fields.getMap`, and
  child relationship describe basics for describe-heavy code paths.
- Fixed local `Messaging.sendEmail` result shaping so multi-message sends return
  one `SendEmailResult` per input message, with the Boolean overload covered by
  compatibility fixtures.
- Expanded data-fidelity coverage for SOQL complex predicates, numeric
  comparison semantics, `Database.Error` result shapes, and
  `Database.UpsertResult.isCreated()`.
- Added no-`GROUP BY` SOQL aggregate support for `COUNT(field)`,
  `COUNT_DISTINCT`, `SUM`, `MIN`, `MAX`, and `AVG` with `AggregateResult`
  `exprN` fields.
- Added SOQL `GROUP BY` with grouped field projection, aggregate `HAVING`, and
  ordering/limits over grouped aggregate rows.
- Added SOQL aggregate aliases on `AggregateResult` rows while preserving
  `exprN` compatibility.
- Added SOQL `GROUP BY ROLLUP`, `GROUP BY CUBE`, and `GROUPING(field)` subtotal
  metadata for aggregate result rows.
- Added common SOQL date literals, including day, month/year, and `*_N_DAYS:n`
  ranges, for Date and Datetime comparisons.
- Added SOQL semi-join and anti-join predicate support for single-field
  subqueries in `IN` and `NOT IN` filters.
- Added SOQL child relationship subquery projection with metadata-driven
  relationship names and VM `List<SObject>` row shapes.
- Made SOQL `LIKE` and `NOT LIKE` matching case-insensitive for ASCII letters.
- Added comma-separated SOQL `ORDER BY ASC` and `ORDER BY DESC` handling for
  regular, aggregate, and child relationship query rows.
- Added SOQL `ORDER BY NULLS FIRST` and `NULLS LAST` modifiers.
- Added SOQL `FIELDS(ALL)`, `FIELDS(STANDARD)`, and `FIELDS(CUSTOM)` projection
  expansion.
- Added SOQL `FOR UPDATE` parsing and execution as a local lock marker.
- Marked `FOR UPDATE` result records with an internal local lock marker.
- Added Salesforce-shaped `attributes.url` values when serializing queried
  SObjects with IDs.
- Added runtime index rebuilds from object definitions and SOQL candidate
  selection for single-field equality indexes.
- Added catchable SOQL `FOR UPDATE` lock-contention errors for rows already
  marked locked in local org state.
- Filtered DML after-trigger contexts to rows that succeeded during
  partial-success engine validation.
- Added simple Metadata API validation-rule loading and DML enforcement with
  structured validation errors.
- Corrected undelete trigger execution so supported after-undelete contexts run
  without invoking before-undelete triggers.
- Added common `addError(message, escapeHtml)` overload handling and field
  addError support for unset schema fields.
- Split Apex upsert trigger execution into supported insert and update trigger
  contexts and added fixture coverage for upsert/undelete trigger paths.
- Count projected child relationship rows in SOQL governor row counters.
- Count supported cascade-delete child records in DML governor row counters.
- Recompute deterministic live heap usage after VM statements so mutated
  collections update `Limits.getHeapSize()`.
- Add deterministic SOQL and DML row-work costs to CPU limit accounting.
- Added `Limits.getBatchJobs`, `getLimitBatchJobs`, `getScheduledJobs`, and
  `getLimitScheduledJobs`.
- Added strict/permissive limit-mode selection to `glade server` Tooling
  `executeAnonymous` and compatibility exec/test fixtures.
- Added common JSON overload support for `serialize` suppress-null behavior,
  `serializePretty`, and `deserializeStrict`.
- Added common `Test.isRunningTest()` and deterministic
  `Test.getStandardPricebookId()` platform API support.
- Added `Database.getQueryLocator(String)` for supported SOQL and batch start
  scopes.
- Added basic `Type.forName(...)` and `Type.newInstance()` factory support.
- Added `Database.setSavepoint()` and `Database.rollback(...)` for local
  org-state snapshots.
- Added `Schema.describeSObjects(...)` basics plus local SObject/field describe
  access booleans.
- Added common `String` helpers and `Pattern`/`Matcher` regex basics.
- Added common `Date`, `Datetime`, and `Time` factories, parsing, arithmetic,
  and component helpers.
- Added common `Math`, `Decimal`, `EncodingUtil.urlEncode/urlDecode`, and
  MD5/SHA1/SHA-256 `Crypto.generateDigest` behavior.
- Expanded `HttpRequest`/`HttpResponse` mock shapes with endpoint, method,
  headers, timeout, status, and body/blob accessors.
- Added `UserInfo`, `Messaging`, `ApexPages`, `URL`, and `PageReference`
  platform API basics.
- Added SOQL `ALL ROWS` support for querying soft-deleted records.
- Added SOQL `WITH SECURITY_ENFORCED`, `WITH USER_MODE`, and
  `WITH SYSTEM_MODE` parsing as local security-mode markers.
- Added local projection validation for SOQL security-mode queries so unknown
  selected fields raise catchable `QueryException`s.
- Added baseline SOQL `TYPEOF` relationship projection for parent lookup
  branches.
- Added multi-hop SOQL parent relationship projection and filtering with nested
  VM SObject row shape.
- Added multi-target relationship metadata loading and polymorphic SOQL parent
  projection/`TYPEOF` resolution from the referenced record type.
- Improved `Database.query` dynamic SOQL binds for operator-adjacent binds,
  dotted bind paths, collection binds, date-literal colons, and catchable
  `QueryException` parse errors.
- Added `Database.queryWithBinds` support for map-provided scalar and collection
  binds, including catchable missing-bind errors.
- Added DML fidelity for implicit external-ID upsert, unique-field checks,
  lookup reference validation, ID/object mismatch errors, soft delete visibility,
  and undelete restoration.
- Added explicit external-ID upsert support for `upsert rows Field__c` and
  `Database.upsert(rows, Field__c, allOrNone)` field tokens.
- Added baseline DML merge support for the `merge` statement and
  `Database.merge`, including duplicate soft-delete, child lookup reparenting,
  and `Database.MergeResult` accessors.
- Added supported merge trigger hooks for master before/after update and
  duplicate before/after delete trigger contexts.
- Added cascade soft-delete behavior from relationship metadata, including
  Metadata API `deleteConstraint` loading for local fixtures.
- Added object-level and field-level `SObject.addError`, `hasErrors`, and
  `getErrors` handling in before-trigger DML, including row-level `SaveResult`
  error shaping and `Database.Error.getFields()` attribution.
- Preserved multiple `addError` calls per row as multiple `Database.Error`
  entries on DML result objects.
- Split governor counters for future calls, queueable jobs, batch jobs,
  scheduled jobs, and email invocations while keeping the aggregate async job
  counter.
- Tightened trigger context shape with `Trigger.isExecuting`, operation flags,
  `Trigger.size`, nullable unavailable contexts, and `Trigger.newMap`/
  `Trigger.oldMap` coverage for update and delete triggers.
- Preserved DML result alignment for bulk partial-success trigger flows where
  before-trigger `addError` filters failed rows before after triggers run.
- Added deterministic trigger recursion guard rollback with catchable
  `DmlException`.
- Added SQLite schema migrations/versioning for persistent org databases and
  exposed the schema version in DB inspection summaries.
- Added a storage DB lifecycle compatibility fixture covering SQLite seed,
  inspect, export, and reset behavior.
- Preserved source ranges through parser syntax diagnostics, compiled project
  method/trigger bodies, VM statement traces, runtime/test failure stacks, DAP
  stack frames, and profile source ranges.
- Added sema checks for invalid `override` markers, abstract methods declared
  on concrete classes, and missing concrete interface/abstract implementations.
- Added method-body sema diagnostics for local initializer and simple assignment
  type mismatches.
- Added method-body sema diagnostics for simple return type mismatches.
- Added sema and VM guards for non-void methods that fall through without a
  return value.
- Added simple binary expression typing in sema for numeric arithmetic, string
  concatenation, comparisons, and boolean operators.
- Added a sema numeric widening baseline for method-call matching from
  `Integer` to `Long`, `Decimal`, and `Double`, plus decimal-literal argument
  typing.
- Added a sema and VM object assignability baseline for class inheritance and
  interfaces across locals, assignments, returns, params, fields, and overload
  matching.
- Added sema return-type inference for known receiver method calls and chained
  constructor call expressions.
- Added sema and VM overload specificity baselines for exact matches, narrowest
  numeric widening, and nearest class/interface ancestors ahead of `Object`.
- Completed the overload specificity baseline with pairwise candidate comparison,
  ambiguous overload diagnostics/errors, and `null` calls choosing a strictly
  narrower applicable overload.
- Added an IR-backed method-body sema pass that checks scoped local reads across
  declarations, assignments, conditions, returns, calls, loops, switch, and
  try/catch/finally bodies.
- Extended the IR-backed method-body sema pass with Boolean condition checks and
  scoped declaration, assignment, and return type checks.
- Extended the IR-backed method-body sema pass with known user-object field
  read/write validation, including inherited fields.
- Extended the IR-backed method-body sema pass with known receiver and
  same-class method-call validation for unknown methods and argument mismatches.
- Extended the IR-backed method-body sema pass with constructor-call validation
  for unknown types, non-instantiable types, and argument mismatches.
- Completed inherited/interface/virtual/super sema coverage for this/super field
  and method return inference, assignments, returns, interface receivers, and
  superclass-typed virtual calls.
- Added IR-backed non-void return path analysis for `if`, `switch`, and
  try/catch control flow.
- Added inherited instance fields to method-body sema scopes.
- Added constructor chaining validation in sema for `this(...)`/`super(...)`
  placement, arity, and non-instantiable interface/enum/abstract constructor
  calls.
- Added namespace-qualified type resolution in sema, a small visibility
  diagnostic baseline, namespace-qualified class-name parsing in the VM, and
  runtime namespace checks that require global class and member access across
  namespace boundaries.
- Added qualified nested type symbols and a nested class construction, method,
  and static member execution baseline.
- Completed the inner/nested type runtime baseline with owner-relative nested
  type resolution, nested constructors/fields/methods/static members, nested
  interfaces, nested enum values/methods, and nested user-object identity
  equality coverage.
- Added Apex static and instance initializer block execution for project classes,
  including static reset behavior that reapplies static initializer blocks.
- Added `this(...)` and `super(...)` constructor chaining for supported project
  classes, with runtime guards for interface/enum/abstract instantiation.
- Added a runtime guard that blocks abstract method invocation.
- Added VM property getter/setter body execution, source-ordered field
  initialization/reset metadata, runtime visibility and namespace access checks,
  protected visibility through inheritance chains, `@TestVisible` method access
  from test classes, and overload selection by argument types with numeric and
  class/interface specificity baselines.
- Completed class/instance runtime fidelity for field initializer expressions,
  initializer block ordering, static reset, runtime access modifiers, and
  namespace boundaries.
- Added runtime virtual dispatch coverage through superclass-typed and
  interface-typed references.
- Completed runtime dispatch for declaring-class `super` calls, inherited
  concrete methods ahead of interface fallback methods, and inherited static
  fields/methods through subclass names.
- Added interface fallback method lookup, enum `name`/`ordinal`/`values`, and
  interface-based exception catch matching in the VM.
- Added VM coverage for `finally` execution across return, return override, and
  uncaught throw unwinding.
- Completed the control-flow edge baseline for loop `break`/`continue`, switch
  local `break`, enhanced-for signals, and `finally` preservation/override of
  pending loop, return, and throw signals.
- Completed the coercion baseline for numeric widening, null/object/enum
  assignment, invalid String/Boolean and narrowing rejection, collection member
  generics, method params/returns, fields, and schema-backed DML storage.
- Added explicit object `toString()` dispatch, including user-defined overrides,
  a default object fallback, and debug/assert message display.
- Added runtime coverage for user object identity equality.
- Added a VM Decimal baseline with decimal literals, integer/decimal arithmetic,
  assignment checks, storage conversion, and JSON number conversion.
- Completed the supported test-fidelity baseline: `@TestSetup` now runs once per
  class into a setup data snapshot, each test method gets an isolated org/VM
  clone with static reset, `Test.startTest()`/`Test.stopTest()` restore governor
  windows, `System.runAs` scopes UserInfo user/profile identity,
  FeatureManagement permission checks, and the supported mixed-DML guard,
  Queueable drain starts with fresh async statics, and assertion/runtime stacks
  keep file/line frames.
- Completed the supported async test baseline with `@future` method draining,
  Batchable start/execute chunking/finish, Schedulable execution, Queueable
  chaining limits, local `AsyncApexJob`/`CronTrigger` records, and an async-heavy
  compatibility fixture.
- Added binary smoke coverage for parser, exec, test, db, server, LSP
  diagnostics, profile, and compatibility commands.

Upgrade notes:

- No migration is required for this unreleased preview state.
- Persistent database and fixture formats are still preview interfaces.
