---
layout: home

head:
  - - script
    - defer: true
      src: /js/highlight.js
  - - script
    - defer: true
      src: /js/home.js

hero:
  name: "GLADE / LOCAL APEX RUNTIME"
  text: "The local workbench for Apex."
  tagline: Build, check, test, and debug Apex workflows locally - from one Go binary.
  actions:
    - theme: brand
      text: Install Glade
      link: /guide/installation
    - theme: alt
      text: Open Playground
      link: /guide/playground
    - theme: alt
      text: Read Docs
      link: /guide/installation
---

<p class="home-proof-line">Single Go binary · local SQLite state · macOS/Linux · no deploy loop for fast checks</p>

<div class="home-install">
  <div class="home-install-inner" role="group" aria-label="Install Glade command">
    <div class="home-terminal-command">
      <span class="home-terminal-prompt">$</span>
      <code id="install-cmd">curl -fsSL https://glade.sh/install.sh | sh</code>
    </div>
    <div class="home-install-actions">
      <button class="home-install-copy" data-copy-target="install-cmd">Copy</button>
      <a class="home-install-link" href="https://glade.sh/install.sh">View install script</a>
    </div>
  </div>
  <div class="home-install-meta">
    <span>release channel preview</span>
    <span>macOS/Linux</span>
    <span>installs to ~/.local/bin</span>
  </div>
</div>

<div class="home-features">
  <a class="home-feature" href="/guide/cli-reference">
    <span class="home-feature-icon"><IconSearchCheck aria-hidden="true" :size="32" :stroke-width="1.8" /></span>
    <strong>Check source</strong>
    <span>Catch Apex issues locally before deploys fail.</span>
    <code><span class="home-command-line">glade check</span><span class="home-command-line">--project .</span><span class="home-command-line">--json</span></code>
  </a>
  <a class="home-feature" href="/guide/local-testing">
    <span class="home-feature-icon"><IconFlaskConical aria-hidden="true" :size="32" :stroke-width="1.8" /></span>
    <strong>Run tests</strong>
    <span>Run supported Apex tests with isolated local data.</span>
    <code><span class="home-command-line">glade test changed</span><span class="home-command-line">--project .</span><span class="home-command-line">--since HEAD</span></code>
  </a>
  <a class="home-feature" href="/guide/cli-reference">
    <span class="home-feature-icon"><IconSquareTerminal aria-hidden="true" :size="32" :stroke-width="1.8" /></span>
    <strong>Try anonymous Apex</strong>
    <span>Run quick Apex snippets and probes against the local runtime.</span>
    <code><span class="home-command-line">glade exec</span><span class="home-command-line">'System.debug(1+1);'</span></code>
  </a>
  <a class="home-feature" href="/guide/local-api-server">
    <span class="home-feature-icon"><IconServerCog aria-hidden="true" :size="32" :stroke-width="1.8" /></span>
    <strong>Run a local API</strong>
    <span>Test Salesforce-shaped REST flows against local state.</span>
    <code><span class="home-command-line">glade server --db</span><span class="home-command-line">local.sqlite</span></code>
  </a>
</div>

<div class="home-section">
  <div class="home-section-grid">
    <div>
      <p class="home-eyebrow">PLAYGROUND</p>
      <h2 class="home-h2">Try the runtime before installing.</h2>
      <p class="home-p">Load examples, inspect source, run anonymous Apex, and view output in a local-style workbench.</p>
    </div>
    <div class="home-section-action">
      <button class="home-run-example" type="button" data-run-example>
        <IconPlayCircle aria-hidden="true" :size="17" :stroke-width="2" />
        <span>Run Example</span>
      </button>
    </div>
  </div>
  <div class="home-panel home-panel-soft">
    <div class="home-panel-top">
      <span>glade playground / local workspace</span>
      <span class="home-status-pill home-status-idle" data-run-status>idle</span>
    </div>
    <div class="home-playground-grid">
      <div class="home-playground-side" data-example-active="account">
        <p>EXAMPLES</p>
        <button class="home-playground-item active" type="button" data-example-id="account" aria-pressed="true">Account trigger basics</button>
        <button class="home-playground-item" type="button" data-example-id="soql" aria-pressed="false">SOQL query shape</button>
        <button class="home-playground-item" type="button" data-example-id="rollback" aria-pressed="false">DML rollback path</button>
      </div>
      <pre class="home-code-block"><code class="language-apex" data-example-code>public class RunMe {
  public static void main() {
    Account a = new Account(Name = 'Twin Lakes');
    insert a;
    System.debug([SELECT Name FROM Account].size());
  }
}</code></pre>
      <div class="home-playground-output">
        <p>OUTPUT</p>
        <div class="home-output-item">
          <span>status</span>
          <strong data-output-key="status">Not run</strong>
        </div>
        <div class="home-output-item">
          <span>compile + execute</span>
          <strong data-output-key="timing">--</strong>
        </div>
        <div class="home-output-item">
          <span>log</span>
          <strong data-output-key="log">Run Example to see output</strong>
        </div>
        <div class="home-output-item">
          <span>local state</span>
          <strong data-output-key="state">No local writes yet</strong>
        </div>
      </div>
    </div>
  </div>
</div>

<div class="home-section">
  <div class="home-section-grid">
    <div>
      <p class="home-eyebrow">RUNTIME MAP</p>
      <h2 class="home-h2">A small runtime with visible parts.</h2>
      <p class="home-p">Glade keeps parsing, local execution, data, and proof surfaces inspectable instead of hidden.</p>
    </div>
    <div class="home-runtime-cards home-runtime-flow">
      <div class="home-runtime-card">
        <span>parse / sema</span>
        <h3>Apex front end</h3>
        <p>Source model, symbols, grouping, diagnostics, and lowering.</p>
      </div>
      <div class="home-runtime-card">
        <span>vm / data</span>
        <h3>Local execution</h3>
        <p>SObjects, SOQL, DML, triggers, limits, and storage.</p>
      </div>
      <div class="home-runtime-card">
        <span>support / proof</span>
        <h3>Visible support map</h3>
        <p>What works, what has limits, and the checked rows behind each claim.</p>
      </div>
    </div>
  </div>
</div>

<div class="home-section">
  <div class="home-final-cta">
    <p class="home-eyebrow">LOCAL FIRST</p>
    <h2 class="home-h2">Ready to try it locally?</h2>
    <p class="home-p">Install Glade, open a workspace, and start checking Apex from your machine.</p>
    <div class="home-install-inner home-install-inner-compact" role="group" aria-label="Install Glade command">
      <div class="home-terminal-command">
        <span class="home-terminal-prompt">$</span>
        <code id="install-cmd-final">curl -fsSL https://glade.sh/install.sh | sh</code>
      </div>
      <div class="home-install-actions">
        <button class="home-install-copy" data-copy-target="install-cmd-final">Copy</button>
      </div>
    </div>
  </div>
  <div class="home-next-cards">
    <a class="home-next-card" href="/guide/installation">
      <strong>Docs</strong>
      <span>Install, editor setup, server use, CLI reference, and compatibility notes.</span>
    </a>
    <a class="home-next-card" href="/guide/playground">
      <strong>Playground</strong>
      <span>Start the local browser workbench from your machine.</span>
    </a>
    <a class="home-next-card" href="https://github.com/glade-sh/glade">
      <strong>GitHub</strong>
      <span>Source, issues, releases, fixtures, and history.</span>
    </a>
    <a class="home-next-card" href="https://glade.sh/install.sh">
      <strong>install.sh</strong>
      <span>The same installer used by the command above.</span>
    </a>
  </div>
</div>
