# Build A Plugin

A Glade plugin is an executable process. Glade does not load plugin code into
the product binary. The executable must support `manifest --json`.

```bash
./glade-plugin-quality manifest --json
glade plugins link --exec ./glade-plugin-quality
```

## Runtime contract

When Glade dispatches a plugin command, it passes the original arguments and
streams stdout and stderr. It also sets:

```text
GLADE_PLUGIN_HOST=glade
GLADE_PLUGIN_API_VERSION=glade.plugin.v1
```

The installed manifest is authoritative for runtime dispatch. Marketplace
metadata helps users choose a plugin and helps Glade validate catalog installs.

## Archive layout

Package release archives with this layout:

```text
plugin.json
checksums.txt
bin/glade-plugin-quality
```

`checksums.txt` carries SHA-256 rows for `plugin.json` and the executable.
Glade rejects absolute paths, parent directory traversal, checksum mismatches,
unsafe manifest names, unsafe versions, and command roots that collide with
built-in Glade commands.

## Naming

Keep `plugin.json` names path-safe:

```json
{
  "apiVersion": "glade.plugin.v1",
  "name": "quality",
  "version": "1.2.0"
}
```

The marketplace coordinate can be `@acme/quality`. The runtime manifest name
remains `quality`.
