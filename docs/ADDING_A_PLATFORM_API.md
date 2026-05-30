# Adding a Salesforce API to glade

This is the runbook for adding new Salesforce platform functionality (a class,
namespace, method, or surface) when Salesforce ships something new or a gap is
found. It tells you **where each piece lives** so the change is easy to find,
review, and keep consistent.

The golden rule from `docs/ARCHITECTURE_STANDARDS.md` still applies: register the
surface first, tie every behavior claim to a compatibility fixture, and never
panic on user Apex.

## The map: where things live

| Concern | Package | What it owns |
| --- | --- | --- |
| Surface index | `internal/surface` | The registry of high-level Salesforce surfaces, each naming its owner package(s) and focused test command. The front door. |
| Static dispatch (`Foo.bar()`) | `internal/vm` `dispatch.go` | Resolving `Namespace.Type.method` global/static calls. |
| Instance dispatch (`x.bar()`) | `internal/vm` `method_dispatch.go` | Resolving member calls on a receiver value. |
| Platform object members | `internal/vm` `platform_member_registry.go` + `platform_*` files | Table-driven handlers for platform receiver types. |
| Stdlib value members | `internal/vm` `stdlib*.go` | `String`, `Integer`, `Decimal`, `List`/`Set`/`Map`, `Pattern`/`Matcher`, etc. |
| Constructors (`new Foo()`) | `internal/vm` `construct_runtime.go` | Building platform values. |
| Type/method signatures (sema) | `internal/sema` `platform_signatures.go` | Compile-time knowledge of platform method params/returns. |
| Symbol stubs (load-bearing) | `internal/typesys` `system_stub_symbols_generated.go` | Generated symbol surface so code referencing the API type-checks. |
| Object schema / fields | `internal/storage` `standard_*` (generated) + `schema` | Standard objects, fields, picklists. |
| SOQL/SOSL | `internal/soql` | Query parsing (`parser.go`) and execution (`soql.go`). |
| DML + automation | `internal/dml` | DML pipeline, validation, triggers, formula/rollup side effects. |
| HTTP/REST surface | `internal/server` | Salesforce-shaped REST endpoints. |
| Capability status | `internal/capability` `catalog.go` | The machine-readable feature matrix and MVP gate. |
| Compatibility proof | `internal/compat` + `docs/fixtures/*.json` | Black-box fixtures that pin behavior to public Salesforce semantics. |

## Decision: which kind of API is it?

- **A new method on an existing platform type** → add a `case`/handler in the
  relevant `internal/vm` member dispatcher (stdlib type → `stdlib_*.go`;
  platform receiver type → a `platform_*` handler) and a sema signature in
  `internal/sema/platform_signatures.go`.
- **A whole new platform receiver type / namespace** → register a handler in
  `internal/vm/platform_member_registry.go` (`platformObjectMemberSurfaces`),
  put the handler body in a new `internal/vm/platform_<area>.go` file, and add a
  `surface.Descriptor` in `internal/surface/surface.go`.
- **A new constructor** → `internal/vm/construct_runtime.go`.
- **A new standard object/field** → regenerate schema (`scripts/generate-standard-schema.mjs`); do not hand-edit the `*_generated.go` files.
- **A new REST endpoint** → `internal/server`.

## Steps

1. **Register the surface.** Add or update a `surface.Descriptor` in
   `internal/surface/surface.go` with the owner runtime/server package and a
   focused `go test ...` command. `internal/surface` is the index of record; do
   this before widening runtime, server, capability, or compat behavior.

2. **Add the sema signature** in `internal/sema/platform_signatures.go` so calls
   type-check with correct params/return. If the type is new, ensure its symbols
   exist (regenerate `internal/typesys` stubs if needed).

3. **Implement the runtime handler** in `internal/vm`:
   - stdlib type → the matching `stdlib_*.go` dispatcher;
   - platform receiver type → a handler registered in
     `platform_member_registry.go`, body in `platform_<area>.go`;
   - constructor → `construct_runtime.go`.
   Keep handlers as plain functions taking `*VM` — do not capture per-call state
   in closures (it allocates and is slower). Return an explicit
   `UnsupportedFeature` error for anything you do not implement; never panic.

4. **Add a compatibility fixture first** under `docs/fixtures/` using
   `internal/compat.FixtureBuilder`, proving the behavior against public
   Salesforce semantics. Behavior changes are only credible with a fixture.

5. **Update the capability entry** in `internal/capability/catalog.go`. Only move
   a status to `supported` once fixtures cover it; `partial`/`stub`/`unsupported`
   otherwise, with a clear `Notes` gap description.

6. **Regenerate the docs** the capability change feeds:
   ```bash
   go run ./cmd/glade compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
   go run ./cmd/glade compat gaps      --output docs/KNOWN_GAPS.md
   go run ./cmd/glade compat stdlib    --output docs/STDLIB_COVERAGE.md
   ```

7. **Validate.** Run the surface's focused test command, then the affected
   packages, then `scripts/smoke.sh`. Confirm allocations/perf are unchanged on
   hot dispatch paths (`go test -bench . -benchmem` + `benchstat`).

## Anti-patterns to avoid

- Adding a project-specific runtime route or stdlib stub to make one example
  project pass. Fix the general parser/sema/VM/SOQL/DML/storage/server behavior.
- Building dispatch tables or closures per call. Build them once (see
  `platformObjectMemberSurfaces`, cached with `sync.Once`).
- Silent fallbacks. Prefer an explicit unsupported-feature diagnostic.
- Hand-editing `*_generated.go` files. Change the generator and regenerate.
