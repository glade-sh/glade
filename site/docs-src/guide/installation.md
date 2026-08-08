# Installation

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Install</p>
  <p>Recommended path: use the one-line installer, confirm the version, then initialize Glade inside a Salesforce DX project.</p>
  <ul>
    <li>Use the script for macOS and Linux release archives.</li>
    <li>Use a manual archive when CI or policy requires pinned artifacts.</li>
    <li>Build from source when you are developing Glade.</li>
  </ul>
</div>

Glade ships as a single local binary for macOS and Linux. Install it, confirm
the binary, then complete the project-aware checks from a Salesforce DX
workspace.

## Choose an install path

<div class="docs-install-grid">
  <a class="docs-install-card" href="#one-line-install">
    <p class="docs-card-kicker">Recommended</p>
    <strong>macOS/Linux script</strong>
    <span>Installs the current release to <code>~/.local/bin</code>.</span>
  </a>
  <a class="docs-install-card" href="#manual-release-archive">
    <p class="docs-card-kicker">Pinned</p>
    <strong>Manual release archive</strong>
    <span>Use in CI or when policy requires pinned artifacts.</span>
  </a>
  <a class="docs-install-card" href="#build-from-source">
    <p class="docs-card-kicker">Source</p>
    <strong>Build from source</strong>
    <span>Use for Glade development or unreleased changes.</span>
  </a>
  <a class="docs-install-card" href="#editor-and-ci">
    <p class="docs-card-kicker">Editor and CI</p>
    <strong>VS Code and automation</strong>
    <span>Install the bundled VS Code extension or place the binary in a CI runner.</span>
  </a>
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

Confirm the binary after it is on your path:

```bash
glade version
```

`glade doctor` is project-aware. Run it after the project initialization steps
below, not from an arbitrary directory.

## Update Glade

Use the same installer path for updates. Check the command first when you want
to see what will run:

```bash
glade update --dry-run
GLADE_UPDATE_ALLOW_SHELL=1 glade update
glade version
```

`glade update` prints the exact installer command before it runs. It updates the
directory that contains the current `glade` binary. The environment guard keeps
self-replacement explicit.

Manual fallback:

- Release archives: <https://github.com/glade-sh/glade/releases>
- Checksums: <https://github.com/glade-sh/glade/releases/latest/download/SHA256SUMS.txt>

## Security verification for a manual release archive {#manual-release-archive}

Release archives publish checksums, CycloneDX SBOMs, and GitHub artifact
attestations. The release workflow verifies both archive provenance and the
CycloneDX attestation before uploading platform assets. Use this path when
policy requires pinned proof:

```bash
GLADE_MANIFEST_URL=https://downloads.glade.sh/latest/release-manifest.json
GLADE_VERSION="$(curl -fsSL "$GLADE_MANIFEST_URL" | sed -nE 's/^[[:space:]]*"version": "(v[^"]+)",?$/\1/p')"
[ -n "$GLADE_VERSION" ] || { echo "could not resolve the stable Glade version" >&2; exit 1; }
case "$(uname -s)" in Darwin) GLADE_OS=darwin ;; Linux) GLADE_OS=linux ;; *) echo "unsupported operating system" >&2; exit 1 ;; esac
case "$(uname -m)" in arm64|aarch64) GLADE_ARCH=arm64 ;; x86_64|amd64) GLADE_ARCH=amd64 ;; *) echo "unsupported architecture" >&2; exit 1 ;; esac
GLADE_ARCHIVE="glade_${GLADE_VERSION}_${GLADE_OS}_${GLADE_ARCH}.tar.gz"
GLADE_BASE="https://downloads.glade.sh/${GLADE_VERSION}"
curl -fLO "${GLADE_BASE}/${GLADE_ARCHIVE}"
curl -fLO "${GLADE_BASE}/SHA256SUMS.txt"
GLADE_CHECKSUM_LINE="$(grep "  \./${GLADE_ARCHIVE}$" SHA256SUMS.txt)"
[ -n "$GLADE_CHECKSUM_LINE" ] || { echo "checksum entry not found" >&2; exit 1; }
if command -v shasum >/dev/null 2>&1; then printf '%s\n' "$GLADE_CHECKSUM_LINE" | shasum -a 256 -c -; else printf '%s\n' "$GLADE_CHECKSUM_LINE" | sha256sum -c -; fi
gh attestation verify "$GLADE_ARCHIVE" -R glade-sh/glade
gh attestation verify "$GLADE_ARCHIVE" -R glade-sh/glade \
  --predicate-type https://cyclonedx.org/bom
tar -xzf "$GLADE_ARCHIVE"
./glade version
```

Download the matching `*.sbom.json` release asset when your review process
requires a dependency inventory.

## Editor and CI {#editor-and-ci}

Use the bundled VS Code extension for editor workflows. For automation, skip
to [CI usage](#ci-usage).

### Install the VS Code extension

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
go run ./cmd/glade check --project path/to/sfdx-project
```

## First project run

From an SFDX project root:

```bash
glade init --project . --yes
glade config validate --project .
glade config show --project .
glade doctor
glade check --project .
glade test --project . --json --no-progress
```

Run one class, one method, or tests affected by a git ref:

```bash
glade test --project . --class <YourTestClass> --json
glade test --project . --class <YourTestClass> --method <yourTestMethod> --json
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
    GLADE_MANIFEST_URL=https://downloads.glade.sh/latest/release-manifest.json
    GLADE_VERSION="$(curl -fsSL "$GLADE_MANIFEST_URL" | sed -nE 's/^[[:space:]]*"version": "(v[^"]+)",?$/\1/p')"
    [ -n "$GLADE_VERSION" ] || { echo "could not resolve the stable Glade version" >&2; exit 1; }
    GLADE_ARCHIVE="glade_${GLADE_VERSION}_linux_amd64.tar.gz"
    GLADE_BASE="https://downloads.glade.sh/${GLADE_VERSION}"
    curl -fLO "${GLADE_BASE}/${GLADE_ARCHIVE}"
    curl -fLO "${GLADE_BASE}/SHA256SUMS.txt"
    GLADE_CHECKSUM_LINE="$(grep "  \./${GLADE_ARCHIVE}$" SHA256SUMS.txt)"
    [ -n "$GLADE_CHECKSUM_LINE" ] || { echo "checksum entry not found" >&2; exit 1; }
    printf '%s\n' "$GLADE_CHECKSUM_LINE" | sha256sum -c -
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
```

::: tip Next step
Continue with the [first project run](#first-project-run), or follow the
[first local check](/guide/quickstart) one step at a time.
:::
