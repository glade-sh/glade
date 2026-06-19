# Plugins

Glade plugins are standalone executables installed and run through
`glade plugins`. The default Glade install stays small. First-party and linked
plugins add heavier workflows when a project needs them. Registry-backed
installs are preview until a registry, archive URL, or local plugin is
configured.

Most users can ignore plugin author docs on first run. Install first-party
plugins only when you need compatibility fixtures, capability reports,
compatibility dashboards, or project-specific checks.

::: warning Registry preview
Registry commands below need a configured registry, a direct archive URL, or a
locally linked plugin.
Base Glade install and local Apex workflows do not require plugins.
:::

## First-party plugins

- `@glade/compat` - compatibility fixtures, runtime capability reports, and compatibility
  scanners.
- `@glade/performance` - advisory Salesforce performance scans.

```bash
# Requires a live plugin registry or configured custom registry.
glade plugins available
glade plugins install @glade/compat
glade plugins install @glade/performance
```

The short aliases `compat` and `performance` resolve to `@glade/compat` and
`@glade/performance`.

## Registry preview plugins

```bash
# Requires a live plugin registry or configured custom registry.
glade plugins available
glade plugins search
glade plugins search quality
glade plugins info @acme/quality
glade plugins install @acme/quality
```

Registry catalogs are configured. Third-party publishers can also use a custom
registry or a direct archive URL. `available` and bare `search` list the
installable catalog before you know a plugin name.

Install a remote archive only with a pinned digest:

```bash
glade plugins install https://github.com/acme/glade-plugin-quality/releases/download/v1.2.0/glade-plugin-quality_1.2.0_darwin_arm64.tar.gz --sha256 <hash>
```

For local plugin development, link an executable from disk:

```bash
glade plugins link --exec ./glade-plugin-quality
glade plugins lock --include-linked
```

Use `--include-linked` only for local development lock files. Team and CI lock
files should restore registry or archive installs.

## Build and publish

Plugin authors ship an executable that supports `manifest --json` and package
it as a checksum-verified archive. The runtime manifest stays path-safe. Scoped
package names live in the configured registry catalog.

```bash
glade plugins link --exec ./glade-plugin-quality
```

See the build, manifest, publish, and lock-file guides for the full contract.
