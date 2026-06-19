# Project Configuration

Glade reads a `glade.yml` file from the project tree. It also reads
`sfdx-project.json` when present. The Glade file is for local runtime choices
that Salesforce project files do not carry.

## Start

Create a starter file from an existing project. At a terminal, `glade init`
prompts for package dirs, namespace, and org features. Use `--yes` to accept
inferred defaults without prompts:

```bash
glade init --project . --yes
```

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

The parser accepts a small YAML subset. Lists use inline brackets.

```yaml
project:
  root: .
  packageDirs: [force-app]
  defaultNamespace: pkg
  managedPackageDependencies: []
  packageShims: []
org:
  features: [PersonAccounts]
```

Supported keys:

| Key | Meaning |
| --- | --- |
| `project.root` | Project root. Relative paths resolve from `glade.yml`. |
| `project.packageDirs` | Source package directories. Overrides `sfdx-project.json`. |
| `project.defaultNamespace` | Default namespace for package-local code. |
| `project.managedPackageDependencies` | Managed package source or artifact references. |
| `project.packageShims` | Local source roots that provide test/runtime bodies for captured package artifacts. |
| `org.features` | Scratch-org style features for local runtime behavior. |

`glade config validate` reports unsupported keys and parse errors before heavier
commands run.

## Package Artifacts

Use an artifact dependency when a local project needs package contracts but not
the package source:

```yaml
project:
  managedPackageDependencies: ["pkg:artifact:.glade/packages/pkg.glade-package.json:1.2.3.4"]
```

The artifact supplies global Apex signatures, schema, labels, static resources,
and code-intelligence symbols. Captured package methods have no local body. Add
a shim root when tests need local behavior:

```yaml
project:
  packageShims: ["pkg:test-support/package-shims/pkg"]
```

Shim classes compile under the package namespace and override no-body captured
methods.
