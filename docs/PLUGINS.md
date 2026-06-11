# Glade Plugins

Glade plugins are standalone executables. The product CLI discovers them from
`~/.glade/plugins/installed.json` and dispatches by the first command word.
Core Glade commands keep priority. A plugin cannot claim `test`, `schema`,
`inspect`, `plugins`, or another core command root.

Plugins are process extensions, not Go shared libraries. Glade does not load
plugin code into its own process. It runs the plugin executable, passes the
original arguments, and streams stdout and stderr.

## First-Party Plugins

Glade ships a small product runtime. Heavier maintenance and advisory tools
ship as first-party plugins.

| Plugin | Command roots | Purpose |
| --- | --- | --- |
| `compat` | `compat`, `surface`, `local-tests`, `post-parity`, `examples`, `dashboard`, `gaps`, `stdlib` | Compatibility fixtures, local-test readiness reports, surface ledgers, docs inventory, generated support reports, and gap scanners. |
| `performance` | `performance` | Advisory Salesforce project performance scans. Replaces the old base `glade inspect performance` path. |

The first-party plugin source lives in the sibling `glade-tools` workspace.
Users install and run it through `glade plugins`.

## Install And List

Install a first-party plugin by name:

```bash
glade plugins install compat
glade plugins install performance
glade plugins list
```

Name installs use the configured plugin registry. The production default is
`https://plugins.glade.sh/index.json`; local development can set
`GLADE_PLUGIN_REGISTRY_URL` to a test or staging registry.

Install a local release archive:

```bash
glade plugins install ./glade-plugin-compat_0.1.0_darwin_arm64.tar.gz
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

Remove a plugin:

```bash
glade plugins remove compat
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

Linked plugins are skipped by default because local paths are not portable. Use
`glade plugins lock --include-linked` only for local development.

## Build Your Own Plugin

Every plugin executable must support:

```bash
glade-plugin-compat manifest --json
```

The manifest shape is:

```json
{
  "apiVersion": "glade.plugin.v1",
  "name": "compat",
  "version": "0.1.0",
  "summary": "Compatibility fixtures, surface ledgers, and maintenance scanners.",
  "commands": [
    {
      "path": ["compat"],
      "summary": "Run compatibility fixture and report commands."
    },
    {
      "path": ["surface"],
      "summary": "Refresh and inspect the Salesforce surface ledger."
    },
    {
      "path": ["local-tests"],
      "summary": "Report local Apex test execution readiness."
    },
    {
      "path": ["post-parity"],
      "summary": "Scan a project for unsupported surfaces."
    },
    {
      "path": ["examples"],
      "summary": "Scan example projects and report support status."
    },
    {
      "path": ["dashboard"],
      "summary": "Generate compatibility dashboard output."
    },
    {
      "path": ["gaps"],
      "summary": "Generate known-gaps output."
    },
    {
      "path": ["stdlib"],
      "summary": "Generate standard-library coverage output."
    }
  ],
  "minimumGladeVersion": "0.1.0",
  "source": "github.com/glade-sh/glade/tools"
}
```

The host validates the manifest before install or link. The manifest name and
version become filesystem path segments, so they must be simple tokens:
letters, digits, `.`, `_`, and `-`.

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
bin/glade-plugin-<name>
```

`checksums.txt` contains SHA-256 rows for each archived file. Glade rejects
absolute paths, parent directory traversal, checksum mismatches, and command
roots that collide with core commands.

Minimal archive layout for a plugin named `quality`:

```text
plugin.json
checksums.txt
bin/glade-plugin-quality
```

Minimal `checksums.txt`:

```text
<sha256>  plugin.json
<sha256>  bin/glade-plugin-quality
```

## Registry Shape

Registry installs read a JSON index. `glade plugins install compat` fetches the
configured index, selects the current OS and architecture asset, verifies the
archive SHA-256, then installs the archive.

```json
{
  "version": 1,
  "plugins": [
    {
      "name": "compat",
      "version": "0.1.0",
      "assets": [
        {
          "os": "darwin",
          "arch": "arm64",
          "url": "https://example.com/glade-plugin-compat_0.1.0_darwin_arm64.tar.gz",
          "sha256": "<archive sha256>"
        }
      ]
    }
  ]
}
```

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
