# Installation

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Install</p>
  <p>Recommended path: use the one-line installer, then run <code>glade doctor</code>.</p>
  <ul>
    <li>Use the script for macOS and Linux release archives.</li>
    <li>Use a manual archive when CI or policy requires pinned artifacts.</li>
    <li>Build from source when you are developing Glade.</li>
  </ul>
</div>

Glade ships as a single local binary for macOS and Linux. Install it, verify
your environment with `glade doctor`, then run your first project check from an
SFDX workspace.

## Choose an install path

<div class="docs-install-grid">
  <div class="docs-install-card">
    <p class="docs-card-kicker">Recommended</p>
    <strong>macOS/Linux script</strong>
    <span>Installs the current release to <code>~/.local/bin</code>.</span>
  </div>
  <div class="docs-install-card">
    <p class="docs-card-kicker">Pinned</p>
    <strong>Manual release archive</strong>
    <span>Use in CI or when policy requires pinned artifacts.</span>
  </div>
  <div class="docs-install-card">
    <p class="docs-card-kicker">Source</p>
    <strong>Build from source</strong>
    <span>Use for Glade development or unreleased changes.</span>
  </div>
  <div class="docs-install-card">
    <p class="docs-card-kicker">Editor and CI</p>
    <strong>VS Code and automation</strong>
    <span>Install the bundled VS Code extension or place the binary in a CI runner.</span>
  </div>
</div>

## One-line install

| OS | CPU | Status |
| --- | --- | --- |
| macOS | arm64 | supported release archive |
| macOS | amd64 | supported release archive |
| Linux | amd64 | supported release archive |
| Linux | arm64 | supported release archive |
| Windows | amd64/arm64 | build from source for now |

For macOS and Linux:

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

The script installs the latest release to `~/.local/bin` by default. Override the install directory or version when needed:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR=/usr/local/bin sh
curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z sh
```

Check the binary after it is on your path:

```bash
glade version
glade doctor
```

Expected: `glade doctor` prints status rows for the parser, toolchain, config,
and runtime, then ends with `Ready.`.

```text
glade doctor
Glade doctor

Project      ✓ SFDX project found
Parser       ✓ ok (tree-sitter)
Toolchain    ✓ <glade data dir> (ok (global))
Config       ✓ glade.yml
Runtime      ✓ glade <version> · go<version> · <os>/<arch>

Ready.
```

## Update Glade

Use the same installer path for updates. Check the command first when you want
to see what will run:

```bash
glade update --dry-run
GLADE_UPDATE_ALLOW_SHELL=1 glade update
glade version
glade doctor
```

`glade update` prints the exact installer command before it runs. It updates the
directory that contains the current `glade` binary. The environment guard keeps
self-replacement explicit.

Manual fallback:

- Release archives: <https://github.com/glade-sh/glade/releases>
- Checksums: <https://github.com/glade-sh/glade/releases/latest/download/SHA256SUMS.txt>

## Security verification

Release archives publish checksums, CycloneDX SBOMs, and GitHub artifact
attestations. The release workflow verifies both archive provenance and the
CycloneDX attestation before uploading platform assets. Use this path when
policy requires pinned proof:

```bash
GLADE_ARCHIVE=glade_vX.Y.Z_linux_amd64.tar.gz
curl -L -o "$GLADE_ARCHIVE" "$GLADE_RELEASE_URL"
curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
grep -F "  ./$GLADE_ARCHIVE" SHA256SUMS.txt | shasum -a 256 -c -
gh attestation verify "$GLADE_ARCHIVE" -R glade-sh/glade
gh attestation verify "$GLADE_ARCHIVE" -R glade-sh/glade \
  --predicate-type https://cyclonedx.org/bom
tar -xzf "$GLADE_ARCHIVE"
./glade version
./glade doctor
```

Download the matching `*.sbom.json` release asset when your review process
requires a dependency inventory.

## Install VS Code Extension

Release archives include the Glade VS Code extension at
`share/glade/editor/vscode-glade.vsix`. After the `glade` binary is on your
path, check the editor install path and install the bundled extension:

```bash
glade editor doctor vscode
glade editor install vscode --force
glade editor doctor vscode --editor cursor
glade editor install vscode --editor windsurf --force
```

Open an SFDX project in VS Code. The extension adds Glade Home, a Glade
Activity Bar, native Test Explorer entries, local CodeLens actions, DAP debug
launches, named local data environments, Apex & SOQL scratch buffers, and a
Plugins view for linked and installed plugin actions. See
[Editor, LSP, and DAP](/guide/editor).

## Build from source

Use this path when developing Glade or trying the current repository state before a release is cut.

Prerequisites:

- Go 1.26 or newer
- Git
- A C compiler with CGO enabled. On macOS, install Xcode Command Line Tools. On
  Debian or Ubuntu, install `build-essential`.

```bash
git clone https://github.com/glade-sh/glade.git
cd glade
go build -o glade ./cmd/glade
./glade version
./glade doctor
```

Run against an SFDX project. No Salesforce org login is required for these local commands:

```bash
./glade check --project path/to/sfdx-project --json
./glade test --project path/to/sfdx-project --json
./glade exec "System.debug('hello from source build');"
```

During development you can run the CLI without building a binary:

```bash
go run ./cmd/glade version
go run ./cmd/glade doctor
go run ./cmd/glade check --project path/to/sfdx-project
```

## First project run

From an SFDX project root:

```bash
glade init --project . --yes
glade config validate --project .
glade config show --project .
glade check --project .
glade test --project . --json --no-progress
```

Run one class, one method, or tests affected by a git ref:

```bash
glade test --project . --class RefinementServiceTest --json
glade test --project . --class RefinementServiceTest --method testRefinesFileRow --json
glade test changed --project . --since origin/main --json --no-progress
```

## CI usage

Build from source in GitHub Actions:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
- run: sudo apt-get update && sudo apt-get install -y build-essential
- run: CGO_ENABLED=1 go install github.com/glade-sh/glade/cmd/glade@latest
- run: glade doctor --json
- run: glade check --project .
- run: glade test --project . --json
- run: glade check --project . --format sarif --output glade-check.sarif
```

Or download a release artifact and verify checksums:

```yaml
- run: |
    GLADE_ARCHIVE=glade_vX.Y.Z_linux_amd64.tar.gz
    curl -L -o "$GLADE_ARCHIVE" "$GLADE_RELEASE_URL"
    curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
    grep -F "  ./$GLADE_ARCHIVE" SHA256SUMS.txt | shasum -a 256 -c -
    gh attestation verify "$GLADE_ARCHIVE" -R glade-sh/glade
    tar -xzf "$GLADE_ARCHIVE"
    install -m 0755 glade ~/.local/bin/glade
    glade version
```

## Troubleshooting

If `glade` is not found after install, add `~/.local/bin` to your shell path:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then open a new shell and run:

```bash
glade version
glade doctor
```

::: tip Next step
For a first project run, use the [Tester field guide](/guide/tester-field-guide).
For a narrow first run, create project config:
[Configure a Glade Project](/guide/configuration), then check
[What Glade runs locally](/guide/support-map).
:::
