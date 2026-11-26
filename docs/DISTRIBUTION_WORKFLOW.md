# Distribution Workflow

Use this runbook to cut and publish a Glade release with predictable output.

The release train has four steps: the local gate on a clean candidate, the
hosted checks on the exact `main` commit, the tag, and distribution. Run each
gate once per exact tree. Any new commit needs its own hosted checks.

## 1. Preflight

Prepare the matching version section in [RELEASE_NOTES.md](RELEASE_NOTES.md)
before freezing the release commit. Run from a clean checkout of the candidate:

```bash
npm ci --prefix site
npm ci --prefix third_party/lwc
GLADE_TOOLS_DIR=/path/to/the capture tools
```

The distribution build rejects tracked or untracked changes so that embedded Go
VCS metadata identifies a clean candidate. Commit intended release changes
before this gate; use a separate checkout when other work is in progress.

Install both lockfile-pinned dependency sets before the long gate. The site
proof needs the VitePress toolchain. LWC integration tests need the checked
compiler/runtime toolchain; the release summary rejects their intentional
dependency-missing skips instead of treating them as proof.

Set `GLADE_TOOLS_DIR` to the intended tools checkout (it defaults to the sibling
`../glade-tools`). The first build checks its `glade-tools` command against this
candidate using the tools dependency metadata in read-only mode. Resolve any
dependency or build failures before running the release gates. Neither checkout
is modified.

The command runs `git diff --check`, the tools lockstep build, the exact-once site release proof, the
checked local Go release lanes, and a distribution build with product smoke
tests. The site proof writes `site/.vitepress/release-check.json`. Go lanes
run one at a time to bound memory use (the script defaults `GOMAXPROCS` to 2)
and write raw events plus a validated `package-summary.json` under
`ci-artifacts/local-release/`. Package discovery
or lane ownership drift fails closed. If a command fails, stop and fix before
tagging.

The local gate does not run `check:built`, rendered-site browser tests, or preview
smoke. Follow [site/README.md](../site/README.md#checks-and-release-proof) for
those checks; the CI site job also requires them.

Run the first project check from [INSTALL.md](INSTALL.md) on one real Salesforce DX
project before tagging.
When a release also ships first-party plugin archives, run the matching tools
gate before tagging:

```bash
(cd ../glade-tools && scripts/release-check.sh)
```

Before opening or updating a release PR, run the Required CI local gate on the
clean candidate tree:

```bash
scripts/ci-local-all
```

Use Go 1.27.4 with CGO, Node 22 and npm, `actionlint`, the CodeQL CLI and Go
query packs, and Playwright Chromium for the site and LWC runtime. The runner
installs lockfile-pinned npm dependencies with `npm ci`; install the matching
Playwright Chromium beforehand using the repository's
`scripts/ci-install-playwright.sh`. It reports each lane and its log path and
exits nonzero for a failure or `SKIP-ENV`. A missing Node, browser, Go, actionlint
or CodeQL toolchain is not a pass. By default shards run serially; `--jobs N`
allows concurrent shard workers on a host with sufficient capacity. Use
`--log-dir PATH` to choose a new evidence directory. CodeQL uses the workflow's
fast query configuration on the whole local tree when PR diff information is
unavailable; it does not upload GitHub results. Local results do not replace
the required hosted checks or release attestations.

## 2. Freeze the Approved Pair, Tag and Push

Merge the release changes to `main` first. Freeze the product and Tools commits,
wait for the product's exact-SHA main-push `Required CI`, and run Salesforce
correctness for that exact pair. PR checks and manual CI runs do not qualify.
`scripts/verify-required-ci.sh` finds the Required CI job; `scripts/verify-salesforce-check.sh`
finds the Salesforce check. The read-only preflight runs both. It reuses the
hosted approvals; it does not rerun tests or create tags. Keep its output with
the release evidence.

Before tagging, also read the operator checks for the same commit: the
`Browser`, `Race` and `Security` workflows. Record a failure or a known issue
in [RELEASE_NOTES.md](RELEASE_NOTES.md). Review the CodeQL waivers in
[CODEQL.md](CODEQL.md#waiver-review-2026-10-22) and record the
date. Both CodeQL profiles succeed with a warning when their analysis budget
expires. The release workflow runs the full scan on the tagged SHA. Findings
and errors block release preparation. A timeout adds a warning to the draft
release notes.

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
archives on matching CGO-enabled runners. It checks `glade doctor --json` for
`"parserOK": true`, runs an extracted-binary parser smoke, and publishes
`SHA256SUMS.txt` plus release manifests for the installer. If the GitHub release
is absent, the workflow creates it as a draft with the matching section from
[RELEASE_NOTES.md](RELEASE_NOTES.md). It uploads
and verifies the complete asset set, then publishes the draft as its last step.
On a rerun, the workflow updates the CodeQL timeout warning on a draft if the
scan status changes and preserves the rest of the notes. The workflow skips
existing assets with identical bytes and fails if regenerated bytes differ.
It verifies published releases and leaves them unchanged.
For interrupted draft publication, rerun only failed jobs so successful platform
artifacts are reused. The aggregate bundle is deterministic; if it was already
uploaded, its full file contents are checked and its original bytes are retained.
Do not select "Re-run all jobs" after partial uploads.
The notes script fails if the section is missing, empty, or
contains a literal `\n` sequence.

Published GitHub product and plugin release assets and notes are immutable on rerun.
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
./glade doctor --json
```

Confirm `parserOK` is `true`. A full `Ready.` doctor result also depends on the
project setup; the distribution completion check below initializes an isolated
project before checking it.

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
"$tmp/bin/glade" doctor --json
```

Check a pinned install too:

```bash
tmp="$(mktemp -d)"
curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z GLADE_INSTALL_DIR="$tmp/bin" GLADE_HOME="$tmp/home" sh
"$tmp/bin/glade" version
"$tmp/bin/glade" doctor --json
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

### Static publication in the Release workflow

The Release workflow publishes the static download host itself. Its
`distribute` job runs after `publish`, so the GitHub Release is published
first and the static host second. The job:

1. stops with an error naming the repository secret if `CLOUDFLARE_API_TOKEN`
   is not set (the account comes from the `CLOUDFLARE_ACCOUNT_ID` variable);
2. requires the annotated tag, its commit, and a published (not draft) GitHub
   Release;
3. downloads `glade-release-artifacts-vX.Y.Z.tar.gz` and the top-level release
   assets with `gh release download`, unpacks the bundle into `dist`, compares
   the bundled manifest, checksums, index and approvals with the release
   assets, checks every archive with `sha256sum --check`, and verifies each
   archive's attestation for the release workflow, tag and product SHA;
4. installs the lockfile-pinned Wrangler in `scripts/package.json` with
   `npm ci --prefix scripts --ignore-scripts`;
5. runs `node scripts/release-publish.mjs dist "$VERSION" "$GLADE_SHA"`; the
   publisher repeats its own authority and attestation checks before any write;
6. reads the public objects back: `latest/release-manifest.json` must name the
   version and match the bundle, `vX.Y.Z/release-manifest.json` must match the
   bundle bytes, `index.json` must list the version as latest, and every
   archive URL must answer;
7. POSTs to the `glade-site` Cloudflare Pages deploy hook in the
   `CF_PAGES_DEPLOY_HOOK` secret, so Pages rebuilds `main` against the moved
   channel. The hook URL is never printed. If the secret is missing, this step
   fails with an error naming it; the downloads are already published by then;
8. polls `https://glade.sh/site-build.json` every 20 seconds for up to
   15 minutes until `releaseVersion` equals the version, then reads
   `siteCommit` and requires it to be reachable from `main`;
9. checks out that deployed commit, runs `npm ci --ignore-scripts` for its
   `site/` and installs Chromium, then runs
   `npm run smoke:postdeploy -- --base-url https://glade.sh --expected-commit <siteCommit>`.

The Pages Git integration also rebuilds the site when a release merge lands,
but that build runs before the tag moves `latest`, so it shows the previous
version. The deploy hook rebuild after publication corrects it. The expected
commit is the `main` commit Pages built, not the tag commit.

The job never changes the GitHub Release. Its token has only `actions`,
`attestations`, `checks` and `contents` read access.

To publish only the static host for a tag whose GitHub Release is already
published, dispatch the workflow from `main` in distribute-only mode. Pass the
tag's `Glade-Tools-SHA` trailer as `glade_tools_sha`; the job checks it against
the tag. No archive is rebuilt and no other release job runs. A dispatch runs
the dispatched ref's own workflow and publisher, so distribute-only mode runs
only from `refs/heads/main`: from any other ref the `distribute-ref` job fails
and `distribute`, which holds the Cloudflare secret, does not start.

```bash
tools_sha="$(scripts/verify-salesforce-check.sh --tag-tools-sha vX.Y.Z)"
gh workflow run release.yml --repo glade-sh/glade --ref main \
  -f distribute_only=true -f version=vX.Y.Z -f glade_tools_sha="$tools_sha"
```

Use the same dispatch to retry a failed `distribute` job after fixing its
cause, for example after adding the missing secret. Rerunning the failed job
from the tag's own run also works. Already published versioned objects with
identical bytes are skipped; different bytes stop publication.

### Manual fallback

Use the manual command only when the workflow cannot run. Download and unpack
the `glade-release-artifacts-vX.Y.Z.tar.gz` GitHub Release asset:

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

The checked publisher uses Wrangler's authenticated, temporary remote preview R2 binding, atomic
`onlyIf` writes, SHA-256 validation, and complete readback. It does not deploy a
persistent Worker or install product dependencies. Point `WRANGLER_MODULE` to an
existing Wrangler package (4.37 or newer; `npm ci --prefix scripts --ignore-scripts`
installs the pinned one at `scripts/node_modules/wrangler`), and authenticate with
`wrangler login` if its existing session cannot refresh. Supply the approved product commit, not
the current working branch's HEAD.

```bash
export CLOUDFLARE_ACCOUNT_ID='<cloudflare-account-id>'
export WRANGLER_MODULE='/absolute/path/to/node_modules/wrangler'
node scripts/release-publish.mjs dist "$VERSION" "$GLADE_SHA"
```

The source must be the unpacked, checksum-verified GitHub bundle. Before any
bucket write, the publisher independently checks the CI and Salesforce
authority against GitHub and verifies each staged archive's provenance and
CycloneDX attestation against the release workflow, tag and product SHA. It
also compares each staged SBOM with the attested predicate. This requires an
authenticated `gh` with access to the authority and attestations. Manual policy
review remains required. The publisher requires the original approval JSON in
`dist/vX.Y.Z/`; for an older immutable
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

After channel publication, publish the site. Its build writes
`site/release-manifest.json` from the live
`https://downloads.glade.sh/latest/release-manifest.json`; the file is not
checked in. The `distribute` job redeploys the site through the Pages deploy
hook and runs the postdeploy smoke. Only if that step cannot run, retry the
Cloudflare Pages deployment of `main` by hand, or use the manual
`wrangler pages deploy` in `site/README.md`, then run the postdeploy smoke
yourself. Then complete the default and pinned install checks above. Both must report the intended version and a
working parser. The completion check below must also report ready projects
before announcing distribution complete.

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

## 5. Announce the Release

Use the notes already frozen in [RELEASE_NOTES.md](RELEASE_NOTES.md) and published
by the Release workflow. Announce only after distribution verification passes.
The notes should describe:

- supported behavior changes
- unsupported-boundary changes
- upgrade notes
