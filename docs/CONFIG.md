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
`managedPackageDependencies` also accepts a block list of mappings.

```yaml
project:
  root: .
  packageDirs: [force-app]
  defaultNamespace: pkg
  namespaceRemaps: []
  managedPackageDependencies: []
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
| `project.defaultNamespace` | Default namespace for package-local code. |
| `project.namespaceRemaps` | Namespace aliases for local source dependencies. |
| `project.managedPackageDependencies` | Managed package source or artifact references. |
| `project.schemaSnapshot` | Schema snapshot path, relative to `glade.yml`; requires `schemaSnapshotSHA256`. |
| `project.schemaSnapshotSHA256` | Paired 64-character hexadecimal SHA-256 digest for `schemaSnapshot`. |
| `project.packageShims` | Local source roots that provide test/runtime bodies for captured package artifacts. |
| `org.features` | Scratch-org style features for local runtime behavior. |
| `test.permissionSets` | Inline list of project permission-set API names to assign to the running test user. |
| `test.permissionSetGroups` | Inline list of project permission-set-group API names to assign to the running test user. |
| `org.domainUrl` | Canonical HTTP(S) org origin used by `URL.getOrgDomainUrl()`; defaults to `https://local.glade.example`. |

When an SFDX project declares a package dependency and a sibling project in the
same source workspace declares that package in its own manifest, Glade resolves
that sibling by package name. Use `managedPackageDependencies` for a dependency
outside the workspace or when the source package needs an explicit namespace
remap.

`glade config validate` reports unsupported keys and parse errors before heavier
commands run. It also prints nonfatal warnings for unread settings and malformed
scratch definitions, with the source file and setting key. Setting values are
not included in these warnings. An unread key can be valid Salesforce
configuration; the warning means it has no local mapping in Glade.

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

The native comparison records three differences: Standard User describe access
is false; native `Security.stripInaccessible(CREATABLE)` removes `Name` where
Glade retains it; and an API 67 plain `COUNT()` denial raises
`System.SecurityException` natively while Glade reports
`System.QueryException`. In the restricted-user rows, user-mode DML also raises
`System.SecurityException`; plain `INSERT` without grants raises it at API 67,
while the captured API 62 insert succeeds.

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
