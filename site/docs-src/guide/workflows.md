---
pageType: hub
canonicalTask: /guide/workflows
---

# Choose a Glade workflow

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guides</p>
  <p>Start with the job in front of you. Each path names the interface, project setup, expected local result, and Salesforce boundary.</p>
</div>

## Daily code loop

<div class="docs-route-grid docs-workflow-grid">
  <a class="docs-route-card" href="/guide/quickstart"><strong>Check Apex source</strong><span>Get a named local diagnostic or a clean result.<br>Start: <code>glade check --project .</code><br>Interface: CLI or VS Code · Requires: project</span></a>
  <a class="docs-route-card" href="/guide/workflows/apex-tests"><strong>Run Apex tests</strong><span>Run all, focused, changed, or failed tests locally.<br>Start: <code>glade test --project .</code><br>Interface: CLI or VS Code · Requires: project</span></a>
  <a class="docs-route-card" href="/guide/workflows/debug-apex"><strong>Debug Apex</strong><span>Use breakpoints or profile a saved log or local trace.<br>Start: <code>glade dap --project .</code><br>Interface: CLI or VS Code · Requires: project</span></a>
  <a class="docs-route-card" href="/guide/playground"><strong>Execute Apex and SOQL</strong><span>Run anonymous Apex and queries with the CLI or your locally hosted Playground.<br>Start: <code>glade exec --project . "System.debug('local');"</code><br>Interface: CLI, VS Code, or local Playground · Requires: project</span></a>
</div>

## Local state

<div class="docs-route-grid docs-workflow-grid">
  <a class="docs-route-card" href="/guide/workflows/local-data"><strong>Work with local data</strong><span>Create a named SQLite-backed environment, seed records, and use supported local API routes.<br>Start: <code>glade org create refinement-local</code><br>Interface: CLI or VS Code · Requires: project and local data</span></a>
</div>

## UI preview

<div class="docs-route-grid docs-workflow-grid">
  <a class="docs-route-card" href="/guide/workflows/lwc-preview"><strong>Preview LWC</strong><span>Open local component and page routes in the Workbench Console.<br>Start: <code>glade dev lwc --project . --open</code><br>Interface: browser · Requires: project and toolchain</span></a>
  <a class="docs-route-card" href="/guide/workflows/visualforce-preview"><strong>Preview Visualforce</strong><span>Serve supported pages and controller flows locally.<br>Start: <code>glade dev vf --project .</code><br>Interface: browser · Requires: project</span></a>
</div>

## Extend your workflow

- [Use an AI assistant](/guide/ai-assisted-apex) with a repeatable local check and test loop.
- [Review a large project](/guide/enterprise-workflows) with assessment, graph, and refactor reports.
- [Choose plugins](/guide/plugins) for advisory scans or authorized package capture.
- [Pilot Glade with a team](/guide/tester-field-guide) before promoting a local path to a CI gate.

## Team automation

<div class="docs-route-grid docs-workflow-grid">
  <a class="docs-route-card" href="/guide/workflows/ci"><strong>Add Glade to CI</strong><span>Publish JSON, SARIF, JUnit, and stable exit-code evidence.<br>Start: <code>glade check --project . --json</code><br>Interface: CI · Requires: project and pinned binary</span></a>
</div>

Use [How Glade works](/guide/modules) for subsystem boundaries, the [CLI
reference](/reference/cli) for exact flags, and [What runs locally](/guide/support-map)
before relying on a hosted Salesforce edge.
