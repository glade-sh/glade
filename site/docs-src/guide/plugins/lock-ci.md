# Plugin Lock Files And CI

`glade plugins lock` records exact plugin state for repeatable CI runs.

::: warning Registry preview
The default public plugin registry is not live yet. Coordinate installs and
lock restores need a live registry or custom registry. Direct archives and
local links remain available for maintainer workflows.
:::

```bash
# Requires a live plugin registry or configured custom registry.
glade plugins install @glade/compat
glade plugins lock
glade plugins restore
```

## Lock file

`glade.plugins.lock.json` stores canonical package coordinates and exact asset
identity:

```json
{
  "version": 1,
  "plugins": [
    {
      "name": "@glade/compat",
      "version": "0.1.0",
      "registry": "https://plugins.glade.sh/index.json",
      "os": "darwin",
      "arch": "arm64",
      "sha256": "<archive sha256>",
      "trust": "first-party",
      "publisher": "glade"
    }
  ]
}
```

`restore` never installs `latest`. It checks registry digests before download
and verifies the locked manifest version and archive digest.

## CI

```bash
glade plugins restore
glade plugins doctor
glade test --project . --json
```

Community or unlisted installs in CI require `--yes` or a lock file. Linked
plugins are skipped by default because local paths are not portable.
