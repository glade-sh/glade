# Plugins

Most Glade work does not require plugins. The base runtime covers local Apex
checks, focused tests, anonymous Apex, SOQL, DML, local data, VS Code, and CI
output.

Use plugins when you need a first-party extension that stays outside the base runtime.
Glade plugins are standalone executables installed and run through
`glade plugins`.

::: tip Default registry
The default public registry at `https://plugins.glade.sh/index.json` serves
`@glade/compat`, `@glade/orgpackage`, and `@glade/performance` at `0.2.9`.
Base Glade install and local Apex workflows do not require plugins. Direct
archives and local links remain available for offline, private, and development
use.
:::

## First-party plugins

- `@glade/performance` - advisory Salesforce performance scans.
- `@glade/orgpackage` - captures installed package contracts from a Salesforce org
  into a Glade package artifact.

```bash
glade plugins available
glade plugins install @glade/performance
glade plugins install @glade/orgpackage
```

The short aliases `performance` and `orgpackage` resolve to
`@glade/performance` and `@glade/orgpackage`. After `@glade/orgpackage` is
installed or linked, glade package capture ... dispatches to `glade orgpackage capture ...`.

## Maintainer support tools

`@glade/compat` is a first-party maintainer plugin. It carries compatibility
fixtures, runtime capability reports, compatibility dashboards, and support
scanners used to extend and check Glade itself. Keep that work out of the
base runtime. Use the maintainer lane when you need it.

## Registry and third-party plugins

```bash
glade plugins available
glade plugins search
glade plugins search quality
glade plugins info @acme/quality
glade plugins install @acme/quality
```

Registry catalogs are configured. Third-party publishers can also use a custom
registry or a direct archive URL. `available` and bare `search` list the
installable catalog before you know a plugin name.

Install a remote archive only with a pinned digest:

```bash
glade plugins install https://github.com/acme/glade-plugin-quality/releases/download/v1.2.0/glade-plugin-quality_1.2.0_darwin_arm64.tar.gz --sha256 <hash>
```

Team and CI lock files should restore registry or archive installs.

## Plugin docs

<div class="docs-route-grid">
  <a class="docs-route-card" href="/guide/plugins/first-party">
    <strong>First-party plugins</strong>
    <span>Choose first-party workflows when the base runtime is not enough.</span>
  </a>
  <a class="docs-route-card" href="/guide/plugins/install-manage">
    <strong>Install and manage</strong>
    <span>Install registry, archive, and linked plugins.</span>
  </a>
  <a class="docs-route-card" href="/guide/plugins/lock-ci">
    <strong>Lock files and CI</strong>
    <span>Restore the same plugin set on another machine.</span>
  </a>
</div>
