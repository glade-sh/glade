# Plugin lock files and CI

`glade plugins lock` records exact plugin state for repeatable CI runs.

::: tip Default registry
The default public registry serves the three first-party packages at `0.2.9`.
Its URL is `https://plugins.glade.sh/index.json`.
Direct archives and local links remain available for offline, private, and
development use.
:::

```bash
glade plugins install @glade/performance
glade plugins install @glade/orgpackage
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
      "name": "@glade/performance",
      "version": "X.Y.Z",
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
