---
pageType: reference
canonicalTask: /guide/quickstart
---

# Config reference

Glade reads `glade.yml` from the project tree and layers it with
`sfdx-project.json` when present. The Glade file carries local runtime choices
that Salesforce project files do not.

The expanded source API support window needs no new setting. The keys below
are accepted by this release.

## Configuration Precedence

Glade resolves settings in this order:

1. CLI flags
2. `glade.yml`
3. `sfdx-project.json`
4. Glade defaults

## Create config

Create a starter file from an existing Salesforce DX project:

```bash
glade init --project . --yes
```

Check it:

```bash
glade config validate --project .
glade config show --project .
glade config show --project . --json
```

Set values by hand or from CI:

```bash
glade init --project . --yes \
  --namespace pkg \
  --package-dir force-app \
  --feature PersonAccounts
```

Use `--force` to replace an existing file.

## Inputs and constraints

| Input | Type | When omitted | Constraint |
| --- | --- | --- | --- |
| `project.root` | path string | configuration/project discovery supplies the root | Relative paths resolve from the config directory. |
| `project.packageDirs` | string list | discovered Salesforce DX package directories | Every selected directory must exist. |
| `project.defaultNamespace` | string | project namespace when available | Keep dependency namespaces and remaps consistent. |
| `project.namespaceRemaps` | string list | no remaps | Each entry pairs source and local namespaces. |
| `project.dependencies` | mapping list | no explicit source mappings | Map consumer-declared package names or aliases to local directories. |
| `test.user.*` | strings | deterministic running-user defaults | Validate time zone, locale, language and currency keys. |
| `test.clock.start` | RFC3339 string | `2026-05-02T12:00:00Z` | Use Z or an explicit UTC offset. |
| `project.managedPackageDependencies` | dependency list | no configured dependencies | Use a source path or the documented artifact syntax. |
| `project.dependencyPaths` | path list | normal local workspace discovery | Relative paths resolve from `glade.yml`; each entry adds a directory and its immediate child projects. |
| `project.packageShims` | string list | no local method bodies from shims | A captured artifact still owns the contract. |
| `project.schemaSnapshot` | path string | no imported describe snapshot | Requires `schemaSnapshotSHA256`. |
| `project.schemaSnapshotSHA256` | string | no snapshot digest | Requires a snapshot and 64 hexadecimal characters matching its exact bytes. |
| `org.features` | string list | discovered/default local feature set | Features without a local implementation do not add functionality. |
| `org.domainUrl` | string | `https://local.glade.example` at runtime | Absolute HTTP(S) origin; no credentials, path, query, or fragment. |
| `test.permissionSets` | string list | no configured assignments | Project permission-set API names assigned to the running test user. |
| `test.permissionSetGroups` | string list | no configured group assignments | Project permission-set-group API names assigned to the running test user. |

## Config file

`glade.yml` intentionally supports a small YAML subset. Use inline lists such
as `[force-app]`; avoid anchors, merges, and complex YAML features.

```yaml
project:
  root: .
  packageDirs: [force-app]
  defaultNamespace: pkg
  namespaceRemaps: []
  dependencies: []
  managedPackageDependencies: []
  dependencyPaths: []
  packageShims: []
  schemaSnapshot: .glade/schema/org.json
  schemaSnapshotSHA256: <64-character-sha256>
org:
  features: [PersonAccounts]
test:
  permissionSets: [Synthetic_Access]
  permissionSetGroups: []
```

| Key | Meaning |
| --- | --- |
| `project.root` | Project root. Relative paths resolve from `glade.yml`. |
| `project.packageDirs` | Source package directories. Overrides `sfdx-project.json`. |
| `project.defaultNamespace` | Default namespace for package-local code. |
| `project.namespaceRemaps` | Namespace aliases for local source dependencies. |
| `project.dependencies` | Declared package names or SFDX aliases mapped to local source paths. |
| `project.managedPackageDependencies` | Managed package source or artifact references. |
| `project.dependencyPaths` | Extra local directories searched for declared SFDX package dependencies. |
| `project.packageShims` | Local source roots that provide test/runtime bodies for captured package artifacts. |
| `project.schemaSnapshot` | Imported describe schema used by local `check` and `test`. Relative paths resolve from `glade.yml`. |
| `project.schemaSnapshotSHA256` | Required SHA-256 of the exact snapshot file bytes. |
| `org.features` | Scratch-org style features for local runtime behavior. |
| `org.domainUrl` | Canonical HTTP(S) org origin used by `URL.getOrgDomainUrl()`; defaults to `https://local.glade.example`. |
| `test.permissionSets` | Project permission-set API names assigned before Apex tests run. |
| `test.permissionSetGroups` | Project permission-set-group API names assigned before Apex tests run. |
| `test.user.timeZone` | Default running test user's time zone. |
| `test.user.locale` | Default running test user's locale key. |
| `test.user.language` | Default running test user's language key. |
| `test.user.defaultCurrencyIsoCode` | Default running test user's currency code. |
| `test.clock.start` | Deterministic test clock start as an RFC3339 timestamp. |

### Test user and clock

Set the default running test user and deterministic clock in `glade.yml`:

```yaml
test:
  user:
    timeZone: GMT
    locale: en_GB
    language: en_US
    defaultCurrencyIsoCode: USD
  clock:
    start: '2026-05-04T12:00:00Z'
```

These settings apply to Apex tests, including static initialization, `@TestSetup`,
and async work. `System.runAs` uses that user's own settings and restores the
configured running user on exit. The configured user fields can also be queried
from `User`. They do not change the Organization defaults or the host clock.

Omitted values keep the existing defaults: `America/Los_Angeles`, `en_US` locale
and language, `USD`, and a clock starting at `2026-05-02T12:00:00Z`. Choose a weekday
when a test deliberately assumes that today and tomorrow are in the same week.
Choose `GMT` when a UTC timestamp's calendar date is the intended local date.

`timeZone` must be a recognized IANA identifier (or `UTC`/`GMT`); host-dependent
`Local` and unknown identifiers fail validation. A recognized zone outside the
local datetime runtime's supported zones produces an explicit test startup error.
`locale` uses a recognized language and region, such as `en_US`, `en_GB`, or `de_DE`;
`language` uses a recognized language key, such as `en_US` or `de`. Currency uses
an uppercase ISO currency code. Existing locale formatting coverage still applies;
default datetime formatting includes British and German forms.

`clock.start` requires an RFC3339 timestamp with seconds and `Z` or an explicit
UTC offset. The runtime normalizes it to UTC at millisecond precision and retains
its deterministic clock ticks; this setting does not freeze time. Each test and
setup execution starts from the configured instant. Apex `Date.today()`, local
Datetime conversions, SOQL `TODAY`/`THIS_WEEK`/`NEXT_WEEK`, and formula `TODAY()`
use the running user's calendar; Datetime `now()` and formula `NOW()` share the
configured instant. GMT conversions retain GMT semantics.

Run `glade config validate` to check the values. `glade config show` and
`glade doctor` show the effective test environment when any value is configured;
JSON doctor output includes `testEnvironment`. These settings affect `glade test`,
including its cached runner, and do not configure `glade exec` or server requests.

### Local source dependencies

When class source copies occur in different package directories, Glade selects
the later lexicographic source path for checking and execution. `glade check`
reports a warning listing both paths and the selected copy. Set
`project.packageDirs` to select a single directory explicitly. Duplicate
declarations within one package still receive normal duplicate diagnostics.

When an SFDX project declares a package dependency and a sibling project in the
same source workspace declares that package in its own manifest, Glade resolves
that sibling by package name. Use `project.dependencies` to map a dependency to
a known local checkout or bare source directory:

```yaml
project:
  dependencies:
    - name: SharedLibrary
      path: ../vendor/shared-library
    - name: SharedToolsAlias
      path: ../shared-project
      package: SharedTools
      version: 1.2.3
```

`name` matches the `package` value in `sfdx-project.json`, including an alias
listed in `packageAliases`. `path` is relative to the `glade.yml` file and may
point to an SFDX project or a bare source directory without a manifest. When
pointing to an SFDX project, `package` optionally selects one
`packageDirectories[].package`; if omitted, the project’s default package
directories are used, or all declared package directories if none is marked
default. Source namespaces come from the source project and are not inferred
from the mapping name. The optional `version` is recorded for the local
dependency and does not prevent loading a checkout.

An explicit `dependencies` entry takes precedence over a managed dependency or
implicit sibling search for the same name. Duplicate names in `dependencies`
are configuration errors. If implicit discovery finds more than one matching
project, Glade reports the candidate paths as an ambiguity. Use
`managedPackageDependencies` for managed package sources or artifacts and
`namespaceRemaps` when a source package needs an explicit namespace remap.

Only dependencies declared in the consumer manifest are loaded from these mappings.
An explicit `project.dependencies` mapping takes precedence over workspace discovery.

`glade config validate` reports unsupported keys and parse errors before heavier
commands run. It also prints nonfatal warnings for unread settings and malformed
scratch definitions, with the source file and setting key. Setting values are
not included in these warnings. An unread key can be valid Salesforce
configuration; the warning means it has no local mapping in Glade.

### Undeclared setup suggestions

`glade doctor` scans project source (`.cls`, `.trigger`, flows, field and
permission-set metadata, Visualforce, and LWC `.html`/`.js`) and prints the
`glade.yml` lines for setup the source uses but the project does not declare.
`glade doctor --json` returns the same data as `orgSetup`. When tests fail,
Glade prints a short setup hint on stderr and points to doctor for evidence.
Suggestions only advise. They change no runtime behavior, and doctor also scans projects that
have no `glade.yml`.

| Evidence in source | Suggestion |
| --- | --- |
| `IsPersonAccount`, `PersonContactId`, `Person*` contact fields such as `PersonEmail`, or `*__pc` fields | `org.features: [PersonAccounts]` |
| `FROM Network`, `FROM NetworkMember`, `Network.getNetworkId()`, a `User.IsPortalEnabled` field access/query, or a `User.UserType` equality such as `CustomerSuccess` | `Communities` |
| `Site.getBaseUrl()`, `Site.login()` and similar | `Sites` |
| `CurrencyIsoCode`, `FROM CurrencyType`, `FROM DatedConversionRate`, `convertCurrency(` | `MultiCurrency` |
| Queries, schema references, or typed uses of `Territory2`, `Territory2Model`, `Territory2Type`, territory associations, or `Opportunity.Territory2Id` | Reports Enterprise Territory Management as `not modelled`; no `org.features` line is suggested. |
| A test class that uses `PermissionSetAssignment` and names a project permission set | `test.permissionSets: [Name]` |
| A SOQL literal for `RecordType.DeveloperName`/`Name`, a direct `FROM RecordType` query with literal `SObjectType` and `DeveloperName`/`Name`, a record type describe-map lookup, or a `UserRole` name lookup whose metadata file is absent | Add matching `recordType-meta.xml` or `.role-meta.xml` metadata to a package directory. |
| A feature in `config/project-scratch-def.json` | Shows the matching `glade.yml` `org.features` line when mapped, or `not modelled` when unsupported. |
| A `dependency_missing` diagnostic for an SFDX package dependency | Keep the package under `packageDirectories[].dependencies` and add its containing directory to `project.dependencyPaths`. |

A feature already declared in `glade.yml`, a discovered scratch definition,
or a mapped setting is not suggested. Record type and role suggestions come
from literal names in Apex queries,
record type describe-map lookups, or `UserRole` construction. A matching
record-type or Role metadata file suppresses the suggestion. `Master` is an
implicit record type and is not suggested. Names assembled at runtime are not
inferred.

For Experience Cloud and Territory2 advice, bare local identifiers, labels, and
Apex comments do not count as platform evidence. Static and dynamic queries,
typed field access (including `get('IsPortalEnabled')` on a `User`), schema
references, and metadata field/object references do count. RecordType queries
also recognize root aliases such as `FROM RecordType rt WHERE
rt.SObjectType = 'Account' AND rt.DeveloperName = 'Synthetic_Client'`;
relationship predicates such as `rt.CreatedBy.Name` are not record-type names.
When different types reuse a receiver name, the scanner conservatively omits
that typed-receiver evidence; explicit query/schema evidence still counts.
External user-type comparisons
recognize `CustomerSuccess`, `PowerCustomerSuccess`, `PowerPartner`, and
`CspLitePortal`. These are setup hints; declaring `Communities` does not create
licensed users or provision an Experience Cloud site. A `__pc` reference also
needs the corresponding custom Contact field metadata.

Scratch-definition features are listed with their local mapping. Recognized
features point to `org.features`; unrecognized names are marked `not modelled`.
This reports configuration coverage only: listing a feature does not add local
behavior or enable hosted Salesforce services. A `dependency_missing` hint is
shown only when the project loader emits that diagnostic; it points to
`project.dependencyPaths` for local SFDX source while keeping the package
declaration in `sfdx-project.json`. Existing `org.features` and
`test.permissionSets` entries are kept in suggested config blocks. A
permission-set name counts only when a matching `.permissionset-meta.xml` exists
in the project.

Glade reads supported org-shape settings from CumulusCI files, but it does not
run CumulusCI flows, import their dataset files, or invoke managed-package
post-install scripts. Put records needed by local Apex tests in test-owned setup
code, such as an `@TestSetup` method.
## Dependency paths

For a dependency declared in `packageDirectories[].dependencies`, add a checkout
outside the usual local workspace search with `project.dependencyPaths`:

```yaml
project:
  dependencyPaths: ["../../libs", "/opt/sf-libs"]
```

Each entry can be a project directory or a parent containing project directories.
A checkout matches when its `sfdx-project.json` declares the requested package
name. Configured namespaced `managedPackageDependencies` resolve first; remaining
packages use local workspace discovery plus these paths. One match loads;
`dependency_missing` means no matching source was found, and
`dependency_ambiguous` lists multiple matching roots. Narrow the search or remove
duplicate checkouts to resolve ambiguity. `packageAliases` does not map package
names to local paths.

The project's own `glade.yml`, or otherwise the nearest ancestor configuration,
supplies these paths. `glade config show` prints them. Changing the paths or
adding a dependency there invalidates cached project state for the next run.

## Org features and test permissions

Glade no longer infers `PersonAccounts` from field references in Apex. Declare
`org.features: [PersonAccounts]` in `glade.yml`, or enable the feature in a
discovered scratch definition. Features configure local schema and runtime
behavior; they do not provision hosted Salesforce services.

Features are combined from `glade.yml`, `config/project-scratch-def.json`,
`config/hc-project-scratch-def.json`, and relative `config_file:` entries in
`cumulusci.yml` or `cumulusci.template.yml`. Those relative paths must stay inside
the project. A false setting does not remove a feature enabled by another input.

`test.permissionSets` and `test.permissionSetGroups` assign project metadata to
the running test user before tests execute. Use metadata API names, without the
file suffix, rather than labels or Salesforce record IDs.

### Setup suggestions

```bash
glade doctor --project .
glade doctor --project . --json
```

Doctor scans source for explicit identifiers associated with undeclared
`PersonAccounts`, `Communities`, `Sites`, `MultiCurrency`, and project
permission-set assignments. It prints suggested `glade.yml` lines and returns
them as [`orgSetup`](/reference/json-schema#doctor-json) in JSON. It also scans
projects without `glade.yml`. Advice preserves existing configured entries and
does not enable features, assign permissions, or change the doctor exit code.
When a test run fails and setup suggestions exist, `glade test` prints the same
advice on stderr.

## Pinned describe schema

Import an already captured Salesforce describe response, then hash the exact
output file:

```bash
glade schema import describe \
  --input reports/org-describe.json \
  --output .glade/schema/org.json
shasum -a 256 .glade/schema/org.json
```

Copy the first `shasum` column into `schemaSnapshotSHA256`. Both keys are
required together. Glade refuses changed bytes and reports both the configured
and actual digest, so update the digest only after intentionally replacing the
snapshot. Local metadata overrides overlapping snapshot objects and fields.

The `.glade/symbols` code-intelligence cache remains advisory. It is not read by
the local Apex runtime; only this explicitly configured snapshot feeds `check`
and `test`.

## Runtime environment

`org.domainUrl` accepts an absolute HTTP(S) origin and removes a trailing slash.
It is independent of the incoming server request URL. For example:

```yaml
org:
  domainUrl: https://local-org.example.test
```

Set `GLADE_TEST_TIMEOUT` in the command environment, rather than `glade.yml`,
for the per-test wall-clock cap (default `10m`; `0` disables it).
`--test-timeout` overrides the environment. Invalid or negative durations fail
validation. This also applies to `@TestSetup`, daemon, watch, and `glade dev`
test runs. It does not change CPU or governor limits.

## Namespace remaps

Use a namespace remap when source-backed package code still names its production
namespace, but the local consumer depends on that code under another namespace.
Glade applies the remap in memory while reading dependency Apex and metadata.
Files on disk stay unchanged.

```yaml
project:
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["stagepkg:../base-source"]
```

Apex string literals are remapped too. Dynamic SOQL and metadata names see the
runtime namespace in the local test and exec loop.

If a source-backed dependency's `sfdx-project.json` namespace differs from the
configured dependency namespace, Glade requires a matching namespace remap. For
example, source namespace `BasePkg` loaded as `stagepkg` needs
`namespaceRemaps: ["BasePkg:stagepkg"]`.

## Package artifacts

Use an artifact dependency when a project needs installed package contracts
without package source:

```yaml
project:
  managedPackageDependencies: ["pkg:artifact:.glade/packages/pkg.glade-package.json:1.2.3.4"]
```

Capture the artifact from a packaging or subscriber org with the first-party
org package plugin:

```bash
glade plugins install @glade/orgpackage
glade package capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet
```

`glade package capture` dispatches to `glade orgpackage capture` when
`@glade/orgpackage` is installed or linked. Captured methods compile from
signatures. They do not run local behavior unless a shim root supplies source:

```yaml
project:
  packageShims: ["pkg:test-support/package-shims/pkg"]
```

Shim classes compile under the package namespace and supply bodies for captured
methods that would otherwise have signatures only. They do not replace the
artifact contract; keep the artifact as the dependency and use shims only for
local test or runtime behavior.

## Common Validation Errors

```text
unknown package directory: force-app/main/default
```

Check `project.packageDirs` against `sfdx-project.json`.

For undeclared org setup, run `glade doctor --project .` and review its suggested
configuration. Declaring a feature without a local implementation does not add
hosted behavior.

::: tip Next step
Run local tests without an org: [Run Apex Tests Locally](/guide/local-testing).
:::
