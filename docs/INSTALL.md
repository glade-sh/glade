# Install Glade

`glade` is distributed as a CLI binary with bundled runtime and editor assets.
Release artifacts are parser-capable CGO builds for macOS and Linux on amd64
and arm64. Each release build also publishes `SHA256SUMS.txt`.

Project home: <https://glade.sh>

## One-Line Install

The installer supports macOS and Linux on arm64 and amd64. It needs `curl`, a
working Python 3, `mkfifo`, and either `shasum` or `sha256sum`. Python 3
coordinates the installation lock. On macOS, install the Xcode Command Line
Tools with `xcode-select --install` or put another Python 3 on your `PATH`.
The GitHub CLI (`gh`) enables attestation verification.

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

The script installs the `glade` binary, including parser support, to
`~/.local/bin`. It installs the LWC runtime/toolchain and bundled editor assets
under `~/.local/share/glade` by default. Set `GLADE_INSTALL_DIR` and
`GLADE_HOME` to change those destinations. Override both destinations or pin a
version when needed:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR=/usr/local/bin GLADE_HOME="$HOME/.local/share/glade" sh
curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z sh
```

Then check the binary:

```bash
export PATH="$HOME/.local/bin:$PATH"
glade version
```

To keep this `PATH` in new terminals, use the line for your shell:

```bash
printf '%s\n' 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.zshrc"  # zsh
printf '%s\n' 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc" # bash
```

For a bash login shell, use `~/.bash_profile` if it does not already load
`~/.bashrc`.

`glade doctor` is project-aware. Run it after initializing Glade inside a
Salesforce DX project, as shown in [First Project Run](#first-project-run).

## Security verification

The installer checks the archive's SHA-256 checksum before extraction. With
`gh` installed, it also checks release-workflow provenance and the CycloneDX
attestation. A failed check stops installation. Without `gh`, the installer
prints that provenance was not checked and continues after the checksum passes.

To require both attestations, install `gh` and run:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_REQUIRE_PROVENANCE=1 sh
```

The installer prefers the per-archive Sigstore provenance bundle. It can use
GitHub's public attestation API for the separate CycloneDX attestation and for
older releases without a bundle. If public lookup fails, sign in with
`gh auth login` to let `gh` fetch the attestations.

An installer fetched and run directly from `glade.sh` is itself trusted at
download time; a substituted installer could bypass these checks.
`glade update` runs the installer embedded in the installed binary, which
checks the downloaded archive before replacing the installation.

Use the verification-only helper in the [Security Policy](../SECURITY.md#verify-a-release-archive).
From a reviewed repository checkout, run:

```bash
sh scripts/verify-release-download.sh
```

The helper requires `curl`, `jq`, `gh`, `awk`, `mktemp`, `uname`, and either
`shasum` or `sha256sum`. It stops at every failed gate, retains the downloaded
files for inspection, and does not extract, install, or execute Glade.

Download the matching `*.sbom.json` release asset when your review process
requires a dependency inventory.

## Update

Use the same installer path for upgrades:

```bash
glade update --dry-run
GLADE_UPDATE_ALLOW_SHELL=1 glade update
glade version
```

`glade update` prints an executable update command before it runs. It updates the
directory that contains the current `glade` binary. The environment guard keeps
self-replacement explicit.

Managed installations use an installation lock to protect files during updates
and reads. If Glade cannot open or create that lock, it refuses to use the
managed installation rather than reading files while an update may be active.
For a shared installation, ask its owner to make the existing lock readable to
the users who need it, or select a separately managed installation you can
access. The installer and updater also require permission to acquire the lock.

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

Open a Salesforce DX project in VS Code. The extension adds Glade Home, a Glade
Activity Bar, native Test Explorer entries, local CodeLens actions, DAP debug
launches, named local data environments, Apex & SOQL scratch buffers, and a
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

- Go 1.27.4 or newer
- Git
- A C compiler with CGO enabled (CGO is on by default). Glade's Apex parser is a
  generated tree-sitter grammar that requires CGO; without it, `check`, `test`,
  and `parse` on project sources fail with an `APEXPARSECGO` error. Install
  Xcode Command Line Tools on macOS (`xcode-select --install`) or `build-essential`
  on Debian/Ubuntu (`sudo apt-get install build-essential`).

```bash
git clone https://github.com/glade-sh/glade.git
cd glade
CGO_ENABLED=1 go build -o bin/glade ./cmd/glade
./bin/glade version
```

The binary from a source build comes without the bundled LWC toolchain. For
LWC previews, run `./bin/glade toolchain install --from .` from the checkout.
See the [LWC local shell guide](LWC_LOCAL_SHELL.md) for its Node dependencies.

Run the locally built binary against a Salesforce DX project. No Salesforce org login is
required for these local commands:

```bash
./bin/glade check --project path/to/sfdx-project --json
./bin/glade test --project path/to/sfdx-project --json
./bin/glade exec "System.debug('hello from source build');"
```

During development, run commands directly from source without creating a binary:

```bash
go run ./cmd/glade version
go run ./cmd/glade check --project path/to/sfdx-project
```

If you want `glade` on your `PATH`:

```bash
mkdir -p "$HOME/.local/bin"
install -m 0755 bin/glade ~/.local/bin/glade
export PATH="$HOME/.local/bin:$PATH"
glade version
```

## First Project Run

If you do not have a project ready, use the
[Quickstart](https://glade.sh/guide/quickstart). Its disposable
terminal sample uses commands available in the current stable release and
executes one named test before treating the setup as successful.

For an existing Salesforce DX project, run parse/check/tests without connecting
to an org:

```bash
cd path/to/sfdx-project
glade init --project . --yes
glade config validate --project .
glade config show --project .
glade doctor --project .
glade check --project .
glade test --project . --json
```

The project source default, per-class metadata, LWC bundle version, and local
HTTP route version are independent. Glade supports Apex source API 62.0–68.0
and LWC and Visualforce API 59.0–67.0. Glade preserves well-formed older source
versions outside that support window. Do not change Salesforce metadata only to make a local
result green. Glade does not log in, deploy, check hosted permissions or
services, or replace final Salesforce validation.

Install advisory scanners when needed. They are plugins, not product runtime
packages:

```bash
# Uses the default public registry. Custom registries, direct archives, and local
# links are also supported.
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
default public registry is `https://plugins.glade.sh/index.json` and serves the
three first-party packages. Direct archives and local links remain available
for offline, private, and development use. The first-party plugin catalog,
local archive install path, and author contract are documented in
[PLUGINS.md](PLUGINS.md).

Run a focused class or only tests affected by changes since a git ref:

```bash
glade test --project . --class <YourTestClass> --json
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

Download `SHA256SUMS.txt` and the platform archive
`glade_vX.Y.Z_<os>_<cpu>.tar.gz` from the [GitHub Releases page](https://github.com/glade-sh/glade/releases)
or the [downloads mirror](https://downloads.glade.sh/). On the mirror, use
`https://downloads.glade.sh/vX.Y.Z/<filename>`. Replace `vX.Y.Z` with the
published tag in both filenames and URLs. Use `darwin` for macOS or `linux`
for Linux. Choose `arm64` for Apple silicon or other ARM64 systems and
`amd64` for x86-64. The `glade-release-artifacts-*.tar.gz` file is a release
audit bundle. The platform archive contains the CLI and its bundled assets.

For a fresh install, run this after downloading both files to an empty
directory. For attestation verification, follow [Security verification](#security-verification)
before extracting. For an existing installation, use [Update](#update) so
managed assets are replaced together.

```bash
(
  set -e
  archive=glade_vX.Y.Z_linux_amd64.tar.gz
  awk -v path="./$archive" '$2 == path { print; count++ } END { if (count != 1) exit 1 }' \
    SHA256SUMS.txt > selected-checksum.txt
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c selected-checksum.txt
  else
    sha256sum -c selected-checksum.txt
  fi
  tar -xzf "$archive"
  mkdir -p "$HOME/.local/bin" "$HOME/.local/share/glade"
  cp -R share/glade/. "$HOME/.local/share/glade/"
  install -m 0755 glade "$HOME/.local/bin/glade"
  "$HOME/.local/bin/glade" version
)
```

Windows has no native release archive. See the [release asset list](../SECURITY.md#release-assets)
for the matching SBOM, Sigstore provenance bundle, and release approval files.

## CI Usage

For hosted CI, use a pinned release with `gh` and
`GLADE_REQUIRE_PROVENANCE=1`. Follow the [CI guide](https://glade.sh/guide/workflows/ci)
for report and artifact steps. Replace `vX.Y.Z` with a release you reviewed:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z GLADE_REQUIRE_PROVENANCE=1 sh
```

To build from source in Linux CI, use a separate checkout and pin its ref.
This example assumes the Salesforce DX project is at the workspace root.
For LWC previews, see [LWC_LOCAL_SHELL.md](LWC_LOCAL_SHELL.md).

```yaml
- uses: actions/checkout@v4
  with:
    repository: glade-sh/glade
    ref: vX.Y.Z
    path: .glade-source
- uses: actions/setup-go@v5
  with:
    go-version-file: .glade-source/go.mod
- run: sudo apt-get update && sudo apt-get install -y build-essential
- name: Build Glade
  working-directory: .glade-source
  run: CGO_ENABLED=1 go build -o "$RUNNER_TEMP/glade" ./cmd/glade
- name: Check and test the Salesforce project
  run: |
    if [ ! -f glade.yml ]; then
      "$RUNNER_TEMP/glade" init --project . --yes
    fi
    "$RUNNER_TEMP/glade" check --project .
    "$RUNNER_TEMP/glade" test --project . --json
```

Build from the checkout rather than `go install ...@latest`: Glade's `go.mod`
replaces the parser dependency with its checked-in local module.

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
curl -s -H 'X-Glade-Client: 1' -X POST http://127.0.0.1:8080/services/data/v65.0/glade/reset/data
curl -s -H 'X-Glade-Client: 1' -X POST 'http://127.0.0.1:8080/services/data/v65.0/glade/reset?scope=users,limits,async'
```

Use `glade db inspect --json` before and after mutating server requests as the
basic operational check. Counts should change after successful mutations and
stay fixed after failed mutations.

On loopback, the local API server accepts missing `Authorization` headers and local
`Authorization: Bearer ...` user stubs. On an exposed listener, requests require
the configured capability or a token issued by that server. Use the
`X-GLADE-User-Id` header only to select an existing local `User` record for test
requests. Direct REST DML uses that local user for system field stamping;
Tooling `executeAnonymous` still uses the VM's local default user context. Do
not expose `glade server` to untrusted networks without an authenticating reverse
proxy.

Local server, database UI, and development listeners reject unrecognized HTTP
Host authorities and browser requests from another origin. For a wildcard
network listener, set `GLADE_SERVER_ALLOWED_HOSTS` to a comma-separated list of
hostnames accepted by that listener. Tooling `executeAnonymous` over REST (all methods) and SOAP requires the
`GLADE_SERVER_CAPABILITY` value in `X-Glade-Capability`, or a server-issued token.
On exposed listeners the same requirement applies to all routes, including VF
controllers, LWC actions, Apex REST, data, composite and tooling. Set the
capability before starting the listener and use it to obtain an issued token.
`--allow-public-demo` disables this exposed-listener gate for intentional public
demos: anyone who can reach the listener can execute project code and read or
change its data. Use only disposable data in an isolated environment. Requests without an
`Origin` or trusted `Sec-Fetch-Site` metadata require the capability or an
`X-Glade-Client: 1` header from a non-browser client. User-initiated browser
navigation with `Sec-Fetch-Site: none` shows a confirmation before opening the
requested page; an authenticating reverse proxy can provide the client header
to bypass that confirmation. Treat the capability as a secret and do not expose
it to untrusted preview code. These controls do not isolate code running in a
preview from other content on the same origin. Lightning preview also requires a
trusted temporary directory; if cache creation fails, set `TMPDIR` to a directory
with secure ownership and permissions and adequate free space. Use a path
without `..` components for `TMPDIR` and test-daemon sockets.
The default test daemon also requires a trusted project and socket-directory
ancestry; move the project or temporary directory to a securely owned location
if daemon startup reports an insecure discovery directory.
On Unix, discovery checks that ancestors are owned by the current user or root
and rejects group- or world-writable directories unless they have the sticky
bit. The socket directory itself must be owned by the current user and cannot
be group- or world-writable. If optional daemon discovery is untrusted, `glade
test` runs locally instead of connecting; an explicit `--connect` reports an
error. This fallback does not authorize project toolchain discovery: the LWC
toolchain uses the installed user share or explicitly authorized roots, and
legacy project discovery requires `GLADE_ALLOW_PROJECT_TOOLCHAIN=1`. Protect
the user share and authorized roots from other users; Glade does not check their
ownership or ancestor permissions. See [LWC local shell](LWC_LOCAL_SHELL.md).

Cross-origin browser requests remain rejected even if they include the client
header. Issued tokens can be revoked through `POST /services/oauth2/revoke`
with a form-encoded `token` parameter or `GET /services/oauth2/revoke?token=…`.
Prefer POST because query parameters can be recorded in access logs. Set `GLADE_SERVER_TOKEN_TTL` to a positive
Go duration such as `30m` to expire newly issued tokens; by default they do not
expire by time. Invalid TTL settings prevent token issuance.

For a reverse proxy that terminates TLS or uses a different external port, set
`GLADE_SERVER_PROXY_ORIGINS` to a comma-separated list of exact external origins,
such as `https://preview.example.test,https://preview.example.test:8443`.
Configure the proxy to preserve the external `Host` header. The listener checks
browser origins against this list and does not trust `Forwarded` or
`X-Forwarded-*` headers for authorization. Allowlist only origins controlled by
the operator, and authenticate and restrict access at the proxy.

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
counters. Point the playground at an existing Salesforce DX project to edit that
project's supported files directly:

```bash
glade playground --examples --db .glade/playground/org.sqlite
```

```bash
glade playground --project . --db .glade/playground/org.sqlite
```

The foreground project runs as local source in the playground, even when its
Salesforce DX descriptor declares a package namespace. Managed package dependencies keep
their configured namespaces.

Use `--project-ref name=path` to add local Salesforce DX folders to the scratch
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
