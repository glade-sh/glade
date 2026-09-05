---
pageType: reference
canonicalTask: /guide/quickstart
---

# Config reference

Glade reads `glade.yml` from the project tree and layers it with
`sfdx-project.json` when present. The Glade file carries local runtime choices
that Salesforce project files do not.

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
| `project.managedPackageDependencies` | dependency list | no configured dependencies | Use a source path or the documented artifact syntax. |
| `project.packageShims` | string list | no local method bodies from shims | A captured artifact still owns the contract. |
| `project.schemaSnapshot` | path string | no imported describe snapshot | Requires `schemaSnapshotSHA256`. |
| `project.schemaSnapshotSHA256` | string | no snapshot digest | Requires a snapshot and 64 hexadecimal characters matching its exact bytes. |
| `org.features` | string list | discovered/default local feature set | Unsupported feature names fail validation. |

## Config file

`glade.yml` intentionally supports a small YAML subset. Use inline lists such
as `[force-app]`; avoid anchors, merges, and complex YAML features.

```yaml
project:
  root: .
  packageDirs: [force-app]
  defaultNamespace: pkg
  namespaceRemaps: []
  managedPackageDependencies: []
  packageShims: []
  schemaSnapshot: .glade/schema/org.json
  schemaSnapshotSHA256: <64-character-sha256>
org:
  features: [PersonAccounts]
```

| Key | Meaning |
| --- | --- |
| `project.root` | Project root. Relative paths resolve from `glade.yml`. |
| `project.packageDirs` | Source package directories. Overrides `sfdx-project.json`. |
| `project.defaultNamespace` | Default namespace for package-local code. |
| `project.namespaceRemaps` | Namespace aliases for local source dependencies. |
| `project.managedPackageDependencies` | Managed package source or artifact references. |
| `project.packageShims` | Local source roots that provide test/runtime bodies for captured package artifacts. |
| `project.schemaSnapshot` | Imported describe schema used by local `check` and `test`. Relative paths resolve from `glade.yml`. |
| `project.schemaSnapshotSHA256` | Required SHA-256 of the exact snapshot file bytes. |
| `org.features` | Scratch-org style features for local runtime behavior. |

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

```text
unsupported org feature: SomeFeature
```

Use only features Glade models locally.

::: tip Next step
Run local tests without an org: [Run Apex Tests Locally](/guide/local-testing).
:::
