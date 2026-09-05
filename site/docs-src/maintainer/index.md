---
pageType: hub
canonicalTask: /maintainer/
---

# Contributors {#maintainer}

Use these contributor guides when changing Glade itself. User setup stays in the guide.
Runtime coverage, release proof, plugin runtime work, and glade-tools work live
here.

Glade stays the product front door. Keep parsing, indexing, semantic checks,
the VM, the test runner, SOQL, DML, storage, schema, server runtime, and product
CLI commands in this repository.

Keep heavy maintenance work in first-party tools and plugins. That includes
compatibility fixtures, capability catalogs, dashboards, surface ledgers,
open-source corpus scans, Salesforce docs inventories, and generated maintenance
reports.

## Before changing source

Read [AGENTS.md](https://github.com/glade-sh/glade/blob/main/AGENTS.md) and
[AI contributor setup](https://github.com/glade-sh/glade/blob/main/docs/AI_SETUP.md).
Inspect the current branch, commit, and working tree; preserve unrelated owner
changes. Use Go from `go.mod` and `CGO_ENABLED=1` for declaration parsing.
Choose the focused validation for the changed surface before a broad release gate.

## Maintainer paths

<div class="docs-route-grid">
  <a class="docs-route-card" href="/maintainer/extend-runtime">
    <strong>Extend runtime support</strong>
    <span>Write the failing fixture or product test first, then patch the runtime.</span>
  </a>
  <a class="docs-route-card" href="/maintainer/glade-tools">
    <strong>glade-tools</strong>
    <span>Use the first-party toolkit for ledgers, captures, and plugin artifacts.</span>
  </a>
  <a class="docs-route-card" href="/maintainer/plugin-runtime">
    <strong>Plugin runtime</strong>
    <span>Keep plugins executable, manifest-driven, and outside the base runtime.</span>
  </a>
  <a class="docs-route-card" href="/maintainer/release">
    <strong>Release runbook</strong>
    <span>Run one proof command before product, plugin, and docs release work.</span>
  </a>
  <a class="docs-route-card" href="/maintainer/editor-extension"><strong>Develop the editor extension</strong><span>Package and validate the bundled VSIX from the checked extension source.</span></a>
</div>
