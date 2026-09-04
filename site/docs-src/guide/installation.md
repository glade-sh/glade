# Installation

<script setup>
import releaseManifest from '../../release-manifest.json'
</script>

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Install the stable Glade binary on macOS or Linux, verify its version, then continue inside a Salesforce DX project.</p>
</div>

Verified stable release: **{{ releaseManifest.version }}**

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

The script installs the current stable release to `~/.local/bin` by default.
Expected: `glade version` prints **{{ releaseManifest.version }}**. If the shell
cannot find `glade`, repair the current shell with:

```bash
export PATH="$HOME/.local/bin:$PATH"
glade version
```

Add the PATH line to your shell configuration to retain it in new terminals.

Override the destination or pin a release when needed:

```bash
curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR=/usr/local/bin sh
curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z sh
```

Replace `vX.Y.Z` with the exact published release you reviewed. CI examples in
this guide pin their tested release directly.

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
check. Continue with the [first local check](/guide/quickstart), which enters or
initializes a Salesforce DX project before running:

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
manual verification path. It resolves the version and archive from the same
published manifest used by the installer and this page.

Its fail-closed release check verifies the CycloneDX attestation:

```bash
gh attestation verify "$GLADE_ARCHIVE" -R glade-sh/glade \
  --predicate-type https://cyclonedx.org/bom
```
