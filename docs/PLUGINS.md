# Glade Plugins

Glade plugins are standalone executables. The product CLI discovers installed
plugins from `~/.glade/plugins/installed.json` and dispatches by the first
command word. Core Glade commands keep priority.

Plugins are process extensions, not Go shared libraries. Glade does not load
plugin code into its own process. It runs the plugin executable, passes the
original arguments, and streams stdout and stderr.

## First-Party Plugins

Glade ships a small product runtime. Heavier maintenance and advisory tools
ship as first-party plugins.

| Plugin | Alias | Command roots | Purpose |
| --- | --- | --- | --- |
| `@glade/compat` | `compat` | `compat`, `surface`, `local-tests`, `post-parity`, `examples`, `dashboard`, `gaps`, `stdlib` | Compatibility fixtures, local-test readiness reports, surface ledgers, docs inventory, generated support reports, and parity scanners. |
| `@glade/performance` | `performance` | `performance` | Advisory Salesforce project performance scans. Replaces the old base `glade inspect performance` path. |

The first-party plugin source lives in the sibling `glade-tools` workspace.
Users install and run plugins through `glade plugins`. The product repository
does not depend on `glade-tools` internals.

## Install And List

Install first-party plugins with canonical coordinates:

```bash
glade plugins install @glade/compat
glade plugins install @glade/performance
glade plugins list
```

The short aliases `compat` and `performance` resolve to `@glade/compat` and
`@glade/performance`.
Registry installs appear by canonical coordinate in `plugins list`,
`plugins which`, and `plugins doctor`. Linked development plugins without a
catalog coordinate use their manifest name.

Name installs use the configured plugin registry. The production default is
`https://plugins.glade.sh/index.json`; local development can set
`GLADE_PLUGIN_REGISTRY_URL` to a test or staging registry.

Search and inspect the marketplace:

```bash
glade plugins search quality
glade plugins info @acme/quality
glade plugins install @acme/quality@1.2.0
```

Install a local release archive:

```bash
glade plugins install ./glade-plugin-compat_0.1.0_darwin_arm64.tar.gz
```

Install a remote archive with a required digest:

```bash
glade plugins install https://github.com/acme/glade-plugin-quality/releases/download/v1.2.0/glade-plugin-quality_1.2.0_darwin_arm64.tar.gz --sha256 <hash>
```

Link a development binary without copying it:

```bash
cd path/to/glade-tools
go build -o /tmp/glade-plugin-compat ./cmd/glade-plugin-compat
glade plugins link --exec /tmp/glade-plugin-compat
```

Linked plugins are recorded in the same installed state, but Glade does not
copy or delete their executable.

## Run Plugin Commands

Once installed or linked, the plugin command root behaves like a Glade command:

```bash
glade compat local-tests --project . --parallel auto --json
glade surface refresh --docs "$GLADE_SALESFORCE_DOCS_SOURCE" --out tmp/surface
glade performance scan --project . --json
```

Glade does not parse plugin-specific flags. It streams stdout and stderr from
the executable.

Ask which plugin owns a command:

```bash
glade plugins which compat
glade plugins which performance
```

Check installed plugin executables and manifests:

```bash
glade plugins doctor
```

Remove a plugin by canonical coordinate:

```bash
glade plugins remove @glade/compat
```

## Lock And Restore

Write reproducible plugin state for CI:

```bash
glade plugins lock
```

Restore it:

```bash
glade plugins restore
```

`glade.plugins.lock.json` records canonical package coordinates, exact
versions, registry, platform, trust label, publisher, and archive digest.
Restore never installs `latest`; it checks registry digests before download
and verifies the locked manifest version and archive digest.

Linked plugins are skipped by default because local paths are not portable. Use
`glade plugins lock --include-linked` only for local development.

## Build Your Own Plugin

Every plugin executable must support:

```bash
glade-plugin-quality manifest --json
```

The manifest shape is:

```json
{
  "apiVersion": "glade.plugin.v1",
  "name": "quality",
  "version": "1.2.0",
  "summary": "Project quality checks.",
  "commands": [
    {
      "path": ["quality"],
      "summary": "Run project quality checks."
    }
  ],
  "minimumGladeVersion": "0.1.0",
  "source": "github.com/acme/glade-plugin-quality"
}
```

The host validates the manifest before install or link. The manifest `name`
and `version` become filesystem path segments, so they must be simple
path-safe tokens. For `@acme/quality`, the runtime manifest name remains
`quality`.

Command paths are advertised paths. Dispatch uses the first segment only. This
manifest entry:

```json
{ "path": ["quality", "scan"], "summary": "Scan project quality." }
```

lets users run:

```bash
glade quality scan --project .
```

The plugin process receives the same arguments: `quality scan --project .`.
Glade also exports:

```text
GLADE_PLUGIN_HOST=glade
GLADE_PLUGIN_API_VERSION=glade.plugin.v1
```

Archives must contain:

```text
plugin.json
checksums.txt
bin/glade-plugin-quality
```

`checksums.txt` contains SHA-256 rows for each archived file. Glade rejects
absolute paths, parent directory traversal, checksum mismatches, unsafe
manifest names, unsafe versions, and command roots that collide with core
commands.

Minimal `checksums.txt`:

```text
<sha256>  plugin.json
<sha256>  bin/glade-plugin-quality
```

## Marketplace Registry

Registry installs read a JSON index. `glade plugins install @glade/compat`
fetches the configured index, selects the current OS and architecture asset,
verifies the archive SHA-256, then installs the archive.

```json
{
  "version": 1,
  "plugins": [
    {
      "name": "@glade/compat",
      "aliases": ["compat"],
      "version": "0.1.0",
      "publisher": "glade",
      "trust": "first-party",
      "summary": "Compatibility fixtures, surface ledgers, and maintenance scanners.",
      "commands": ["compat", "surface", "local-tests", "post-parity", "examples", "dashboard", "gaps", "stdlib"],
      "docsURL": "https://glade.sh/guide/plugins/first-party",
      "sourceURL": "https://github.com/glade-sh/glade-tools",
      "minimumGladeVersion": "0.1.0",
      "assets": [
        {
          "os": "darwin",
          "arch": "arm64",
          "url": "https://plugins.glade.sh/v0.1.0/glade-plugin-compat_0.1.0_darwin_arm64.tar.gz",
          "sha256": "<archive sha256>"
        }
      ]
    }
  ]
}
```

The marketplace is a catalog. It does not replace archive installation.

## Troubleshooting

If a command is unknown, first check the install state:

```bash
glade plugins list
glade plugins which compat
```

If the plugin exists but dispatch fails, run:

```bash
glade plugins doctor
```

If a registry is not available, install a local archive or set
`GLADE_PLUGIN_REGISTRY_URL` to a reachable index.
