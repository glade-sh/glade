# Install glade

`glade` is distributed as a single binary. Release artifacts are parser-capable
CGO builds for macOS and Linux on amd64 and arm64. Each release build also
publishes `SHA256SUMS.txt`.

Project home: <https://glade.sh>

## One-Line Install

For macOS and Linux:

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

The script installs the latest release to `~/.local/bin` by default. Override
the target directory or version when needed:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR=/usr/local/bin sh
curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z sh
```

Then check the binary:

```bash
glade version
glade doctor   # confirm: "Ready."
```

## Security verification

Release archives publish checksums, CycloneDX SBOMs, and GitHub artifact
attestations. Use this path when a laptop policy needs pinned proof:

```bash
curl -L -o glade.tar.gz "$GLADE_RELEASE_URL"
curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
shasum -a 256 -c SHA256SUMS.txt
gh attestation verify glade.tar.gz -R glade-sh/glade
tar -xzf glade.tar.gz
./glade version
./glade doctor
```

Download the matching `*.sbom.json` release asset when your review process
requires a dependency inventory.

## Update

Use the same installer path for upgrades:

```bash
glade update --dry-run
GLADE_UPDATE_ALLOW_SHELL=1 glade update
glade version
glade doctor
```

`glade update` prints the exact installer command before it runs. It updates the
directory that contains the current `glade` binary. The environment guard keeps
self-replacement explicit.

## Install VS Code Extension

Release archives include the Glade VS Code extension at
`share/glade/editor/vscode-glade.vsix`. After the `glade` binary is on your
path, check the editor install path and install the bundled extension:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

Open an SFDX project in VS Code. The extension adds Glade Home, a Glade
Activity Bar, native Test Explorer entries, local CodeLens actions, DAP debug
launches, named local data environments, Exec & SOQL scratch buffers, and a
Plugins view for linked and installed plugin actions. See [EDITOR.md](EDITOR.md).

## Shell Completion

`glade` can print completion scripts for bash, zsh, and fish:

```bash
glade completion bash
glade completion zsh
glade completion fish
```

For a one-session bash install:

```bash
source <(glade completion bash)
```

For zsh, write the generated script into a directory already listed in
`$fpath`, then restart the shell:

```bash
mkdir -p ~/.zsh/completions
glade completion zsh > ~/.zsh/completions/_glade
```

For fish:

```bash
mkdir -p ~/.config/fish/completions
glade completion fish > ~/.config/fish/completions/glade.fish
```

## Build And Run From Source

Use this path for Glade development or for trying the current repository state
before a release is cut.

Prerequisites:

- Go 1.26 or newer
- Git
- A C compiler with CGO enabled (CGO is on by default). glade's Apex parser is a
  generated tree-sitter grammar that requires CGO; without it, `check`, `test`,
  and `parse` on project sources fail with an `APEXPARSECGO` error. Install
  Xcode Command Line Tools on macOS (`xcode-select --install`) or `build-essential`
  on Debian/Ubuntu (`sudo apt-get install build-essential`).

```bash
git clone https://github.com/glade-sh/glade.git
cd glade
go build -o glade ./cmd/glade
./glade version
./glade doctor   # confirm: "Ready."
```

Run the locally built binary against an SFDX project. No Salesforce org login is
required for these local commands:

```bash
./glade check --project path/to/sfdx-project --json
./glade test --project path/to/sfdx-project --json
./glade exec "System.debug('hello from source build');"
```

During development, run commands directly from source without creating a binary:

```bash
go run ./cmd/glade version
go run ./cmd/glade doctor
go run ./cmd/glade check --project path/to/sfdx-project
```

If you want `glade` on your `PATH`:

```bash
install -m 0755 glade ~/.local/bin/glade
glade version
```

## First Project Run

Run parse/check/tests against an SFDX project without connecting to an org:

```bash
cd path/to/sfdx-project
glade doctor
glade init --project . --yes
glade config validate --project .
glade config show --project .
glade check --project .
glade test --project . --json
```

Install advisory scanners when needed. They are plugins, not product runtime
packages:

```bash
# Requires a live plugin registry, custom registry, direct archive, or linked plugin.
glade plugins available
glade plugins install @glade/performance
glade performance scan --project . --json
```

Capture installed package contracts from an org when a local project depends on
a package but should not carry its source:

```bash
glade plugins install @glade/orgpackage
glade package capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet
```

`glade package capture` dispatches to `glade orgpackage capture` when the
`@glade/orgpackage` plugin is installed or linked. The short aliases
`performance` and `orgpackage` resolve to `@glade/performance` and
`@glade/orgpackage`. `@glade/compat` is maintainer-facing support tooling. The
public plugin registry is preview. Direct archives and local links are the fallback paths until a
registry is configured. The first-party plugin catalog, local archive install
path, and author contract are documented in [PLUGINS.md](PLUGINS.md).

Run a focused class or only tests affected by changes since a git ref:

```bash
glade test --project . --class RefinementServiceTest --json
glade test changed --project . --since origin/main --json --no-progress
mkdir -p reports
glade performance scan --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

See [CONFIG.md](CONFIG.md) for `glade.yml`, `glade init`, and config inspection.
See [LOCAL_TESTING.md](LOCAL_TESTING.md) for class/method filters,
dependency-selected test runs, and `glade test serve`. For when the test startup
cache is created, how it stays fresh, and how to recover from a bad cache, see
[TEST_STARTUP_CACHE.md](TEST_STARTUP_CACHE.md).
For a compact pilot handoff that covers VS Code, AI, CI, and report workflows,
see [TESTER_FIELD_GUIDE.md](TESTER_FIELD_GUIDE.md).

## Manual Install

Download the archive for your platform from the release artifacts, verify the
checksum, and place the binary on your `PATH`.

```bash
shasum -a 256 -c SHA256SUMS.txt
tar -xzf glade_VERSION_linux_amd64.tar.gz
install -m 0755 glade ~/.local/bin/glade
glade version
```

For macOS, use the `darwin` archive matching your CPU. Windows release archives
are not published by the CGO-enabled release workflow.

## CI Usage

CI jobs can either build from source or download a release artifact.

Build from source:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
- run: sudo apt-get update && sudo apt-get install -y build-essential
- run: CGO_ENABLED=1 go install github.com/glade-sh/glade/cmd/glade@latest
- run: glade doctor --json
- run: glade check --project .
- run: glade test --project . --json
```

Use a release artifact:

```yaml
- run: |
    curl -L -o glade.tar.gz "$GLADE_RELEASE_URL"
    curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
    shasum -a 256 -c SHA256SUMS.txt
    gh attestation verify glade.tar.gz -R glade-sh/glade
    tar -xzf glade.tar.gz
    install -m 0755 glade ~/.local/bin/glade
    glade version
    glade doctor
```

## Persistent Local Server

Use `--db` when the local Salesforce-shaped API server should keep org state
across restarts.

```bash
glade db reset --db .glade/refinement-local.sqlite --json
glade server --db .glade/refinement-local.sqlite --addr 127.0.0.1:8080
```

Seed and inspect the same file with the DB commands:

```bash
glade db seed --db .glade/refinement-local.sqlite data/file-rows.json --json
glade db inspect --db .glade/refinement-local.sqlite --json
glade db query --db .glade/refinement-local.sqlite --project . --json "SELECT Id, Name FROM FileRow__c"
glade db describe --db .glade/refinement-local.sqlite --project . --json FileRow__c
glade db export --db .glade/refinement-local.sqlite > refinement-export.json
```

The running server exposes fixture and reset endpoints under the REST version
path. Full reset remains `POST /services/data/v65.0/glade/reset`. Scoped resets
can target only data or platform state:

```bash
curl -s -X POST http://127.0.0.1:8080/services/data/v65.0/glade/reset/data
curl -s -X POST 'http://127.0.0.1:8080/services/data/v65.0/glade/reset?scope=users,limits,async'
```

Use `glade db inspect --json` before and after mutating server requests as the
basic operational check. Counts should change after successful mutations and
stay fixed after failed mutations.

The local API server accepts missing `Authorization` headers and local
`Authorization: Bearer ...` values without validating OAuth tokens. Use the
`X-GLADE-User-Id` header only to select an existing local `User` record for test
requests. Direct REST DML uses that local user for system field stamping;
Tooling `executeAnonymous` still uses the VM's local default user context. Do
not expose `glade server` to untrusted networks without an authenticating reverse
proxy.

## Local Apex Playground

Use `glade playground` for a local browser workbench with a file tree, Apex class
editor, execute-anonymous pane, cached results, logs, variables, limits, traces,
and org diff output.

```bash
glade playground --db .glade/playground/org.sqlite --addr 127.0.0.1:1789 --open
```

The playground stores scratch files under `.glade/playground/workspaces/default`
when no project is supplied. Pass `--examples` to include built-in example
projects for DML, SOQL, triggers, relationships, maps, and governor-limit
counters. Point the playground at an existing SFDX project to edit that
project's supported files directly:

```bash
glade playground --examples --db .glade/playground/org.sqlite
```

```bash
glade playground --project . --db .glade/playground/org.sqlite
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
top-level `glade.yml`/`glade.yaml` files are not imported. Built-in examples are
hidden when project references are supplied. If the folder has no
`anonymous.apex` or `seed.json`, the loader adds default scratch files. Only
`seed.json` is treated as playground data; other JSON files remain metadata:

```bash
glade playground --project-ref "Local Probe=../some-sfdx-project" --open
```

It binds to localhost by default. Do not expose it to untrusted networks; it runs
local Apex through the Glade VM and can mutate the selected local org database in
persist mode.

## Homebrew

Homebrew distribution is not published yet. Use the one-line installer or a
manual release archive for now.
