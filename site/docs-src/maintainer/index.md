# Maintainer

This lane is for people changing Glade itself. User setup stays in the guide.
Runtime coverage, release proof, plugin runtime work, and glade-tools work live
here.

glade stays the product front door. Keep parsing, indexing, semantic checks,
the VM, the test runner, SOQL, DML, storage, schema, server runtime, and product
CLI commands in this repository.

Keep heavy maintenance work in first-party tools and plugins. That includes
compatibility fixtures, capability catalogs, dashboards, surface ledgers,
example-project scans, Salesforce docs inventories, and generated maintenance
reports.

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
</div>
