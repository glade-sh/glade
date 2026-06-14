# LWC Local Shell And Visualforce Lightning Out Roadmap

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement a chosen phase with parallel subagent squads wherever ownership is disjoint. Each phase plan has checkbox (`- [ ]`) steps and must be executed to its done gate before claiming support.

**Goal:** Build practical local LWC rendering parity across both Glade LWC hosts: a Salesforce-like Lightning shell for direct component preview, record pages, app pages, home pages, custom tabs, and controller-backed data flows, plus Visualforce pages that render LWCs through Lightning Out.

**Architecture:** Glade owns a local Lightning Experience shell, a Visualforce Lightning Out host, metadata resolver, LWC browser server, Salesforce module shims, LDS-like data cache, and Apex controller bridge. Both hosts share `internal/lwcbrowser`, `lwcruntime`, wire routes, Apex invocation, LDS/UI API shims, base components, and service modules. `glade-tools` owns scratch-org capture, corpus probes, generated support ledgers, and compatibility dashboards.

**Tech Stack:** Go server and CLI, existing `internal/lwc` compiler, `internal/lwcbrowser` module shims, `lwcruntime` browser runtime, Glade VM for Apex, local org storage, Playwright browser tests, `oaer-probe-max` scratch org probes.

---

## Research Sources

Official Salesforce docs checked on 2026-06-14:

- [Run a Live Component Preview](https://developer.salesforce.com/docs/platform/lwc/guide/get-started-test-components.html): Salesforce Live Preview supports app, site, and isolated component previews with auto-update on save, but app/site metadata changes still require deploy and server restart.
- [XML Configuration File Elements](https://developer.salesforce.com/docs/platform/lwc/guide/reference-configuration-tags.html): every LWC bundle has `componentName.js-meta.xml`; exposure, targets, `targetConfig`, properties, objects, and form factors drive builder placement.
- [Configure a Component for Lightning App Builder](https://developer.salesforce.com/docs/platform/lwc/guide/use-config-for-app-builder.html): App Builder config includes page types, properties, supported objects, and type precedence from XML.
- [lightning__RecordPage target](https://developer.salesforce.com/docs/platform/lwc/guide/targets-lightning-record-page.html): record-page target configs include properties, supported objects, and supported form factors.
- [Configure Components for Custom Tabs](https://developer.salesforce.com/docs/platform/lwc/guide/use-config-custom-tab.html): `lightning__Tab` exposes a component as a custom tab and maps to `standard__navItemPage`.
- [PageReference Types](https://developer.salesforce.com/docs/platform/lwc/guide/reference-page-reference-type.html): shell navigation needs app, nav item, object, record, relationship, quick action, flow, web, and URL-addressable component page references.
- [lightning/uiRecordApi](https://developer.salesforce.com/docs/platform/lwc/guide/reference-lightning-ui-api-record.html), [getRecord](https://developer.salesforce.com/docs/platform/lwc/guide/reference-wire-adapters-record.html), and [getObjectInfo](https://developer.salesforce.com/docs/platform/lwc/guide/reference-wire-adapters-object-info.html): LDS-backed modules include record wire, object metadata, imperative create/update/delete, refresh, and field helpers.
- [FlexiPage Metadata](https://developer.salesforce.com/docs/atlas.en-us.api_meta.meta/api_meta/meta_flexipage.htm): Lightning pages are FlexiPages with type, template, regions, component instances, properties, visibility rules, events, and actions.

Local docs scrape checked:

- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/lwc/reference-configuration-tags.md`
- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/lwc/reference-page-reference-type.md`
- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/lwc/reference-salesforce-modules.md`
- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/lwc/reference-ui-api.md`
- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/metadata-api/meta_flexipage.md`
- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/cli-reference/cli_reference_template_commands_unified.md`

## Current Glade Tracks

- `internal/lwc` parses and renders a subset of LWC templates. `internal/lwc/meta.go` only reads `isExposed` and top-level targets.
- `internal/lwcbrowser` compiles project LWCs, builds a manifest, serves import maps, and shims several `@salesforce/*` modules.
- `internal/server/lightning_wire.go` handles Apex wire, `getRecord`, and `getObjectInfo` routes.
- `internal/server/visualforce.go`, `internal/lightningout/parse.go`, and `testdata/local-tests/lightning-out-vf/` already provide a Visualforce Lightning Out host where LWCs render through `/apex/<PageName>`.
- `lwcruntime/src/shims/wire-adapter.mjs` provides fetch-backed wire adapters.
- `internal/lwcbrowser/salesforce_modules.go` makes `lightning/navigation` throw. No local shell injects real `CurrentPageReference`.
- `internal/project` and `internal/metadata` already discover FlexiPage and tab files, but do not parse them for rendering.
- `glade-tools` already has a Visualforce scratch capture command and an `oracleprobe` package. LWC parity capture belongs there.

## Parity Boundary

Exact Lightning Experience parity is not practical. Salesforce owns private shell services, private page chrome, private LDS internals, private base component implementation details, and org runtime services that cannot be cloned with confidence.

Practical parity is useful and reachable. Glade should render local LWC work in two hosts: a local Lightning shell that follows public targets, public FlexiPage metadata, public PageReference contracts, public `@salesforce` modules, local org data, and real local Apex controllers; and a Visualforce Lightning Out host that follows public `$Lightning.use()` / `$Lightning.createComponent()` behavior inside `/apex/<PageName>`. When behavior is private or ambiguous, Glade must report a named unsupported feature and link the scratch-org capture evidence.

Every feature marked supported must state host coverage:
- `lwc.host.lightning-shell`
- `lwc.host.visualforce-lightning-out`

If a feature works in only one host, the support ledger must name the other host as `partial`, `unsupported`, or `salesforce-only` with a stable diagnostic.

## Feature Choice Matrix

| Feature Set | Phase Plan | User Value | Depends On |
| --- | --- | --- | --- |
| Scratch oracle and fixture baseline | [Phase 0](lwc-local-shell/phase-00-baseline-oracle.md) | Know current gaps in both Lightning shell and Visualforce Lightning Out hosts before building | none |
| LWC meta and target config | [Phase 1](lwc-local-shell/phase-01-metadata-target-model.md) | Components know where and how they can render | Phase 0 recommended |
| Direct component preview and host contract | [Phase 2](lwc-local-shell/phase-02-direct-component-shell.md) | Fast isolated local LWC development and a shared runtime contract for Visualforce Lightning Out | Phase 1 |
| Record page shell | [Phase 3](lwc-local-shell/phase-03-record-page-shell.md) | Record-context components get `recordId`, object, record header, and regions | Phase 1, Phase 2 |
| App, home, and tab shells | [Phase 4](lwc-local-shell/phase-04-app-home-tab-shell.md) | FlexiPage app pages and custom tabs run locally | Phase 1, Phase 2 |
| Wires, LDS, and Apex controllers | [Phase 5](lwc-local-shell/phase-05-wire-lds-apex.md) | Components test against real local controllers and records in both hosts | Phase 2, Phase 3 for record context |
| Base Lightning components and SLDS | [Phase 6](lwc-local-shell/phase-06-base-components-slds.md) | Common `lightning-*` components behave well in local shell and Visualforce Lightning Out pages | Phase 2, Phase 5 for data-bound forms |
| Navigation, services, quick actions | [Phase 7](lwc-local-shell/phase-07-navigation-services-actions.md) | Navigation, toasts, LMS, modals, and actions act like host-aware shell services | Phase 3, Phase 4 |
| Browser tests, support ledger, docs | [Phase 8](lwc-local-shell/phase-08-tests-ledger-docs.md) | CI proof and honest support map split by host | Phases being certified |

## Current Product UX

The merged product command starts the local shell. Developers open the route
that matches the target:

```bash
glade dev lwc --project .
glade dev lwc --project . --port 8080
glade dev vf --project .
glade dev vf --project . --port 8080
```

Maintenance and oracle commands stay in `glade-tools`. Today the LWC capture
command prepares fixture-manifest targets; browser/org capture and ledger
generation are later compat-plugin phases.

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --include-hosts lightning-shell,visualforce-lightning-out --out /tmp/glade-lwc-capture.json
# future:
glade compat lwc ledger --captures /tmp/glade-lwc-capture.json --output docs/generated/LWC_SHELL_SUPPORT.md
```

Future UX phases can add selector flags such as `--component`, `--record-page`,
`--app-page`, `--home-page`, `--tab`, and `--open`. They are not part of the
current `glade dev lwc` help text.

## Parallel Subagent Squad Rule

Each phase must be executed with parallel subagent squads where work does not share mutable files. Use `superpowers:subagent-driven-development` and assign one squad per disjoint owner: metadata, shell routes, Visualforce host, runtime shims, LDS/UI API, Apex bridge, base components, CLI/docs, oracle, and review. Shared file owners land one patch at a time. The review squad checks the phase done gate, runs the commands, and rejects partial feature claims.

Do not run multiple broad gates at once. The squads can build in parallel, but final verification is serial: focused package tests first, then one wide Go or browser suite at a time.

## Full-Phase Rule

When an agent picks a phase, it implements the phase in full for every host in that phase. No half-supported route, no silent placeholder, no hidden fallback, and no Lightning-shell-only claim when the same runtime feature should also work through Visualforce Lightning Out. If Salesforce behavior is not practical, the agent adds a named unsupported diagnostic, test coverage for the diagnostic, and a support ledger row naming the host.
