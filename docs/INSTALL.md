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
GLADE_INSTALL_DIR=/usr/local/bin curl -fsSL https://glade.sh/install.sh | sh
GLADE_VERSION=vX.Y.Z curl -fsSL https://glade.sh/install.sh | sh
```

Then check the binary:

```bash
glade version
glade doctor   # confirm: "parser: ok (tree-sitter)"
```

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
./glade doctor   # confirm: "parser: ok (tree-sitter)"
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

The short aliases `compat` and `performance` resolve to `@glade/compat` and
`@glade/performance`. The public plugin registry is preview. Direct archives
and local links are the fallback paths until a registry is configured. The
first-party plugin catalog, local archive install path, and author contract are
documented in [PLUGINS.md](PLUGINS.md).

Run a focused class or only tests affected by changes since a git ref:

```bash
glade test --project . --class AccountServiceTest --json
glade test changed --project . --since origin/main --json
glade performance scan --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

See [CONFIG.md](CONFIG.md) for `glade.yml`, `glade init`, and config inspection.
See [LOCAL_TESTING.md](LOCAL_TESTING.md) for class/method filters,
dependency-selected test runs, and `glade test serve`. For when the test startup
cache is created, how it stays fresh, and how to recover from a bad cache, see
[TEST_STARTUP_CACHE.md](TEST_STARTUP_CACHE.md).

For a short release-candidate dogfood pass, use
[`DOGFOOD_CHECKLIST.md`](DOGFOOD_CHECKLIST.md).

## Distribute to a Few Machines (Pre-Release)

Before publishing public releases you can hand `glade` to a handful of machines.
The one rule to remember: **the binary is CGO-linked, so a build runs only on
the same OS and CPU architecture it was built on.** Build per platform, then
copy.

### 1. Build a working binary

On a machine of the target platform (for example, an Apple Silicon Mac for other
Apple Silicon Macs):

```bash
scripts/build-local.sh
```

This builds a CGO-enabled host binary into `dist/`, verifies the parser is wired
up (`doctor` must report `parser: ok`), and writes a `.tar.gz` (or `.zip` on
Windows) plus a `.sha256`. Override the version or output directory with
`VERSION=` and `DIST_DIR=`.

### 2. Copy to the target machines

Same OS and architecture only. For example:

```bash
scp dist/glade_*_darwin_arm64.tar.gz user@host:/tmp/
```

For a Linux server, build the archive on Linux (or in the bundled container —
see below) and copy that.

### 3. Install and verify on each machine

```bash
shasum -a 256 -c glade_*_darwin_arm64.tar.gz.sha256   # optional integrity check
tar -xzf glade_*_darwin_arm64.tar.gz
install -m 0755 glade ~/.local/bin/glade

glade version
glade doctor          # MUST show "parser: ok (tree-sitter)"
```

`glade doctor` is the acceptance check: if it prints `parser: UNAVAILABLE`, the
binary was built without CGO and will not parse project Apex. Rebuild with a C
compiler present.

On macOS, Gatekeeper may quarantine an unsigned binary copied from another
machine. Clear it with:

```bash
xattr -d com.apple.quarantine ~/.local/bin/glade
```

### Building a Linux binary from macOS

CGO cross-compilation needs a target toolchain, so the simplest path is the
bundled container, which builds on a glibc base:

```bash
docker build -t glade-local .
docker create --name glade-extract glade-local
docker cp glade-extract:/usr/local/bin/glade ./glade-linux
docker rm glade-extract
```

Copy `glade-linux` to the Linux machine and verify with `glade doctor`. The
Linux host's glibc must be compatible with the build base (Debian bookworm).

### Alternative: build from source on each machine

The most robust option when machines already have Go 1.26 and a C compiler is to
build from source on each one (see "Build And Run From Source" above). No
cross-compilation, no architecture mismatches.

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
- run: go install github.com/glade-sh/glade/cmd/glade@latest
- run: glade check --project .
- run: glade test --project . --json
```

Use a release artifact:

```yaml
- run: |
    curl -L -o glade.tar.gz "$GLADE_RELEASE_URL"
    curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
    shasum -a 256 -c SHA256SUMS.txt
    tar -xzf glade.tar.gz
    install -m 0755 glade ~/.local/bin/glade
    glade version
    glade doctor
```

## Persistent Local Server

Use `--db` when the local Salesforce-shaped API server should keep org state
across restarts.

```bash
glade db reset --db .glade/local-org.sqlite --json
glade server --db .glade/local-org.sqlite --addr 127.0.0.1:8080
```

Seed and inspect the same file with the DB commands:

```bash
glade db seed --db .glade/local-org.sqlite seed.json --json
glade db inspect --db .glade/local-org.sqlite --json
glade db export --db .glade/local-org.sqlite > exported-fixture.json
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

Homebrew distribution is not published yet. The easiest path is a dedicated
tap repo and a formula update on each tagged release:

1. Build release artifacts and checksums from a tag (`vX.Y.Z`).
2. Update `glade.rb` in your tap with the matching archive URL and SHA256.
3. Commit and push the tap update.
4. Validate with `brew install <tap>/glade` and `glade version`.

For the full release-to-distribution checklist, see
[`docs/DISTRIBUTION_WORKFLOW.md`](DISTRIBUTION_WORKFLOW.md).

Formula template:

```ruby
class Glade < Formula
  desc "Clean-room local Apex runtime"
  homepage "https://github.com/glade-sh/glade"
  url "https://github.com/glade-sh/glade/releases/download/VERSION/glade_VERSION_darwin_arm64.tar.gz"
  sha256 "REPLACE_WITH_RELEASE_SHA256"
  version "VERSION"

  def install
    bin.install "glade"
  end

  test do
    system "#{bin}/glade", "version"
  end
end
```
