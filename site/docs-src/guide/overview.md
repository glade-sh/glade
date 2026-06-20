# What is Glade?

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Start</p>
  <p>Glade is a local Apex runtime and developer workbench for supported checks, tests, snippets, reports, and local Salesforce API routes.</p>
  <ul>
    <li>See what runs on your machine.</li>
    <li>Know when to keep Salesforce in the loop.</li>
    <li>Start the first local check and test path.</li>
  </ul>
</div>

Glade is a local Apex runtime and developer workbench. It loads Salesforce DX
projects, parses and checks supported Apex, runs local Apex tests, executes
anonymous Apex, serves local Visualforce pages and local Salesforce API routes,
and exposes support gaps instead of hiding them.

## Start with

<div class="docs-route-grid">
  <a class="docs-route-card" href="/guide/installation">
    <strong>Install</strong>
    <span>Install Glade and check the binary.</span>
  </a>
  <a class="docs-route-card" href="/guide/tester-field-guide">
    <strong>Tester field guide</strong>
    <span>Run a first project evaluation with CLI, VS Code, CI, and handoff proof.</span>
  </a>
  <a class="docs-route-card" href="/guide/ai-assisted-apex">
    <strong>AI-assisted Apex</strong>
    <span>Add a reusable Glade and TDD prompt to your AI coding agent.</span>
  </a>
  <a class="docs-route-card" href="/guide/quickstart">
    <strong>First local check</strong>
    <span>Run the shortest check and test loop.</span>
  </a>
  <a class="docs-route-card" href="/guide/support-map">
    <strong>What runs locally</strong>
    <span>Check local behavior and what still requires Salesforce.</span>
  </a>
  <a class="docs-route-card" href="/guide/workbench">
    <strong>Interactive capability map</strong>
    <span>Try checked Apex support labels in the browser workbench.</span>
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
- You want local Salesforce API routes for development loops.
- You want local assessment, cruft review, or refactor-proof reports for a large Apex project.
- You want deterministic local harnesses for supported platform helper rows instead of live hosted service calls.

## Use Salesforce when

- You need live auth, sessions, identity, or org-hosted process engines.
- You need exact Salesforce-hosted Visualforce chrome, lifecycle timing, or byte-for-byte PDF output.
- You need exact Salesforce-hosted Lightning Experience chrome, console navigation, permissions, full UI API, or every base component edge.
- You need Bulk API beyond simple scalar local query whole-result CSV, Streaming, Pub/Sub, GraphQL, metadata deploy/retrieve jobs, or Tooling objects beyond the checked local source/schema metadata baseline.
- You need exact production governor accounting.

## Capability claims

Glade models the local paths it can prove. Unsupported platform services fail
with stable diagnostics instead of pretending to work.

Next: [Tester field guide](/guide/tester-field-guide), [Quickstart](/guide/quickstart), [Enterprise workflows](/guide/enterprise-workflows), or [What Glade runs locally](/guide/support-map).
