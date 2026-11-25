---
pageType: guide
canonicalTask: /guide/installation
---

# Installation

<script setup>
import releaseManifest from '../../release-manifest.json'
</script>

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Install the stable Glade binary on macOS or Linux, verify its version, then continue inside a Salesforce DX project.</p>
</div>

Stable download release: **{{ releaseManifest.version }}**

## Before you start

Use a terminal on one of the supported OS and CPU pairs below. The installer
needs `curl`, a working Python 3, `mkfifo`, and either `shasum` or
`sha256sum`. Python 3 coordinates the installation lock. On macOS, install
the Xcode Command Line Tools with `xcode-select --install` or put another
Python 3 on your `PATH`. The GitHub CLI (`gh`) enables attestation verification.
Installation does not require a Salesforce DX project or a Salesforce login.

## Supported release archives

| OS | CPU | Status |
| --- | --- | --- |
| macOS | arm64 | supported release archive |
| macOS | amd64 | supported release archive |
| Linux | amd64 | supported release archive |
| Linux | arm64 | supported release archive |
| Windows | amd64/arm64 | no native release archive; source builds and WSL are not verified by this walkthrough |

## Install and verify

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
```

The script installs the `glade` binary, including parser support, to
`~/.local/bin`. It installs the LWC runtime/toolchain and bundled editor assets
under `~/.local/share/glade` by default. Set `GLADE_INSTALL_DIR` and
`GLADE_HOME` to change those destinations.

`glade version` prints the installed version. This site's release
snapshot is **{{ releaseManifest.version }}**. The live download channel can
advance before the site rebuilds. If the shell cannot find `glade`, repair the
current shell with:

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

Override both destinations or pin a release when needed:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR=/usr/local/bin GLADE_HOME="$HOME/.local/share/glade" sh
curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z sh
```

Replace `vX.Y.Z` with the exact published release you reviewed. CI examples in
this guide pin their tested release directly.

### Require release attestations

The installer checks the archive's SHA-256 checksum. If `gh` is available, it
also verifies release-workflow provenance and the CycloneDX attestation before
extracting the archive. If `gh` is missing, it reports that provenance was
not checked and continues after the checksum passes.

To require both attestations, install `gh` and run:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_REQUIRE_PROVENANCE=1 sh
```

A failed check stops the installer. It prefers the per-archive Sigstore
provenance bundle and can fetch public attestations without a GitHub login.
If public lookup fails, sign in with `gh auth login` and retry. See
[installer troubleshooting](/help/troubleshooting#the-installer-stops).

## Manual download

Get `SHA256SUMS.txt` and `glade_vX.Y.Z_<os>_<cpu>.tar.gz` from
[GitHub Releases](https://github.com/glade-sh/glade/releases) or
`https://downloads.glade.sh/vX.Y.Z/<filename>`. Replace `vX.Y.Z` with the
published tag. For `<os>`, choose `darwin` for macOS or `linux`. For
`<cpu>`, choose `arm64` for Apple silicon or other ARM64 systems, or
`amd64` for x86-64. The `glade-release-artifacts-*.tar.gz` asset is the
release audit bundle.

Use the [manual install steps](https://github.com/glade-sh/glade/blob/main/docs/INSTALL.md#manual-install)
to verify the checksum with `shasum` or `sha256sum` and install the archive.
See the [release asset list](https://github.com/glade-sh/glade/blob/main/SECURITY.md#release-assets)
for the SBOM, Sigstore provenance bundle, and release approval files.

## Update

Preview the installer command before allowing self-replacement:

```bash
glade update --dry-run
GLADE_UPDATE_ALLOW_SHELL=1 glade update
glade version
```

`glade update` updates the directory that contains the current `glade` binary.
The environment guard keeps self-replacement explicit.

## Continue in a project

`glade doctor` is project-aware. Do not use it as the binary installation
check. Follow the [first local check](/guide/quickstart) in the Quickstart.
Enter or initialize a Salesforce DX project before running:

```bash
glade doctor --project .
```

## Other installation paths

<div class="docs-route-grid docs-install-grid">
  <a class="docs-route-card docs-install-card" href="/guide/security-trust#release-proof"><strong>Verify a release</strong><span>Pin the stable manifest, select the archive, verify checksums, SBOM, and attestation.</span></a>
  <a class="docs-route-card docs-install-card" href="/guide/editor"><strong>Install the VS Code extension</strong><span>Install the bundled extension and open Glade Home.</span></a>
  <a class="docs-route-card docs-install-card" href="/guide/workflows/ci"><strong>Install in CI</strong><span>Pin and verify the binary before noninteractive checks.</span></a>
  <a class="docs-route-card docs-install-card" href="/guide/build-from-source"><strong>Build from source</strong><span>Use for Glade development or unreleased repository changes.</span></a>
</div>

The [security and release trust guide](/guide/security-trust) is the canonical
manual verification path. The [verification-only script](https://github.com/glade-sh/glade/blob/main/scripts/verify-release-download.sh)
checks the archive checksum, release-workflow provenance, and CycloneDX
attestation without extracting or running Glade.
