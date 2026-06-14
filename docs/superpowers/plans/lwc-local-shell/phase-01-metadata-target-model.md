# Phase 1: Metadata And Target Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Parse LWC config, FlexiPage metadata, and custom tab metadata into a typed model that the shell can resolve.

**Architecture:** Add `internal/lwcshell` as the local page model. It consumes `project.Project` file paths, parses XML into small structs, validates target support, and emits render targets without starting a browser.

**Tech Stack:** Go XML decoding, existing `internal/project` discovery, existing `internal/lwc` bundle metadata, table-driven tests.

---

## Feature Delivered

After this phase, Glade can answer: “Which LWC can render on which page, with which properties, for which object and form factor?”

## Files

- Create: `internal/lwcshell/model.go`
- Create: `internal/lwcshell/component_meta.go`
- Create: `internal/lwcshell/flexipage.go`
- Create: `internal/lwcshell/tab.go`
- Create: `internal/lwcshell/resolve.go`
- Create: `internal/lwcshell/diagnostic.go`
- Create: `internal/lwcshell/*_test.go`
- Modify: `internal/lwc/meta.go`
- Test data: `testdata/local-tests/lwc-shell`

## Parallel Squads

- LWC meta squad owns `component_meta.go` and `internal/lwc/meta.go`.
- FlexiPage squad owns `flexipage.go`.
- Tab squad owns `tab.go`.
- Resolver squad owns `resolve.go` after parser tests pass.
- Review squad owns acceptance and public diagnostics.

## Implementation Steps

- [ ] Define `RenderTargetKind` constants: `Component`, `RecordPage`, `AppPage`, `HomePage`, `Tab`.
- [ ] Define `PageContext` in `model.go`:

```go
type PageContext struct {
	Kind          RenderTargetKind
	ComponentName string
	PageName      string
	RecordID      string
	ObjectAPIName string
	AppName       string
	TabName       string
	FormFactor    string
	UserID        string
	Locale        string
	Namespace     string
	State         map[string]string
}
```

- [ ] Define `ComponentConfig` with `IsExposed`, `APIVersion`, `Targets`, `TargetConfigs`, `Properties`, `SupportedObjects`, and `SupportedFormFactors`.
- [ ] Extend `internal/lwc.ParseComponentMeta` or wrap it from `internal/lwcshell` so current callers keep compiling.
- [ ] Parse `targetConfigs>targetConfig` attributes, property tags, objects, and supported form factors. Preserve XML type precedence.
- [ ] Parse FlexiPage fields: `masterLabel`, `type`, `sobjectType`, `template`, `flexiPageRegions`, `itemInstances`, legacy `componentInstances`, `componentInstanceProperties`, `identifier`, `visibilityRule`, and `events`.
- [ ] Parse custom tabs enough to identify tab full name, label, LWC component target, and FlexiPage target. Record unsupported tab flavors with diagnostics.
- [ ] Implement `ResolveComponentTarget(project, name, context)` and reject non-exposed or target-mismatched components with stable diagnostic codes:
  - `GLADELWC001` component not found
  - `GLADELWC002` component not exposed
  - `GLADELWC003` target not supported
  - `GLADELWC004` object not supported for record page
  - `GLADELWC005` form factor not supported
  - `GLADELWC006` page metadata not found
  - `GLADELWC007` tab target unsupported
- [ ] Implement `ResolvePageTarget(project, PageContext)` that returns page regions with ordered component instances and property values.
- [ ] Add tests for API 48 legacy array values and API 49 `valueList` values in FlexiPage component properties.

## Verification

```bash
go test ./internal/lwc ./internal/lwcshell ./internal/project ./internal/metadata -count=1
```

```bash
go test ./internal/lwcshell -run 'Meta|FlexiPage|Resolve|Tab' -count=1
```

## Done Gate

- All phase diagnostics have tests.
- Record page object restrictions work for allowed and blocked objects.
- App page, home page, tab, and direct component targets resolve from `testdata/local-tests/lwc-shell`.
- No browser server code changes are required for this phase.
