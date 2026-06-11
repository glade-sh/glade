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
  name: "LOCAL APEX RUNTIME / ORG: NONE"
  text: "Run Apex without the org round trip."
  tagline: Glade parses, type-checks, and executes Salesforce Apex on your machine. Tests, SOQL, DML, triggers, limits, editor tools, CI reports, and a local Salesforce-shaped API all run from one Go binary.
  actions:
    - theme: brand
      text: Install
      link: /guide/installation
    - theme: alt
      text: Open Playground
      link: https://play.glade.sh/playground/
    - theme: alt
      text: Docs
      link: /guide/installation

features:
  - icon: <span class="home-icon">&#9745;</span>
    title: Check source
    details: Build local diagnostics before deploy time.
    link: /guide/cli-reference
    linkText: glade check --project . --json
  - icon: <span class="home-icon">&#9875;</span>
    title: Run tests
    details: Execute supported Apex tests with isolated local data.
    link: /guide/local-testing
    linkText: glade test changed --project . --since HEAD
  - icon: <span class="home-icon">&#9000;</span>
    title: Try anonymous Apex
    details: Use the runtime directly for small probes.
    link: /guide/cli-reference
    linkText: glade exec 'System.debug(1+1);'
  - icon: <span class="home-icon">&#9785;</span>
    title: Run a local API
    details: Exercise Salesforce-shaped REST flows against local state.
    link: /guide/local-api-server
    linkText: glade server --db local.sqlite
---

<div class="home-install">
  <div class="home-install-inner">
    <code id="install-cmd">curl -fsSL https://glade.sh/install.sh | sh</code>
    <button class="home-install-copy" data-copy-target="install-cmd">Copy</button>
  </div>
  <div class="home-install-meta">
    <span>release channel: preview</span>
    <span>macOS / Linux</span>
    <span>path: ~/.local/bin</span>
  </div>
</div>

<div class="home-section">
  <div class="home-section-grid">
    <div>
      <p class="home-eyebrow">PLAYGROUND</p>
      <h2 class="home-h2">A browser workbench, not a toy terminal.</h2>
      <p class="home-p">Load examples, inspect source, run anonymous Apex, watch output, and browse local data. The UI stays plain so the Apex stays in view.</p>
    </div>
  </div>
  <div class="home-panel home-panel-soft">
    <div class="home-panel-top">
      <span>glade playground / local workspace</span>
      <span class="text-cyan">ready</span>
    </div>
    <div class="home-playground-grid">
      <div class="home-playground-side">
        <p>EXAMPLES</p>
        <div class="home-playground-item active">Account trigger basics</div>
        <div class="home-playground-item">SOQL query shape</div>
        <div class="home-playground-item">DML rollback path</div>
      </div>
      <pre class="home-code-block"><code class="language-apex">public class RunMe {
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
          <strong>Pass</strong>
        </div>
        <div class="home-output-item">
          <span>compile + execute</span>
          <strong>38 ms</strong>
        </div>
        <div class="home-output-item">
          <span>log</span>
          <strong>USER_DEBUG 1</strong>
        </div>
      </div>
    </div>
  </div>
</div>

<div class="home-section">
  <div class="home-section-grid">
    <div>
      <p class="home-eyebrow">RUNTIME MAP</p>
      <h2 class="home-h2">The pieces stay visible.</h2>
      <p class="home-p">Glade is not a mock server with a few happy paths. It is a clean-room Apex front end and runtime with support claims tied to fixtures.</p>
    </div>
    <div class="home-runtime-cards">
      <div class="home-runtime-card">
        <span>parse / sema</span>
        <h3>Apex front end</h3>
        <p>Source model, symbol graph, diagnostics, and lowering.</p>
      </div>
      <div class="home-runtime-card">
        <span>vm / data</span>
        <h3>Local execution</h3>
        <p>SObjects, SOQL, DML, triggers, limits, and storage.</p>
      </div>
      <div class="home-runtime-card">
        <span>reports / docs</span>
        <h3>Proof surface</h3>
        <p>Config, CI artifacts, support maps, known gaps, and generated docs.</p>
      </div>
    </div>
  </div>
</div>

<div class="home-section">
  <h2 class="home-h2">Next Door</h2>
  <p class="home-p">Use it, wire it, inspect it.</p>
  <div class="home-next-cards">
    <a class="home-next-card" href="/guide/installation">
      <strong>Docs</strong>
      <span>Install, editor setup, server use, CLI reference, and compatibility notes.</span>
    </a>
    <a class="home-next-card" href="https://play.glade.sh/playground/">
      <strong>Playground</strong>
      <span>Run Apex examples in a browser-backed Glade loop.</span>
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
