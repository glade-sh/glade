# Managed Package Dependency Plan

Status date: 2026-05-19.

This plan covers Salesforce managed-package dependencies for local Apex test
execution when GLADE has access to the dependency package source code. The first
implementation should be intentionally source-backed and explicit: a consuming
project config points GLADE at local dependency project roots, their namespaces,
and optionally the package version they represent. Later implementations can
cache those inputs as version-pinned package artifacts.

Current implementation note: `glade.yml` source-backed and artifact-backed
dependency config, dependency project loading, dependency JSON reporting,
dependency symbol/schema loading, and a first package artifact model now exist.
The remaining plan should focus on stronger cross-namespace access enforcement
and using artifact-backed package contracts in `nams-workspace`.

The immediate motivator is the enterprise example-project corpus. Projects such
as `src-nmb-nc-develop` and `nams-workspace` reference the installed `znu`
managed package through Apex namespace references like `znu.Address` and
schema-token references like `znu__CartItemLine__c`. Those references should not
be classified as ordinary project compile gaps until GLADE has attempted to load
the prerequisite managed package dependency.

## Objective

Make `glade test` and `compat local-tests` resolve installed managed-package
dependencies before compiling and running the consuming project.

The first release claim should be:

- A project can declare one or more local source-backed managed package
  dependencies.
- GLADE loads dependency package symbols, schema, and metadata before current
  project symbols.
- Cross-namespace access follows Salesforce managed-package rules: consuming
  packages can only access dependency Apex APIs that are externally visible,
  primarily `global` top-level types and `global` members.
- Namespaced SObjects, fields, custom metadata types, labels, resources, and
  other metadata resolve through the installed package namespace.
- Missing, mismatched, or inaccessible dependency surfaces produce explicit
  diagnostics such as `dependency_missing`, `dependency_version_mismatch`, or
  `dependency_access_denied`, not broad `compile_gap` noise.

This is not a plan to infer arbitrary package behavior from consuming code. If
the dependency is required, GLADE should load a real dependency source tree or a
previously built package artifact.

## First-Iteration Configuration

Use `glade.yml` for the first iteration because it already carries local
project-level settings and avoids needing to interpret every CumulusCI or SFDX
dependency shape up front.

Proposed minimal shape:

```yaml
project:
  root: .
  defaultNamespace: namz
  packageDirs: [sfdx-source/post]
  managedPackageDependencies: [
    "znu:/Users/matt/Dev/packages/znu:04t000000000000AAA"
  ]
```

The inline-list form keeps compatibility with the current minimal YAML parser.
Each item is:

```text
namespace:path[:version]
```

Where:

- `namespace` is the installed package namespace without `__`.
- `path` is a local project root containing either `sfdx-project.json` or
  Metadata API-style source.
- `version` is optional in the first iteration and may be a package version id,
  semantic version, or source snapshot label. It should be preserved in JSON
  output and artifact manifests even before strict version enforcement exists.

Artifact-backed config should use the same installed namespace but a distinct
artifact marker so a package artifact is not mistaken for a source root:

```yaml
project:
  root: .
  defaultNamespace: namz
  managedPackageDependencies: [
    "znu:artifact:../.glade/packages/znu/package.glade.json"
  ]
```

For source checkouts whose local namespace differs from the installed namespace,
the artifact builder must accept the installed namespace explicitly. This is the
`src-nmb-nu-develop` to `znu` case: the source project may report `NU`, while
the consuming org sees installed package namespace `znu`.

Future config can graduate to structured YAML if the config parser grows beyond
the current scalar and inline-list subset:

```yaml
managedPackageDependencies:
  - namespace: znu
    project: /Users/matt/Dev/packages/znu
    versionId: 04t000000000000AAA
    versionNumber: 1.42.0
    artifact: .glade/packages/znu/04t000000000000AAA/package.json
```

Do not block the first implementation on automatic CumulusCI parsing. Add CCI
and SFDX package dependency discovery after the explicit path config works.

## Package Artifact Model

Even in the source-backed first iteration, build an internal package artifact
model. It gives GLADE one representation for live source dependencies and
eventual cached dependency artifacts.

Artifact identity:

- Namespace.
- Package name, if known.
- Package version id or version label, if known.
- Source root.
- Source hash or manifest hash.
- Source API version.
- Build timestamp for diagnostics only, not behavior.

Apex contract:

- Subscriber-visible top-level classes, interfaces, enums, and triggers. The
  artifact must export only `global` dependency types.
- Subscriber-visible nested types, enum values, constructors, methods,
  properties, and fields. The artifact must export only `global` members.
- Modifiers and sharing declarations.
- Source ranges and declaring package identity.
- Executable IR for source-backed dependencies when available.

Schema contract:

- Namespaced custom objects such as `znu__CartItemLine__c`.
- Namespaced fields such as `znu__Product__c`.
- Relationships and child relationships.
- Record types, field sets, validation rules, value sets, and custom settings.
- Custom metadata type definitions and records.

Metadata contract:

- Labels and translations.
- Static resources and content assets.
- Tabs, layouts, profiles, permission sets, and presentation metadata needed by
  local tests.
- Named credentials, remote sites, workflow, flow, email templates, and other
  metadata that package code or consuming tests can observe.

Runtime contract:

- Source-backed dependency code should compile to IR and run where GLADE supports
  the language/runtime features used by the dependency.
- Unsupported dependency behavior should produce package-scoped unsupported
  diagnostics that identify the package namespace, version, type/member, and
  consuming call site.
- Artifact-only dependencies can later expose contract stubs for global APIs and
  schema while classifying calls without source-backed bodies as unsupported.

## Load Order

Local test execution should build the org/project model in this order:

1. Standard Salesforce platform catalog and generated standard schema.
2. Managed package dependencies in dependency order.
3. Current project package directories.
4. Test data, local fixtures, setup metadata, and test transaction state.

Dependency order should be explicit at first: list dependencies in the order
they would be installed. Later work can add dependency metadata to each artifact
and topologically sort them.

When a dependency project itself declares dependencies, use the same loader
recursively and detect cycles with a stable diagnostic.

## Namespace Resolution

The resolver needs to distinguish four cases:

- Same-package symbols: ordinary unqualified references inside the current
  package namespace.
- Current package namespace-token schema aliases: `namz__Thing__c` resolving to
  current project metadata when appropriate.
- Installed package Apex references: `znu.Address`, `znu.Service.Request`, or
  `Type.forName('znu', 'Address')`.
- Installed package schema-token references: `znu__Thing__c`,
  `znu__Thing__c.znu__Field__c`, relationship names, and custom metadata names.

Rules:

- Do not synthesize unknown package types from consuming references.
- It is acceptable to create placeholder schema definitions only when source
  metadata proves the object exists through lookup fields or owned package
  metadata. Placeholders must be marked inferred and should not promote runtime
  support beyond describe/load behavior.
- Apex namespace qualification should resolve against dependency package
  indexes before falling back to platform namespaces.
- Schema-token qualification should resolve against the installed package schema
  registry keyed by namespace.
- Diagnostics must include the namespace and package version where available.

## Global Access Rules

Managed package boundaries must be represented in the type and runtime model.
This is the main semantic difference between loading another source directory
and loading an installed managed package.

Initial rule set:

- A consuming package can reference a dependency top-level Apex type only if the
  type is `global`.
- A consuming package can call or read a dependency member only if the member is
  `global`, or if Salesforce exposes the specific member shape through a public
  global type contract.
- `public`, `protected`, and package-private dependency APIs are visible only
  inside the dependency package.
- Dependency tests should not be discovered as tests for the consuming project
  unless explicitly requested.
- Dependency triggers and automation should be installed metadata and can fire
  for dependency objects when runtime support exists, but dependency package
  test classes should not inflate consuming package test counts.
- `@TestVisible` does not make dependency internals visible to a consuming
  package test.
- Cross-namespace runtime access checks must mirror sema checks so reflection,
  dynamic dispatch, and property access cannot bypass global access.

Useful diagnostics:

- `dependency_access_denied`: package found, symbol exists, but not globally
  visible from the consuming package.
- `dependency_member_access_denied`: type is visible, member is not.
- `dependency_test_internal_access`: consuming test attempted to access package
  internals through `@TestVisible` or test-only source.

## CLI And JSON Behavior

`glade test --json` and `compat local-tests --json` should report dependency
loading explicitly.

Add a dependency section:

```json
{
  "dependencies": [
    {
      "namespace": "znu",
      "sourceRoot": "/Users/matt/Dev/packages/znu",
      "version": "04t000000000000AAA",
      "status": "loaded",
      "apexTypes": 1200,
      "objects": 180,
      "labels": 450,
      "diagnostics": []
    }
  ]
}
```

Outcome classification should distinguish:

- `dependency_missing`: configured or discovered dependency cannot be found.
- `dependency_load_error`: dependency source exists but cannot be parsed or
  indexed enough to expose its contract.
- `dependency_version_mismatch`: a required version is not the loaded version.
- `dependency_access_denied`: dependency contract exists but global access rules
  reject the reference.
- `compile_gap`: dependency loaded successfully, but GLADE cannot compile
  supported project code.
- `runtime_gap`: dependency loaded and code compiled, but runtime behavior is
  incomplete.

This separation matters because a missing `znu` artifact is a setup problem,
while an inaccessible `znu` member is likely a real source/package-boundary
problem, and a failing dependency method body is an GLADE runtime fidelity gap.

## Implementation Phases

### Phase MP1: Explicit Config And Discovery

Goal: let users point a project at local source-backed managed package
dependencies.

Current status: mostly implemented for source-backed dependencies. `glade.yml`
parses `project.managedPackageDependencies`, resolves relative source paths,
loads dependency projects, reports missing/load errors, and carries dependency
summaries into test and compatibility JSON.

Primary write scope: `internal/config`, `internal/project`, `internal/gladecli`,
`internal/compat`.

Tasks:

- Extend `glade.yml` parsing with `project.managedPackageDependencies` as an
  inline list of `namespace:path[:version]` entries.
- Add a typed config model for managed package dependencies.
- Resolve dependency paths relative to the config file directory unless they are
  absolute.
- Load dependency project roots with the existing project loader.
- Add JSON reporting for configured, loaded, missing, and invalid dependency
  entries.
- Add unit tests for config parsing, relative path handling, duplicate
  namespaces, and invalid entry diagnostics.

Validation:

```bash
go test ./internal/config ./internal/project ./internal/gladecli ./internal/compat
go run ./cmd/glade test --project testdata/local-tests/managed-package-consumer --json
```

Exit criteria:

- A fixture consumer project can declare a local `znu`-style dependency root.
- Missing dependency config returns `dependency_missing`, not unknown type
  diagnostics.
- Dependency load diagnostics are visible in JSON output.

### Phase MP2: Source-Backed Package Contract Builder

Goal: turn dependency source into a package artifact model without changing
current runtime semantics yet.

Current status: partially implemented. `internal/packageartifact` can build a
JSON artifact from a loaded project and type/schema index, including only
subscriber-visible `global` Apex types and members, installed-namespace custom
objects/fields/custom metadata records, labels, static resources, source root,
source API version, and source hash. The builder command is:

```bash
go run ./cmd/glade package build \
  --project example-projects/src-nmb-nu-develop \
  --namespace znu \
  --version src-nmb-nu-develop@<source-hash-or-git-sha> \
  --output example-projects/.glade/packages/znu/package.glade.json
```

Primary write scope: new `internal/packageartifact` or `internal/managedpkg`,
plus `internal/typesys`, `internal/schema`, and metadata loaders.

Tasks:

- Build an in-memory package artifact from loaded dependency source.
- Preserve namespace, package root, package directory, and source file
  provenance on every symbol and metadata definition.
- Separate package Apex contract from package test classes.
- Export a compact JSON artifact for debugging and future cache use.
- Add fixture coverage for classes, nested types, custom objects, custom
  metadata, labels, and static resources.

Validation:

```bash
go test ./internal/packageartifact ./internal/typesys ./internal/schema
go run ./cmd/glade compat local-tests --project testdata/local-tests/managed-package-consumer --json
```

Exit criteria:

- Dependency symbols and metadata appear in JSON/package artifact output with
  namespace and source provenance.
- Dependency test classes are excluded from consuming project test discovery.

### Phase MP3: Cross-Namespace Type And Schema Resolution

Goal: make consuming project compile resolution use loaded dependency packages.

Current status: source-backed and artifact-backed dependency symbols and schema
are appended before current project symbols. A tiny artifact-backed consumer
fixture resolves `znu.Address` and `znu__CartItemLine__c`-style references with
zero dependency diagnostics. A real `src-nmb-nu-develop` artifact built with
`--namespace znu` currently exports `510` global Apex types, `2,420` global
members, and `169` installed-namespace objects; `znu.Address`,
`znu.Pluggable`, and `znu__CartItemLine__c` resolve from that artifact.

Primary write scope: `internal/typesys`, `internal/sema`, `internal/schema`,
`internal/soql`.

Tasks:

- Resolve `namespace.Type` and nested `namespace.Outer.Inner` references through
  dependency package symbol indexes.
- Resolve `namespace__Object__c`, `namespace__Field__c`, and relationship names
  through dependency schema registries.
- Resolve dependency custom metadata type references and SOQL targets.
- Resolve `Label.namespace.Name`, static resources, and schema import forms for
  installed package metadata.
- Add stable diagnostics for unknown namespace, missing package, unknown symbol,
  and inaccessible symbol.

Validation:

```bash
go test ./internal/typesys ./internal/sema ./internal/schema ./internal/soql
go run ./cmd/glade test --project testdata/local-tests/managed-package-consumer --json
go run ./cmd/glade compat local-tests --project example-projects/src-nmb-nc-develop --timeout 30000 --top-failures 8 --json
```

Exit criteria:

- A source-backed dependency resolves Apex and SObject references from a
  consuming project.
- `znu.Address`-style blockers move from unknown type to either successful
  compile or package-scoped access/runtime diagnostics.

### Phase MP4: Global Access Enforcement

Goal: make managed package boundaries behave differently from same-project
multi-package-directory loading.

Primary write scope: `internal/sema`, `internal/vm`, `internal/typesys`.

Tasks:

- Add package identity to type/member lookup results.
- Enforce `global` type visibility for cross-package references.
- Enforce `global` member visibility for cross-package calls, field reads,
  property access, constructor calls, and nested type access.
- Ensure `@TestVisible` and package-private/public APIs are not visible across
  managed package boundaries.
- Mirror the same checks in runtime dynamic dispatch, reflection-like type
  lookup, constructors, and property access.
- Add tests for same-package `public` access, cross-package denied `public`
  access, cross-package allowed `global` access, nested global types, and
  private/protected denial.

Validation:

```bash
go test ./internal/sema ./internal/vm ./internal/apextest
go run ./cmd/glade test --project testdata/local-tests/managed-package-access --json
```

Exit criteria:

- Public dependency APIs are rejected across namespace boundaries.
- Global dependency APIs compile and run when their bodies use supported GLADE
  runtime features.
- Runtime access cannot bypass sema access rules.

### Phase MP5: Source-Backed Runtime Execution

Goal: execute dependency global APIs from source when the dependency body is
within GLADE-supported runtime behavior.

Primary write scope: `internal/ir`, `internal/vm`, `internal/apextest`,
`internal/dml`, `internal/storage`.

Tasks:

- Compile dependency global classes and their required internal implementation
  graph to IR.
- Allow dependency internals to call public/package-private members inside the
  dependency package while preserving cross-package access restrictions.
- Install dependency triggers, workflow, flow, labels, resources, and schema
  into the test org model.
- Ensure dependency static state resets follow test method isolation rules.
- Ensure dependency DML and SOQL use namespaced schema and relationship names.
- Classify unsupported dependency runtime features with package-scoped
  diagnostics instead of panics or generic runtime errors.

Validation:

```bash
go test ./internal/vm ./internal/apextest ./internal/dml ./internal/storage
go run ./cmd/glade test --project testdata/local-tests/managed-package-runtime --json
```

Exit criteria:

- A consuming test can call a dependency global service backed by dependency
  source.
- Dependency internal helper calls work inside the dependency package.
- Unsupported dependency body behavior reports the package namespace/version and
  call site.

### Phase MP6: Version-Pinned Artifact Loading And Cache

Goal: make dependency loading reproducible and fast without reparsing full
source trees on every local test run.

Current status: the contract path is implemented. `glade package build` writes a
stable JSON artifact with an explicit installed namespace and source hash, and
`glade.yml` can load it with `namespace:artifact:path`. The remaining cache work
is version mismatch handling, invalidation policy, endpoint-gap reporting for
global signatures that reference non-exported types, and optional compiled IR.

Primary write scope: `internal/packageartifact`, cache storage, CLI plumbing.

Tasks:

- Keep `glade package build --project <dependency-root> --namespace <installed-ns>
  --version <version> --output <artifact>` as the artifact build surface.
- Serialize package contracts into a stable JSON artifact; compiled IR can be
  added later after the contract path proves useful.
- Record source hashes and fail closed when a configured version expects a
  different artifact.
- Load artifact files from `glade.yml` when present. The first contract loader is
  done; runtime execution from compiled artifact bodies is later work.
- Add cache invalidation for source-backed dependencies when the source hash
  changes.

Validation:

```bash
go test ./internal/packageartifact ./internal/gladecli
go run ./cmd/glade package build --project testdata/local-tests/managed-package-dependency --namespace znu --version 04tTEST --output /tmp/znu.glade-package.json
go run ./cmd/glade test --project testdata/local-tests/managed-package-consumer --json
```

Exit criteria:

- A dependency can be loaded from a version-pinned artifact instead of source.
- Version mismatches are reported before project compilation.
- Cached package loading is measurably faster than source loading on large
  dependency trees.

### Phase MP7: Dependency Discovery From Existing Salesforce Tooling

Goal: reduce explicit config once the core dependency model works.

Primary write scope: `internal/project`, optional CumulusCI parser package,
`internal/config`.

Tasks:

- Read SFDX package dependency declarations where present.
- Read CumulusCI dependency declarations and namespace/package metadata where
  present.
- Map discovered package names or version ids to local package artifact roots
  through `glade.yml` aliases.
- Keep explicit `glade.yml` entries as the override when tooling metadata is
  ambiguous.

Validation:

```bash
go test ./internal/project ./internal/config
go run ./cmd/glade compat local-tests --project example-projects/nams-workspace --timeout 30000 --top-failures 8 --json
```

Exit criteria:

- Example projects can resolve configured dependency artifacts with minimal
  project-specific config.
- Ambiguous or missing dependency discovery produces actionable diagnostics.

## Fixture Plan

Add owned fixtures instead of depending on proprietary or third-party package
internals:

- `testdata/local-tests/managed-package-dependency`: a tiny package with
  namespace `znu`, global and public Apex APIs, nested types, custom objects,
  labels, resources, custom metadata, and a trigger.
- `testdata/local-tests/managed-package-consumer`: a separate project that
  references `znu.Address`, `znu.Service.Request`, `znu__Thing__c`, labels,
  resources, and custom metadata from the dependency.
- `testdata/local-tests/managed-package-access`: access-boundary tests proving
  `global` is allowed and `public` is denied across package boundaries.
- `testdata/local-tests/managed-package-runtime`: runtime tests for global
  service calls, dependency internal helper calls, dependency DML/SOQL, trigger
  firing, and static-state reset.

The fixture dependency should intentionally include APIs that are not globally
visible so regressions in access enforcement are easy to catch.

## Example-Project Application

For the current corpus, the plan should be applied in this order:

1. Create or locate a local source checkout for the package that owns namespace
   `znu`. For the current local corpus, use `src-nmb-nu-develop` as the source
   checkout and build it with installed namespace `znu`.
2. Build a compact package artifact:

   ```bash
   go run ./cmd/glade package build \
     --project example-projects/src-nmb-nu-develop \
     --namespace znu \
     --version src-nmb-nu-develop@<source-hash-or-git-sha> \
     --output example-projects/.glade/packages/znu/package.glade.json
   ```

3. Add a local `glade.yml` for the consuming example project that maps `znu` to
   that artifact, then fall back to source-backed config only when artifact
   loading is not enough for the specific test.
4. Run `compat local-tests` and confirm the top blocker changes from unknown
   `znu` type/object to either successful compile or explicit package-scoped
   access/runtime diagnostics.
5. Keep the source-backed path as a rebuild/debug path and use the artifact path
   for repeatable squad and CI runs.

Do not fix `znu` references by adding current-project stubs or hard-coded
runtime behavior. The dependency should be loaded as an installed package.

## Open Questions

- How should GLADE represent protected extension points in managed package code
  if enterprise projects use inheritance across namespace boundaries?
- Which package-version identifier should be canonical when local source does
  not include a `04t` id?
- How much dependency implementation should be compiled eagerly versus lazily
  from global API entry points?
- Should dependency package tests be runnable through an explicit
  `--include-dependency-tests` flag for package authors?
- How should package artifacts handle encrypted/protected custom metadata or
  metadata intentionally hidden from subscribers?
