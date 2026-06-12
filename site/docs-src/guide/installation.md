# Installation

Glade ships as a single local binary for macOS and Linux. Install it, verify
your environment with `glade doctor`, then run your first project check from an
SFDX workspace.

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
GLADE_INSTALL_DIR=/usr/local/bin curl -fsSL https://glade.sh/install.sh | sh
GLADE_VERSION=vX.Y.Z curl -fsSL https://glade.sh/install.sh | sh
```

Check the binary after it is on your path:

```bash
glade version
glade doctor
```

Expected:

```text
glade doctor
parser: ok (tree-sitter)
```

Manual fallback:

- Release archives: <https://github.com/glade-sh/glade/releases>
- Checksums: <https://github.com/glade-sh/glade/releases/latest/download/SHA256SUMS.txt>

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
glade test --project . --json
```

Run one class, one method, or tests affected by a git ref:

```bash
glade test --project . --filter AccountServiceTest --json
glade test --project . --filter AccountServiceTest.testCreatesAccount --json
glade test changed --project . --since origin/main --json
```

## CI usage

Build from source in GitHub Actions:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
- run: go install github.com/glade-sh/glade/cmd/glade@latest
- run: glade check --project .
- run: glade test --project . --json
- run: glade check --project . --format sarif --output glade-check.sarif
```

Or download a release artifact and verify checksums:

```yaml
- run: |
    curl -L -o glade.tar.gz "$GLADE_RELEASE_URL"
    curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
    shasum -a 256 -c SHA256SUMS.txt
    tar -xzf glade.tar.gz
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
Create project config: [Configure a Glade Project](/guide/configuration), then
check [What Glade supports](/guide/support-map).
:::
