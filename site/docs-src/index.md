---
layout: home

head:
  - - script
    - defer: true
      src: /js/highlight.js
  - - script
    - defer: true
      src: /js/home.js
---

<section class="home-hero-shell" aria-label="Glade homepage hero">
  <div class="home-hero-copy">
    <p class="home-type-eyebrow">Local Apex runtime for Salesforce teams</p>
    <h1>Apex feedback before you deploy.</h1>
    <p class="home-lead">Run Apex checks, focused tests, snippets, and debug-log profiling locally — with copyable commands and visible Salesforce boundaries.</p>
    <div class="home-cta-row">
      <a class="home-cta primary" href="#local-apex-workbench" data-demo-link>Try the playground</a>
      <a class="home-cta" href="/guide/installation">Install Glade</a>
      <a class="home-cta link" href="/guide/support-map">View support map</a>
    </div>
    <p class="home-local-line">127.0.0.1 is a fine place to test Apex.</p>
    <div class="home-boundary-line" aria-label="Local runtime boundary">
      <span>No org login required for supported local loops.</span>
      <span>Live org services still require Salesforce.</span>
    </div>
    <p class="home-proof-line">check source · run focused tests · execute snippets · profile logs · emit JSON</p>
  </div>
  <aside class="home-hero-readout" aria-label="Current Glade check preview">
    <div class="home-hero-proof home-hero-proof-failed">
      <div class="home-hero-readout-head">
        <span>LOCAL CHECK OUTPUT</span>
        <strong class="home-hero-state-failed">caught locally</strong>
      </div>
      <pre class="home-hero-output" aria-label="Glade check failure output"><code data-hero-command>✗ 1 diagnostic found&#10;force-app/main/default/classes/AccountService.cls:2:3&#10;error GLADESEMA002 unknown type "Invoice__c"&#10;&#10;Try: glade schema load --project .</code></pre>
    </div>
    <div class="home-hero-proof home-hero-proof-passed">
      <div class="home-hero-readout-head">
        <span>LOCAL TEST OUTPUT</span>
        <strong class="home-hero-state-passed">passed locally</strong>
      </div>
      <pre class="home-hero-output" aria-label="Glade test passing output"><code data-hero-success>✓ 1 selected, 1 passed, 0 failed&#10;PassingTest.passes 8ms&#10;&#10;No org deploy required.</code></pre>
    </div>
  </aside>
</section>

<section class="home-workbench home-panel" id="local-apex-workbench" data-scenario-workbench aria-label="Local Apex workbench demo">
  <div class="home-workbench-head">
    <div>
      <p class="home-eyebrow">Local Workbench</p>
      <h2 class="home-h2">Pick a workflow. Run the command. Inspect what Glade proved.</h2>
      <p class="home-p">Each workflow swaps the Apex input, command output, result, and local command.</p>
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
      <pre id="command-output-panel" class="home-command-output" role="tabpanel" aria-labelledby="output-tab-output"><code data-command-output data-output-view="output">$ glade check --project . --no-progress&#10;Glade check&#10;&#10;✗ 1 diagnostic found&#10;&#10;force-app/main/default/classes/AccountService.cls:2:3&#10;error GLADESEMA002 method "latestInvoice" references unknown type "Invoice__c"&#10;&#10;Try:&#10;  glade schema load --project .&#10;  glade check --project .</code></pre>
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
        <strong data-proof-title>What Glade proved</strong>
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

<section class="home-support-map home-section" aria-label="Runtime Support">
  <div class="home-section-grid">
    <div>
      <p class="home-eyebrow">Support map</p>
      <h2 class="home-h2">No hidden runtime boundary.</h2>
      <p class="home-p">Glade runs supported workflows locally and marks the places where Salesforce is still required.</p>
      <div class="home-boundary-note" role="note">
        <strong>Live org services require Salesforce.</strong>
        <span>Glade keeps that boundary visible instead of folding it into local proof.</span>
      </div>
      <a class="home-section-link" href="/guide/support-map">Read the full support map</a>
    </div>
    <div class="home-support-table-wrap">
      <table>
        <thead>
          <tr>
            <th>Capability</th>
            <th>Local support</th>
            <th>Boundary</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td data-label="Capability">Apex source checks</td>
            <td data-label="Local support"><span class="home-status home-status-supported"><span class="home-status-dot"></span>supported locally</span></td>
            <td data-label="Boundary">project files</td>
          </tr>
          <tr>
            <td data-label="Capability">Changed-test selection</td>
            <td data-label="Local support"><span class="home-status home-status-supported"><span class="home-status-dot"></span>supported locally</span></td>
            <td data-label="Boundary">project graph</td>
          </tr>
          <tr>
            <td data-label="Capability">Anonymous Apex</td>
            <td data-label="Local support"><span class="home-status home-status-supported"><span class="home-status-dot"></span>supported locally</span></td>
            <td data-label="Boundary">local state</td>
          </tr>
          <tr>
            <td data-label="Capability">DML insert/update</td>
            <td data-label="Local support"><span class="home-status home-status-supported"><span class="home-status-dot"></span>supported locally</span></td>
            <td data-label="Boundary">SQLite-backed</td>
          </tr>
          <tr>
            <td data-label="Capability">SOQL query</td>
            <td data-label="Local support"><span class="home-status home-status-partial"><span class="home-status-dot"></span>partial</span></td>
            <td data-label="Boundary">supported subset</td>
          </tr>
          <tr>
            <td data-label="Capability">Debug-log profiling</td>
            <td data-label="Local support"><span class="home-status home-status-supported"><span class="home-status-dot"></span>supported locally</span></td>
            <td data-label="Boundary">saved logs</td>
          </tr>
          <tr>
            <td data-label="Capability">Org-specific metadata</td>
            <td data-label="Local support"><span class="home-status home-status-requires-config"><span class="home-status-dot"></span>requires config</span></td>
            <td data-label="Boundary">supply local metadata</td>
          </tr>
          <tr>
            <td data-label="Capability">Live org services</td>
            <td data-label="Local support"><span class="home-status home-status-requires-org"><span class="home-status-dot"></span>requires org</span></td>
            <td data-label="Boundary">not emulated</td>
          </tr>
          <tr>
            <td data-label="Capability">Unsupported platform APIs</td>
            <td data-label="Local support"><span class="home-status home-status-unsupported"><span class="home-status-dot"></span>unsupported</span></td>
            <td data-label="Boundary">surfaced visibly</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</section>

<section class="home-install home-section">
  <div class="home-install-copyblock">
    <p class="home-eyebrow">Install</p>
    <h2 class="home-h2">Run this workflow locally.</h2>
    <p class="home-p">Install Glade, check your environment, then run the selected workflow command:</p>
    <p class="home-install-workflow" data-install-workflow-label>Selected workflow: Catch deploy issues</p>
  </div>
  <div class="home-install-inner" role="group" aria-label="Install Glade command">
    <div class="home-terminal-command">
      <span class="home-terminal-prompt">$</span>
      <code id="install-cmd">curl -fsSL https://glade.sh/install.sh | sh</code>
    </div>
    <div class="home-install-actions">
      <button class="home-install-copy" type="button" data-copy-target="install-cmd">Copy install</button>
      <button class="home-install-copy" type="button" data-copy-target="install-workflow-cmds">Copy full sequence</button>
      <a class="home-install-link" href="https://glade.sh/install.sh">View install script</a>
      <a class="home-install-link" href="https://github.com/glade-sh/glade/releases">Releases</a>
      <a class="home-install-link" href="https://github.com/glade-sh/glade/releases/latest/download/SHA256SUMS.txt">Checksums</a>
    </div>
  </div>
  <pre class="home-install-verify" aria-label="Verify Glade install"><code id="install-workflow-cmds" data-install-commands>curl -fsSL https://glade.sh/install.sh | sh&#10;glade doctor&#10;glade check --project . --no-progress</code></pre>
</section>

<section class="home-section">
  <div class="home-next-cards">
    <a class="home-next-card" href="/guide/tester-field-guide">
      <span>START HERE</span>
      <strong>First run guide</strong>
      <span>Install, run doctor, and check a project.</span>
    </a>
    <a class="home-next-card" href="/guide/playground">
      <span>DOCS</span>
      <strong>Browser workbench</strong>
      <span>Try the workflows without installing.</span>
    </a>
    <a class="home-next-card" href="/guide/editor">
      <span>EDITOR</span>
      <strong>VS Code extension</strong>
      <span>Run Glade from your editor.</span>
    </a>
    <a class="home-next-card" href="https://github.com/glade-sh/glade">
      <span>SOURCE</span>
      <strong>GitHub</strong>
      <span>Source, issues, releases, and roadmap.</span>
    </a>
  </div>
</section>
