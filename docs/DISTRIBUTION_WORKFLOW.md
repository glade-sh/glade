# Distribution Workflow

Use this runbook to cut and publish a Glade release with predictable output.

## 1. Preflight

From the repo root:

```bash
scripts/release-check.sh
```

If a command fails, stop and fix before tagging.
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

```bash
npx --yes wrangler r2 object put glade-downloads/index.json --remote --file dist/index.json
npx --yes wrangler r2 object put glade-downloads/latest/release-manifest.json --remote --file dist/latest/release-manifest.json
npx --yes wrangler r2 object put glade-downloads/vX.Y.Z/release-manifest.json --remote --file dist/release-manifest.json
npx --yes wrangler r2 object put glade-downloads/vX.Y.Z/SHA256SUMS.txt --remote --file dist/vX.Y.Z/SHA256SUMS.txt
npx --yes wrangler r2 object put glade-downloads/vX.Y.Z/glade_VERSION_darwin_arm64.tar.gz --remote --file dist/vX.Y.Z/glade_VERSION_darwin_arm64.tar.gz
```

Upload each platform archive under `glade-downloads/vX.Y.Z/`. Then check:

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
