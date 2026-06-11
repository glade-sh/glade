# Plugins

Glade plugins are standalone executables. Glade discovers installed plugins
from `~/.glade/plugins/installed.json`, matches the first command word, then
runs the plugin process with the original arguments. Core commands keep
priority. A plugin cannot override `test`, `schema`, `inspect`, `plugins`, or
another built-in command.

## First-party plugins

| Plugin | Command roots | Purpose |
| --- | --- | --- |
| `compat` | `compat`, `surface`, `local-tests`, `post-parity`, `examples`, `dashboard`, `gaps`, `stdlib` | Compatibility fixtures, local-test readiness, surface ledgers, docs inventory, generated support reports, and gap scans. |
| `performance` | `performance` | Advisory Salesforce performance scans. |

Install them by name:

```bash
glade plugins install compat
glade plugins install performance
glade plugins list
```

Run plugin commands like normal Glade commands:

```bash
glade compat local-tests --project . --parallel auto --json
glade surface refresh --docs "$GLADE_SALESFORCE_DOCS_SOURCE" --out tmp/surface
glade performance scan --project . --json
```

Name installs use the configured registry. The production default is
`https://plugins.glade.sh/index.json`. Local development can set
`GLADE_PLUGIN_REGISTRY_URL`.

## Local archives and links

Install a release archive:

```bash
glade plugins install ./glade-plugin-compat_0.1.0_darwin_arm64.tar.gz
```

Link a development binary:

```bash
cd path/to/glade-tools
go build -o /tmp/glade-plugin-compat ./cmd/glade-plugin-compat
glade plugins link --exec /tmp/glade-plugin-compat
```

Linked plugins are recorded in the same state file, but Glade does not copy or
delete the executable.

## Lock and restore

Write plugin state for CI:

```bash
glade plugins lock
```

Restore it:

```bash
glade plugins restore
```

Linked plugins are skipped unless you pass `--include-linked`.

## Build a plugin

Every plugin executable must support:

```bash
glade-plugin-example manifest --json
```

The manifest declares the protocol version, plugin name, version, summary, and
command paths:

```json
{
  "apiVersion": "glade.plugin.v1",
  "name": "quality",
  "version": "0.1.0",
  "summary": "Project quality checks.",
  "commands": [
    {
      "path": ["quality", "scan"],
      "summary": "Scan project quality."
    }
  ],
  "minimumGladeVersion": "0.1.0",
  "source": "github.com/example/glade-plugin-quality"
}
```

A user can then run:

```bash
glade quality scan --project .
```

The plugin receives `quality scan --project .`. Glade streams stdout and
stderr, and it sets:

```text
GLADE_PLUGIN_HOST=glade
GLADE_PLUGIN_API_VERSION=glade.plugin.v1
```

Archives contain:

```text
plugin.json
checksums.txt
bin/glade-plugin-<name>
```

`checksums.txt` carries SHA-256 rows for `plugin.json` and the executable.
Glade rejects path traversal, checksum mismatches, unsafe names or versions,
and command roots that collide with built-in commands.

## Troubleshooting

```bash
glade plugins list
glade plugins which compat
glade plugins doctor
```

If the registry is unavailable, install a local archive or point
`GLADE_PLUGIN_REGISTRY_URL` at a reachable index.
