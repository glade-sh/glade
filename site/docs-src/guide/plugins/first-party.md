# First-Party Plugins

First-party plugins ship heavier Glade workflows without adding them to the
default install. They use the same executable runtime as third-party plugins.

## `@glade/compat`

Install:

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

The compat plugin owns compatibility fixtures, local-test readiness reports,
surface ledgers, generated support reports, and parity scanners.

## `@glade/performance`

Install:

```bash
glade plugins install @glade/performance
```

The short alias `performance` resolves to `@glade/performance`.

Commands:

- `glade performance scan --project .`

The performance plugin owns advisory Salesforce project performance scans. It
does not replace measured profiling. Use trace input when you need ranked
runtime cost.
