---
pageType: interactive reference
canonicalTask: /guide/workbench
aside: false
head:
  - - link
    - rel: stylesheet
      href: /css/home.css
  - - link
    - rel: stylesheet
      href: /css/workbench.css
---

# Website capability explorer

The website capability explorer maps checked APIs to their local boundary. Type Apex expressions and see whether the API runs locally, runs locally with limits, or requires Salesforce.

Use the editor as a browser capability lookup: type a dot, read the label, and see the boundary before you depend on an API. Edits stay in your browser; this page does not execute Apex or send visitor source anywhere.

[Try the capability editor](#apex-editor-heading) · [Replay a workflow](#local-apex-workbench)

<span id="capability-explorer"></span>

<GladeEditorWorkbench />

For actual execution on your machine, install Glade and run the [local Playground](/reference/cli#glade-playground):

```bash
glade playground --examples --addr 127.0.0.1:1789 --open
```

The local Playground serves your own Glade runtime. Its execution and local data are separate from the illustrations on this website.

<section class="coverage-workbench" data-coverage-workbench aria-label="Glade interactive capability map">
  <div class="coverage-workbench-intro">
    <p class="home-eyebrow">Capability cards</p>
    <h2 class="home-h2">A few surfaces worth trying first.</h2>
    <p class="home-p">These examples come from checked capability rows and the curated editor demo. Each label says whether the API runs locally, runs with named limits, or requires Salesforce.</p>
  </div>
  <div class="coverage-workbench-cards">
    <article>
      <span class="home-completion-status home-completion-status-supported">Runs locally</span>
      <code>Database.insert</code>
      <p>Partial-success DML, save results, errors, and local row changes.</p>
    </article>
    <article>
      <span class="home-completion-status home-completion-status-supported">Runs locally</span>
      <code>BusinessHours.nextStartDate</code>
      <p>Seeded schedules, time zones, holidays, and deterministic local calendar math.</p>
    </article>
    <article>
      <span class="home-completion-status home-completion-status-limited">Runs locally with limits</span>
      <code>Schema.DescribeSObjectResult</code>
      <p>Object labels, fields, record types, and child relationships from local metadata.</p>
    </article>
    <article>
      <span class="home-completion-status home-completion-status-supported">Runs locally</span>
      <code>Answers.findSimilar</code>
      <p>Deterministic empty list. Glade does not perform hosted similarity search.</p>
    </article>
  </div>
</section>


<div class="workbench-page">
  <section class="home-workbench home-panel" id="local-apex-workbench" data-scenario-workbench aria-label="Illustrative capability workflow replay">
    <div class="home-workbench-head">
      <div>
        <p class="home-eyebrow">Workflow gallery</p>
        <h2 class="home-h2">Replay an illustrative workflow.</h2>
        <p class="home-p">Replay prepared command output, JSON, trace, and results. This is a scripted illustration: no Apex runs here, and editing the capability lookup does not change these results. Copy a command to run it yourself in an initialized local project.</p>
      </div>
      <div class="home-workbench-actions">
        <span class="home-workflow-count" data-workflow-count aria-label="Workflow 1 of 4">1 / 4</span>
        <button class="home-run-button" type="button" data-run-scenario data-run-state="idle">Replay scenario</button>
        <p class="home-demo-notice">Illustrative replay — this page does not execute edited Apex.</p>
      </div>
    </div>
    <div class="home-workflow-tabs" role="tablist" aria-label="Demo workflows">
      <button id="check" class="home-workflow-tab active" type="button" role="tab" data-scenario-id="check" data-active="true" aria-selected="true" aria-controls="workbench-demo-panel">
        <span class="home-scenario-kicker"><span>Check</span><em class="home-selected-indicator" data-selected-label>Selected</em></span>
        <strong>Catch deploy issues</strong>
        <small>1 diagnostic caught</small>
      </button>
      <button id="test" class="home-workflow-tab" type="button" role="tab" data-scenario-id="test" data-active="false" aria-selected="false" aria-controls="workbench-demo-panel" tabindex="-1">
        <span class="home-scenario-kicker"><span>Test</span><em class="home-selected-indicator" data-selected-label></em></span>
        <strong>Run focused tests</strong>
        <small>1 passed · 0 failed</small>
      </button>
      <button id="exec" class="home-workflow-tab" type="button" role="tab" data-scenario-id="exec" data-active="false" aria-selected="false" aria-controls="workbench-demo-panel" tabindex="-1">
        <span class="home-scenario-kicker"><span>Exec</span><em class="home-selected-indicator" data-selected-label></em></span>
        <strong>Execute Apex locally</strong>
        <small>USER_DEBUG emitted</small>
      </button>
      <button id="debug" class="home-workflow-tab" type="button" role="tab" data-scenario-id="debug" data-active="false" aria-selected="false" aria-controls="workbench-demo-panel" tabindex="-1">
        <span class="home-scenario-kicker"><span>Debug</span><em class="home-selected-indicator" data-selected-label></em></span>
        <strong>Profile debug logs</strong>
        <small>4 events parsed</small>
      </button>
    </div>
    <div class="home-result-summary" data-result-summary data-result-state="failed" aria-live="polite">FAILED · 1 diagnostic · 1 type checked · exit code 1</div>
    <div id="workbench-demo-panel" class="home-workbench-grid" role="tabpanel" aria-labelledby="check" tabindex="0">
      <section class="home-command-panel" aria-label="Command output">
        <div class="home-panel-top">
          <strong>Command output</strong>
          <span data-command-label>glade check</span>
        </div>
        <div class="home-output-tabs" role="tablist" aria-label="Command output format">
          <button id="output-tab-output" class="home-output-tab active" type="button" role="tab" data-output-tab="output" aria-selected="true" aria-controls="command-output-panel">Output</button>
          <button id="output-tab-json" class="home-output-tab" type="button" role="tab" data-output-tab="json" aria-selected="false" aria-controls="command-output-panel" tabindex="-1">JSON</button>
          <button id="output-tab-trace" class="home-output-tab" type="button" role="tab" data-output-tab="trace" aria-selected="false" aria-controls="command-output-panel" tabindex="-1">Trace</button>
        </div>
        <pre id="command-output-panel" class="home-command-output" role="tabpanel" aria-labelledby="output-tab-output" tabindex="0"><code data-command-output data-cli-output data-output-view="output">$ glade check --project . --no-progress&#10;Glade check&#10;&#10;✗ 1 diagnostic found&#10;&#10;force-app/main/default/classes/RefinementService.cls:2:3&#10;error GLADESEMA002 method "latestInvoice" references unknown type "Invoice__c"&#10;&#10;Try:&#10;  glade schema load --project .&#10;  glade check --project .</code></pre>
      </section>
      <section class="home-source-panel" aria-label="Workflow input">
        <div class="home-panel-top">
          <strong data-source-title>Apex input</strong>
          <span data-source-label>RefinementService.cls:2</span>
        </div>
        <pre class="home-source-code" tabindex="0"><code class="language-apex" data-source-code data-highlighted-line="2">public with sharing class RefinementService {&#10;  public static Invoice__c latestInvoice() {&#10;    return null;&#10;  }&#10;}</code></pre>
      </section>
      <section class="home-changed-panel" aria-label="What changed">
        <div class="home-panel-top">
          <strong data-proof-title>Illustrative result</strong>
          <span data-support-status>supported locally</span>
        </div>
        <ul data-changed-summary>
          <li>Deploy-blocking type reference caught locally.</li>
          <li>No Salesforce deploy required.</li>
          <li>No local records changed.</li>
          <li>JSON output available for CI.</li>
        </ul>
      </section>
      <section class="home-result-panel" aria-label="Command result">
        <div class="home-panel-top">
          <strong>Result</strong>
          <span class="home-status home-status-failed" data-result-status><span class="home-status-dot"></span>failed</span>
        </div>
        <dl class="home-result-metrics" data-result-metrics>
          <div><dt>Diagnostics</dt><dd>1</dd></div>
          <div><dt>Types checked</dt><dd>1</dd></div>
          <div><dt>Org calls</dt><dd>0</dd></div>
          <div><dt>Exit code</dt><dd>1</dd></div>
        </dl>
      </section>
    </div>
    <div class="home-command-strip">
      <div class="home-command-row">
        <span>Local run</span>
        <code id="workbench-command" data-command-strip>glade check --project . --no-progress</code>
        <button type="button" data-copy-target="workbench-command" aria-label="Copy workbench command">Copy command</button>
      </div>
      <div class="home-command-row">
        <span>CI / automation</span>
        <code id="workbench-ci-command" data-ci-command-strip>glade check --project . --json --no-progress</code>
        <button type="button" data-copy-target="workbench-ci-command" aria-label="Copy workbench JSON command">Copy JSON command</button>
      </div>
      <div class="home-command-strip-foot">
        <a href="/reference/cli" data-docs-link>View docs</a>
        <span class="home-copy-status" data-copy-status role="status" aria-live="polite"></span>
      </div>
    </div>
  </section>
</div>
