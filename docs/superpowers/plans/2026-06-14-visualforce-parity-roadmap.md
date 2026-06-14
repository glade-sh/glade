# Visualforce Parity Roadmap

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development for every implementation phase. Each phase uses parallel squads with disjoint ownership. Steps use checkbox (`- [ ]`) syntax for tracking. Do not mark a phase done until every gate in that phase passes.

**Goal:** Reach practical Visualforce rendering parity: local pages render, post, refresh, invoke controllers, serve data-bound fields, host Lightning Out/LWC, produce PDFs, enforce documented limits, and match Salesforce behavior closely enough for local development, tests, CI, and AI-assisted review.

**Architecture:** Keep product behavior in `glade`: `internal/visualforce`, `internal/server`, `internal/vm`, `internal/lwcbrowser`, docs, and product fixtures. Keep parity capture, generated scoreboards, large fixture sweeps, and docs inventory work in `/Users/matt/Dev/glade-tools`. Every phase starts from black-box Salesforce evidence, local docs, and failing product tests, then lands product code with a checked support map.

**Tech Stack:** Go 1.26, existing Apex VM, `golang.org/x/net/html`, existing local org storage/schema/runtime, `lwcruntime`, VitePress docs, `sf` CLI, `oaer-probe-max` scratch org, local Salesforce docs scrape at `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run`.

---

## Practicality Boundary

Literal parity means byte-for-byte HTML, CSS, JavaScript, browser timing, undocumented internal Salesforce scripts, release-specific chrome, and every hosted platform integration. That is not practical. It is not useful either. Salesforce can change those internals without contract.

The useful target is **contract parity**:

- Same controller lifecycle, data binding, navigation, errors, security fences, and Apex-visible behavior.
- Same rendered structure for documented Visualforce components where developers depend on it.
- Same AJAX/partial-refresh observable behavior for local deterministic pages.
- Same `PageReference.getContent()` and `getContentAsPDF()` contract, with documented local fidelity limits.
- Same limits, validation, and explicit unsupported diagnostics for hosted-only surfaces.
- Checked evidence from Salesforce for every supported claim.

Stop using the phrase "full Visualforce support" until this roadmap's Phase 11 gate passes. Before then, docs should say which areas are supported, partial, or unsupported.

## Worker Rules

Every phase must be executed with squads. One worker owns one write surface. No worker rewrites another worker's files. The main agent reviews and integrates.

Required squad pattern for each phase:

- **Oracle squad:** `glade-tools`, scratch-org fixtures, capture, diff reports. No product imports from `glade-tools`.
- **Runtime squad:** product renderer, lifecycle, VM, server routes.
- **Component squad:** component registry, component renderers, local fixtures.
- **Docs/support squad:** checked support table, public docs, local-testing docs.
- **Review squad:** read-only code review after integration.

No phase is done if any squad leaves an explicit unsupported marker for a surface the phase claims as supported.

## Global Gates

Every phase must end with:

```bash
go test ./internal/visualforce ./internal/server ./internal/vm -count=1
go test ./...
npm --prefix lwcruntime test
scripts/smoke.sh
```

When docs or site files change:

```bash
npm --prefix site ci
npm --prefix site test
npm --prefix site run build
rm -rf site/node_modules site/.vitepress/dist
```

When `glade-tools` changes:

```bash
cd /Users/matt/Dev/glade-tools
go test ./...
```

When Salesforce parity claims change:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools visualforce capture \
  --target-org oaer-probe-max \
  --project <probe-project> \
  --pages <comma-separated-pages> \
  --out /tmp/glade-vf-parity-capture.json
jq '.ok,.counts' /tmp/glade-vf-parity-capture.json
```

Expected for claimed pages:

```text
true
htmlFail: 0
pdfFail: 0
```

## Phase 0: Stabilize Current Branch

**Purpose:** Finish the current Visualforce branch as the new baseline. No roadmap work starts on a shaky floor.

**Squads:**

- Runtime squad owns `internal/visualforce/page.go`, `internal/vm/ui_invocation.go`, and navigation tests.
- Review squad owns read-only review of POST lifecycle, standard controller actions, and PageReference navigation.

**Tasks:**

- [ ] Keep server-side `PageReference` forwards limited to `/apex/...` targets.
- [ ] Keep record URLs such as `/001000000000001AAA` from being treated as Visualforce page names.
- [ ] Keep standard-controller actions routed through VM platform member dispatch.
- [ ] Remove duplicate tests and run `gofmt`.
- [ ] Run focused tests: `go test ./internal/visualforce ./internal/server ./internal/vm -count=1`.
- [ ] Run full product and tool gates.

**Done:** The current branch passes all global gates and the scratch-org capture remains `11 htmlPass`, `11 pdfPass`, `0 fail`.

## Phase 1: Parity Oracle And Scoreboard

**Purpose:** Make Salesforce the measuring stick. Every later phase must know what it is matching.

**Squads:**

- Oracle squad owns `/Users/matt/Dev/glade-tools/internal/compat/visualforce_capture.go`.
- Fixture squad owns `/tmp` generated projects and later checked fixture templates in `glade-tools`.
- Product squad owns only small checked product fixtures under `testdata/local-tests/visualforce-rendering/`.

**Tasks:**

- [ ] Extend `glade-tools visualforce capture` to write raw metadata per page: status code, content type, redirect URL, headers that matter, HTML hash, PDF hash, body size, and normalized text.
- [ ] Add local render capture in `glade-tools`: invoke product renderer through a built binary or subprocess, never by importing `glade-tools` into product code.
- [ ] Add `glade-tools visualforce diff --salesforce <json> --local <json> --out <json>` with per-page and per-component differences.
- [ ] Build a 50-page probe project from local docs groups: lifecycle, fields, tables, AJAX, templates, custom components, static resources, remoting, remote objects, upload, flow, Lightning Out, PDF, standard controller, standard set controller.
- [ ] Capture all 50 pages against `oaer-probe-max`.
- [ ] Save curated, redacted fixtures in `glade-tools/docs/fixtures/visualforce/`.

**Done:** A single command produces a local-vs-Salesforce report with pass/fail counts by page, component, lifecycle stage, and owner lane. No secrets. No session IDs. No debug-log truncation.

## Phase 2: Page Lifecycle And View State

**Purpose:** Match the core Visualforce request loop before adding more components.

**Squads:**

- Runtime squad owns `internal/visualforce/page.go`, `lifecycle.go`, `form_binding.go`, and `viewstate.go`.
- VM squad owns controller construction, extension construction, `ApexPages.Action.invoke`, and standard controller dispatch.
- Server squad owns CSRF, POST, redirects, forwards, content type, cache headers, and AJAX response shape.

**Scope:**

- GET constructor order.
- Page `action`.
- POST restore, setters, validation, action, getters, render.
- Controller extensions.
- Standard controller and standard set controller state.
- `PageReference` redirect and server-side forward.
- `transient` fields excluded from view state.
- SObject, list, map, scalar, and nested controller field restore.

**Tasks:**

- [ ] Replace string-only controller snapshots with typed view-state values.
- [ ] Store controller, extension, standard-controller, component, and page-message state separately.
- [ ] Restore object identity enough for repeated getters/setters inside one request.
- [ ] Add tests for multiple forms, missing CSRF, stale CSRF, tampered state, expired state, and view-state size limit.
- [ ] Add scratch-org parity pages for extension actions, setters, field errors, and redirect/forward behavior.

**Done:** Local POST behavior matches Salesforce on the lifecycle fixture suite. View-state errors are stable and documented.

## Phase 3: Expressions, Globals, And Formula Semantics

**Purpose:** Make Visualforce expressions boring. Components depend on this.

**Squads:**

- Expression squad owns `internal/visualforce/expression_parser.go` and `expression_eval.go`.
- VM squad owns formula/function hooks reused from Apex where possible.
- Oracle squad owns expression-only parity fixtures.

**Scope:**

- Property chains and method calls.
- Bracket access and map/list indexing.
- Boolean, numeric, string, null, date, datetime coercion.
- `$CurrentPage`, `$Label`, `$Resource`, `$ObjectType`, `$User`, `$Profile`, `$Permission`, `$Setup`, `$Site`, `$Component`.
- Functions used in Visualforce: `IF`, `CASE`, `BLANKVALUE`, `NULLVALUE`, `TEXT`, `VALUE`, `URLFOR`, `JSENCODE`, `HTMLENCODE`, `URLENCODE`.
- Template expressions in attributes and text nodes.

**Tasks:**

- [ ] Generate an expression parity fixture page with each global and function.
- [ ] Add parser tests for every grammar form.
- [ ] Add evaluator tests with typed expected values.
- [ ] Add Salesforce capture and local diff for expression results.
- [ ] Document unsupported globals with exact diagnostics.

**Done:** Expression failures are rare, specific, and covered by a scoreboard row. No renderer component should work around expression bugs with local string hacks.

## Phase 4: Component Catalog Closure

**Purpose:** Account for every documented Visualforce component and make support claims exact.

**Squads:**

- Catalog squad owns `component_catalog.go`, generated source, and docs scrape mapping.
- Component squads split by families:
  - Core output/input/layout components.
  - Tables, lists, repeats, panels, tabs.
  - Templates and custom components.
  - AJAX components.
  - Data/detail/related-list components.
  - Hosted-only namespace components.
- Docs squad owns support-map output.

**Tasks:**

- [ ] For all 161 `visualforce/pages_compref_*.md` files, assign exactly one status: `supported`, `partial`, or `unsupported`.
- [ ] For `partial`, list exact missing attributes or behaviors.
- [ ] For `unsupported`, list the hosted dependency or missing local subsystem.
- [ ] Add renderer tests for every `supported` component.
- [ ] Add explicit diagnostic tests for every `unsupported` component family.
- [ ] Add a generated support table checked into product docs.

**Done:** The component registry has no unknown docs-backed entries. Public docs are generated from the same support facts as tests.

## Phase 5: Data-Bound Forms And Field Rendering

**Purpose:** Make standard controller pages and ordinary CRUD forms useful.

**Squads:**

- Schema squad owns field metadata interpretation.
- Renderer squad owns `outputField`, `inputField`, `detail`, `relatedList`, and validation markup.
- Lifecycle squad owns submitted values, conversion, errors, and rerender after failed save.

**Scope:**

- Text, textarea, checkbox, number, percent, currency, date, datetime, picklist, multi-picklist, lookup, master-detail, email, phone, URL, rich text.
- Required fields and validation errors.
- FLS/read-only/createable/updateable flags from local schema.
- Standard controller record loading from `id`.
- StandardSetController list views and pagination.

**Tasks:**

- [ ] Create field matrix fixtures with one page per field family.
- [ ] Add local schema metadata for required, read-only, picklist, lookup, and relationship cases.
- [ ] Match Salesforce markup enough for selectors and local browser tests.
- [ ] Add POST conversion tests for true/false, null, empty string, date/time, multi-select, and lookup ids.
- [ ] Add standard-controller save/delete/view/cancel tests with local org state.

**Done:** A generated CRUD page using standard Visualforce components can be rendered, edited, saved, failed with errors, and rerendered locally.

## Phase 6: AJAX And Partial Refresh

**Purpose:** Match the user-visible behavior of Visualforce AJAX.

**Squads:**

- Server squad owns partial response format and region processing.
- Client squad owns emitted JavaScript.
- Runtime squad owns action ordering and action status state.
- Browser squad owns Playwright or Node DOM tests.

**Scope:**

- `reRender`.
- `apex:actionFunction`.
- `apex:actionSupport`.
- `apex:actionRegion`.
- `apex:actionStatus`.
- `apex:actionPoller`.
- `apex:param` parameter ordering.
- Validation, immediate actions, timeout, disabled/enabled state.

**Tasks:**

- [ ] Add browser tests for click, keyup, form submit, poller tick, and status start/stop.
- [ ] Add server tests for partial target lookup by local and nested form IDs.
- [ ] Add region tests proving only the right submitted values are processed.
- [ ] Add parity captures for the same pages in Salesforce.

**Done:** AJAX fixtures pass in local browser tests and match Salesforce capture on action result, target HTML, and view-state refresh.

## Phase 7: JavaScript Remoting And Remote Objects

**Purpose:** Support the old but common Visualforce JavaScript data paths.

**Squads:**

- Remoting squad owns `internal/visualforce/remoting.go` and server routes.
- Remote Objects squad owns `remote_objects.go` and CRUD dispatch.
- Security squad owns request size, timeout, CSRF/session checks, and JSON shape.

**Scope:**

- `@RemoteAction` discovery on controller and extensions.
- Static public/global method requirement.
- Remoting request/response envelope and exception shape.
- 4 MB remoting request limit.
- Timeout defaults and max.
- Remote Objects model generation.
- Create, retrieve, update, delete against local org state.

**Tasks:**

- [ ] Add scratch-org pages that call remoting and Remote Objects from JavaScript.
- [ ] Add local browser tests that execute those calls.
- [ ] Add unsupported diagnostics for hosted-only behavior.
- [ ] Add support-map rows with evidence links.

**Done:** Remoting and Remote Objects work for local deterministic org data without leaking into generic REST handlers.

## Phase 8: Files, Static Resources, Assets, And CSP

**Purpose:** Make pages with assets behave like real pages.

**Squads:**

- Upload squad owns multipart parsing and `apex:inputFile`.
- Resource squad owns static resource URLs, zip subpaths, cache busting, and content type.
- Security squad owns CSP, XSS, escaping, and cache headers.

**Scope:**

- 10 MB file upload limit.
- Blob, filename, content type, size binding.
- Static resource direct and nested paths.
- `apex:stylesheet`, `apex:includeScript`, `apex:image`.
- `contentType`, `cache`, `cspHeader`, `showHeader`, `sidebar`, `standardStylesheets`, `lightningStylesheets`.

**Tasks:**

- [ ] Add multipart upload server tests.
- [ ] Add controller binding tests for Blob and filename.
- [ ] Add static resource zip fixture and nested URL tests.
- [ ] Add CSP/cache/content-type response tests.
- [ ] Add XSS parity tests for escaped and unescaped output.

**Done:** Asset-heavy pages run locally without special exceptions. Security behavior is explicit and tested.

## Phase 9: PDF Fidelity

**Purpose:** Move from a basic PDF Blob to useful local PDF rendering.

**Squads:**

- PDF toolchain squad owns `internal/visualforce/pdf.go`.
- Browser/render squad owns HTML-to-PDF rendering through a deterministic bundled or discoverable engine.
- Oracle squad owns Salesforce PDF captures and text/image comparisons.

**Scope:**

- `renderAs="pdf"`.
- `PageReference.getContentAsPDF()`.
- CSS, page size, image embedding, fonts, simple pagination.
- PDF limits: 60 MB PDF, 30 MB images, 15 MB HTML response.
- Explicit fallback when the full PDF toolchain is absent.

**Tasks:**

- [ ] Choose the product PDF engine. Prefer an existing local/bundled browser toolchain if available.
- [ ] Add renderer interface tests for HTML, CSS, images, and pagination.
- [ ] Add PDF text extraction checks for expected content.
- [ ] Add Salesforce PDF capture comparison on text and size, not byte equality.
- [ ] Document fidelity level.

**Done:** Local PDFs are good enough for CI assertions and human inspection. Byte equality is not claimed.

## Phase 10: Lightning Out And LWC In Visualforce

**Purpose:** Make Visualforce-hosted Lightning useful beyond the current smoke fixture.

**Squads:**

- Aura/Lightning Out squad owns `$Lightning.use`, `$Lightning.createComponent`, dependency resolution, and callbacks.
- LWC module squad owns `@salesforce/*`, LDS, wire adapters, labels, resources, Apex calls.
- Browser squad owns multi-component interaction tests.

**Scope:**

- Aura app extending `ltng:outApp`.
- `aura:dependency` scanning.
- Multiple components on one page.
- Events between components and host page.
- `@salesforce/apex`, `getRecord`, labels, resources, user/i18n modules.
- Error callbacks and missing dependency diagnostics.

**Tasks:**

- [ ] Add a multi-component Lightning Out fixture with Apex, LDS, labels, resources, and events.
- [ ] Add browser tests for callback status and DOM update.
- [ ] Add server tests for missing Aura app and missing component diagnostics.
- [ ] Add parity capture page for Salesforce HTML boot scripts and local behavior.

**Done:** A Visualforce page can host realistic LWC workflows locally without manual browser setup.

## Phase 11: Flow And Hosted-Only Boundaries

**Purpose:** Decide what Glade should emulate and what it should fence.

**Squads:**

- Flow squad owns `flow:interview` and local Flow runtime integration if practical.
- Boundary squad owns hosted-only diagnostics for Chatter, Live Agent, Knowledge, Wave, Sites, Support, Messaging, and other service-backed components.
- Docs squad owns support wording.

**Decision gate for `flow:interview`:**

- Implement if existing local Flow metadata and runtime can execute screen-flow variables and finish navigation with bounded work.
- Keep unsupported if it requires building a separate Flow runtime not already on the product roadmap.

**Tasks:**

- [ ] Inventory every non-`apex` Visualforce namespace component from the docs catalog.
- [ ] For each namespace, classify as local-emulatable, product dependency, or hosted-only.
- [ ] Implement `flow:interview` only if the decision gate passes.
- [ ] Add explicit diagnostics for every hosted-only component.
- [ ] Add docs that explain these fences as product boundaries, not missing random work.

**Done:** No component fails by surprise. Unsupported components fail with stable, useful messages.

## Phase 12: Large Corpus And Public Claim Gate

**Purpose:** Prove this against real projects before changing public wording.

**Squads:**

- Corpus squad owns example projects and selected large internal projects.
- Performance squad owns timing, memory, cache, and timeout budgets.
- Docs squad owns public support map.
- Review squad owns final code review and release gate.

**Tasks:**

- [ ] Run `glade-tools` visualforce discovery across `example-projects`.
- [ ] Run against a selected large Visualforce-heavy project.
- [ ] Record blocker counts before and after.
- [ ] Profile render hot paths for large pages and repeated POST/AJAX loops.
- [ ] Generate the final support map from checked facts.
- [ ] Remove "full Visualforce unsupported" wording only where the facts justify it.

**Done:** Public docs can claim practical Visualforce parity with a generated component table, known hosted-only boundaries, and scratch-org evidence.

## Phase Execution Order

Run phases in this order:

```text
0 -> 1 -> 2 -> 3 -> 4
              \-> 5
              \-> 6
              \-> 7
              \-> 8
              \-> 9
              \-> 10
4 + 5 + 6 + 7 + 8 + 9 + 10 -> 11 -> 12
```

Phases 3 through 10 can have parallel squads after Phase 2 lands. Phase 4 must keep the catalog truth current while those squads work.

## Phase Handoff Template

When asking an agent to implement a phase, use this prompt shape:

```text
Implement Phase <N> from docs/superpowers/plans/2026-06-14-visualforce-parity-roadmap.md in full.

Use superpowers:subagent-driven-development.
Dispatch parallel squads exactly as the phase defines.
Do not stop after partial code edits.
Do not claim support until Salesforce capture, local tests, docs/support map, and review gates pass.
Keep product code in /Users/matt/Dev/glade.
Keep parity tooling and generated maintenance reports in /Users/matt/Dev/glade-tools.
Return changed files, commands run, scratch-org evidence, and remaining explicit unsupported boundaries.
```

## Final Parity Gate

Practical parity is reached when all of these are true:

- All docs-backed Visualforce components have a checked `supported`, `partial`, or `unsupported` status.
- Every `supported` component has product tests and scratch-org evidence.
- Every `partial` component lists exact missing behavior.
- Every `unsupported` component has a stable diagnostic.
- GET, POST, AJAX, remoting, remote objects, uploads, static resources, PDFs, standard controllers, custom components, templates, and Lightning Out pass the fixture suite.
- Large corpus scans show no unknown Visualforce blocker classes.
- Public docs are generated from the support facts.
- `go test ./...`, `glade-tools go test ./...`, `npm --prefix lwcruntime test`, `scripts/smoke.sh`, and site tests/build all pass.

Only then should docs say Glade has practical Visualforce rendering parity.
