# Distribution Workflow

Use this runbook to cut and publish a Glade release with predictable output.

## 1. Preflight

From the repo root:

```bash
npm ci --prefix site
npm ci --prefix third_party/lwc
scripts/release-check.sh
```

Install both lockfile-pinned dependency sets before the long gate. The site
proof needs the VitePress toolchain. LWC integration tests need the checked
compiler/runtime toolchain; the release summary rejects their intentional
dependency-missing skips instead of treating them as proof.

The command runs `git diff --check`, the exact-once site release proof, the
checked local Go release lanes, and product smoke tests. The site proof writes
`site/.vitepress/release-check.json`. Go lanes default to serial execution for
bounded memory use and write raw events plus a validated
`package-summary.json` under `ci-artifacts/local-release/`. Package discovery
or lane ownership drift fails closed. If a command fails, stop and fix before
tagging.
Run the first project check from [INSTALL.md](INSTALL.md) on one real Salesforce DX
project before tagging.
When a release also ships first-party plugin archives, run the matching tools
gate before tagging:

```bash
(cd ../glade-tools && scripts/release-check.sh)
```

## 2. Freeze the Approved Pair, Tag and Push

Merge the release changes to `main` first. Freeze the product and Tools commits,
wait for the product's exact-SHA main-push `Required CI`, and run Salesforce
correctness for that exact pair. PR checks and manual CI runs do not qualify.
The read-only preflight reuses the hosted approvals; it does not rerun tests or
create tags. Keep its output with the release evidence.

```bash
VERSION=vX.Y.Z
GLADE_SHA='<lowercase-40-hex-approved-product-commit>'
TOOLS_SHA='<lowercase-40-hex-approved-tools-commit>'
bash scripts/release-preflight.sh "$GLADE_SHA" "$TOOLS_SHA"
git tag -a "$VERSION" "$GLADE_SHA" -m "Release $VERSION" -m "Glade-Tools-SHA: $TOOLS_SHA"
git push origin "refs/tags/$VERSION"
```

Do not move a pushed release tag or create an empty trigger commit to repair a
failed approval. Both actions change the identity under review. A missing
approval needs the existing exact candidate's workflow, not a new commit.
Before any upload, retry only the failed jobs after the missing approval passes.

The `Release` GitHub Actions workflow builds parser-capable macOS and Linux
archives on matching CGO-enabled runners, verifies `glade doctor` reports
`Ready.`, and publishes `SHA256SUMS.txt` plus release manifests for the
installer. If the GitHub release is absent, the workflow creates it as a draft
with the matching section from [RELEASE_NOTES.md](RELEASE_NOTES.md). It uploads
and verifies the complete asset set, then publishes the draft as its last step.
If it already exists, the workflow reuses its metadata, title, and body without
editing them. An existing asset with identical bytes is skipped; differing bytes
fail rather than replace published bytes. A published release is verified and
left unchanged. Regenerated archives are not assumed byte-identical: a
published rerun with differing bytes fails and leaves the release unchanged.
For interrupted draft publication, rerun only failed jobs so successful platform
artifacts are reused. The aggregate bundle is deterministic; if it was already
uploaded, its full file contents are checked and its original bytes are retained.
Do not select "Re-run all jobs" after partial uploads.
The notes script fails if the section is missing, empty, or
contains a literal `\n` sequence.

GitHub product and plugin release assets and notes are immutable on rerun.
Do not repair a published artifact in place. Cut a new version with corrected
assets and notes instead.

Tag `glade-tools` at the approved Tools SHA with the same version when plugin
archives are part of the release. Product CI does not check out Tools. The
product release always binds the explicit `Glade-Tools-SHA` trailer, never a
fallback to moving Tools `main`.

## 3. Verify Artifacts

Download one archive and checksums from the GitHub Release, then verify:

```bash
grep "  \./glade_VERSION_linux_amd64.tar.gz$" SHA256SUMS.txt | shasum -a 256 -c -
tar -xzf glade_VERSION_linux_amd64.tar.gz
./glade version
```

Check the GitHub release body before publishing wider:

```bash
gh release view vX.Y.Z --json body --jq .body
```

The body should use real blank lines. It should not contain the two characters
`\n`.

Verify the public install script after Pages deploys:

```bash
tmp="$(mktemp -d)"
curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR="$tmp/bin" GLADE_HOME="$tmp/home" sh
"$tmp/bin/glade" version
"$tmp/bin/glade" doctor
```

Check a pinned install too:

```bash
tmp="$(mktemp -d)"
curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z GLADE_INSTALL_DIR="$tmp/bin" GLADE_HOME="$tmp/home" sh
"$tmp/bin/glade" version
"$tmp/bin/glade" doctor
```

The release workflow uploads platform assets to the GitHub Release, downloads
them back in the publish job, and assembles these product download files:

| Path | Purpose |
| --- | --- |
| `https://downloads.glade.sh/index.json` | Channel index for installers and update checks. |
| `https://downloads.glade.sh/latest/release-manifest.json` | Latest stable product manifest. |
| `https://downloads.glade.sh/vX.Y.Z/release-manifest.json` | Pinned version manifest. |
| `https://downloads.glade.sh/vX.Y.Z/SHA256SUMS.txt` | Pinned checksums. |

`site/install.sh` checks the product download host first and falls back to the
GitHub release API while the static host is being filled.

Publish the product download files to the static host after downloading and
unpacking the `glade-release-artifacts-vX.Y.Z.tar.gz` GitHub Release asset:

```bash
tmp="$(mktemp -d)"
gh release download vX.Y.Z --pattern "glade-release-artifacts-vX.Y.Z.tar.gz" --dir "$tmp"
mkdir -p dist
tar -C dist -xzf "$tmp/glade-release-artifacts-vX.Y.Z.tar.gz"
```

Use a publication tool that performs a **conditional create** for every
`vX.Y.Z/**` object. Its write must carry `If-None-Match: *`, or it must use an
equivalent no-clobber primitive enforced by the publisher. Do not use a bare
`wrangler r2 object put` command for versioned objects: it can replace
published bytes.

The checked publisher uses Wrangler's authenticated remote R2 binding, atomic
`onlyIf` writes, SHA-256 validation, and complete readback. It does not deploy a
persistent Worker or install product dependencies. Point `WRANGLER_MODULE` to an
existing Wrangler package (4.37 or newer), and authenticate with `wrangler login`
if its existing session cannot refresh. Supply the approved product commit, not
the current working branch's HEAD.

```bash
export CLOUDFLARE_ACCOUNT_ID='<cloudflare-account-id>'
export WRANGLER_MODULE='/absolute/path/to/node_modules/wrangler'
node scripts/release-publish.mjs dist "$VERSION" "$GLADE_SHA"
```

The source must be the unpacked, checksum-verified GitHub bundle. Verify all
platform provenance and CycloneDX attestations before publication. The publisher
requires the original approval JSON in `dist/vX.Y.Z/`; for an older immutable
release, download each named approval artifact from its successful Release run
directly into that directory. Never synthesize or copy approvals from another
candidate. Conflicting versioned bytes, stale approvals, a newer live channel,
lost historical index entries, or changed channel ETags stop publication.

Publish the pinned manifest, checksums, and every platform archive under
`glade-downloads/vX.Y.Z/`. Read each versioned object back and compare its
bytes and SHA-256 value with the GitHub Release artifact before publishing any
pointer. The same conditional-create and readback rule applies to every
versioned plugin archive and checksum under `plugins.glade.sh/vX.Y.Z/`.

Only after every versioned object has been created and verified, update the
channel index and latest manifest. In other words, **mutable pointers last**:
`downloads.glade.sh/index.json`,
`downloads.glade.sh/latest/release-manifest.json`, and the plugin
`index.json` are the only mutable publication objects.

GitHub publication alone is not distribution completion. Keep the existing
versioned bytes unchanged. Publish the approval JSON files under the versioned
prefix along with the release manifest. Future releases also retain these files
as immutable GitHub assets; do not add assets to an already immutable release.
For an older release, recover its original approval JSON from the successful
Release workflow and retain it as create-only static companion evidence.

After channel publication, sync `site/release-manifest.json` using
`npm run release:sync --prefix site`, publish the site, and complete the default
and pinned install checks above. Both must report the intended version and a
ready doctor result before announcing distribution complete.

Run the completion check after the site deploys. It checks the public channel,
pinned manifest, site version, and both installer paths without modifying the
operator's installed Glade or home directory:

```bash
bash scripts/release-distribution-check.sh "$VERSION"
```

Then check:

```bash
curl -fsSL https://downloads.glade.sh/index.json
curl -fsSL https://downloads.glade.sh/latest/release-manifest.json
curl -fsSL https://downloads.glade.sh/vX.Y.Z/SHA256SUMS.txt
```

## 4. Update Homebrew Tap

In your tap repo:

1. Update `glade.rb` URL to the `vX.Y.Z` release archive.
2. Update `sha256` to the release checksum.
3. Commit and push.

Validate install:

```bash
brew update
brew install <tap>/glade
glade version
```

## 5. Publish Notes

Update [RELEASE_NOTES.md](RELEASE_NOTES.md):

- supported behavior changes
- unsupported-boundary changes
- upgrade notes
