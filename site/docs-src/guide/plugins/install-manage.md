# Install And Manage Plugins

Use canonical coordinates for marketplace installs.

```bash
glade plugins install @glade/compat
glade plugins install @glade/performance
glade plugins install @acme/quality@1.2.0
```

The short aliases `compat` and `performance` resolve to `@glade/compat` and
`@glade/performance`.
Registry installs appear by canonical coordinate in `plugins list`,
`plugins which`, and `plugins doctor`. Linked development plugins without a
catalog coordinate use their manifest name.

## Find and inspect

```bash
glade plugins search quality
glade plugins info @acme/quality
glade plugins list
glade plugins which compat
glade plugins doctor
```

`which` reports the installed plugin that owns a command root. `doctor` checks
installed executables and manifests.

## Remove and restore

```bash
glade plugins remove @glade/compat
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
