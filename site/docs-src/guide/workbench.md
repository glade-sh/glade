---
aside: false
---

# Local coverage workbench

See what Glade runs locally, what has limits, and what still belongs to Salesforce.

Use the editor as a live capability map: type a dot, read the label, and see the boundary before you depend on an API.

<section class="coverage-workbench" data-coverage-workbench aria-label="Glade local coverage workbench">
  <div class="coverage-workbench-intro">
    <p class="home-eyebrow">Coverage cards</p>
    <h2 class="home-h2">A few surfaces worth trying first.</h2>
    <p class="home-p">These examples come from checked support rows and the curated editor demo. Green means Glade has local behavior. Yellow means useful local behavior with named limits. Red means Salesforce owns that service.</p>
  </div>
  <div class="coverage-workbench-cards">
    <article>
      <span class="home-completion-status home-completion-status-supported">Works well</span>
      <code>Database.insert</code>
      <p>Partial-success DML, save results, errors, and local row changes.</p>
    </article>
    <article>
      <span class="home-completion-status home-completion-status-supported">Works well</span>
      <code>BusinessHours.nextStartDate</code>
      <p>Seeded schedules, time zones, holidays, and deterministic local calendar math.</p>
    </article>
    <article>
      <span class="home-completion-status home-completion-status-limited">With limits</span>
      <code>Schema.DescribeSObjectResult</code>
      <p>Object labels, fields, record types, and child relationships from local metadata.</p>
    </article>
    <article>
      <span class="home-completion-status home-completion-status-salesforce">Needs Salesforce</span>
      <code>Answers.findSimilar</code>
      <p>Hosted Answers service data stays marked instead of being silently faked.</p>
    </article>
  </div>
</section>

<GladeEditorWorkbench />

<div class="workbench-page">
  <section class="home-workbench home-panel" id="local-apex-workbench" data-scenario-workbench aria-label="Local coverage workflow demo">
    <div class="home-workbench-head">
      <div>
        <p class="home-eyebrow">Workflow gallery</p>
        <h2 class="home-h2">Run a scenario and see the proof.</h2>
        <p class="home-p">Run a scenario to see the command, JSON, trace, local result, and copyable CLI form.</p>
      </div>
      <div class="home-workbench-actions">
        <span class="home-workflow-count" data-workflow-count aria-label="Workflow 1 of 4">1 / 4</span>
        <button class="home-run-button" type="button" data-run-scenario data-run-state="idle">Run local check</button>
      </div>
    </div>
    <div class="home-workflow-tabs" role="tablist" aria-label="Demo workflows">
      <button class="home-workflow-tab active" type="button" role="tab" data-scenario-id="check" data-active="true" aria-pressed="true" aria-selected="true">
        <span class="home-scenario-kicker"><span>Check</span><em class="home-selected-indicator" data-selected-label>Selected</em></span>
        <strong>Catch deploy issues</strong>
        <small>1 diagnostic caught</small>
      </button>
      <button class="home-workflow-tab" type="button" role="tab" data-scenario-id="test" data-active="false" aria-pressed="false" aria-selected="false">
        <span class="home-scenario-kicker"><span>Test</span><em class="home-selected-indicator" data-selected-label></em></span>
        <strong>Run focused tests</strong>
        <small>1 passed · 0 failed</small>
      </button>
      <button class="home-workflow-tab" type="button" role="tab" data-scenario-id="exec" data-active="false" aria-pressed="false" aria-selected="false">
        <span class="home-scenario-kicker"><span>Exec</span><em class="home-selected-indicator" data-selected-label></em></span>
        <strong>Execute Apex locally</strong>
        <small>USER_DEBUG emitted</small>
      </button>
      <button class="home-workflow-tab" type="button" role="tab" data-scenario-id="debug" data-active="false" aria-pressed="false" aria-selected="false">
        <span class="home-scenario-kicker"><span>Debug</span><em class="home-selected-indicator" data-selected-label></em></span>
        <strong>Profile debug logs</strong>
        <small>4 events parsed</small>
      </button>
    </div>
    <div class="home-result-summary" data-result-summary data-result-state="failed" aria-live="polite">FAILED · 1 diagnostic · 1 type checked · exit code 1</div>
    <div class="home-workbench-grid">
      <section class="home-command-panel" aria-label="Command output">
        <div class="home-panel-top">
          <strong>Command output</strong>
          <span data-command-label>glade check</span>
        </div>
        <div class="home-output-tabs" role="tablist" aria-label="Command output format">
          <button id="output-tab-output" class="home-output-tab active" type="button" role="tab" data-output-tab="output" aria-pressed="true" aria-selected="true" aria-controls="command-output-panel">Output</button>
          <button id="output-tab-json" class="home-output-tab" type="button" role="tab" data-output-tab="json" aria-pressed="false" aria-selected="false" aria-controls="command-output-panel">JSON</button>
          <button id="output-tab-trace" class="home-output-tab" type="button" role="tab" data-output-tab="trace" aria-pressed="false" aria-selected="false" aria-controls="command-output-panel">Trace</button>
        </div>
        <pre id="command-output-panel" class="home-command-output" role="tabpanel" aria-labelledby="output-tab-output"><code data-command-output data-cli-output data-output-view="output">$ glade check --project . --no-progress&#10;Glade check&#10;&#10;✗ 1 diagnostic found&#10;&#10;force-app/main/default/classes/AccountService.cls:2:3&#10;error GLADESEMA002 method "latestInvoice" references unknown type "Invoice__c"&#10;&#10;Try:&#10;  glade schema load --project .&#10;  glade check --project .</code></pre>
      </section>
      <section class="home-source-panel" aria-label="Workflow input">
        <div class="home-panel-top">
          <strong data-source-title>Apex input</strong>
          <span data-source-label>AccountService.cls:2</span>
        </div>
        <pre class="home-source-code"><code class="language-apex" data-source-code data-highlighted-line="2">public with sharing class AccountService {&#10;  public static Invoice__c latestInvoice() {&#10;    return null;&#10;  }&#10;}</code></pre>
      </section>
      <section class="home-changed-panel" aria-label="What changed">
        <div class="home-panel-top">
          <strong data-proof-title>Local result</strong>
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
        <button type="button" data-copy-target="workbench-command">Copy command</button>
      </div>
      <div class="home-command-row">
        <span>CI / automation</span>
        <code id="workbench-ci-command" data-ci-command-strip>glade check --project . --json --no-progress</code>
        <button type="button" data-copy-target="workbench-ci-command">Copy JSON command</button>
      </div>
      <div class="home-command-strip-foot">
        <a href="/guide/cli-reference" data-docs-link>View docs</a>
        <span class="home-shortcuts">Shortcuts: 1-4 switch jobs · R run · C copy</span>
        <span class="home-copy-status" data-copy-status role="status" aria-live="polite"></span>
      </div>
    </div>
  </section>
</div>
