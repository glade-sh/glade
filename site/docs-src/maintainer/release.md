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
3. Tag the release.
4. Let GitHub Actions build archives for supported platforms.
5. Publish the product release manifest to the product download host.
6. Check a fresh install.
7. Check an update from the prior release.

## Plugin release

Cut first-party plugin archives from `glade-tools`. Keep plugin artifacts on the
plugin registry lane. Keep product release assets on the product download lane.

```bash
go run ./cmd/glade-plugin-compat manifest --json
scripts/build-plugin-archives.sh 0.1.0
```

## Docs release

The docs site is the single public docs surface. User docs live under
`/guide/...`. Maintainer docs live under `/maintainer/...`. Synchronized
`glade-tools` docs land under `/maintainer/tools/...`.
