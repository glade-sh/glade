# Visualforce Wrap-Out Phases 1-7 And 10 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the valuable local Visualforce parity work from phases 1 through 7 and phase 10: oracle, lifecycle, expressions, component catalog, data-bound forms, AJAX, remoting/Remote Objects, and Lightning Out/LWC inside Visualforce.

**Architecture:** Keep Glade product behavior in `/Users/matt/Dev/glade`. Keep scratch-org capture, generated probe projects, parity reports, and large catalog maintenance in `/Users/matt/Dev/glade-tools`. Every phase starts with local Salesforce docs evidence, scratch-org evidence from `oaer-probe-max`, and failing tests, then lands product code, docs, and support facts.

**Tech Stack:** Go 1.26, existing Apex VM, `internal/visualforce`, `internal/server`, `internal/vm`, `internal/lwcbrowser`, `lwcruntime`, VitePress docs, sibling `glade-tools`, Salesforce CLI, scratch org alias `oaer-probe-max`, local docs scrape at `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run`.

---

## Starting State

The local renderer is useful already. It has page rendering, POST lifecycle paths, component registry facts, field rendering, AJAX, remoting, Remote Objects, uploads, local PDF fallback, and Lightning Out/LWC bridge pieces.

The registry still marks many components `partial`. The docs-backed catalog contains 161 Visualforce component reference entries. The work left here is not "make everything hosted Salesforce." It is contract parity for local development.

Current local truth sources:

- `/Users/matt/Dev/glade/internal/visualforce/component_catalog.go`
- `/Users/matt/Dev/glade/internal/visualforce/component_registry.go`
- `/Users/matt/Dev/glade/docs/COMPATIBILITY.md`
- `/Users/matt/Dev/glade/site/docs-src/guide/support-map.md`
- `/Users/matt/Dev/glade-tools/internal/compat/visualforce_capture.go`
- `/Users/matt/Dev/glade-tools/internal/compat/visualforce_local_capture.go`
- `/Users/matt/Dev/glade-tools/internal/compat/visualforce_diff.go`
- `/Users/matt/Dev/glade-tools/internal/toolcli/compat_visualforce_command.go`

## Execution Rules

- Use an isolated worktree for implementation.
- Use parallel squads only when file ownership does not overlap.
- Keep `glade` independent from `glade-tools`.
- Do not claim a component `supported` until it has product tests and scratch-org evidence or a precise local contract.
- Keep hosted-only services as explicit unsupported diagnostics.
- Commit after each phase or after each large phase slice.
- Run focused tests before broad tests.

## Global Verification Gates

Run these from `/Users/matt/Dev/glade` after every phase:

```bash
go test ./internal/visualforce ./internal/server ./internal/vm -count=1
go test ./...
npm --prefix lwcruntime test
scripts/smoke.sh
```

Run these when docs or site files change:

```bash
npm --prefix site ci
npm --prefix site test
npm --prefix site run build
rm -rf site/node_modules site/.vitepress/dist
```

Run these when `glade-tools` changes:

```bash
cd /Users/matt/Dev/glade-tools
go test ./...
```

Run this after a parity claim changes:

```bash
cd /Users/matt/Dev/glade
go build -o /tmp/glade-vf-wrapout ./cmd/glade

cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools visualforce capture \
  --target-org oaer-probe-max \
  --project /Users/matt/Dev/glade-tools/docs/fixtures/visualforce/probe-project \
  --out /tmp/glade-vf-salesforce.json \
  --json
go run ./cmd/glade-tools visualforce capture \
  --local \
  --glade-bin /tmp/glade-vf-wrapout \
  --project /Users/matt/Dev/glade-tools/docs/fixtures/visualforce/probe-project \
  --out /tmp/glade-vf-local.json \
  --json
go run ./cmd/glade-tools visualforce diff \
  --salesforce /tmp/glade-vf-salesforce.json \
  --local /tmp/glade-vf-local.json \
  --project /Users/matt/Dev/glade-tools/docs/fixtures/visualforce/probe-project \
  --out /tmp/glade-vf-diff.json \
  --json
jq '.ok,.counts,.diffCount' /tmp/glade-vf-diff.json
```

Expected for pages claimed by the phase:

```text
true
0
```

If a hosted-only page appears in the probe, the expected result is a stable unsupported diagnostic, not a local render pass.

---

## Phase 1: Parity Oracle And Scoreboard

**Goal:** Make one repeatable command show Salesforce-vs-local Visualforce behavior by page, component family, phase, and owner.

**Squads:**

- Oracle squad owns `/Users/matt/Dev/glade-tools/internal/compat/visualforce_capture.go`, `visualforce_local_capture.go`, `visualforce_diff.go`, and tests.
- Fixture squad owns `/Users/matt/Dev/glade-tools/docs/fixtures/visualforce/probe-project`.
- Product squad owns only small product smoke fixtures under `/Users/matt/Dev/glade/testdata/local-tests/visualforce-rendering`.
- Docs squad owns generated summaries in Glade docs.

**Files:**

- Modify: `/Users/matt/Dev/glade-tools/internal/compat/visualforce_capture.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/compat/visualforce_local_capture.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/compat/visualforce_diff.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_visualforce_command.go`
- Test: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_visualforce_command_test.go`
- Create or modify: `/Users/matt/Dev/glade-tools/docs/fixtures/visualforce/probe-project/visualforce-probe-index.json`
- Modify: `/Users/matt/Dev/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/glade/site/docs-src/guide/support-map.md`

**Implementation Steps:**

- [ ] Add or verify capture fields for each page variant: `statusCode`, `contentType`, `redirectURL`, `headers`, `htmlSha256`, `pdfSha256`, `bodySize`, `normalizedText`, `contractText`, `error`, `durationMs`.
- [ ] Add a phase lane field to `visualforce-probe-index.json`: `phase`, `family`, `components`, `claim`.
- [ ] Add 50 probe pages across these groups: lifecycle, expressions, fields, tables, custom components, templates, AJAX, remoting, Remote Objects, upload, static resources, standard controller, standard set controller, PDF, flow diagnostic, Lightning Out, and LWC.
- [ ] Keep probe page source in SFDX layout under `force-app/main/default/pages`, `classes`, `components`, `aura`, `lwc`, `staticresources`, and metadata folders.
- [ ] Extend `visualforce summary --json` to return counts by `phase`, `family`, `claim`, and `status`.
- [ ] Extend `visualforce diff` so text-contract matches do not fail on volatile IDs, CSRF, view state, generated `j_id*`, timestamps, org IDs, or long opaque blobs.
- [ ] Add a `--phase <n>` filter to `visualforce capture`, `diff`, and `summary`.
- [ ] Add tests for `--phase`, missing probe index, unknown page in index, local capture command failure, and diff output with one known mismatch.
- [ ] Generate a short checked report in Glade docs that says which phase lanes have evidence and which remain open.

**Focused Verification:**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/compat ./internal/toolcli -run 'Visualforce|visualforce' -count=1
go run ./cmd/glade-tools visualforce summary \
  --project /Users/matt/Dev/glade-tools/docs/fixtures/visualforce/probe-project \
  --json | jq '.pageCount,.groupCount'
```

Expected:

```text
50
16
```

If the project has more than 50 pages by the time this runs, update the expected value in the test and docs. Do not let the summary silently drift.

**Done Gate:**

- `glade-tools visualforce capture --target-org oaer-probe-max` works on the probe project.
- `glade-tools visualforce capture --local --glade-bin /tmp/glade-vf-wrapout` works on the same project.
- `glade-tools visualforce diff` returns phase/family counts.
- No report contains secrets, session IDs, access tokens, or full org URLs.

---

## Phase 2: Page Lifecycle And View State

**Goal:** Make GET, POST, controller extensions, standard controllers, redirects, forwards, and view state behave as one stable local request loop.

**Squads:**

- Runtime squad owns `/Users/matt/Dev/glade/internal/visualforce/page.go`, `lifecycle.go`, `form_binding.go`, and `viewstate.go`.
- VM squad owns Visualforce-facing ApexPages dispatch in `/Users/matt/Dev/glade/internal/vm`.
- Server squad owns `/Users/matt/Dev/glade/internal/server/visualforce.go`.
- Oracle squad owns lifecycle pages in the probe project.

**Files:**

- Modify: `/Users/matt/Dev/glade/internal/visualforce/viewstate.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/page.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/form_binding.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/lifecycle.go`
- Modify: `/Users/matt/Dev/glade/internal/server/visualforce.go`
- Modify as needed: `/Users/matt/Dev/glade/internal/vm/platform_apexpages*.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/viewstate_test.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/lifecycle_test.go`
- Test: `/Users/matt/Dev/glade/internal/server/visualforce_test.go`
- Test: `/Users/matt/Dev/glade/internal/server/visualforce_error_test.go`

**Implementation Steps:**

- [ ] Add typed controller snapshots for scalars, SObjects, lists, maps, nested Apex objects, and nulls. Store them in `ViewStatePayload.ControllerValues` and `ExtensionValues`.
- [ ] Keep transient fields out of `ViewStatePayload`.
- [ ] Separate page messages, component state, standard controller state, and extension state. Do not flatten them into one string map.
- [ ] Add a view-state size limit test. Use the current Salesforce-compatible local limit chosen for Glade and return a stable unsupported or limit diagnostic when exceeded.
- [ ] Add tests for missing CSRF, stale CSRF, tampered state, expired state, and wrong page name.
- [ ] Add tests for two forms on one page. Posting form A must not process form B's unrelated action.
- [ ] Add tests for extension constructor order: controller first, extension constructors after controller, action after POST restore.
- [ ] Add standard controller tests for record load from `id`, `save`, `delete`, `cancel`, and failed save rerender with messages.
- [ ] Keep server-side forwards restricted to `/apex/<PageName>`. Record URLs such as `/001000000000001AAA` must not become page names.
- [ ] Add probe pages named `Lifecycle_GetAction`, `Lifecycle_PostAction`, `Lifecycle_ExtensionOrder`, `Lifecycle_ViewStateTamper`, and `Lifecycle_ForwardRedirect`.

**Focused Verification:**

```bash
cd /Users/matt/Dev/glade
go test ./internal/visualforce -run 'ViewState|Lifecycle|FormBinding|StandardController' -count=1
go test ./internal/server -run 'Visualforce.*(Post|Error|Redirect|ViewState)' -count=1
```

**Done Gate:**

- POST failures return stable user-facing diagnostics.
- Salesforce probe and local probe agree on action result, navigation, page messages, and rerender text for claimed lifecycle pages.
- The support map lists remaining lifecycle limits by exact name.

---

## Phase 3: Expressions, Globals, And Formula Semantics

**Goal:** Make Visualforce expressions consistent enough that components do not need private string hacks.

**Squads:**

- Expression squad owns parser and evaluator files.
- VM squad owns shared formula/function behavior that already exists in Apex runtime.
- Oracle squad owns expression probe pages.
- Docs squad owns unsupported global wording.

**Files:**

- Modify: `/Users/matt/Dev/glade/internal/visualforce/expression_parser.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/expression_eval.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/expression.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/expression_formula_test.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/render_environment_test.go`
- Modify as needed: `/Users/matt/Dev/glade/internal/vm/platform_formula*.go`

**Implementation Steps:**

- [ ] Add parser tests for property chains, method calls, bracket access, string literals, numeric literals, booleans, null, unary `!`, arithmetic, comparisons, `&&`, `||`, and nested function calls.
- [ ] Add evaluator tests for list indexing, map indexing, SObject field access, controller method calls, and null-safe failures.
- [ ] Finish function support for `IF`, `CASE`, `BLANKVALUE`, `NULLVALUE`, `TEXT`, `VALUE`, `URLFOR`, `JSENCODE`, `HTMLENCODE`, and `URLENCODE`.
- [ ] Finish globals for `$CurrentPage`, `$Label`, `$Resource`, `$ObjectType`, `$User`, `$Profile`, `$Permission`, `$Setup`, `$Site`, and `$Component`.
- [ ] Add stable diagnostics for unsupported globals. The message must include the global name and `unsupported Visualforce global`.
- [ ] Add probe page `Expressions_All` with visible rows for every global and function above.
- [ ] Add probe page `Expressions_Coercion` with date, datetime, decimal, integer, boolean, null, and string coercion rows.
- [ ] Do not add component-specific fallbacks for expression bugs. Fix the parser or evaluator.

**Focused Verification:**

```bash
cd /Users/matt/Dev/glade
go test ./internal/visualforce -run 'Expression|Formula|RenderEnvironment' -count=1
```

**Done Gate:**

- Every supported expression form has parser and evaluator tests.
- Salesforce and local probe text match for claimed expression rows.
- Unsupported globals fail with named diagnostics, not blank output.

---

## Phase 4: Component Catalog Closure

**Goal:** Account for all 161 docs-backed Visualforce component reference entries with exact support facts.

**Squads:**

- Catalog squad owns component catalog generation and registry facts.
- Component squads split the component families.
- Docs squad owns generated public tables.
- Review squad verifies every status change has tests or diagnostics.

**Files:**

- Modify: `/Users/matt/Dev/glade/internal/visualforce/component_catalog.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/component_registry.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/component_references.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/component_catalog_test.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/component_registry_test.go`
- Create: `/Users/matt/Dev/glade/internal/visualforce/component_support_test.go`
- Modify: `/Users/matt/Dev/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/glade/site/docs-src/guide/support-map.md`

**Implementation Steps:**

- [ ] Add a generated support table source in product code. Each row must include `name`, `status`, `reason`, `docSource`, `attributes`, `localEvidence`, and `hostedBoundary`.
- [ ] Keep the existing 161-entry docs count test. If the local docs scrape changes, regenerate the catalog and update the test in the same commit.
- [ ] For every `partial` row, replace family-level text with exact missing attributes or behaviors.
- [ ] For every `unsupported` row, name one of these causes: `hosted-service`, `obsolete-runtime`, `missing-local-subsystem`, or `not-a-standalone-component`.
- [ ] Promote locally complete UI-only components where practical: `apex:panelGrid`, `apex:panelGroup`, `apex:sectionHeader`, `apex:toolbar`, `apex:toolbarGroup`, `apex:tabPanel`, `apex:tab`, `apex:panelBar`, and `apex:panelBarItem`.
- [ ] Keep hosted services unsupported: Analytics, Chatter, Chatter Answers, Ideas, Knowledge, Live Agent, Sites, Social, Support, Topics, Wave, Canvas, vote, milestone tracker, publisher components, Flash, and S-Control.
- [ ] Add renderer tests for every component promoted to `supported`.
- [ ] Add diagnostic tests for every hosted-only family.
- [ ] Add a doc generation step that writes the support table into `docs/COMPATIBILITY.md` and `site/docs-src/guide/support-map.md` from the same facts.

**Focused Verification:**

```bash
cd /Users/matt/Dev/glade
go test ./internal/visualforce -run 'Component(Catalog|Registry|Support)|Render' -count=1
```

**Done Gate:**

- No docs-backed component has an unknown support state.
- No `partial` reason says only "covers a subset".
- Every supported component has at least one renderer test.
- Every unsupported family has one diagnostic test.
- Docs and registry agree.

---

## Phase 5: Data-Bound Forms And Field Rendering

**Goal:** Make ordinary Visualforce CRUD pages useful: render, edit, save, fail validation, show messages, and rerender.

**Squads:**

- Schema squad owns field metadata interpretation.
- Renderer squad owns field output/input components.
- Lifecycle squad owns submitted values, conversion, validation errors, and rerender.
- Standard controller squad owns record and list behavior.

**Files:**

- Modify: `/Users/matt/Dev/glade/internal/visualforce/field_rendering.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/form_binding.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/page.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/render.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/component_registry.go`
- Modify as needed: `/Users/matt/Dev/glade/internal/vm/platform_apexpages*.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/field_rendering_test.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/form_binding_test.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/lifecycle_test.go`

**Implementation Steps:**

- [ ] Add a field matrix fixture covering text, textarea, checkbox, integer, double, percent, currency, date, datetime, picklist, multi-picklist, lookup, master-detail, email, phone, URL, rich text, ID, auto-number, formula, and summary fields.
- [ ] Render `apex:outputField` with display text for each field family.
- [ ] Render `apex:inputField` with the correct HTML control for each editable field family.
- [ ] Honor required, read-only, createable, updateable, nillable, defaulted-on-create, and auto-number flags from local schema.
- [ ] Convert posted strings into typed storage values for boolean, integer, decimal, date, datetime, picklist, multi-picklist, lookup IDs, and empty strings.
- [ ] Add validation errors for missing required fields, bad dates, bad numbers, bad lookup IDs, inactive picklist values, and read-only field edits.
- [ ] Bind failed validation errors to `apex:message`, `apex:messages`, `apex:pageMessage`, and `apex:pageMessages`.
- [ ] Finish standard controller `save`, `delete`, `view`, `edit`, `cancel`, and `quickSave` where locally meaningful.
- [ ] Add standard set controller support for list view rows, selected rows, page size, next, previous, first, last, and result size.
- [ ] Keep inline edit and enhanced list partial unless their client runtime is implemented in this phase.

**Focused Verification:**

```bash
cd /Users/matt/Dev/glade
go test ./internal/visualforce -run 'FieldRendering|FormBinding|StandardController|StandardSet' -count=1
go test ./internal/server -run 'Visualforce.*Field|Visualforce.*Post' -count=1
```

**Done Gate:**

- A standard-controller Account edit page can load from `id`, edit fields, save into local org state, fail validation, show field errors, and rerender with user-entered values.
- Standard set controller pages can page through deterministic local rows.
- Docs list the unsupported field families or UI details by exact name.

---

## Phase 6: AJAX And Partial Refresh

**Goal:** Make Visualforce AJAX deterministic in the local browser.

**Squads:**

- Server squad owns partial response format and target lookup.
- Runtime squad owns action ordering, submitted values, regions, and validation.
- Client squad owns emitted JavaScript.
- Browser squad owns DOM tests in `lwcruntime/test` or the existing Node browser harness.

**Files:**

- Modify: `/Users/matt/Dev/glade/internal/visualforce/ajax.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/partial.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/page.go`
- Modify: `/Users/matt/Dev/glade/internal/server/visualforce.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/ajax_test.go`
- Test: `/Users/matt/Dev/glade/internal/server/visualforce_ajax_test.go`
- Create or modify: `/Users/matt/Dev/glade/lwcruntime/test/visualforce-ajax.test.mjs`

**Implementation Steps:**

- [ ] Support `apex:commandButton` and `apex:commandLink` AJAX submission with `reRender`.
- [ ] Support `apex:actionFunction` with generated callable JavaScript function names.
- [ ] Support `apex:actionSupport` for click, change, keyup, blur, and submit events.
- [ ] Support `apex:actionRegion` so only the region's submitted fields are processed unless `immediate` changes the lifecycle.
- [ ] Support `apex:actionStatus` start/stop text, style, and callbacks.
- [ ] Support `apex:actionPoller` with interval, enabled, action, and `reRender`.
- [ ] Preserve `apex:param` ordering for actions and AJAX calls.
- [ ] Update the hidden view-state field after every successful partial response.
- [ ] Add nested ID target lookup for local IDs and Visualforce-generated IDs.
- [ ] Add browser tests for click, keyup, function call, poller tick, status start/stop, missing target diagnostic, and nested target replacement.

**Focused Verification:**

```bash
cd /Users/matt/Dev/glade
go test ./internal/visualforce -run 'Ajax|Partial|Action' -count=1
go test ./internal/server -run 'VisualforceAjax' -count=1
npm --prefix lwcruntime test -- --run visualforce-ajax
```

**Done Gate:**

- Local browser tests prove DOM updates and view-state refresh.
- Salesforce and local probes agree on action result and visible target text for claimed AJAX pages.
- Missing or invalid targets produce stable diagnostics.

---

## Phase 7: JavaScript Remoting And Remote Objects

**Goal:** Support old Visualforce JavaScript data paths against local Apex and local org state.

**Squads:**

- Remoting squad owns `remoting.go` and remoting server routes.
- Remote Objects squad owns `remote_objects.go` and CRUD dispatch.
- Security squad owns size limits, CSRF/view-state checks, timeout handling, and JSON error shapes.
- Browser squad owns JavaScript execution tests.

**Files:**

- Modify: `/Users/matt/Dev/glade/internal/visualforce/remoting.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/remote_objects.go`
- Modify: `/Users/matt/Dev/glade/internal/server/visualforce.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/remoting_test.go`
- Test: `/Users/matt/Dev/glade/internal/visualforce/remote_objects_test.go`
- Test: `/Users/matt/Dev/glade/internal/server/visualforce_remoting_test.go`
- Create or modify: `/Users/matt/Dev/glade/lwcruntime/test/visualforce-remoting.test.mjs`

**Implementation Steps:**

- [ ] Discover `@RemoteAction` methods on page controllers and extensions.
- [ ] Enforce `static` and `public` or `global`.
- [ ] Support positional JSON argument conversion into Apex values.
- [ ] Execute remoting calls through the local VM with current user, org state, and limit mode.
- [ ] Match the response envelope fields: `action`, `method`, `type`, `tid`, `status`, `result`, `message`, `where`, and `errors`.
- [ ] Enforce the 4 MB remoting request limit.
- [ ] Add timeout handling with a deterministic local max. Return a remoting-style failure envelope.
- [ ] Build Remote Objects descriptors from `apex:remoteObjects`, `apex:remoteObjectModel`, and `apex:remoteObjectField`.
- [ ] Support Remote Objects create, retrieve, update, and delete against local org state.
- [ ] Return Salesforce-shaped errors for unknown object, unknown field, validation failure, missing ID, read-only field, CSRF mismatch, and tampered view state.
- [ ] Add browser tests that call both `Visualforce.remoting.Manager.invokeAction` and generated Remote Objects model methods.

**Focused Verification:**

```bash
cd /Users/matt/Dev/glade
go test ./internal/visualforce -run 'Remoting|RemoteObject' -count=1
go test ./internal/server -run 'VisualforceRemoting|RemoteObjects' -count=1
npm --prefix lwcruntime test -- --run visualforce-remoting
```

**Done Gate:**

- Browser tests execute real local Apex remoting calls.
- Browser tests execute Remote Objects CRUD against local org state.
- Unsupported hosted behavior returns named diagnostics and never falls through to generic REST handlers.

---

## Phase 10: Lightning Out And LWC In Visualforce

**Goal:** Make Visualforce-hosted Lightning useful for realistic local LWC workflows.

**Squads:**

- Aura/Lightning Out squad owns `$Lightning.use`, `$Lightning.createComponent`, Aura dependency scanning, and callbacks.
- LWC module squad owns compiled LWC modules, import maps, and Salesforce module shims.
- Data squad owns `@salesforce/apex`, LDS, labels, resources, user, and i18n support.
- Browser squad owns multi-component behavior tests.

**Files:**

- Modify: `/Users/matt/Dev/glade/internal/visualforce/render.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/component_registry.go`
- Modify: `/Users/matt/Dev/glade/internal/server/lightning.go`
- Modify: `/Users/matt/Dev/glade/internal/server/lightning_shims.go`
- Modify: `/Users/matt/Dev/glade/internal/server/lightning_wire.go`
- Modify: `/Users/matt/Dev/glade/internal/lwcbrowser/manifest.go`
- Modify as needed: `/Users/matt/Dev/glade/internal/lwcbrowser/*.go`
- Modify as needed: `/Users/matt/Dev/glade/internal/lwcruntime/embed/glade.out.js`
- Test: `/Users/matt/Dev/glade/internal/server/lightning_test.go`
- Test: `/Users/matt/Dev/glade/lwcruntime/test/visualforce-dev-server.test.mjs`
- Test: `/Users/matt/Dev/glade/lwcruntime/test/multi-component.test.mjs`
- Test: `/Users/matt/Dev/glade/lwcruntime/test/wire.test.mjs`
- Test: `/Users/matt/Dev/glade/lwcruntime/test/record-wire.test.mjs`
- Create or modify: `/Users/matt/Dev/glade/lwcruntime/test/visualforce-lightningout.test.mjs`

**Implementation Steps:**

- [ ] Support Aura apps that extend `ltng:outApp`.
- [ ] Scan `aura:dependency` entries and resolve LWC components through the compiled LWC manifest.
- [ ] Keep Aura wrapper passthrough aliases. A one-child Aura wrapper should resolve to its underlying LWC module when safe.
- [ ] Keep callback parity: success calls `callback(el, "SUCCESS")`; failure calls `callback(null, "ERROR", message)`.
- [ ] Support multiple `$Lightning.createComponent` calls on one page.
- [ ] Support simple events between host page and mounted LWC components through DOM events.
- [ ] Serve `@salesforce/apex` modules that call local Apex controllers through existing server routes.
- [ ] Serve `lightning/uiRecordApi` with local `getRecord` and the current local record wire shape.
- [ ] Serve labels, schema tokens, static resource URLs, content asset URLs, user IDs, and i18n modules through existing shim routes.
- [ ] Add missing dependency diagnostics for missing Aura app, missing dependency, missing LWC module, bad component name, and unsupported Lightning service.
- [ ] Add a multi-component fixture page with Apex data, LDS data, label text, static resource URL, one event, and one missing dependency negative case.
- [ ] Add scratch-org capture for the same Visualforce page. Compare boot-script contract text and visible post-mount DOM, not private Salesforce script bytes.

**Focused Verification:**

```bash
cd /Users/matt/Dev/glade
go test ./internal/server -run 'Lightning|Visualforce.*Lightning|LWC' -count=1
npm --prefix lwcruntime test -- --run visualforce
npm --prefix lwcruntime test -- --run multi-component
npm --prefix lwcruntime test -- --run wire
npm --prefix lwcruntime test -- --run record-wire
```

**Done Gate:**

- A Visualforce page can host multiple LWCs locally.
- LWC components can use local Apex, local LDS record data, labels, schema imports, resource URLs, user, and i18n shims.
- Missing dependencies fail with stable diagnostics.
- The docs say this is local Lightning Out/LWC support inside Visualforce, not full Lightning Experience parity.

---

## Phase Order And Parallelism

Run the phases in this order:

```text
1 -> 2 -> 3 -> 4
          \-> 5
          \-> 6
          \-> 7
          \-> 10
4 + 5 + 6 + 7 + 10 -> final docs and support map refresh
```

Phase 1 must land first. Phase 2 should land before phases 5, 6, and 7 because forms, AJAX, and remoting all depend on view state and lifecycle. Phase 3 can run beside Phase 2 if it touches only expression files. Phase 4 should run as a catalog squad through the whole effort. Phases 5, 6, 7, and 10 can run in parallel after Phase 2 if each squad keeps file ownership clean.

## Agent Handoff Prompts

Use this prompt for Phase 1:

```text
Implement Phase 1 from /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-14-visualforce-wrapout-phases-1-7-10.md.
Use superpowers:subagent-driven-development.
Keep product code in /Users/matt/Dev/glade and parity tooling in /Users/matt/Dev/glade-tools.
Use oaer-probe-max for scratch-org evidence.
Return changed files, commands run, report paths, pass counts, and remaining unsupported boundaries.
```

Use this prompt for Phase 2:

```text
Implement Phase 2 from /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-14-visualforce-wrapout-phases-1-7-10.md.
Dispatch runtime, VM, server, oracle, and review squads.
Do not claim lifecycle parity until CSRF, tampered view state, expired state, extensions, standard controller, redirect, and forward tests pass.
Return changed files, focused tests, full gates, and scratch-org diff evidence.
```

Use this prompt for Phases 3, 5, 6, 7, or 10:

```text
Implement Phase <N> from /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-14-visualforce-wrapout-phases-1-7-10.md in full.
Use parallel squads only where file ownership is disjoint.
Start with failing tests and scratch-org or docs-backed evidence.
Do not leave silent fallback behavior.
Every unsupported local boundary needs a stable diagnostic and a test.
Return changed files, commands run, scratch-org evidence, docs updates, and exact remaining gaps.
```

## Final Wrap-Out Gate

Run this only after phases 1-7 and 10 are complete:

```bash
cd /Users/matt/Dev/glade
go test ./internal/visualforce ./internal/server ./internal/vm -count=1
go test ./...
npm --prefix lwcruntime test
scripts/smoke.sh
npm --prefix site ci
npm --prefix site test
npm --prefix site run build
rm -rf site/node_modules site/.vitepress/dist

cd /Users/matt/Dev/glade-tools
go test ./...
go run ./cmd/glade-tools visualforce summary \
  --project /Users/matt/Dev/glade-tools/docs/fixtures/visualforce/probe-project \
  --json
```

Then run the scratch-org/local diff against `oaer-probe-max` and save the artifacts under `/tmp` or a checked `glade-tools` report location if the report is intended to remain.

The wrap-out is complete when these facts are true:

- Phase 1 scoreboard is repeatable.
- Phase 2 lifecycle pages match the local contract and scratch-org evidence.
- Phase 3 expressions are covered by parser, evaluator, and probe tests.
- Phase 4 has no unknown docs-backed component rows.
- Phase 5 CRUD pages work locally with real org state.
- Phase 6 AJAX browser tests update DOM and view state.
- Phase 7 remoting and Remote Objects execute through local Apex and local org state.
- Phase 10 Visualforce-hosted LWC works with Apex, LDS, labels, resources, user, and i18n shims.
- Public docs say exactly what is supported, partial, and unsupported.
