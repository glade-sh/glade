# apexrr — plan

Status: historical. This project now implements trace/profile analysis natively
inside `oaer` (`internal/profile` and `oaer profile analyze`) and must not take
an apexrr dependency. Keep this document only as background for report ideas.

## Goal

Phase 1 finds Apex performance bottlenecks in a Salesforce codebase and ranks
them for an architect or code reviewer. Target is large legacy projects like
`alpha-pkg`. Phase 2 (later) proposes and applies refactors with
before/after proof using aer.

## Target ground (alpha-pkg)

- 2,116 Apex classes, 40 triggers, ~298,000 lines of Apex.
- 160 `@AuraEnabled` files, 30 Batchable, 27 Schedulable, 9 Queueable.
- 1,279 inline SOQL call sites.
- **845 describe call sites.** (`getGlobalDescribe` / `.getDescribe()`.) One
  of the largest categories of cost. Any tool that ignores describe traffic
  misses the main road.
- A custom SOQL builder at `next-gen/selectors/dynamic-soql/SOQL.cls` (2,327
  lines). Dynamic SOQL is the norm here, not the edge.

Shape this implies: discovery has to scale to ~230 entry points; profiling
has to reach into scheduled jobs and batch scopes, not just unit tests;
describe calls and `Database.query(String)` have to be first-class.

## aer — what we have

aer (from October Swimmer) is a local Apex interpreter. Source is private,
binaries ship via `aer-dist`. Binary installed at `~/.local/bin/aer`. These
features already exist and apexrr will use them directly:

- `--profile file` — writes pprof CPU profiles readable by `go tool pprof`,
  `pprof`, speedscope, any pprof viewer.
- `--trace file` — writes Chrome Trace Event JSON. Begin/End events per
  method with timestamps, threadIDs, args. Readable by `chrome://tracing`,
  speedscope.app, perfetto.dev. Drop a file in, get a flamegraph.
- `--enforce-governor-limits` — governor limits at aer runtime.
- `Limits.getCpuTime()`, `Limits.getQueries()`, `Limits.getHeapSize()`,
  `Limits.getDmlRows()` all return honest counters. Confirmed on a sample run.
- `--bootstrap-db` — seed a SQLite DB before a run. This is where sandbox
  fixtures live.
- `--faketime` — deterministic `DateTime.now()` / `Date.today()`.
- `aer server` — Salesforce API-compatible local server (persistent SQLite
  backing).
- `aer lsp` — language server, IDE integration for free later.
- Record-triggered Flow conversion is on by default
  (`--no-flow-conversion` disables).

## apexrr — what we build

Three pieces. Nothing more until Phase 1 is proven on `alpha-pkg`.

### 1. Parser + call graph (foundation)

- Pull `github.com/octoberswimmer/apexfmt` as a Go module dependency. It is
  public and has the Apex grammar already bound to Go types. Saves generating
  the Antlr Go target from the apex-dev-tools grammar ourselves.
- Parse every `.cls` and `.trigger` under the input path. Build a symbol
  table: classes, methods, fields, triggers, interfaces, annotations.
- Build a call graph. Resolve method references by static dispatch where
  possible; record unresolved sites for later.
- Fallback: if apexfmt doesn't expose the AST we need, generate one from the
  apex-dev-tools Antlr grammar using Antlr's Go target. First path is cheaper
  if it works.

### 2. Static hotspot pass

AST-level rules, each emitting a finding with `file:line`, `category`,
`severity`, and a one-line "why this hurts." Rules tuned for `alpha-pkg`:

| Rule | Category |
| --- | --- |
| SOQL inside loop (static `[SELECT ...]` and `Database.query(String)`) | soql_in_loop |
| DML inside loop (insert/update/delete/upsert, `Database.*`) | dml_in_loop |
| Uncached describe (`Schema.getGlobalDescribe`, `SObjectType.getDescribe`, field describes outside static-init cache) | uncached_describe |
| Dynamic Apex (`Type.forName`, `newInstance`, `JSON.deserialize(Type)`) | dynamic_apex |
| String concatenation in loop | string_concat_in_loop |
| Call chain deeper than N (default 20) | deep_chain |
| Repeated selector query across handlers on same SObject within a transaction | redundant_selector |
| Trigger without a handler class (inline Apex in `.trigger`) | trigger_logic_inline |
| Governor-limit near-miss from static SOQL/DML counts | governor_near_miss |

Rule set grows from real findings on `alpha-pkg`. Not speculative.

### 3. aer driver + report

For each discovered entry point, drive aer:

- **Fixture source.** Read from a sandbox (sfdx auth), stratify by record
  type / picklist / owner, anonymize (hash PII, keep shape), stash as
  `fixtures/<entry-point>/<scale>.db` (SQLite, as aer expects via
  `--bootstrap-db`).
- **Invocation.** For each entry point, synthesize an anonymous Apex caller
  that routes into the entry point with the fixture loaded:
  - Trigger: insert/update/delete the fixture records of the target SObject;
    aer runs the trigger via its server behavior.
  - `@AuraEnabled` / `@InvocableMethod`: direct call with fixture args.
  - Batchable: `Database.executeBatch(new X(), scope)` at scale.
  - Queueable: `System.enqueueJob(new X())`, `Test.startTest`/`stopTest` to
    drain.
  - Schedulable: call `execute()` directly.
- **Scale sweep.** 1x / 10x / 100x / 1000x record counts. For alpha-pkg
  specifically: `assets/importData/` already has shaped JSON for providers
  and setup data. Use these as the 1x baseline before pulling from a sandbox. Record SOQL count,
  DML count, CPU, heap peak, call chain depth at each scale.
- **Curve fit.** Given four points, label growth as O(1), O(n), O(n log n),
  O(n²), O(n³). O(n²) and worse are always surfaced.
- **Diff mode.** `apexrr diff --before <sha> --after <sha>` runs both trees,
  produces a delta report. This is the bank vault door.

Outputs:

- `report.html` — architect-facing, ranked, one card per entry point, with
  embedded flamegraph (speedscope JSON inlined).
- `report.json` — machine-readable, full fidelity.
- `report.sarif` — CI consumption, GitHub code-scanning compatible.

## Gaps without aer source

aer binary is what we have. Source is private. These are the real gaps and
the workarounds we accept:

1. **Can't add new trace event types.** Today the Chrome trace looks like
   method Begin/End pairs. If aer doesn't emit dedicated `soql` / `describe`
   / `dml` events as first-class trace events, we can't add them. *Workaround:*
   wrap the callee in an Apex shim that reads `Limits` before/after. Gives us
   per-method delta counts. Coarser than per-statement.
2. **Can't instrument describe calls individually.** With 845 sites in
   alpha-pkg, we cannot say "this `getGlobalDescribe()` on line 412 fired"
   from aer alone. *Workaround:* static analysis flags the site, dynamic
   confirms the method ran hot, user connects them.
3. **No query plan visibility.** aer backs SOQL with SQLite but the pprof
   profile doesn't surface per-query time. *Workaround:* query the
   `aer server` SQLite file directly with `EXPLAIN QUERY PLAN` for hot
   queries.
4. **Can't add SOQL syntax support.** If aer rejects `GROUP BY`, `TYPEOF`,
   `WITH USER_MODE`, those paths are unprofilable dynamically. *Workaround:*
   static-only findings for unsupported syntax.
5. **Can't add missing standard library methods.** Same shape as #4.
6. **Version drift.** apexrr couples to aer's CLI surface and output format.
   When aer revs either, apexrr breaks. *Mitigation:* pin aer version, ship
   a compatibility matrix in the README.
7. **Licensing.** `aer license` command exists. Commercial users install
   and license aer themselves.
8. **Can't fix aer's own hot paths.** If aer is slow on a construct in a
   way that skews measurements, we eat it.

Net: gaps push us toward coarser per-method attribution (not per-statement)
and static-only findings for syntax aer can't execute. Phase 1 still lands.
The before/after CPU + SOQL + DML deltas that sell Phase 2 are unaffected.

## Empirical findings from first aer runs on alpha-pkg

Before writing any code, ran `aer test force-app --dry-run` against
alpha-pkg. Two findings inside five minutes. These are real value-add
outputs the smoke harness will formalize.

### Finding 1 — duplicate class name

`BaseSingleProviderProfileInfo` is defined twice under
`force-app/verifiable-app/main/classes/verifiable-api/Abstractions/Models/Api/Internal/Providers/`:

- `Info/BaseSingleProviderProfileInfo.cls` —
  `public virtual inherited sharing class BaseSingleProviderProfileInfo`
- `ProviderInfo/BaseSingleProviderProfileInfo.cls` —
  `public abstract class BaseSingleProviderProfileInfo`

Different modifiers, different field casing (`lastUpdatedAt` vs
`LastUpdatedAt`). On the real platform this only survives if the two copies
are in different managed packages. Worth flagging as a discrete apexrr rule:
`duplicate_class_name`.

### Finding 2 — namespace / package structure is material

Scoping aer to a subtree (`force-app/verifiable-app/main/classes/service`)
surfaces missing references to:

- Sample-prefixed types (`SampleDataset`, `SampleMonitor`,
  `SampleResource`, `SampleSchemas`, `SampleDatasetMetadata`) —
  internal namespaces of alpha-pkg.
- fflib types (`fflib_Application`, `fflib_SObjectDomain`,
  `fflib_SObjectMocks`, `fflib_SecurityUtils`) — open source library present
  in-tree somewhere we didn't include.

Smoke harness has to understand alpha-pkg's real namespace layout and pass
the right `--package` / `--package-dir` / `--default-namespace` flags.
Config lives per-project in `apexrr.yml`.

## Revised build order (empirical-first)

Original plan was parser → rules → driver. Flip it. Let the codebase tell us
which rules matter.

1. **Smoke harness.** Takes an `apexrr.yml` describing package layout and
   namespaces. Runs `aer test` and `aer exec` against configured entry points.
   Captures: did it load? did it run? what broke? for the survivors, trace +
   pprof. Outputs: `smoke_report.md`.
2. **aer-compat finding aggregator.** Roll up every `aer refused to load X`
   and `aer doesn't support Y` into a single report. This is both our issue
   tracker for aer-dist upstream and a real output for the user ("your
   codebase has these 47 cross-class collisions, here are the files").
3. **Trace analyzer.** Parse Chrome Trace Event JSON. Rank methods by
   cumulative time, call count, SOQL/DML/describe/CPU deltas (via wrapped
   `Limits`). Produce the first honest "largest slowness" ranking.
4. **Parser + call graph** via `apexfmt`. Needed once static rules start.
5. **Static rules, driven by what the trace showed was hot.** Three rules
   ship first, chosen from what the trace told us, not from a category hat.
6. **Fixture harvester.** Sandbox → SQLite. Triggers first.
7. **Scale sweep + curve fit.**
8. **Report (HTML + JSON + SARIF), diff mode.**

## Build and deploy pipeline awareness (alpha-pkg)

alpha-pkg is a managed package project (namespace `sample`) using
CumulusCI + the Salesforce CLI for the full lifecycle:

- Dev → scratch org (`npm run org:create` via `setup_scratch_org.sh`)
- Package build and versioning via CumulusCI (`cumulusci.yml`)
- QA packaging org and post-packaging org for regression (`robot:*` scripts)
- `validate:deploy` for check-only deploy gate in CI

apexrr does not hook into this pipeline. It runs locally via aer.
Relevant notes:

- `assets/importData/` contains JSON import files shaped for scratch org
  seeding (providers, setup data). Use these as fixture baseline for
  Phase 1 profiling before pulling from a sandbox.
- A class named `verifiable.cls` shares the project namespace. Auto-detected:
  apexrr suppresses `--default-namespace` for this project to avoid
  double-prefixing of namespace-qualified type references.
- Findings from apexrr are from the package-internal (scratch org)
  perspective. Post-package subscriber behavior (FLS, sharing rules, trigger
  order from installed packages) is outside aer's model.

## What we do NOT build

- **We do not write an Apex interpreter.** aer is that. Any gap there is a
  polite issue on `aer-dist`.
- **We do not write a profile format.** pprof and Chrome Trace Event format
  are both established. Users view in existing tools.
- **We do not manage sandbox auth.** Reuse `sfdx` / `sf` CLI auth. Shell out.

## Known aer gaps (file later, not blockers)

These aren't in the documented feature set. If they trip us on
`alpha-pkg`, file issues on `aer-dist`:

- SOSL (`FIND ... RETURNING ...`).
- SOQL `GROUP BY`, aggregate functions (`COUNT()`, `SUM()`), `TYPEOF`,
  `WITH USER_MODE` / `WITH SECURITY_ENFORCED`, `FIELDS(STANDARD)`,
  `FOR UPDATE`.
- `Approval.process`.
- `Messaging.SingleEmailMessage` side effects.
- Platform event publish / subscribe (`EventBus.publish`, after-insert on `__e`).
- Change Data Capture events.
- `Database.Stateful` state across batch chunks.

Not Phase 1 blockers. Discovered as we run it, filed as we find them.

## Build order

1. **Module setup.** `apexfmt` as a dependency. Verify AST reach. If it's
   thin, switch to Antlr Go target now rather than later.
2. **Entry-point discovery over the AST.** Output: "here are your 230
   doors." Useful on day one. Unit tested against `alpha-pkg`.
3. **Static hotspot pass.** Three rules first: `soql_in_loop`, `dml_in_loop`,
   `uncached_describe`. Run against `alpha-pkg`. Tune on real findings.
4. **aer driver.** Start with `@AuraEnabled` (simplest invocation). Capture
   the trace JSON, parse it, attribute cost per method.
5. **Fixture harvester.** Pull from a sandbox into SQLite. Triggers first.
6. **Scale sweep + curve fit.**
7. **Report (HTML + JSON + SARIF).**
8. **Diff mode.**

## Layout

```
apexrr/
  cmd/apexrr/             CLI
  internal/
    parse/                AST + symbol table (wraps apexfmt)
    graph/                call graph
    discover/             entry-point finder
    rules/                static hotspot rules
    driver/               aer subprocess driver
    trace/                Chrome trace reader + pprof reader
    fixtures/             sandbox harvester
    report/               HTML / JSON / SARIF emitters
  testdata/               sample Apex
  docs/                   plan, design notes
```

## Workstream B (aer contributions) — deferred

We do not fork aer. Source is private. If we hit gaps that block Phase 1, we
file issues politely on `aer-dist` and work around them with static
analysis-only findings for that category until fixed.
