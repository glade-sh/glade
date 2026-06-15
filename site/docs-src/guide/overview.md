# What is Glade?

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Start</p>
  <p>Glade is a local Apex runtime and developer workbench for supported checks, tests, snippets, reports, and Salesforce-shaped local APIs.</p>
  <ul>
    <li>See what runs on your machine.</li>
    <li>Know when to keep Salesforce in the loop.</li>
    <li>Start the first local check and test path.</li>
  </ul>
</div>

Glade is a local Apex runtime and developer workbench. It loads Salesforce DX
projects, parses and checks supported Apex, runs local Apex tests, executes
anonymous Apex, serves local Visualforce pages and a Salesforce-shaped local
REST API, and exposes support gaps instead of hiding them.

## Start with

<div class="docs-route-grid">
  <a class="docs-route-card" href="/guide/quickstart">
    <strong>Quickstart</strong>
    <span>Install Glade and run the first check.</span>
  </a>
  <a class="docs-route-card" href="/guide/support-map">
    <strong>Support map</strong>
    <span>See what runs locally and where boundaries start.</span>
  </a>
  <a class="docs-route-card" href="/guide/cli-reference">
    <strong>CLI Reference</strong>
    <span>Find commands, flags, and common examples.</span>
  </a>
  <a class="docs-route-card" href="/guide/playground">
    <strong>Playground</strong>
    <span>Run built-in examples in a browser workbench.</span>
  </a>
</div>

## First local loop

```bash
glade doctor
glade init --project . --yes
glade check --project .
glade test --project . --class AccountServiceTest
glade test changed --project . --since origin/main
```

## Use Glade when

- You want Apex diagnostics before a deploy.
- You want to run supported Apex tests without logging into an org.
- You want local SOQL, DML, trigger, SObject, and limit feedback.
- You want a Visualforce preview feature for local `/apex/<PageName>` rendering.
- You want an LWC preview feature through `/lwc/preview/*` routes or Visualforce Lightning Out.
- You want a Salesforce-shaped local API for development loops.
- You want local assessment, cruft review, or refactor-proof reports for a large Apex project.
- You want deterministic local harnesses for supported platform helper rows instead of live hosted service calls.

## Use Salesforce when

- You need live auth, sessions, identity, or org-hosted process engines.
- You need exact Salesforce-hosted Visualforce chrome, lifecycle timing, or byte-for-byte PDF output.
- You need exact Salesforce-hosted Lightning Experience chrome, console navigation, permissions, full UI API, or every base component edge.
- You need Bulk API beyond simple scalar local query whole-result CSV, Streaming, Pub/Sub, GraphQL, metadata deploy/retrieve jobs, or Tooling objects beyond the checked local source/schema metadata baseline.
- You need exact production governor accounting.

## Support claims

Glade models the local paths it can prove. Unsupported platform services fail
with stable diagnostics instead of pretending to work.

Next: [Tester field guide](/guide/tester-field-guide), [Quickstart](/guide/quickstart), [Enterprise workflows](/guide/enterprise-workflows), or [Support map](/guide/support-map).
