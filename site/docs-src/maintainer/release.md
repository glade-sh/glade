# Release Runbook

Use one command for the product proof.

```bash
scripts/release-check.sh
```

That command is the local gate. It checks Go, the site, the installer, the
release manifest, and smoke coverage. Add to the script when the release train
gains a new rail.

## Product release

1. Start from a clean branch.
2. Run `scripts/release-check.sh`.
3. Add the `vX.Y.Z` section to `docs/RELEASE_NOTES.md`.
4. Check the notes body: `scripts/release-notes.sh vX.Y.Z`.
5. Tag the release.
6. Let GitHub Actions build archives for supported platforms.
7. Check the GitHub release body for real blank lines, not literal `\n`.
8. Publish the product release manifest and archives to `downloads.glade.sh`.
9. Check a fresh install with temporary `GLADE_INSTALL_DIR` and `GLADE_HOME`.
10. Check a pinned install with `GLADE_VERSION=vX.Y.Z`.
11. Check an update from the prior release.

Use this shape when setting installer environment variables:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR="$tmp/bin" GLADE_HOME="$tmp/home" sh
```

Putting `GLADE_*` before `curl` sets it only for `curl`; the installer runs on
the right side of the pipe.

## Plugin release

Cut first-party plugin archives from `glade-tools`. Keep plugin artifacts on the
plugin registry lane. Keep product release assets on the product download lane.

```bash
go run ./cmd/glade-plugin-compat manifest --json
scripts/build-plugin-archives.sh 0.1.0
```

Run the tools release check before cutting a coordinated plugin tag:

```bash
(cd ../glade-tools && scripts/release-check.sh)
```

Publish plugin archives and `index.json` to `plugins.glade.sh`, then check:

```bash
curl -fsSL https://plugins.glade.sh/index.json
GLADE_HOME="$(mktemp -d)" GLADE_PLUGIN_REGISTRY_URL="https://plugins.glade.sh/index.json" glade plugins available
```

## Docs release

The docs site is the single public docs surface. User docs live under
`/guide/...`. Maintainer docs live under `/maintainer/...`. Synchronized
`glade-tools` docs land under `/maintainer/tools/...`.
