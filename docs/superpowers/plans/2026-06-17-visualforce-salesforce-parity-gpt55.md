# Visualforce Salesforce Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` for implementation. The implementing model should be GPT-5.5 High. Dispatch parallel subagents by squad, keep file ownership disjoint, and integrate through a main conductor agent after each phase. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move Glade Visualforce rendering from useful local preview to practical Salesforce contract parity for local development, tests, CI, and AI-assisted review.

**Architecture:** Salesforce remains the oracle. Product behavior stays in `/Users/matt/Dev/glade`; scratch-org capture, broad fixture generation, scoreboards, and generated maintenance reports stay in `/Users/matt/Dev/glade-tools`. Every supported claim needs local tests, scratch-org evidence from `oaer-probe-max`, a support-ledger row, and public docs.

**Tech Stack:** Go 1.26, existing Apex VM, `internal/visualforce`, `internal/server`, `internal/vm`, `internal/lwcbrowser`, `lwcruntime`, VitePress docs, sibling `glade-tools`, Salesforce CLI, scratch org alias `oaer-probe-max`, local Salesforce docs scrape at `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run`.

---

## Practical Boundary

Full byte-for-byte Salesforce parity is not the target.

Salesforce owns private chrome, private JavaScript, hosted service calls, CDN assets, release-specific markup, and browser timing. Those internals shift underfoot. Glade should not chase them.

The useful target is **contract parity**:

- Same controller construction, extension construction, action ordering, form binding, validation, navigation, and page-message behavior for ordinary Visualforce pages.
- Same observable rendered DOM/text for documented components where developers depend on it.
- Same local AJAX, remoting, Remote Objects, upload, static resource, PDF, and Lightning Out/LWC behavior where Glade can own the runtime.
- Explicit unsupported diagnostics for hosted-only Salesforce services.
- Checked evidence for every `supported`, `partial`, and `unsupported` status.

Do not remove preview wording from public docs until the final claim gate passes.

## Current Baseline To Preserve

Current product surfaces:

- `/Users/matt/Dev/glade/internal/visualforce`
- `/Users/matt/Dev/glade/internal/server/visualforce.go`
- `/Users/matt/Dev/glade/internal/server/lightning*.go`
- `/Users/matt/Dev/glade/internal/vm`
- `/Users/matt/Dev/glade/lwcruntime`
- `/Users/matt/Dev/glade/testdata/local-tests/visualforce-rendering`
- `/Users/matt/Dev/glade/docs/COMPATIBILITY.md`
- `/Users/matt/Dev/glade/docs/LOCAL_TESTING.md`
- `/Users/matt/Dev/glade/site/docs-src/guide/support-map.md`

Current parity-tooling surfaces:

- `/Users/matt/Dev/glade-tools/internal/compat/visualforce_capture.go`
- `/Users/matt/Dev/glade-tools/internal/compat/visualforce_local_capture.go`
- `/Users/matt/Dev/glade-tools/internal/compat/visualforce_diff.go`
- `/Users/matt/Dev/glade-tools/internal/compat/visualforce_diff_normalize.go`
- `/Users/matt/Dev/glade-tools/internal/toolcli/compat_visualforce_command.go`
- `/Users/matt/Dev/glade-tools/docs/fixtures/visualforce`

Current local docs oracle:

- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/visualforce`

## Worktree Layout

Use paired worktree roots so `glade-tools/go.mod` can keep `replace github.com/glade-sh/glade => ../glade`.

Create one integration pair:

```bash
mkdir -p /Users/matt/Dev/vf-parity
git -C /Users/matt/Dev/glade worktree add /Users/matt/Dev/vf-parity/glade -b codex/vf-salesforce-parity
git -C /Users/matt/Dev/glade-tools worktree add /Users/matt/Dev/vf-parity/glade-tools -b codex/vf-salesforce-parity-tools
```

For large parallel phases, create one paired root per write squad:

```bash
mkdir -p /Users/matt/Dev/vf-squads/oracle
git -C /Users/matt/Dev/glade worktree add /Users/matt/Dev/vf-squads/oracle/glade -b codex/vf-oracle-product
git -C /Users/matt/Dev/glade-tools worktree add /Users/matt/Dev/vf-squads/oracle/glade-tools -b codex/vf-oracle-tools
```

Use the same shape for `runtime`, `component`, `browser`, `pdf`, and `docs` only when they need write access at the same time.

Rules:

- The main conductor owns merges into `/Users/matt/Dev/vf-parity/glade`.
- A subagent owns one squad branch and one file family.
- Only Oracle squad touches `oaer-probe-max`.
- Only one scratch-org capture runs at a time.
- Heavy test gates run after integration, not inside every subagent.
- Subagents return changed files, test commands, failing commands, report paths, and support rows.

## Parallel Squad Contract

Every phase uses this default squad set:

- **Oracle squad:** `glade-tools`, scratch-org capture, local capture, diff, fixture project, report JSON.
- **Runtime squad:** `internal/visualforce`, `internal/server`, `internal/vm`.
- **Component squad:** component renderers, registry, catalog, product Visualforce fixtures.
- **Browser squad:** `lwcruntime/test`, emitted JS behavior, Playwright or Node browser tests.
- **Docs/support squad:** `docs/COMPATIBILITY.md`, `docs/LOCAL_TESTING.md`, `site/docs-src/guide/support-map.md`, generated support ledgers.
- **Review squad:** read-only review after integration. Findings first. No code edits.

Do not let two write squads edit the same file in the same phase. If two squads need the same file, the main conductor owns that file.

## Global Gates

Run these after every integrated phase from `/Users/matt/Dev/vf-parity/glade`:

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
cd /Users/matt/Dev/vf-parity/glade-tools
go test ./...
```

Run this after a parity claim changes:

```bash
cd /Users/matt/Dev/vf-parity/glade
go build -o /tmp/glade-vf-parity ./cmd/glade

cd /Users/matt/Dev/vf-parity/glade-tools
go run ./cmd/glade-tools visualforce capture \
  --target-org oaer-probe-max \
  --project /Users/matt/Dev/vf-parity/glade-tools/docs/fixtures/visualforce/probe-project \
  --out /tmp/glade-vf-salesforce.json \
  --json
go run ./cmd/glade-tools visualforce capture \
  --local \
  --glade-bin /tmp/glade-vf-parity \
  --project /Users/matt/Dev/vf-parity/glade-tools/docs/fixtures/visualforce/probe-project \
  --out /tmp/glade-vf-local.json \
  --json
go run ./cmd/glade-tools visualforce diff \
  --salesforce /tmp/glade-vf-salesforce.json \
  --local /tmp/glade-vf-local.json \
  --project /Users/matt/Dev/vf-parity/glade-tools/docs/fixtures/visualforce/probe-project \
  --out /tmp/glade-vf-diff.json \
  --json
jq '.ok,.counts,.diffCount' /tmp/glade-vf-diff.json
```

Expected for newly claimed pages:

```text
true
0
```

If a hosted-only page appears in the probe, the expected result is a stable unsupported diagnostic.

---

## Phase 0: Baseline Audit And Worktree Setup

**Goal:** Start from a clean measured baseline before changing renderer behavior.

**Squads:**

- Conductor squad owns worktree setup and baseline command log.
- Review squad performs read-only review of current Visualforce docs and support rows.

**Files:**

- Modify only if stale: `/Users/matt/Dev/vf-parity/glade/docs/COMPATIBILITY.md`
- Modify only if stale: `/Users/matt/Dev/vf-parity/glade/site/docs-src/guide/support-map.md`
- Create if useful: `/Users/matt/Dev/vf-parity/glade/reports/visualforce-baseline.md`

**Steps:**

- [ ] Create the paired integration worktrees shown above.
- [ ] Run `git status --short` in both repos and record untracked or dirty files.
- [ ] Run `go test ./internal/visualforce ./internal/server ./internal/vm -count=1`.
- [ ] Run `npm --prefix lwcruntime test`.
- [ ] Run `go test ./...` only after focused tests pass.
- [ ] Run `cd /Users/matt/Dev/vf-parity/glade-tools && go test ./...`.
- [ ] Build `/tmp/glade-vf-parity` from the integration worktree.
- [ ] Run current Salesforce/local Visualforce capture against `oaer-probe-max`.
- [ ] Save the exact commands and counts in `reports/visualforce-baseline.md` if `reports/` is already used by this branch; otherwise keep the evidence in `/tmp` and final notes.

**Done Gate:**

- Both repos have clean worktrees except known user files.
- Current tests and capture either pass or have named baseline failures.
- No implementation phase starts with unknown breakage.

## Phase 1: Oracle, Docs Corpus, And Scoreboard

**Goal:** Make Salesforce and the local docs corpus the measuring stick for every later phase.

**Squads:**

- Oracle squad owns `glade-tools` capture, local capture, diff, normalization, and summary.
- Fixture squad owns `glade-tools/docs/fixtures/visualforce/probe-project`.
- Docs-corpus squad owns the scraper inventory reader.
- Docs/support squad owns checked support summaries.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade-tools/internal/compat/visualforce_capture.go`
- Modify: `/Users/matt/Dev/vf-parity/glade-tools/internal/compat/visualforce_local_capture.go`
- Modify: `/Users/matt/Dev/vf-parity/glade-tools/internal/compat/visualforce_diff.go`
- Modify: `/Users/matt/Dev/vf-parity/glade-tools/internal/compat/visualforce_diff_normalize.go`
- Modify: `/Users/matt/Dev/vf-parity/glade-tools/internal/toolcli/compat_visualforce_command.go`
- Test: `/Users/matt/Dev/vf-parity/glade-tools/internal/compat/visualforce_capture_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade-tools/internal/compat/visualforce_diff_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade-tools/internal/toolcli/compat_visualforce_command_test.go`
- Create or modify: `/Users/matt/Dev/vf-parity/glade-tools/docs/fixtures/visualforce/probe-project/visualforce-probe-index.json`
- Modify: `/Users/matt/Dev/vf-parity/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/vf-parity/glade/site/docs-src/guide/support-map.md`

**Steps:**

- [ ] Inventory every `pages_compref_*.md` file under the local Salesforce docs scrape.
- [ ] Store docs-derived component names, namespace, source path, examples-present flag, and likely hosted-service category.
- [ ] Extend capture JSON with `pageName`, `phase`, `family`, `components`, `claim`, `statusCode`, `contentType`, `redirectURL`, selected headers, `htmlSha256`, `pdfSha256`, `bodySize`, `normalizedText`, `contractText`, `error`, and `durationMs`.
- [ ] Extend local capture so it invokes a built Glade binary and never imports product code into `glade-tools`.
- [ ] Extend diff normalization to ignore view-state blobs, CSRF tokens, generated `j_id*` IDs, org IDs, timestamps, nonce values, request IDs, and private Salesforce script URLs.
- [ ] Add `visualforce summary --json` with counts by `phase`, `family`, `component`, `claim`, and `status`.
- [ ] Add `--phase`, `--family`, and `--pages` filters to capture, local capture, diff, and summary.
- [ ] Grow the checked probe project to at least 150 pages, grouped by lifecycle, expressions, components, field rendering, standard controllers, AJAX, remoting, Remote Objects, uploads, static resources, PDF, Lightning Out, LWC, and hosted-only diagnostics.
- [ ] Keep probe fixtures in SFDX layout under `force-app/main/default/pages`, `classes`, `components`, `aura`, `lwc`, `staticresources`, and metadata folders.
- [ ] Add tests for missing probe index, malformed probe index, unknown page, unknown phase, Salesforce capture failure, local capture failure, and one known normalized diff match.
- [ ] Generate a support summary that states how many docs-backed components are `supported`, `partial`, `unsupported`, and `unknown`.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade-tools
go test ./internal/compat ./internal/toolcli -run 'Visualforce|visualforce' -count=1
go run ./cmd/glade-tools visualforce summary \
  --project /Users/matt/Dev/vf-parity/glade-tools/docs/fixtures/visualforce/probe-project \
  --json | jq '.pageCount,.componentCount,.unknownCount'
```

**Done Gate:**

- The probe index has at least 150 pages.
- Every probe page has phase, family, component list, and claim.
- Salesforce capture works against `oaer-probe-max`.
- Local capture works against `/tmp/glade-vf-parity`.
- Diff reports normalized pass/fail counts by phase and component.
- No report includes access tokens, session IDs, full instance URLs, or private org IDs.

## Phase 2: Page Lifecycle, View State, And Navigation

**Goal:** Match the core Visualforce request loop before expanding component claims.

**Squads:**

- Runtime squad owns Visualforce page rendering and lifecycle.
- VM squad owns ApexPages dispatch and controller construction.
- Server squad owns POST, redirects, forwards, response headers, and errors.
- Oracle squad owns lifecycle probe pages.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/page.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/lifecycle.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/viewstate.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/form_binding.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/server/visualforce.go`
- Modify as needed: `/Users/matt/Dev/vf-parity/glade/internal/vm/platform_apexpages*.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/lifecycle_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/viewstate_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/form_binding_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/server/visualforce_test.go`

**Steps:**

- [ ] Add typed view-state snapshots for scalar Apex values, SObjects, lists, maps, nested Apex objects, nulls, page messages, component state, standard controller state, and extension state.
- [ ] Exclude `transient` controller fields from view state.
- [ ] Add tests for missing CSRF, stale CSRF, tampered view state, expired view state, wrong page name, and oversized view state.
- [ ] Add tests for two forms on one page. Posting form A must not process form B action inputs.
- [ ] Add tests for controller first, extension constructor second, POST restore third, setter/update-model next, action next, getter/render last.
- [ ] Add tests for `ApexPages.currentPage()`, parameters, cookies where locally modeled, messages, redirects, and forwards.
- [ ] Keep server-side forwards restricted to `/apex/<PageName>`.
- [ ] Ensure record URLs such as `/001000000000001AAA` do not become Visualforce page names.
- [ ] Add standard controller lifecycle tests for load from `id`, `save`, `delete`, `cancel`, failed save, and message rerender.
- [ ] Add probe pages `Lifecycle_GetAction`, `Lifecycle_PostAction`, `Lifecycle_ExtensionOrder`, `Lifecycle_ViewStateTamper`, `Lifecycle_ForwardRedirect`, and `Lifecycle_MultipleForms`.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'ViewState|Lifecycle|FormBinding|StandardController' -count=1
go test ./internal/server -run 'Visualforce.*(Post|Error|Redirect|ViewState|Forward)' -count=1
```

**Done Gate:**

- Claimed lifecycle probe pages match Salesforce on action result, navigation, visible messages, and rendered text.
- View-state and CSRF failures return stable diagnostics.
- Docs name remaining lifecycle limits.

## Phase 3: Expressions, Globals, And Formula Semantics

**Goal:** Make Visualforce expressions stable enough that renderers do not need private string hacks.

**Squads:**

- Expression squad owns parser and evaluator.
- VM squad owns shared formula/function hooks.
- Oracle squad owns expression probe pages.
- Docs/support squad owns unsupported global wording.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/expression_parser.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/expression_eval.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/expression.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/expression_formula_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/render_environment_test.go`
- Modify as needed: `/Users/matt/Dev/vf-parity/glade/internal/vm/platform_formula*.go`

**Steps:**

- [ ] Add parser tests for property chains, method calls, bracket access, string literals, numeric literals, booleans, null, unary `!`, arithmetic, comparisons, `&&`, `||`, and nested function calls.
- [ ] Add evaluator tests for list indexing, map indexing, SObject field access, Apex object fields, controller method calls, extension method calls, nulls, and failed coercions.
- [ ] Finish or verify `IF`, `CASE`, `BLANKVALUE`, `NULLVALUE`, `TEXT`, `VALUE`, `URLFOR`, `JSENCODE`, `HTMLENCODE`, `URLENCODE`, and arithmetic/date coercion.
- [ ] Finish or verify `$CurrentPage`, `$Label`, `$Resource`, `$ObjectType`, `$User`, `$Profile`, `$Permission`, `$Setup`, `$Site`, and `$Component`.
- [ ] Add stable diagnostics for unsupported globals. The message must include the global name and `unsupported Visualforce global`.
- [ ] Add probe page `Expressions_All` with one visible row for each supported global and function.
- [ ] Add probe page `Expressions_Coercion` with visible date, datetime, decimal, integer, boolean, null, string, list, map, and SObject rows.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'Expression|Formula|RenderEnvironment' -count=1
```

**Done Gate:**

- Every supported expression form has parser and evaluator tests.
- Salesforce and local probe text match for claimed expression rows.
- Unsupported globals fail with named diagnostics.

## Phase 4: Component Catalog And Renderer Closure

**Goal:** Account for every docs-backed Visualforce component with exact support facts and renderer tests.

**Squads:**

- Catalog squad owns generated catalog and registry facts.
- Core component squad owns layout/input/output/table components.
- Hosted-boundary squad owns hosted-only components and diagnostics.
- Docs/support squad owns generated public tables.
- Review squad verifies every status change.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/component_catalog.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/component_registry.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/component_references.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/component_catalog_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/component_registry_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/component_support_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/core_components_test.go`

**Steps:**

- [ ] Generate or verify one support row for every `pages_compref_*.md` entry.
- [ ] Each row must include `name`, `namespace`, `status`, `reason`, `docSource`, `attributes`, `localEvidence`, `scratchEvidence`, and `hostedBoundary`.
- [ ] Replace vague `partial` reasons with exact missing behaviors or attributes.
- [ ] Promote only components with product tests and oracle evidence to `supported`.
- [ ] Keep hosted-service components `unsupported` with one of these causes: `hosted-service`, `obsolete-runtime`, `missing-local-subsystem`, or `not-a-standalone-component`.
- [ ] Finish practical UI-only components where bounded: `apex:panelGrid`, `apex:panelGroup`, `apex:sectionHeader`, `apex:toolbar`, `apex:toolbarGroup`, `apex:tabPanel`, `apex:tab`, `apex:panelBar`, and `apex:panelBarItem`.
- [ ] Add renderer tests for every promoted component.
- [ ] Add diagnostic tests for Analytics, Chatter, Chatter Answers, Ideas, Knowledge, Live Agent, Sites, Social, Support, Topics, Wave, Canvas, vote, milestone tracker, publisher components, Flash, and S-Control.
- [ ] Generate docs from the same support facts used by tests.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'Component(Catalog|Registry|Support)|CoreComponents|Render' -count=1
```

**Done Gate:**

- No docs-backed component has unknown status.
- Every `supported` component has renderer tests.
- Every `partial` row lists exact missing behavior.
- Every hosted-only family has a stable diagnostic test.
- Product docs and registry agree.

## Phase 5: Data-Bound Forms, Field Rendering, And Standard Controllers

**Goal:** Make ordinary CRUD Visualforce pages locally useful.

**Squads:**

- Schema squad owns field metadata interpretation.
- Renderer squad owns `outputField`, `inputField`, `detail`, `relatedList`, and validation markup.
- Lifecycle squad owns submitted values and error rerender.
- Standard controller squad owns record and list behavior.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/field_rendering.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/form_binding.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/page.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/render.go`
- Modify as needed: `/Users/matt/Dev/vf-parity/glade/internal/vm/platform_apexpages*.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/field_rendering_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/form_binding_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/lifecycle_test.go`

**Steps:**

- [ ] Add field matrix fixtures for text, textarea, checkbox, integer, double, percent, currency, date, datetime, picklist, multi-picklist, lookup, master-detail, email, phone, URL, rich text, ID, auto-number, formula, and summary fields.
- [ ] Render `apex:outputField` display text for each field family.
- [ ] Render `apex:inputField` controls for editable field families.
- [ ] Honor required, read-only, createable, updateable, nillable, defaulted-on-create, auto-number, formula, and relationship metadata.
- [ ] Convert posted strings into typed values for boolean, integer, decimal, date, datetime, picklist, multi-picklist, lookup IDs, and empty strings.
- [ ] Add validation errors for missing required fields, bad dates, bad numbers, bad lookup IDs, inactive picklist values, and read-only edits.
- [ ] Bind field and page errors to `apex:message`, `apex:messages`, `apex:pageMessage`, and `apex:pageMessages`.
- [ ] Finish standard controller `save`, `quickSave`, `delete`, `view`, `edit`, and `cancel` where locally meaningful.
- [ ] Finish standard set controller list rows, selected rows, page size, next, previous, first, last, and result size.
- [ ] Add Salesforce probe pages for Account edit, failed validation, read-only field, list pagination, and related list.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'FieldRendering|FormBinding|StandardController|StandardSet|Messages' -count=1
go test ./internal/server -run 'Visualforce.*Field|Visualforce.*Post' -count=1
```

**Done Gate:**

- A standard-controller Account edit page can load, edit, save, fail validation, show field errors, and rerender user-entered values.
- Standard set controller pages page through deterministic local rows.
- Docs list unsupported field families or UI details by exact name.

## Phase 6: AJAX, Partial Refresh, And Client Runtime

**Goal:** Make Visualforce AJAX deterministic in the local browser.

**Squads:**

- Server squad owns partial response format and target lookup.
- Runtime squad owns action ordering, submitted values, regions, and validation.
- Client squad owns emitted JavaScript.
- Browser squad owns DOM tests.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/ajax.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/partial.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/page.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/server/visualforce.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/ajax_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/server/visualforce_ajax_test.go`
- Create or modify: `/Users/matt/Dev/vf-parity/glade/lwcruntime/test/visualforce-ajax.test.mjs`

**Steps:**

- [ ] Support AJAX submission for `apex:commandButton` and `apex:commandLink` with `reRender`.
- [ ] Support `apex:actionFunction` with generated callable JavaScript names.
- [ ] Support `apex:actionSupport` for click, change, keyup, blur, and submit events.
- [ ] Support `apex:actionRegion` so only the region's submitted fields process unless `immediate` changes lifecycle.
- [ ] Support `apex:actionStatus` start/stop text, style, and callbacks.
- [ ] Support `apex:actionPoller` with interval, enabled, action, and `reRender`.
- [ ] Preserve `apex:param` ordering for actions and AJAX calls.
- [ ] Update hidden view-state after each successful partial response.
- [ ] Add nested ID target lookup for local IDs and generated Visualforce IDs.
- [ ] Add browser tests for click, keyup, function call, poller tick, status start/stop, missing target diagnostic, nested target replacement, and validation failure.
- [ ] Add Salesforce probe pages for the same cases and compare visible target text.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'Ajax|Partial|Action' -count=1
go test ./internal/server -run 'VisualforceAjax' -count=1
npm --prefix lwcruntime test -- --run visualforce-ajax
```

**Done Gate:**

- Browser tests prove DOM updates and view-state refresh.
- Salesforce and local probes agree on action result and visible target text for claimed AJAX pages.
- Invalid targets produce stable diagnostics.

## Phase 7: Static Resources, Uploads, Security, And Page Shell

**Goal:** Make asset-heavy and security-sensitive pages behave like real local Visualforce pages.

**Squads:**

- Upload squad owns multipart parsing and `apex:inputFile`.
- Resource squad owns static resources and zip subpaths.
- Security squad owns escaping, CSP, cache, and response headers.
- Shell squad owns Visualforce page attributes such as header/sidebar/stylesheets.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/upload.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/security.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/page.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/render.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/server/visualforce.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/upload_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/security_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/server/visualforce_test.go`

**Steps:**

- [ ] Enforce 10 MB upload limit and return a stable limit diagnostic.
- [ ] Bind uploaded Blob, filename, content type, and size into controller fields.
- [ ] Add multipart form tests for one file, no file, oversized file, and wrong field.
- [ ] Serve static resource URLs for direct files and zipped nested paths.
- [ ] Preserve MIME type and cache-busting behavior for local resource URLs.
- [ ] Support `apex:stylesheet`, `apex:includeScript`, and `apex:image` through static resource paths.
- [ ] Add response tests for `contentType`, `cache`, `cspHeader`, `showHeader`, `sidebar`, `standardStylesheets`, and `lightningStylesheets`.
- [ ] Add escaping tests for safe output, unescaped output, script text, attributes, URL attributes, and JavaScript string contexts.
- [ ] Add Salesforce probe pages for uploads, zipped static resources, stylesheet/script/image output, and selected page attributes.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'Upload|Security|Static|Resource|PageAttribute' -count=1
go test ./internal/server -run 'Visualforce.*(Upload|Static|Header|CSP|Cache)' -count=1
```

**Done Gate:**

- Asset-heavy pages run locally without path rewrites by the user.
- Uploads bind to controller values and enforce limits.
- Security behavior is explicit, tested, and documented.

## Phase 8: JavaScript Remoting And Remote Objects

**Goal:** Support old but common Visualforce JavaScript data paths against local Apex and local org state.

**Squads:**

- Remoting squad owns `remoting.go` and remoting server routes.
- Remote Objects squad owns `remote_objects.go` and CRUD dispatch.
- Security squad owns size limits, CSRF/view-state checks, timeout handling, and JSON error shapes.
- Browser squad owns JavaScript execution tests.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/remoting.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/remote_objects.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/server/visualforce.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/remoting_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/remote_objects_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/server/visualforce_remoting_test.go`
- Create or modify: `/Users/matt/Dev/vf-parity/glade/lwcruntime/test/visualforce-remoting.test.mjs`

**Steps:**

- [ ] Discover `@RemoteAction` methods on page controllers and extensions.
- [ ] Enforce static public/global method requirements.
- [ ] Convert positional JSON arguments into Apex values.
- [ ] Execute remoting calls through the local VM with current user, org state, and limit mode.
- [ ] Match response envelope fields: `action`, `method`, `type`, `tid`, `status`, `result`, `message`, `where`, and `errors`.
- [ ] Enforce 4 MB remoting request limit.
- [ ] Add timeout handling with deterministic local max and remoting-style failure envelope.
- [ ] Build Remote Objects descriptors from `apex:remoteObjects`, `apex:remoteObjectModel`, and `apex:remoteObjectField`.
- [ ] Support Remote Objects create, retrieve, update, and delete against local org state.
- [ ] Return Salesforce-shaped errors for unknown object, unknown field, validation failure, missing ID, read-only field, CSRF mismatch, and tampered view state.
- [ ] Add browser tests that call `Visualforce.remoting.Manager.invokeAction` and generated Remote Objects model methods.
- [ ] Add Salesforce probe pages for success and failure paths.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'Remoting|RemoteObject' -count=1
go test ./internal/server -run 'VisualforceRemoting|RemoteObjects' -count=1
npm --prefix lwcruntime test -- --run visualforce-remoting
```

**Done Gate:**

- Browser tests execute real local Apex remoting calls.
- Browser tests execute Remote Objects CRUD against local org state.
- Unsupported hosted behavior returns named diagnostics and never falls through to generic REST handlers.

## Phase 9: PDF, `getContent`, And `renderAs`

**Goal:** Make local PDFs and Apex `PageReference.getContent*` useful and honest.

**Squads:**

- PDF toolchain squad owns PDF renderer selection and `pdf.go`.
- ApexPages squad owns `PageReference.getContent()` and `getContentAsPDF()` VM behavior.
- Oracle squad owns Salesforce PDF captures and text comparison.
- Docs/support squad owns fidelity wording.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/pdf.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/page.go`
- Modify as needed: `/Users/matt/Dev/vf-parity/glade/internal/vm/platform_apexpages*.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/pdf_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/page_test.go`
- Test as needed: `/Users/matt/Dev/vf-parity/glade/internal/vm/*apexpages*_test.go`

**Steps:**

- [ ] Choose a deterministic local PDF engine. Prefer a discoverable browser-based engine already available in the workspace over a new heavy dependency.
- [ ] Keep a clear fallback diagnostic when the full PDF engine is absent.
- [ ] Render `renderAs="pdf"` from the same Visualforce HTML contract used by browser preview.
- [ ] Enforce PDF limits: 15 MB HTML input, 60 MB PDF output, and 30 MB image input.
- [ ] Add PDF tests for text, stylesheet, image, page size, simple pagination, missing image, and oversized output.
- [ ] Implement or finish `PageReference.getContent()` for local renderable Visualforce pages.
- [ ] Implement or finish `PageReference.getContentAsPDF()` through the PDF path.
- [ ] Enforce Apex-visible restrictions for tests, recursive render, unsupported target, bad URL, oversized response, and absent renderer.
- [ ] Decide `renderAs` support beyond PDF. Support only bounded local formats; otherwise return explicit diagnostics.
- [ ] Compare Salesforce and local PDFs by extracted text, page count, content type, and size bands. Do not claim byte equality.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'PDF|PageReference|GetContent|RenderAs' -count=1
go test ./internal/vm -run 'PageReference|GetContent' -count=1
```

**Done Gate:**

- Local PDFs are useful for CI assertions and human inspection.
- `PageReference.getContent*` works for local Visualforce targets and fails with stable diagnostics outside the contract.
- Docs state PDF fidelity level and do not promise byte-for-byte Salesforce output.

## Phase 10: Lightning Out And LWC Inside Visualforce

**Goal:** Make Visualforce-hosted Lightning useful for realistic local LWC workflows.

**Squads:**

- Aura/Lightning Out squad owns `$Lightning.use`, `$Lightning.createComponent`, Aura dependency scanning, and callbacks.
- LWC module squad owns compiled LWC modules and import maps.
- Data squad owns `@salesforce/apex`, LDS, labels, resources, user, and i18n support.
- Browser squad owns multi-component behavior tests.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/render.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/server/lightning.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/server/lightning_shims.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/server/lightning_wire.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/lwcbrowser/manifest.go`
- Modify as needed: `/Users/matt/Dev/vf-parity/glade/internal/lwcbrowser/*.go`
- Modify as needed: `/Users/matt/Dev/vf-parity/glade/internal/lwcruntime/embed/glade.out.js`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/server/lightning_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/lwcruntime/test/visualforce-dev-server.test.mjs`
- Test: `/Users/matt/Dev/vf-parity/glade/lwcruntime/test/multi-component.test.mjs`
- Test: `/Users/matt/Dev/vf-parity/glade/lwcruntime/test/wire.test.mjs`
- Test: `/Users/matt/Dev/vf-parity/glade/lwcruntime/test/record-wire.test.mjs`
- Create or modify: `/Users/matt/Dev/vf-parity/glade/lwcruntime/test/visualforce-lightningout.test.mjs`

**Steps:**

- [ ] Support Aura apps that extend `ltng:outApp`.
- [ ] Scan `aura:dependency` entries and resolve LWC components through the compiled LWC manifest.
- [ ] Preserve Aura wrapper passthrough aliases. A one-child Aura wrapper should resolve to its underlying LWC module when safe.
- [ ] Match callback shape: success calls `callback(el, "SUCCESS")`; failure calls `callback(null, "ERROR", message)`.
- [ ] Support multiple `$Lightning.createComponent` calls on one Visualforce page.
- [ ] Support simple events between host page and mounted LWC components through DOM events.
- [ ] Serve `@salesforce/apex` modules through local Apex controller routes.
- [ ] Serve `lightning/uiRecordApi` with local `getRecord`, `getObjectInfo`, and mutation helpers where already modeled.
- [ ] Serve labels, schema tokens, static resource URLs, content asset URLs, user IDs, and i18n modules through existing shim routes.
- [ ] Add diagnostics for missing Aura app, missing dependency, missing LWC module, bad component name, and unsupported Lightning service.
- [ ] Add a fixture page with Apex data, LDS data, label text, static resource URL, one event, multiple mounts, and one missing dependency negative case.
- [ ] Add scratch-org capture for the same Visualforce page. Compare boot contract text and visible post-mount DOM, not private Salesforce script bytes.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/server -run 'Lightning|Visualforce.*Lightning|LWC' -count=1
npm --prefix lwcruntime test -- --run visualforce
npm --prefix lwcruntime test -- --run multi-component
npm --prefix lwcruntime test -- --run wire
npm --prefix lwcruntime test -- --run record-wire
```

**Done Gate:**

- A Visualforce page can host multiple LWCs locally.
- LWC components can use local Apex, LDS record data, labels, schema imports, resource URLs, user, and i18n shims.
- Missing dependencies fail with stable diagnostics.
- Docs call this local Lightning Out/LWC support inside Visualforce, not full Lightning Experience parity.

## Phase 11: Hosted-Only Boundaries And Flow Decision

**Goal:** Make unsupported Visualforce behavior fail by design instead of by accident.

**Squads:**

- Boundary squad owns hosted-only classification.
- Flow squad owns `flow:interview` feasibility.
- Component squad owns diagnostics.
- Docs/support squad owns public wording.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/component_registry.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/flow.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/flow_test.go`
- Test: `/Users/matt/Dev/vf-parity/glade/internal/visualforce/component_support_test.go`
- Modify: `/Users/matt/Dev/vf-parity/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/vf-parity/glade/site/docs-src/guide/support-map.md`

**Decision Gate For `flow:interview`:**

- Implement only if an existing Glade local Flow metadata/runtime path can execute screen-flow variables, finish navigation, and diagnostics with bounded work.
- Keep unsupported if this requires building a separate Flow runtime.

**Steps:**

- [ ] Inventory every non-`apex` Visualforce namespace component from the docs catalog.
- [ ] Classify each namespace as local-emulatable, product-dependency, or hosted-only.
- [ ] Decide `flow:interview` using the gate above and record the reason.
- [ ] Add explicit diagnostics for every hosted-only component family.
- [ ] Add tests that prove hosted-only components do not silently render empty content.
- [ ] Add docs that explain these fences as product boundaries.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./internal/visualforce -run 'Flow|ComponentSupport|Unsupported' -count=1
```

**Done Gate:**

- No component fails by surprise.
- Unsupported components fail with stable, useful messages.
- Flow is either implemented through an existing local runtime or fenced with a clear reason.

## Phase 12: Large Corpus, Performance, Docs, And Public Claim Gate

**Goal:** Prove the work against real projects before changing public support language.

**Squads:**

- Corpus squad owns example-project and large-project scans.
- Performance squad owns timing, memory, cache, and timeout budgets.
- Docs/support squad owns public docs and site.
- Review squad owns final read-only review.

**Files:**

- Modify: `/Users/matt/Dev/vf-parity/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/vf-parity/glade/docs/LOCAL_TESTING.md`
- Modify: `/Users/matt/Dev/vf-parity/glade/site/docs-src/guide/support-map.md`
- Modify as needed: `/Users/matt/Dev/vf-parity/glade/docs/RELEASE_NOTES.md`
- Modify as needed: `/Users/matt/Dev/vf-parity/glade-tools/internal/compat/*.go`

**Steps:**

- [ ] Run `glade-tools` Visualforce discovery across `/Users/matt/Dev/glade/example-projects`.
- [ ] Run against one selected Visualforce-heavy project if available locally.
- [ ] Record blocker classes before and after.
- [ ] Profile render hot paths for large pages, repeated GET render, POST rerender, AJAX loops, and Lightning Out pages.
- [ ] Add cache or timeout fixes only where profiling shows measurable cost.
- [ ] Generate final support map from checked support facts.
- [ ] Update docs with commands for `glade dev vf`, support JSON, scratch comparison, PDF support, Lightning Out/LWC support, and known hosted-only boundaries.
- [ ] Verify rendered site routes, not just markdown source.
- [ ] Run the final global gates.
- [ ] Run final scratch-org/local diff against `oaer-probe-max`.
- [ ] Have Review squad inspect the full diff and return findings before merge.

**Focused Verification:**

```bash
cd /Users/matt/Dev/vf-parity/glade
go test ./...
npm --prefix lwcruntime test
scripts/smoke.sh
npm --prefix site test
npm --prefix site run build

cd /Users/matt/Dev/vf-parity/glade-tools
go test ./...
```

**Done Gate:**

- Large corpus scans show no unknown blocker classes.
- Public docs are generated from support facts.
- Final scratch-org/local diff passes for all claimed pages.
- Every remaining gap is `partial` with exact missing behavior or `unsupported` with a stable diagnostic.
- Docs still avoid byte-for-byte Salesforce claims.

---

## Phase Order

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

Phase 1 must land first. Phase 2 should land before phases 5, 6, 8, and 9 because those depend on view state and request lifecycle. Phase 3 can run beside Phase 2 if it touches only expression files. Phase 4 should run as a catalog lane through the entire effort. Phases 5 through 10 can run in parallel after Phase 2 if file ownership stays clean.

## GPT-5.5 High Handoff Prompt

Use this prompt to start the implementation:

```text
Implement /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-17-visualforce-salesforce-parity-gpt55.md in full.

You are GPT-5.5 High.
Use superpowers:subagent-driven-development.
Use parallel subagents by squad.
Create paired worktrees so /Users/matt/Dev/vf-parity/glade-tools keeps its ../glade replacement working.
Keep product code in /Users/matt/Dev/vf-parity/glade.
Keep parity tooling, scratch-org fixtures, and generated maintenance reports in /Users/matt/Dev/vf-parity/glade-tools.
Use oaer-probe-max for Salesforce Visualforce capture.
Use the local docs scrape at /Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run.

Do not stop after partial edits.
Do not claim support without product tests, scratch-org evidence, support rows, and docs.
Do not run multiple scratch-org captures at once.
Do not add glade-tools dependencies to product code.
Do not remove preview wording until Phase 12 passes.

After each phase, return:
- squads dispatched
- changed files
- commands run
- scratch-org artifacts and counts
- local diff report paths
- docs/support rows changed
- remaining explicit gaps
- next phase recommendation
```

## Per-Phase Subagent Prompt Template

Use this for each squad:

```text
You are the <squad name> subagent for Phase <N> of /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-17-visualforce-salesforce-parity-gpt55.md.

Scope:
- Own only these files: <exact file list>.
- Do not edit files outside that list.
- Start with failing tests or oracle evidence.
- Keep hosted-only behavior behind stable diagnostics.
- Return a concise summary, changed files, test commands, failures, and handoff notes.

If you discover that another squad owns the needed file, stop and report the dependency.
```

## Final Parity Claim Gate

Practical Visualforce rendering parity is reached only when all of these are true:

- All docs-backed Visualforce components have checked `supported`, `partial`, or `unsupported` status.
- Every `supported` component has product tests and scratch-org evidence.
- Every `partial` component lists exact missing behavior.
- Every `unsupported` component has a stable diagnostic.
- GET, POST, AJAX, remoting, Remote Objects, uploads, static resources, PDFs, `PageReference.getContent*`, standard controllers, custom components, templates, and Lightning Out/LWC pass the fixture suite.
- Large corpus scans show no unknown blocker classes.
- Public docs are generated from support facts.
- `go test ./...`, `glade-tools go test ./...`, `npm --prefix lwcruntime test`, `scripts/smoke.sh`, and site tests/build all pass.

Only then should docs move from "Visualforce dev rendering preview" to "practical Visualforce rendering support", with hosted-only and byte-for-byte limits still named.
