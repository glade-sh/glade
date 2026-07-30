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
Run the first project check from [INSTALL.md](INSTALL.md) on one real SFDX
project before tagging.
When a release also ships first-party plugin archives, run the matching tools
gate before tagging:

```bash
(cd ../glade-tools && scripts/release-check.sh)
```

## 2. Tag and Push

```bash
git tag vX.Y.Z
git push <remote> vX.Y.Z
```

The `Release` GitHub Actions workflow builds parser-capable macOS and Linux
archives on matching CGO-enabled runners, verifies `glade doctor` reports
`Ready.`, and publishes `SHA256SUMS.txt` plus release manifests for the
installer. If the GitHub release is absent, the workflow creates it once with
the matching section from [RELEASE_NOTES.md](RELEASE_NOTES.md). If it already
exists, the workflow reuses its metadata, title, and body without editing them.
It may add a uniquely named asset; a duplicate asset name fails rather than
replace published bytes. The notes script fails if the section is missing,
empty, or contains a literal `\n` sequence.

GitHub product and plugin release assets and notes are immutable on rerun.
Do not repair a published artifact in place. Cut a new version with corrected
assets and notes instead.

Tag `glade-tools` with the same version when the plugin rail is part of the
release. Product CI falls back to `glade-tools` `main` when a matching tools tag
does not exist, so a product-only tag does not fail before the plugin rail is
cut.

## 3. Verify Artifacts

Download one archive and checksums from the GitHub Release, then verify:

```bash
grep "  \./glade_VERSION_linux_amd64.tar.gz$" SHA256SUMS.txt | shasum -a 256 -c -
tar -xzf glade_VERSION_linux_amd64.tar.gz
./glade version
./glade doctor
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
glade doctor
```

## 5. Publish Notes

Update [RELEASE_NOTES.md](RELEASE_NOTES.md):

- supported behavior changes
- unsupported-boundary changes
- upgrade notes
