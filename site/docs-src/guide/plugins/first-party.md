# First-party plugins

First-party plugins ship heavier Glade workflows without adding them to the
default install. They use the same executable runtime as third-party plugins.

Install these only when the base local runtime is not enough for the job.
`@glade/compat` is for compatibility fixture work, runtime capability reports,
and compatibility scans. `@glade/performance` is for advisory project scans.
`@glade/orgpackage` is for org-backed package artifact capture.

::: warning Registry preview
The default public plugin registry is not live yet. The install commands below
are the canonical coordinates once the registry or a custom registry serves
the archives. Until then, install from a direct archive or link a local plugin
executable for private plugin installs and plugin development.
:::

## `@glade/compat`

Registry install:

```bash
glade plugins install @glade/compat
```

The short alias `compat` resolves to `@glade/compat`.

Commands:

- `glade compat ...`
- `glade surface ...`
- `glade local-tests ...`
- `glade post-parity ...`
- `glade examples ...`
- `glade dashboard ...`
- `glade gaps ...`
- `glade stdlib ...`

The compat plugin owns compatibility fixtures, runtime capability reports, and compatibility
scanners.

## `@glade/performance`

Registry install:

```bash
glade plugins install @glade/performance
```

The short alias `performance` resolves to `@glade/performance`.

Commands:

- `glade performance scan --project .`

The performance plugin owns advisory Salesforce project performance scans. It
does not replace measured profiling. Use trace input when you need ranked
runtime cost.

## `@glade/orgpackage`

Registry install:

```bash
glade plugins install @glade/orgpackage
```

The short alias `orgpackage` resolves to `@glade/orgpackage`.

Commands:

- `glade orgpackage capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet`
- `glade package capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet`

The orgpackage plugin owns live Salesforce org capture for package artifacts.
The base `glade package capture` command is a bridge to that plugin when it is
installed or linked. Base Glade owns artifact loading, type checking, local
runtime boundaries, and optional source shims.
