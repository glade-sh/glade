# Plugins

Glade plugins are standalone executables installed and run through
`glade plugins`. The default Glade install stays small. First-party and
marketplace plugins add heavier workflows when a project needs them.

Most users can ignore plugin author docs on first run. Install first-party
plugins only when you need compatibility fixtures, support reports, or advisory
performance scans.

::: warning Registry preview
The default public plugin registry is not live yet. Registry commands below
need a configured registry, a direct archive URL, or a locally linked plugin.
Base Glade install and local Apex workflows do not require plugins.
:::

## First-party plugins

- `@glade/compat` - compatibility fixtures, surface ledgers, support reports,
  and parity scanners.
- `@glade/performance` - advisory Salesforce performance scans.

```bash
# Requires a live plugin registry or configured custom registry.
glade plugins available
glade plugins install @glade/compat
glade plugins install @glade/performance
```

The short aliases `compat` and `performance` resolve to `@glade/compat` and
`@glade/performance`.

## Marketplace plugins

```bash
# Requires a live plugin registry or configured custom registry.
glade plugins available
glade plugins search
glade plugins search quality
glade plugins info @acme/quality
glade plugins install @acme/quality
```

The default marketplace is curated. Third-party publishers can also use a
custom registry or a direct archive URL. `available` and bare `search` list
the installable catalog before you know a plugin name.

For local plugin development, link an executable from disk:

```bash
glade plugins link --exec ./glade-plugin-quality
```

## Build and publish

Plugin authors ship an executable that supports `manifest --json` and package
it as a checksum-verified archive. The runtime manifest stays path-safe. Scoped
package names live in the marketplace catalog.

```bash
glade plugins link --exec ./glade-plugin-quality
```

See the build, manifest, publish, and lock-file guides for the full contract.
