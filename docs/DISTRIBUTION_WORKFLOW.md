# Distribution Workflow

Use this runbook to cut and publish a Glade release with predictable output.

## 1. Preflight

From the repo root:

```bash
go test ./...
scripts/smoke.sh
```

If a command fails, stop and fix before tagging.
Use [DOGFOOD_CHECKLIST.md](DOGFOOD_CHECKLIST.md) for the final installed-binary
smoke pass on a real Salesforce project.

## 2. Tag and Push

```bash
git tag vX.Y.Z
git push <remote> vX.Y.Z
```

The `Release` GitHub Actions workflow builds parser-capable macOS and Linux
archives on matching CGO-enabled runners, verifies `glade doctor` reports
`Ready.`, and publishes `SHA256SUMS.txt`.

## 3. Verify Artifacts

Download one archive and checksums from the GitHub Release, then verify:

```bash
grep "  \./glade_VERSION_linux_amd64.tar.gz$" SHA256SUMS.txt | shasum -a 256 -c -
tar -xzf glade_VERSION_linux_amd64.tar.gz
./glade version
./glade doctor
```

Verify the public install script after Pages deploys:

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
glade doctor
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
- known gaps that changed
- upgrade notes
