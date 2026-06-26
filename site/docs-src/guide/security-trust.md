# Security & Trust

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Security</p>
  <p>Glade is a local binary. The proof trail is checks, checksums, SBOMs, and release provenance.</p>
  <ul>
    <li>Security scans run in GitHub Actions.</li>
    <li>Release archives publish checksums and attestations.</li>
    <li>Supported local checks do not require a Salesforce org login.</li>
  </ul>
</div>

Security review works best when the facts are close to the installer. Glade
keeps its security proof in the repository, the release workflow, and the
release assets.

## CI gates

| Gate | What it checks |
| --- | --- |
| OpenSSF Scorecard | Repository posture; public badge after the repository is public. |
| govulncheck | Reachable Go vulnerabilities in modules and the Go standard library. |
| CodeQL | GitHub code scanning for Go with the security-extended query suite. |
| gosec | Go source-pattern findings uploaded as SARIF. |
| npm audit | High-severity production dependency findings in packaged JavaScript. |
| Dependency Review | Pull requests that add vulnerable dependencies. |

`gosec` reports are uploaded while the existing baseline is triaged. New
high-severity findings should be fixed or documented before release.

## Release proof

Tagged releases publish:

- Platform archives for macOS and Linux.
- `SHA256SUMS.txt`.
- `release-manifest.json`.
- A CycloneDX SBOM for each archive.
- Artifact attestations for the archive and SBOM.

Verify a downloaded archive:

```bash
curl -L -o glade.tar.gz "$GLADE_RELEASE_URL"
curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
shasum -a 256 -c SHA256SUMS.txt
gh attestation verify glade.tar.gz -R glade-sh/glade
tar -xzf glade.tar.gz
./glade doctor
```

## Laptop behavior

Glade runs on the developer machine. Supported local checks read Salesforce DX
project files, parse Apex, run supported tests, execute supported snippets, and
write optional local run artifacts.

No Salesforce org login is required for supported local checks.

## Network access

Glade uses the network when installing or updating release archives, installing
the local LWC toolchain, or installing plugins from a registry or archive URL.
The local check, test, parse, exec, SOQL, DML, and local API paths do not send
project source to a hosted Glade service.

## Local storage

Glade can write project state under `.glade/`, `glade.yml`, SQLite databases
named by `--db`, plugin files, editor integration files, and LWC toolchain data
under the OS user data directory.

## Review links

- [Security policy](https://github.com/glade-sh/glade/blob/main/SECURITY.md)
- [Security & Trust repo note](https://github.com/glade-sh/glade/blob/main/docs/SECURITY_TRUST.md)
- [Install guide](/guide/installation)
- [Release runbook](/maintainer/release)
