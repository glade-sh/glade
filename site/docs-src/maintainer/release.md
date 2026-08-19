# Release Runbook

Use one command for the product proof.

```bash
npm ci --prefix site
npm ci --prefix third_party/lwc
scripts/release-check.sh
```

Install both lockfile-pinned dependency sets before the long gate. The site
proof needs the VitePress toolchain. LWC integration tests need the checked
compiler/runtime toolchain; the release summary rejects dependency-missing
skips instead of accepting incomplete proof.

That command is the local gate. It checks Go, the site, the installer, the
release manifest, and smoke coverage. Add to the script when the release train
gains a new rail.

The gate runs, in order:

1. `git diff --check`
2. `npm run release:check --prefix site`
3. `scripts/ci-go-test.sh local-release`
4. `scripts/smoke.sh`

The site command runs `verify`, `test:unit`, and `build:site` exactly once,
rejects source changes during the run, and writes
`site/.vitepress/release-check.json`. For a fast site-only loop, use
`npm test`; when changing the release orchestrator, use
`npm run test:release`; use `npm run release:check` for exact site release
proof.

The Go phase checks one authoritative package inventory and writes raw events
plus a validated `package-summary.json` for every lane under
`ci-artifacts/local-release/`. Automatic execution stays serial for predictable
memory use. Only an explicit `LOCAL_GO_TEST_JOBS` value greater than one can
overlap the final independent lanes.

Measure the unchanged gate when comparing resource use:

```bash
scripts/perf-release-check.sh \
  --label release-check-warm \
  --cache-mode warm \
  --output /tmp/glade-release-check-warm \
  -- scripts/release-check.sh
```

The wrapper writes `release-check.json` with timing, maximum RSS, file I/O,
toolchain, commit, command, and caller-declared cache mode. It neither clears
nor primes caches and is not a gate. Keep measurement output outside the
repository. `scripts/release-check.sh` remains the correctness authority.

## Product release

1. Start from a clean branch.
2. Run `scripts/release-check.sh`.
3. Add the `vX.Y.Z` section to `docs/RELEASE_NOTES.md`.
4. Check the notes body: `scripts/release-notes.sh vX.Y.Z`.
5. Tag the release.
6. Let GitHub Actions build archives for supported platforms.
7. Check the GitHub release body for real blank lines, not literal `\n`.
8. Publish each `vX.Y.Z/**` product object with a conditional create
   (`If-None-Match: *` or an equivalent publisher-enforced no-clobber write).
   Read it back and verify its bytes and SHA-256 against the GitHub Release.
9. Update mutable pointers last: `index.json` and `latest/release-manifest.json`
   move only after every versioned product object verifies.
10. Check a fresh install with temporary `GLADE_INSTALL_DIR` and `GLADE_HOME`.
11. Check a pinned install with `GLADE_VERSION=vX.Y.Z`.
12. Check an update from the prior release.

GitHub product and plugin release assets and notes are immutable on rerun.
If an artifact or note is wrong after publication, cut a corrected new version;
do not overwrite the release or an object under its versioned prefix.

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
scripts/build-plugin-archives.sh X.Y.Z
```

Run the tools release check before cutting a coordinated plugin tag:

```bash
(cd ../glade-tools && scripts/release-check.sh)
```

Create every versioned plugin archive and checksum with the same conditional
create/no-clobber rule, then read it back and verify its bytes and SHA-256.
Update the mutable plugin `index.json` last, then check:

```bash
curl -fsSL https://plugins.glade.sh/index.json
GLADE_HOME="$(mktemp -d)" GLADE_PLUGIN_REGISTRY_URL="https://plugins.glade.sh/index.json" glade plugins available
```

## Docs release

The docs site is the single public docs surface. User docs live under
`/guide/...`. Maintainer docs live under `/maintainer/...`.

After the stable download pointer moves, sync the checked site input on the
docs change and review it before merge:

```bash
cd site
npm run release:sync
npm run release:sync:check
git diff -- release-manifest.json
```

Cloudflare Pages project `glade-sh` publishes from product `main`. After the
merge, require the Git integration deployment or deploy the clean, exact main
build:

```bash
cd site
npm ci
npm run build
npx --yes wrangler pages deploy .vitepress/dist --project-name glade-sh --branch main \
  --commit-hash "$(git -C .. rev-parse HEAD)" --commit-dirty=false
```

The deployment is not accepted until the blocking post-deploy reconciliation
passes. It checks the exact commit, stable manifest, GitHub latest release,
checksums, advertised archives, installer, search, canonical routes, redirects,
registry, sitemap, and malformed copy tokens:

```bash
expected_sha="$(git -C .. rev-parse HEAD)"
npm run smoke:postdeploy -- --base-url https://glade.sh --expected-commit "$expected_sha"
```
