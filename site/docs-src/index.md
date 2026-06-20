---
layout: home
---

<section class="home-hero-shell" aria-label="Glade homepage hero">
  <div class="home-hero-copy">
    <p class="home-type-eyebrow">Run, test, and debug Apex locally</p>
    <h1>Local Apex runtime for SFDX projects</h1>
    <p class="home-deck">Run supported Apex checks, focused tests, SOQL/DML, triggers, and anonymous Apex against local project state. Debug supported paths from VS Code. Check human and AI-generated changes before the org round trip. Salesforce remains the validation gate.</p>
    <div class="home-cta-row">
      <a class="home-cta primary" href="/guide/installation" data-demo-link>Install Glade</a>
      <a class="home-cta link" href="/guide/quickstart">Run your first local check</a>
    </div>
    <p class="home-local-line">No Salesforce org login required for supported local checks.</p>
  </div>

  <div class="home-loop-visual" aria-label="Project loop command and diagnostic">
    <div class="home-loop-top">
      <strong>Project loop</strong>
      <span>Local check</span>
    </div>
    <div class="home-loop-command">
      <code>$ glade check --project . --no-progress</code>
    </div>
    <div class="home-loop-result">
      <div class="home-loop-result-status">
        <span class="home-loop-mark warn" aria-hidden="true">!</span>
        <div>
          <span class="home-loop-state-label">Diagnostic</span>
          <strong>Cannot resolve variable renewalQuote</strong>
        </div>
      </div>
      <p>RenewalQuoteService.cls:42</p>
    </div>
    <div class="home-loop-metrics" aria-label="Local check result summary">
      <span><strong>1</strong>diagnostic</span>
      <span class="home-loop-metric-proof"><strong>0</strong>deploys</span>
      <span><strong>412ms</strong>runtime</span>
    </div>
  </div>
</section>

<section class="home-capability-section home-command-section" aria-label="Daily local workflow">
  <div>
    <p class="home-eyebrow">Daily local workflow</p>
    <h2 class="home-h2">One local loop for CLI, VS Code, AI, and CI.</h2>
  </div>
  <div class="home-command-grid">
    <article class="home-command-card">
      <h3>CLI</h3>
      <p>Check source, run focused tests, execute anonymous Apex, and inspect SOQL/DML behavior from your terminal.</p>
    </article>
    <article class="home-command-card">
      <h3>VS Code</h3>
      <p>Open Glade Home for local proof, data, debug, and ship actions. Run local tests from Test Explorer and CodeLens.</p>
    </article>
    <article class="home-command-card">
      <h3>AI-assisted changes</h3>
      <p>Give agents a small local contract: run a check, quote the diagnostic, fix the smallest source change, and rerun the same command.</p>
    </article>
    <article class="home-command-card">
      <h3>CI</h3>
      <p>Use JSON, SARIF, JUnit, stable exit codes, affected-test selection, and saved run artifacts in pull request workflows.</p>
    </article>
  </div>
</section>

<div class="home-support-preview" data-generated-support-preview aria-label="Apex capability preview">
  <header class="home-support-preview-header">
    <h3>What runs locally</h3>
    <p>Examples from the checked capability map.</p>
  </header>
  <div class="home-capability-list">
    <div class="home-capability-row"><code>Database.insert</code><span class="home-completion-status home-completion-status-supported">Runs locally</span></div>
    <div class="home-capability-row"><code>Schema.DescribeSObjectResult</code><span class="home-completion-status home-completion-status-limited">Runs with limits</span></div>
    <div class="home-capability-row"><code>Answers.findSimilar</code><span class="home-completion-status home-completion-status-salesforce">Requires Salesforce</span></div>
  </div>
  <a href="/guide/support-map">What runs locally</a>
</div>

<section class="home-capability-section" aria-label="What runs locally">
  <div>
    <p class="home-eyebrow">What runs locally</p>
    <h2 class="home-h2">Check what runs locally before relying on it.</h2>
  </div>
  <div class="home-coverage-grid">
    <p class="home-grid-intro">Glade lists local behavior in three groups: runs locally, runs with limits, and requires Salesforce.</p>
    <article>
      <h3>Runs locally</h3>
      <ul>
        <li>Apex parse + semantic checks</li>
        <li>Focused Apex tests</li>
        <li>SOQL, DML, triggers, and SObjects</li>
        <li>Anonymous Apex</li>
        <li>Local Salesforce API routes</li>
        <li>JSON, SARIF, and JUnit output</li>
      </ul>
    </article>
    <article>
      <h3>Runs with limits</h3>
      <ul>
        <li>Describe and schema behavior</li>
        <li>Callout mocks</li>
        <li>Messaging result objects</li>
        <li>Visualforce and LWC local shells remain preview features.</li>
        <li>Deterministic search and SOSL helpers</li>
      </ul>
    </article>
    <article>
      <h3>Requires Salesforce</h3>
      <ul>
        <li>Live auth and sessions</li>
        <li>Hosted service engines</li>
        <li>Exact Lightning Experience behavior</li>
        <li>Metadata deploy and retrieve</li>
        <li>Streaming, Pub/Sub, and GraphQL</li>
        <li>Exact production governor accounting</li>
      </ul>
    </article>
  </div>
</section>

<section class="home-capability-section home-data-section" aria-label="Local data and playground">
  <div>
    <p class="home-eyebrow">Local data</p>
    <h2 class="home-h2">Local data without a scratch org</h2>
  </div>
  <div>
    <p class="home-p">Run anonymous Apex, SOQL, DML, triggers, local API routes, and playground examples against local project state. Use SQLite-backed environments when a loop needs persistence.</p>
    <div class="home-command-block">
      <pre><code>glade playground --project . --open
glade server --project . --db .glade/local-org.sqlite --addr 127.0.0.1:8080
glade db seed --db .glade/local-org.sqlite --project . seed.json
glade org create my-glade-org</code></pre>
    </div>
    <p class="home-boundary-line">Use <code>glade org</code> when a supported <code>sf</code> command needs a local target. It is not a real scratch org. Live auth, hosted services, deploy and retrieve, and exact production behavior stay with Salesforce.</p>
  </div>
</section>

<section class="home-capability-section home-plugin-section" aria-label="Optional plugins">
  <div>
    <p class="home-eyebrow">Extension points</p>
    <h2 class="home-h2">Optional plugins</h2>
  </div>
  <div>
    <p class="home-p">The base runtime stays focused on local Apex workflows. Add plugins only when a project needs capability reports, advisory scans, or custom local checks.</p>
    <p class="home-p">Base Glade workflows do not require plugins. Registry commands are preview until a registry, archive URL, or linked plugin is configured.</p>
    <div class="home-command-block">
      <pre><code>glade plugins list
glade plugins install @glade/performance
glade plugins install @glade/orgpackage</code></pre>
    </div>
    <p class="home-p"><a href="/guide/plugins">See first-party plugin install and lock-file docs.</a></p>
  </div>
</section>

<section class="home-capability-section home-boundary-section" aria-label="Salesforce validation boundary">
  <div>
    <p class="home-eyebrow">Validation boundary</p>
    <h2 class="home-h2">Salesforce remains the validation gate.</h2>
  </div>
  <div>
    <p class="home-p">Use Salesforce for live auth, hosted service engines, deploy and retrieve, exact Lightning Experience behavior, Streaming, Pub/Sub, GraphQL, and exact production governor accounting.</p>
  </div>
</section>

<section class="home-install-strip" aria-label="Install Glade">
  <code id="install-cmd">curl -fsSL https://glade.sh/install.sh | sh<br>glade doctor<br>glade check --project .</code>
  <div>
    <p>Supported paths run locally. Unsupported platform services fail with named diagnostics. Salesforce remains the validation gate.</p>
    <button type="button" data-copy-target="install-cmd" aria-label="Copy install command">Copy install</button>
    <span class="home-copy-status" data-copy-status role="status" aria-live="polite"></span>
  </div>
</section>
