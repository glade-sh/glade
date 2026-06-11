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
| `org.features` | Scratch-org style features for local runtime behavior. |

`glade config validate` reports unsupported keys and parse errors before heavier
commands run.
