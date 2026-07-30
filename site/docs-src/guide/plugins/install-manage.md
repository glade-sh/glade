# Install and manage plugins

Use canonical coordinates for marketplace installs.

::: tip Default registry
The default public registry is `https://plugins.glade.sh/index.json`. It
serves `@glade/compat`, `@glade/orgpackage`, and `@glade/performance` at
their current published versions. Use `glade plugins available` to inspect
them. Direct archives and local links remain available for offline, private,
and development use.
:::

```bash
glade plugins available
glade plugins install @glade/performance
glade plugins install @glade/orgpackage
glade plugins install @acme/quality@1.2.0
```

The short aliases `performance` and `orgpackage` resolve to
`@glade/performance` and `@glade/orgpackage`.
Registry installs appear by canonical coordinate in `plugins list`,
`plugins which`, and `plugins doctor`. Linked development plugins without a
catalog coordinate use their manifest name.

## Find and inspect

```bash
glade plugins available
glade plugins search
glade plugins search quality
glade plugins info @acme/quality
glade plugins list
glade plugins which performance
glade plugins which orgpackage
glade plugins doctor
```

`available` lists every plugin in the configured marketplace. Bare `search`
lists the same catalog; add a query to filter it. `which` reports the
installed plugin that owns a command root. `doctor` checks installed
executables and manifests.

## Remove and restore

```bash
glade plugins remove @glade/performance
glade plugins lock
glade plugins restore
```

Lock files record canonical names, exact versions, platform, registry, and
archive digest.

## Local development

```bash
go build -o ./glade-plugin-quality ./cmd/glade-plugin-quality
glade plugins link --exec ./glade-plugin-quality
```

Linked plugins are recorded in installed state. Glade does not copy the binary
and does not delete it during removal.
