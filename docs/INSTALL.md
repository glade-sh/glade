# Install oaer

`oaer` is distributed as a single binary. Release artifacts are built by the
`Release` GitHub Actions workflow for:

- macOS amd64 and arm64
- Linux amd64 and arm64
- Windows amd64

Each release build also publishes `SHA256SUMS.txt`.

## Manual Install

Download the archive for your platform from the release artifacts, verify the
checksum, and place the binary on your `PATH`.

```bash
shasum -a 256 -c SHA256SUMS.txt
tar -xzf oaer_VERSION_linux_amd64.tar.gz
install -m 0755 oaer ~/.local/bin/oaer
oaer version
```

For macOS, use the `darwin` archive matching your CPU. For Windows, extract the
`.zip` archive and place `oaer.exe` in a directory on `%PATH%`.

## CI Usage

CI jobs can either build from source or download a release artifact.

Build from source:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
- run: go install github.com/open-aer/oaer/cmd/oaer@latest
- run: oaer compat mvp
```

If your CI job has access to the scraped public Apex docs corpus, set
`OAER_APEX_DOCS_SOURCE` and run the docs support gate. The gate regenerates the
docs inventory, capability catalog, product namespace typed-stub report, and
fixture evidence report, then fails if fixture evidence points at a symbol
missing from the catalog.

```yaml
- run: scripts/apex-docs-support-gate.sh
  env:
    OAER_APEX_DOCS_SOURCE: /path/to/salesforce-docs/apex
```

The product namespace report can also be generated directly from a catalog when
reviewing broad typed-stub coverage:

```bash
oaer compat docs-inventory --source /path/to/salesforce-docs/apex --output inventory.json
oaer compat catalog --inventory inventory.json --output catalog.json
oaer compat product-namespaces --catalog catalog.json --json
oaer compat product-namespaces --source /path/to/salesforce-docs/apex --output docs/generated/PRODUCT_NAMESPACE_COVERAGE.md
```

When refreshing broad standard SObject field coverage from public Apex stubs,
regenerate the field overlay before running storage or VM compatibility checks:

```bash
node scripts/generate-sobject-stub-overlay.mjs /path/to/fulgor/stubs/apex-sobject-stubs internal/storage/standard_sobject_stub_overlay_generated.go
```

When refreshing broad system, Schema, Database, and product namespace type
shape from public Apex stubs, regenerate the type-symbol overlay before running
type-system or semantic-analysis checks:

```bash
node scripts/generate-system-stub-symbols.mjs /path/to/fulgor/stubs/apex-system-stubs internal/typesys/system_stub_symbols_generated.go
```

Tooling snippet oracle reports can be captured from a scratch org and validated
as stable JSON artifacts:

```bash
oaer probe tooling-snippet --target-org oaer-probe-lab --manifest docs/generated/TOOLING_SNIPPET_MANIFEST.json --output tmp/tooling-snippet-results.json
oaer compat tooling-fixtures tmp/tooling-snippet-results.json
```

Use a release artifact:

```yaml
- run: |
    curl -L -o oaer.tar.gz "$OAER_RELEASE_URL"
    curl -L -o SHA256SUMS.txt "$OAER_CHECKSUMS_URL"
    shasum -a 256 -c SHA256SUMS.txt
    tar -xzf oaer.tar.gz
    install -m 0755 oaer ~/.local/bin/oaer
    oaer version
```

## Persistent Local Server

Use `--db` when the local Salesforce-shaped API server should keep org state
across restarts.

```bash
oaer db reset --db .oaer/local-org.sqlite --json
oaer server --db .oaer/local-org.sqlite --addr 127.0.0.1:8080
```

Seed and inspect the same file with the DB commands:

```bash
oaer db seed --db .oaer/local-org.sqlite docs/fixtures/storage-db-lifecycle.json --json
oaer db inspect --db .oaer/local-org.sqlite --json
oaer db export --db .oaer/local-org.sqlite > exported-fixture.json
```

The running server exposes fixture and reset endpoints under the REST version
path. Full reset remains `POST /services/data/v65.0/oaer/reset`. Scoped resets
can target only data or platform state:

```bash
curl -s -X POST http://127.0.0.1:8080/services/data/v65.0/oaer/reset/data
curl -s -X POST 'http://127.0.0.1:8080/services/data/v65.0/oaer/reset?scope=users,limits,async'
```

Use `oaer db inspect --json` before and after mutating server requests as the
basic operational check. Counts should change after successful mutations and
stay fixed after failed mutations.

The local API server accepts missing `Authorization` headers and local
`Authorization: Bearer ...` values without validating OAuth tokens. Use the
`X-OAER-User-Id` header only to select an existing local `User` record for test
requests. Direct REST DML uses that local user for system field stamping;
Tooling `executeAnonymous` still uses the VM's local default user context. Do
not expose `oaer server` to untrusted networks without an authenticating reverse
proxy.

## Local Apex Playground

Use `oaer playground` for a local browser workbench with a file tree, Apex class
editor, execute-anonymous pane, cached results, logs, variables, limits, traces,
and org diff output.

```bash
oaer playground --db .oaer/playground/org.sqlite --addr 127.0.0.1:1789 --open
```

The playground stores scratch files under `.oaer/playground/workspaces/default`
when no project is supplied. Pass `--examples` to include built-in example
projects for DML, SOQL, triggers, relationships, maps, and governor-limit
counters. Point the playground at an existing SFDX project to edit that
project's supported files directly:

```bash
oaer playground --examples --db .oaer/playground/org.sqlite
```

```bash
oaer playground --project . --db .oaer/playground/org.sqlite
```

The foreground project runs as local source in the playground, even when its
SFDX descriptor declares a package namespace. Managed package dependencies keep
their configured namespaces.

Use `--project-ref name=path` to add local SFDX folders to the scratch
workspace's project selector without editing them in place. Loading a reference
copies supported `.cls`, `.trigger`, `.apex`, `.json`, `.xml`, `.yml`, and
`.yaml` files into the managed scratch workspace while preserving their relative
folder paths. Dot files and dot directories are skipped. The copied project is
treated as local source: the copied `sfdx-project.json` namespace is cleared and
top-level `oaer.yml`/`oaer.yaml` files are not imported. Built-in examples are
hidden when project references are supplied. If the folder has no
`anonymous.apex` or `seed.json`, the loader adds default scratch files. Only
`seed.json` is treated as playground data; other JSON files remain metadata:

```bash
oaer playground --project-ref "Local Probe=../some-sfdx-project" --open
```

It binds to localhost by default. Do not expose it to untrusted networks; it runs
local Apex through the OAER VM and can mutate the selected local org database in
persist mode.

## Homebrew

Homebrew distribution is not published yet. A future tap formula should use the
release archive URLs and checksums generated by the release workflow:

```ruby
class Oaer < Formula
  desc "Clean-room local Apex runtime"
  homepage "https://github.com/open-aer/oaer"
  url "https://github.com/open-aer/oaer/releases/download/VERSION/oaer_VERSION_darwin_arm64.tar.gz"
  sha256 "REPLACE_WITH_RELEASE_SHA256"
  version "VERSION"

  def install
    bin.install "oaer"
  end

  test do
    system "#{bin}/oaer", "version"
  end
end
```
