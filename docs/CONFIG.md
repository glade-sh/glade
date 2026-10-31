# Project Configuration

Glade reads a `glade.yml` file from the project tree. It also reads
`sfdx-project.json` when present. The Glade file is for local runtime choices
that Salesforce project files do not carry.

The expanded source API support window needs no new setting. The keys below
are accepted by this release.

## Start

Create a starter file from an existing project. At a terminal, `glade init`
prompts for package dirs, namespace, and org features. Use `--yes` to accept
inferred defaults without prompts:

```bash
glade init --project . --yes
```

`glade config init` is the same initializer under the `config` command group.

Check it:

```bash
glade config validate --project .
glade config show --project .
glade config show --project . --json
```

`glade init` infers package directories and namespace from `sfdx-project.json`.
Pass flags to set values by hand or for CI:

```bash
glade init --project . --yes \
  --namespace pkg \
  --package-dir force-app \
  --feature PersonAccounts
```

Use `--force` to replace an existing `glade.yml`.

## File Format

The parser accepts a small YAML subset. Most lists use inline brackets;
`dependencies` and `managedPackageDependencies` also accept block lists of mappings.

```yaml
project:
  root: .
  packageDirs: [force-app]
  extraMetadataDirs: []
  defaultNamespace: pkg
  namespaceRemaps: []
  dependencies: []
  managedPackageDependencies: []
  dependencyPaths: []
  packageShims: []
org:
  features: [PersonAccounts]
test:
  permissionSets: [Synthetic_Access]
  permissionSetGroups: []
```

Supported keys:

| Key | Meaning |
| --- | --- |
| `project.root` | Project root. Relative paths resolve from `glade.yml`. |
| `project.packageDirs` | Source package directories. Overrides `sfdx-project.json`. |
| `project.extraMetadataDirs` | Additional source and metadata directories, relative to `glade.yml`. Leaves package directories in place. Missing directories are configuration errors. |
| `project.defaultNamespace` | Default namespace for package-local code. |
| `project.namespaceRemaps` | Namespace aliases for local source dependencies. |
| `project.dependencies` | Declared package names or SFDX aliases mapped to local source paths. |
| `project.managedPackageDependencies` | Managed package source or artifact references. |
| `project.dependencyPaths` | Inline list of local directories to search for declared SFDX package dependencies, relative to the supplying `glade.yml`. |
| `project.schemaSnapshot` | Schema snapshot path, relative to `glade.yml`; requires `schemaSnapshotSHA256`. |
| `project.schemaSnapshotSHA256` | Paired 64-character hexadecimal SHA-256 digest for `schemaSnapshot`. |

| `project.packageShims` | Local source roots that provide test/runtime bodies for captured package artifacts. |
| `pdf.command` | Absolute executable path for optional external HTML-to-PDF rendering; empty uses the local text writer. |
| `org.features` | Scratch-org style features for local runtime behavior. |
| `test.permissionSets` | Inline list of project permission-set API names to assign to the running test user. |
| `test.permissionSetGroups` | Inline list of project permission-set-group API names to assign to the running test user. |
| `org.domainUrl` | Canonical HTTP(S) org origin used by `URL.getOrgDomainUrl()`; defaults to `https://local.glade.example`. |
| `test.user.timeZone` | Default running test user's time zone. |
| `test.user.locale` | Default running test user's locale key. |
| `test.user.language` | Default running test user's language key. |
| `test.user.defaultCurrencyIsoCode` | Default running test user's currency code. |
| `test.clock.start` | Deterministic test clock start as an RFC3339 timestamp. |

A snapshot's matching hash verifies its contents; it does not establish that
its field list is complete. To assert that an object contains every field from
the represented org, set `"fieldsComplete": true` on that object in the pinned
snapshot. Omitted or false assertions leave unknown fields open. An object with
`"partial": true` remains open even if it also asserts completeness. Only make
this assertion when the snapshot includes the whole field surface needed by the
project, including installed and org-specific fields.

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

An SFDX project declares dependencies in
`packageDirectories[].dependencies[].package`. Glade matches that package name
to a `packageDirectories[].package` entry in a local project's own
`sfdx-project.json`. Glade reads local source only; it does not install a
package or fetch one from the network.

### Dependency paths

Use `project.dependencyPaths` when a local dependency checkout is outside the
project's parent and grandparent, or lives under a shared library directory.
For example, if `consumer/glade.yml` is beside `consumer/sfdx-project.json` and
the dependency project is under `libs/base`, configure the parent directory:

```yaml
project:
  dependencyPaths: ["../libs", "/opt/sf-libs"]
```

`dependencyPaths` only resolves dependencies the consumer declares in
`sfdx-project.json`; listing a directory does not import every project in it.

Each entry is a directory. Glade checks the entry itself and each immediate
child directory that holds an `sfdx-project.json`; it does not walk deeper
subdirectories. Relative paths resolve from the `glade.yml` that supplied the
setting. The project's own file takes precedence over the nearest ancestor
`glade.yml`; the lists are not merged.

Lookup steps:

1. A matching explicit `project.dependencies` mapping.
2. A matching `project.managedPackageDependencies` entry that loads; this skips
   local lookup.
3. The parent and grandparent directories themselves, plus `<base>/<name>` and
   `<base>/packages/<name>` for package-name variants, and sibling project
   directories directly under the parent that contain an
   `sfdx-project.json`.
4. Each `dependencyPaths` entry, in the order listed.
5. If the consumer manifest's `packageAliases` object has a key matching the
   dependency name, immediate child directories under each immediate directory
   beneath the project root's grandparent.

Local candidates from all applicable locations count together. A candidate
matches only when one of its package directories declares the
dependency's package name (case and surrounding space ignored). One match
loads. More than one local match reports `dependency_ambiguous` and lists every
root; narrow `dependencyPaths` or remove the duplicate checkout. No match reports
`dependency_missing`, which names the package and suggests the `glade.yml`
setting to add. For a namespaced dependency, use
`project.managedPackageDependencies` when the source needs an explicit namespace
remap or the dependency is represented by an artifact. Nothing outside the
listed directories is scanned.

Changing the list, or a dependency appearing at a listed path, invalidates
cached project state, so the next run rebuilds. `glade config show` prints the
configured paths. A missing dependency diagnostic names the package and gives
an example `project.dependencyPaths` entry for pointing Glade at a local
checkout.

`packageAliases` does not map an alias to a local directory; a matching entry
only enables the bounded grandparent category scan above.

### Dependency paths

An `sfdx-project.json` can list packages under `packageDirectories[].dependencies`.
Glade resolves each name to a local checkout that declares that package in
its own `sfdx-project.json`. `project.dependencyPaths` adds directories to the
search when the checkout is not beside your project:

```yaml
project:
  dependencyPaths: ["../../libs", "/opt/sf-libs"]
```

Each entry is a directory. Glade tests the entry itself and each of its child
directories that holds an `sfdx-project.json`. Matching namespaced
`managedPackageDependencies` resolve first. Remaining dependencies use the
normal parent/grandparent and sibling-project search plus `dependencyPaths`.
When the dependency also has a `packageAliases` entry, Glade additionally checks
projects within category directories under the grandparent.

A candidate matches only when one of its package directories declares the
dependency's package name (case and surrounding space ignored). One match
loads. More than one match reports `dependency_ambiguous` and lists every root;
narrow `dependencyPaths` or remove the duplicate checkout. No match reports
`dependency_missing`, which names the package and the key to add. Discovery is
limited to these local workspace locations and explicit paths.

The list comes from the project's own `glade.yml`, or else the nearest
ancestor `glade.yml`. Changing it, or a dependency appearing at a listed path,
invalidates cached project state, so the next run rebuilds. `glade config show`
prints the configured paths.

Mapping `packageAliases` entries to local directories is not part of this key.

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

## Experience Cloud metadata

Communities is off by default. Enable the existing feature and include any
supplemental deployment directories used by your project:

```yaml
project:
  extraMetadataDirs: [config/experience-cloud]
org:
  features: [Communities]
```

Without Communities, `Schema.getGlobalDescribe()` omits `Network` and
`Type.forName('Schema.Network')` returns null. The unqualified platform
`Network` class remains available. Enabling
Communities exposes its schema but creates no Network rows by itself.
Glade loads `*.network-meta.xml` and `*.network` files from package directories
and `extraMetadataDirs`. Each file supplies a Network name from its filename,
plus `Status` and `UrlPathPrefix` from its XML. IDs are local and deterministic.
An empty singleton query throws the native, catchable `QueryException`.

The same project metadata is available to anonymous execution and the Apex test
runner. Network metadata does not establish a hosted request context, create
guest users or provision a live Experience Cloud site. Additional Network
settings and member provisioning are not modeled by this loader.

## Org Features

`org.features` and `glade init --feature` accept the same feature names. Use the
canonical spellings below. Features configure specific local schema and runtime
behavior; enabling a name does not provision a Salesforce service.

| Feature | Local behavior |
| --- | --- |
| `PersonAccounts` | Person Account fields, record types, and contact relationship. |
| `MultiCurrency` | Currency fields, currency setup objects, and `UserInfo.isMultiCurrencyOrganization()`. |
| `StateAndCountryPicklist` | State and country code fields. |
| `Communities` | Local community schema; alias: `FlowSites`. |
| `Sites` | Local site/community schema and guest profiles. |
| `ContactsToMultipleAccounts` | Account-contact relationship schema. |
| `PlatformCache` | Local cache schema; alias: `ProviderFreePlatformCache`. |
| `AddCustomApps:N` | Seed `N` local application records; for example, `AddCustomApps:3`. |
| `Quotes` | `Opportunity.SyncedQuoteId` field availability. |
| `HealthCloud` | Writable `Account.IsActive` Boolean field, defaulting to `false`; accepts names beginning with `HealthCloud`, including `HealthCloudAddOn`. |
| `TranslationLanguage-fr` | Model an active French Translation Workbench language for the captured `Account.Industry` standard value-set `toLabel` route. Include the corresponding `Industry-fr.standardValueSetTranslation` metadata. |

`Chatter`, `LightningExperience`, `EnableSetPasswordInApi`, and
`AnalyticsAdminPerms` are retained as capability flags. They
do not expose synthetic feature fields on `Organization` or implement hosted
service behavior. Feature names without a local implementation do not add that
functionality. In particular, a scratch definition can request services or
licenses that the offline runtime does not implement.

### Discovery and merging

Glade combines features from these sources at the project root:

- The nearest `glade.yml`, through `org.features`.
- `config/project-scratch-def.json` and `config/hc-project-scratch-def.json`.
- Relative JSON `config_file:` entries in `cumulusci.yml` and
  `cumulusci.template.yml`. Paths must stay inside the project root.

Scratch definitions can express `features` as a string or an array of strings.
The result is a union across all discovered files, including separate CumulusCI
org configurations such as dev and beta. There is no implicit selection of one
CumulusCI org. A setting set to `false` does not remove a feature enabled by
another source.

Glade reads supported org-shape settings from CumulusCI files, but it does not
run CumulusCI flows, import their dataset files, or invoke managed-package
post-install scripts. Put records needed by local Apex tests in test-owned setup
code, such as an `@TestSetup` method.

Glade does not search for arbitrary scratch definition filenames or recurse
through nested package directories to discover definitions. An `orgs/*.json`
file is read only when referenced by a supported `config_file:` entry. Missing
default files are optional. Malformed definitions contribute no features or
settings; `config validate` warns about them. It validates discovered files,
so it cannot diagnose an unreferenced definition elsewhere in the project.

### Mapped settings

These scratch-definition settings enable a feature only when their value is
the JSON boolean `true`. The string `"true"` does not enable it. Keys are case
sensitive. A recognized key set to `false` is still read and produces no
unread-key warning.

| Setting | Feature |
| --- | --- |
| `communitiesSettings.enableNetworksEnabled` | `Communities` |
| `communitiesSettings.enableOotbProfExtUserOpsEnable` | `Communities` |
| `chatterSettings.enableChatter` | `Chatter` |
| `lightningExperienceSettings.enableS1DesktopEnabled` | `LightningExperience` |
| `currencySettings.enableMultiCurrency` | `MultiCurrency` |
| `quoteSettings.enableQuote` | `Quotes` |

Other settings remain in the source file and do not configure local org shape.
For example, `userManagementSettings.enableNewUserEmailVerification` does not
enable `EnableSetPasswordInApi`. `glade config validate` reports these unread
keys without rejecting the project.

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

### Anonymous execution and tests

`glade exec` starts with a lean local org, then loads project schema and runtime
metadata when a project is selected. `glade test` starts each test org from the
full standard schema catalog before applying project metadata and feature
policy. Objects can also be materialized as execution needs them. Consequently,
`Schema.getGlobalDescribe()` and `Type.forName()` can observe different loaded
schema between anonymous execution and tests. Neither command's raw object
count is a Salesforce org inventory.

Both paths use the project's discovered feature inputs. Features provide only
their implemented local behavior; live Experience Cloud provisioning and other
hosted services remain outside these configuration options. Feature definition
files also affect the [test startup cache](TEST_STARTUP_CACHE.md).

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

## Namespace Remaps

Use a namespace remap when a source dependency keeps its production namespace
in code but the consumer depends on a different local namespace. In the consumer
case, the source dependency uses the production namespace `BasePkg`, while the
consumer depends on `stagepkg`:

```yaml
project:
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["stagepkg:../base-source"]
```

The remap is applied in memory when Glade reads source-backed dependency code
and metadata. Files on disk are not rewritten. Apex string literals are remapped
too, so dynamic SOQL and metadata names see the runtime namespace.

If a source-backed dependency's `sfdx-project.json` namespace differs from the
configured dependency namespace, Glade requires a matching namespace remap. For
example, source namespace `BasePkg` loaded as `stagepkg` needs
`namespaceRemaps: ["BasePkg:stagepkg"]`.

## Package Artifacts

Use an artifact dependency when a local project needs package contracts but not
the package source:

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
`@glade/orgpackage` is installed or linked. The artifact supplies global Apex
signatures, schema, labels, static resources, and code-intelligence symbols.
Captured package methods have no local body. Add a shim root when tests need
local behavior:

```yaml
project:
  packageShims: ["pkg:test-support/package-shims/pkg"]
```

Shim classes compile under the package namespace and supply bodies for captured
methods that would otherwise have signatures only. They do not replace the
artifact contract; keep the artifact as the dependency and use shims only for
local test or runtime behavior.

## Test User Permissions

When a project has no profile metadata, Glade keeps its local running test user
permissive for custom-object CRUD and field-level security. This follows the
deploying-administrator context; it does not model a Salesforce Standard User,
which can have no access to a new custom object. Keep explicit restricted-user
tests separate with `System.runAs`.

The native Standard User captures at source APIs 62–67 have no access to a
new custom object without profile or permission-set grants. Describe returns
false for its CRUD and optional-field FLS. `Security.stripInaccessible(CREATABLE)`
removes `Name` as well as the optional field when root CRUD enforcement is
false. The `READABLE` root-CRUD-enforced twin raises `System.NoAccessException`.
User-mode queries and inserts deny access at every captured API. Plain
`Database.countQuery` calls and `Database.insert` calls succeed through API 66;
at API 67, these calls in `System.runAs` raise `System.SecurityException`.
Explicit `SYSTEM_MODE`
queries and inserts succeed throughout this captured range.

Use `test.permissionSets` when a project's setup instructions require an admin
to assign permission sets before running tests. Names are API names from
`.permissionset-meta.xml` files in the project's source package directories,
not labels. For an ordinary, unmuted permission set group, use
`test.permissionSetGroups` and include the group's member permission sets in
the project metadata. Muting permission sets are not supported for configured
assignments and produce a config error.

Glade creates normal `PermissionSetAssignment` records on the test run's private
org before static initializers, `@TestSetup`, and test methods. Existing runtime
permission checks resolve the assigned metadata's custom permissions, CRUD,
and field access. Custom permissions must be declared in project metadata and
granted by a permission set; there is no separate custom-permission override.
Assignments apply to the running user. `System.runAs(other)` uses the other
user's own profile and assignments. Repeated names create one assignment.
This option assigns the listed grants; it does not select a profile or replace
the permissive no-profile default. For strict access checks, run code as a user
with a restricted profile and that user's own permission assignments.

Missing permission-set or group names produce a config error naming the missing
API name. `glade config validate` checks these references; `glade config show`
(including `--json`) displays the configured lists. Empty or omitted lists add
no assignments. These options configure `glade test`; they do not change a
persisted local database or anonymous execution.

## PDF rendering

Blob.toPdf, PageReference.getContent and getContentAsPDF render locally with a text-only PDF writer that follows the layout of Salesforce's renderer (PDF-1.4, Letter, Times-Roman, Flate streams). It is not Salesforce's Flying Saucer/iText engine: CSS, tables, images and embedded fonts are not rendered, and output is deterministic where Salesforce's is not. Set pdf.command to an external HTML-to-PDF program for real rendering.

PageReference.getContent returns HTML for pages without renderAs="pdf". The pdf.command setting applies to Visualforce page rendering; Blob.toPdf uses the local writer.

The page-size and page-break controls `@page { size: A4 }`,
`@page { size: landscape }`, and `page-break-before: always` are supported.
Other layout is approximate. Default local output uses 36 pt margins and
12 pt Times-Roman with WinAnsi text; unsupported glyphs are omitted.

```yaml
pdf:
  command: /absolute/path/to/html-to-pdf
```

The executable receives HTML on stdin and the page URL as its first argument,
and must write PDF bytes to stdout. Glade does not install a browser renderer.
`glade doctor` reports the selected page renderer.
