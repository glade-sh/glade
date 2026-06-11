# Project Configuration

Glade reads `glade.yml` from the project tree and layers it with
`sfdx-project.json` when present. The Glade file carries local runtime choices
that Salesforce project files do not.

## Create config

Create a starter file from an existing SFDX project:

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

## File shape

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

| Key | Meaning |
| --- | --- |
| `project.root` | Project root. Relative paths resolve from `glade.yml`. |
| `project.packageDirs` | Source package directories. Overrides `sfdx-project.json`. |
| `project.defaultNamespace` | Default namespace for package-local code. |
| `project.managedPackageDependencies` | Managed package source or artifact references. |
| `org.features` | Scratch-org style features for local runtime behavior. |

::: tip Next step
Run local tests without an org: [Local Testing](/guide/local-testing).
:::
