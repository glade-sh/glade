# Plugin Marketplace And Docs Design

**Date:** 2026-06-11
**Status:** Approved
**Goal:** Make first-party and third-party Glade plugins feel like one cohesive
Glade system while keeping the default Glade install small.

---

## Context

Glade now has an executable plugin host. The product CLI owns the normal
runtime commands and `glade plugins`. First-party heavy tools live in the
sibling `glade-tools` repository and ship as plugin archives. The initial
first-party plugins are:

- `@glade/compat` for compatibility fixtures, local-test readiness, surface
  ledgers, generated support reports, and parity scanners.
- `@glade/performance` for advisory Salesforce performance scans.

The current docs describe plugin mechanics, but they still make plugins feel
adjacent to Glade. The desired user experience is one Glade docs site, one
Glade plugin command family, and one marketplace/catalog model.

## Decisions

1. `install.sh` installs only the `glade` binary.
2. Glade plugins do not use npm as a transport or runtime dependency.
3. Plugin names use npm-like coordinates:
   - `@glade/compat`
   - `@glade/performance`
   - `@acme/quality`
   - `@acme/quality@1.2.0`
4. First-party aliases remain available:
   - `compat` resolves to `@glade/compat`
   - `performance` resolves to `@glade/performance`
5. Lock files store canonical names and exact versions.
6. The default marketplace starts as curated. It is not an open package
   commons in v1.
7. Third-party publishers can use custom registries or direct archive URLs
   without waiting for default marketplace inclusion.

## User Model

Install Glade:

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

Install first-party plugins:

```bash
glade plugins install @glade/compat
glade plugins install @glade/performance
```

Search and inspect marketplace plugins:

```bash
glade plugins search quality
glade plugins info @acme/quality
```

Install a third-party plugin from the default marketplace:

```bash
glade plugins install @acme/quality
glade plugins install @acme/quality@1.2.0
```

Install from a publisher-owned registry:

```bash
glade plugins install @acme/quality --registry https://plugins.acme.com/index.json
```

Install a release archive directly:

```bash
glade plugins install ./glade-plugin-quality_1.2.0_darwin_arm64.tar.gz
glade plugins install https://github.com/acme/glade-plugin-quality/releases/download/v1.2.0/glade-plugin-quality_1.2.0_darwin_arm64.tar.gz --sha256 <hash>
```

Link a local development binary:

```bash
glade plugins link --exec ./glade-plugin-quality
```

## Marketplace Architecture

The marketplace is a catalog. It does not replace archive installation.

Catalog entries describe:

- canonical plugin name
- aliases
- version
- publisher identity
- trust label
- summary
- command roots
- docs URL
- source URL
- supported platforms
- archive URLs
- archive SHA-256 values
- minimum Glade version

The default registry URL remains the normal install source for curated plugins.
Custom registries use the same JSON shape. Direct archive installs bypass the
catalog but still validate archive layout, checksums, manifest, and command
roots.

## Registry Shape

The registry index should evolve from bare name/version assets into catalog
metadata:

```json
{
  "version": 1,
  "plugins": [
    {
      "name": "@glade/compat",
      "aliases": ["compat"],
      "version": "0.1.0",
      "publisher": "glade",
      "trust": "first-party",
      "summary": "Compatibility fixtures, surface ledgers, and maintenance scanners.",
      "commands": ["compat", "surface", "local-tests", "post-parity", "examples", "dashboard", "gaps", "stdlib"],
      "docsURL": "https://glade.sh/guide/plugins/compat",
      "sourceURL": "https://github.com/glade-sh/glade-tools",
      "minimumGladeVersion": "0.1.0",
      "assets": [
        {
          "os": "darwin",
          "arch": "arm64",
          "url": "https://github.com/glade-sh/glade-tools/releases/download/v0.1.0/glade-plugin-compat_0.1.0_darwin_arm64.tar.gz",
          "sha256": "<archive sha256>"
        }
      ]
    }
  ]
}
```

The installed manifest remains authoritative for runtime dispatch. Registry
metadata helps users choose and helps Glade validate that the downloaded plugin
matches the catalog entry.

## Trust Labels

The marketplace uses plain trust labels:

- `first-party`: built and published by the Glade project.
- `verified-publisher`: publisher ownership has been checked.
- `community`: listed in the marketplace, but publisher identity has not been
  verified beyond submission review.
- `unlisted`: installed by custom registry, archive path, archive URL, or link.

Warnings should be direct:

- community plugin installs print a warning.
- unlisted remote archive installs require `--sha256`.
- CI installs of community or unlisted plugins require `--yes` or a lock file.

## Publish Flow

V1 marketplace publication is PR-based.

1. Author builds platform archives.
2. Author publishes archives at durable release URLs.
3. Author submits a marketplace entry by PR.
4. CI downloads every asset.
5. CI verifies archive SHA-256.
6. CI verifies archive layout:
   - `plugin.json`
   - `checksums.txt`
   - `bin/glade-plugin-<name>`
7. CI verifies `checksums.txt`.
8. CI runs `manifest --json`.
9. CI rejects unsafe names, unsafe versions, missing assets, broken docs links,
   and command roots that collide with built-in Glade commands.
10. Maintainers merge approved entries.

Later, this can become a hosted publisher portal. The registry contract should
not require that portal.

## Lock Files And Restore

`glade plugins lock` writes canonical package coordinates and exact asset
identity:

```json
{
  "version": 1,
  "plugins": [
    {
      "name": "@acme/quality",
      "version": "1.2.0",
      "registry": "https://plugins.glade.sh/index.json",
      "os": "darwin",
      "arch": "arm64",
      "sha256": "<archive sha256>"
    }
  ]
}
```

`glade plugins restore` never installs `latest`. It installs the exact locked
version and verifies the locked digest. This makes plugin installs predictable
in CI.

## Docs Site Information Architecture

The main Glade docs site owns the plugin story.

Sidebar section:

- Plugins
  - Overview
  - First-Party Plugins
  - Marketplace
  - Install And Manage Plugins
  - Build A Plugin
  - Publish A Plugin
  - Manifest Reference
  - Lock Files And CI

First-party plugin pages live in the main docs site:

- `@glade/compat`
- `@glade/performance`

Third-party marketplace entries appear in the main docs as catalog cards or a
searchable list. Each entry links to external docs by default. A third-party
plugin only gets hosted first-party-style docs if the Glade project chooses to
curate that documentation.

Installation docs keep the first run lean, then point to optional first-party
plugin installs. Compatibility, support-map, and performance docs should link
to the relevant plugin page when the feature requires a plugin.

## CLI Surface

New or expanded commands:

```bash
glade plugins search <query>
glade plugins info <name>
glade plugins install <name>[@version] [--registry <url>] [--yes]
glade plugins install <archive-path>
glade plugins install <archive-url> --sha256 <hash> [--yes]
```

Existing commands remain:

```bash
glade plugins list
glade plugins which <command-root>
glade plugins doctor
glade plugins remove <name>
glade plugins lock
glade plugins restore
glade plugins link --exec <path>
```

The command parser must distinguish:

- scoped registry coordinates: `@scope/name`, `@scope/name@version`
- short first-party aliases: `compat`, `performance`
- local archive paths
- remote archive URLs

## Error Handling

Errors should name the bad material:

- unknown plugin name: suggest `glade plugins search <term>`.
- ambiguous alias: print matching canonical names.
- unsupported platform: list available platforms.
- checksum mismatch: print the asset URL and expected digest.
- manifest/catalog mismatch: print the field that disagrees.
- command collision: print the colliding root and owning built-in command.
- untrusted CI install: explain `--yes` or lock-file restore.

No error should advise npm install.

## Testing

Focused product tests:

```bash
go test ./internal/pluginhost ./internal/gladecli -count=1
```

Docs checks:

```bash
npm --prefix site run build
git diff --check
```

Marketplace contract tests should cover:

- scoped coordinate parsing.
- alias resolution.
- exact version selection.
- custom registry install.
- direct URL install requires `--sha256`.
- lock writes canonical names.
- restore installs exact versions.
- trust warning behavior.
- manifest/catalog mismatch rejection.

Release proof should build first-party plugin archives, install them through the
same code path users run, and verify:

```bash
glade plugins doctor
glade compat local-tests --help
glade performance scan --help
```

## V1 Scope

In scope:

- scoped plugin coordinates.
- first-party aliases.
- curated marketplace JSON.
- custom registry install.
- direct archive URL install with required SHA-256.
- marketplace search and info.
- main docs site plugin section.
- first-party plugin docs in the main docs site.
- lock and restore canonicalization.

Out of scope:

- npm transport.
- Node dependency.
- hosted publisher portal.
- automatic plugin install from `install.sh`.
- auto-update daemon.
- runtime plugin sandboxing beyond executable process boundaries.

## Open Edges

None for v1 design. Publisher portal, signatures beyond SHA-256, and hosted
third-party docs are future work.
