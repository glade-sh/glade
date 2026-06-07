# Adding a Salesforce API to glade

This is the runbook for adding new Salesforce platform functionality (a class,
namespace, method, or surface) when Salesforce ships something new or a gap is
found. It tells you **where each piece lives** so the change is easy to find,
review, and keep consistent.

The golden rule from `docs/ARCHITECTURE_STANDARDS.md` still applies: register the
surface first, tie every behavior claim to a compatibility fixture, and never
panic on user Apex.

## Finding the next gap (instead of waiting for a failure)

For the broad Salesforce surface view, start with the Surface Ledger. It joins
the scraped docs, an org/API snapshot, Glade's type and behavior surface, and
fixture/oracle evidence into one set of rows.

Set the docs source once. Prefer the workspace copy under `example-projects`:

```bash
export GLADE_SALESFORCE_DOCS_SOURCE="$PWD/example-projects/Salesforce Docs Scraper/salesforce-docs"
```

If that workspace copy is missing, fall back to the downloaded scrape:

```bash
export GLADE_SALESFORCE_DOCS_SOURCE="/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper (1)/salesforce-docs"
```

Check the declared docs universe before assigning broad work:

```bash
go run ./cmd/glade compat surface sources \
  --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
  --check docs/generated/salesforce/SURFACE_SOURCES.md
```

This must report zero missing Atlas docsets, zero partial Atlas docsets, LWC
present, site references present, and zero missing local markdown.

If the workspace copy only has the legacy six docsets, refresh the reference
material before assigning broad surface work:

```bash
cd "example-projects/Salesforce Docs Scraper/salesforce-scraper"
python3 scrape.py --reference-sources
python3 scrape.py \
  --docsets apex apex-guide object-reference field-reference soql-sosl visualforce lightning lwc rest-api tooling-api metadata-api soap-api bulk-api ui-api platform-events streaming-api connect-rest-api service-connector-api-reference limits-reference cli-reference analytics-cli-reference commerce-cli-reference \
  --version latest \
  --concurrency 2 \
  --request-delay 0.5 \
  --output ../salesforce-docs-expanded
python3 scrape.py \
  --docsets site-references \
  --site-metadata ../sitemap.json \
  --concurrency 2 \
  --output ../salesforce-docs-expanded
python3 scrape.py \
  --reference-coverage \
  --site-metadata ../sitemap.json \
  --output ../salesforce-docs-expanded
```

That expanded source adds the Apex Developer Guide, Object Reference for the
Salesforce Platform, Salesforce Field Reference Guide, SOQL/SOSL Reference, API
guides, Visualforce, Aura, LWC, Service Cloud Connector API Reference, Analytics
CLI, Commerce CLI, and modern `/docs/.../references` verticals such as
AMPscript, Agentforce, Pub/Sub API, GraphQL, and Salesforce Connect Amazon RDS.
The coverage report must show `Atlas docsets missing: 0`,
`Atlas docsets partial: 0`, `lwc` as `present`, and `Missing local markdown: 0`
before treating the expanded source as complete.

Then run the offline ledger refresh. This uses the checked Tooling completions
snapshot, so it does not need a live org:

```bash
tmp="$(mktemp -d)"
go run ./cmd/glade compat surface refresh \
  --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"

open "$tmp/SURFACE_DASHBOARD.md"
```

The command writes:

```text
DOCS_SNAPSHOT.json
ORG_SNAPSHOT.json
GLADE_SNAPSHOT.json
EVIDENCE_SNAPSHOT.json
SURFACE_LEDGER.json
SURFACE_DASHBOARD.md
SURFACE_GAPS.md
SURFACE_FAILURES.md
SURFACE_RELEASE_DIFF.md
```

The terminal output is intentionally small: implemented, partial, passive,
explicit unsupported, gap counts, failure counts, and the dashboard path. On a
warm local checkout this is a short run; it does not run the example-project
local-test corpus.

Create an agent packet from the generated ledger before editing runtime code:

```bash
go run ./cmd/glade compat surface packet \
  --ledger "$tmp/SURFACE_LEDGER.json" \
  --area Core.Runtime.System.FeatureManagement \
  --out docs/agent-packets/salesforce/FeatureManagement.md
```

Packets name the row filter, first rows to explain, ownership boundaries,
parallel-work rules, focused tests, fixture expectations, ratchet target, and
area ratchet command. A worker should claim one packet and stay inside it.

Close every packet with the same validation block:

```text
focused tests run
fixture command run
surface refresh run
area ratchet command run
before counts
after counts
next top row
```

Run `go test ./internal/repoguard` after code changes. If a docs row is missing
or malformed, choose one path before runtime work: re-scrape docs, copy improved
docs into the external docs source, patch the docs parser to read existing docs
correctly, or add a small checked fixture when public docs are ambiguous. Do not
invent runtime behavior to cover a docs defect.

Review packet output by checklist: no corpus-specific runtime hacks, public
Salesforce behavior is cited by docs or fixture, shape and behavior are not
claimed without evidence, packet area did not expand during work, and generated
docs are updated when capability changes.

Start breadth agents only after packets are ready, in this order:

```text
Ledger.Identity
FeatureManagement
Database.Batchable
Schema.Describe
ApexPages.Controllers
REST.Resources
Tooling.Objects
```

Use a live org when you want to refresh the org/API snapshot instead of the
checked Tooling completions file:

```bash
go run ./cmd/glade compat surface refresh \
  --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
  --target-org glade-probe-lab \
  --release v66.0 \
  --out docs/generated/salesforce/releases/v66.0
```

To inspect or gate a generated ledger:

```bash
go run ./cmd/glade compat surface explain \
  --ledger "$tmp/SURFACE_LEDGER.json" \
  --id 'apex:System.Label.get(String,String)'

go run ./cmd/glade compat surface check \
  --ledger "$tmp/SURFACE_LEDGER.json" \
  --max-missing-shape 33070 \
  --max-missing-behavior 0 \
  --max-parser-failures 0
```

Use the lower-level surface commands only after `refresh` when you need to
inspect or debug one input:

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

Don't hand-patch one method at a time off a test failure. The front door for
broad Salesforce feature work is `compat surface refresh`. Use the older docs
inventory and reconcile commands only to debug the docs/catalog join or inspect
Apex-specific rows:

```bash
# Build the inventory + catalog from the scraped Apex docs, then reconcile the
# documented surface against what glade actually knows.
glade compat docs-inventory --source "$APEX_DOCS" --output /tmp/inv.json
glade compat reconcile --inventory /tmp/inv.json            # summary + coverage
glade compat reconcile --inventory /tmp/inv.json --json     # full ranked worklist
```

The reconciler derives a status per documented surface:

- `supported`/`partial` — hand-verified executable behavior (`StdlibMatrix`).
- `unsupported` — intentional rejection with a stable diagnostic.
- `typed` — owning type is type-known (compiles) but has no executable verdict.
- `unknown` — owning type is not type-known; references will not even resolve.
- `doc` — language or guide surface, not a runtime target.

The `worklist` is sorted by impact (executable-parity `unknown` first). Use it as
inspection evidence, not as the broad packet selector. `scripts/apex-docs-support-gate.sh`
runs this lower-level check and can ratchet the runtime-target `unknown` count
via `GLADE_APEX_DOCS_MAX_UNKNOWN`.

### Behavioral contracts (what a surface must *do*)

The shape is not the whole story. `getContentAsPDF` is type-known and returns a
`Blob`, but the docs say it is *treated as a callout in a test method and fails*.
Mine those constraints straight from the doc prose so the runtime honors the
documented contract instead of a hand-patched VM branch:

```bash
glade compat doc-contracts --inventory /tmp/inv.json            # summary by kind
glade compat doc-contracts --inventory /tmp/inv.json --json     # full contracts
glade compat doc-contracts --inventory /tmp/inv.json --behavior callout-in-test
```

Each contract pins a behavior (`callout-in-test`, `unavailable-in-test`,
`not-in-triggers`, `throws`, `deprecated`, ...) to the exact symbol the docs
govern. Use these as the spec when wiring or correcting a surface.

### Driving implementation from documented gaps

Use the surface ledger as the front door. Refresh it from docs and Tooling
completions, explain one row, then add the smallest fixture or runtime change
that proves the surface:

```bash
tmp="$(mktemp -d)"
go run ./cmd/glade compat surface refresh \
  --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"
go run ./cmd/glade compat surface explain --ledger "$tmp/SURFACE_LEDGER.json" --surface-id "<surface-id>"
```

If public behavior is ambiguous, capture org evidence outside the runtime and
record it in a focused compatibility fixture before promoting the behavior.

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
   packages, then `scripts/smoke.sh`. Performance and optimization are
   paramount, so confirm allocations/perf are unchanged on hot dispatch paths
   (`go test -bench . -benchmem` + `benchstat`). Do not keep a faster path that
   changes Salesforce-shaped behavior, test isolation, limits, or metadata
   semantics.

## Anti-patterns to avoid

- Adding a project-specific runtime route or stdlib stub to make one example
  project pass. Fix the general parser/sema/VM/SOQL/DML/storage/server behavior.
- Building dispatch tables or closures per call. Build them once (see
  `platformObjectMemberSurfaces`, cached with `sync.Once`).
- Silent fallbacks. Prefer an explicit unsupported-feature diagnostic.
- Hand-editing `*_generated.go` files. Change the generator and regenerate.
