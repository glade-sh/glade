# Plugin Marketplace

The marketplace is a catalog of plugin archives. Glade downloads the archive
for your OS and CPU, verifies SHA-256, verifies the archive checksums, reads
`manifest --json`, then records the install.

```bash
glade plugins available
glade plugins search
glade plugins search quality
glade plugins info @acme/quality
glade plugins install @acme/quality
```

`available` and bare `search` list every installable plugin in the configured
marketplace. Add a query when you want to narrow the list.

## Trust labels

- `first-party`: built and published by the Glade project.
- `verified-publisher`: publisher ownership has been checked.
- `community`: listed in the marketplace after review.
- `unlisted`: installed from a custom registry, direct archive, or link.

Community and unlisted installs print direct warnings. Remote archive installs
require a SHA-256 digest.

## Custom registries

```bash
glade plugins install @acme/quality --registry https://plugins.acme.com/index.json
```

Custom registries use the same JSON catalog shape as the default marketplace.

## Direct archives

```bash
glade plugins install ./glade-plugin-quality_1.2.0_darwin_arm64.tar.gz
glade plugins install https://github.com/acme/glade-plugin-quality/releases/download/v1.2.0/glade-plugin-quality_1.2.0_darwin_arm64.tar.gz --sha256 <hash>
```

Direct archive installs bypass catalog lookup. Glade still validates archive
layout, checksums, manifest fields, and command roots.
