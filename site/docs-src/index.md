---
layout: home
head:
  - - link
    - rel: stylesheet
      href: /css/home.css
---

<section class="home-hero-shell" aria-label="Glade homepage hero">
  <div class="home-hero-copy">
    <p class="home-type-eyebrow">Local runtime for Salesforce DX projects</p>
    <h1>Run and test Salesforce Apex locally.</h1>
    <p class="home-deck">Glade checks source and runs supported Apex tests, SOQL/DML, and debug flows against your Salesforce DX project—locally, without an org login.</p>
    <div class="home-cta-row">
      <a class="home-cta primary" href="/guide/installation" data-demo-link>Install Glade</a>
      <a class="home-cta link" href="/guide/quickstart">Run your first local check</a>
    </div>
    <p class="home-release-line">Latest stable release: <a class="home-release-version" href="https://github.com/glade-sh/glade/releases/tag/v0.2.11">v0.2.11</a></p>
    <p class="home-local-line">No Salesforce org login is required for supported local checks.</p>
  </div>

  <div class="home-loop-visual" aria-label="Project loop command and diagnostic">
    <div class="home-loop-top">
      <strong>Project loop</strong>
      <span>Example local output</span>
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
      <span><strong>412ms</strong>example runtime</span>
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

<section class="home-capability-section home-boundary-section" aria-label="What runs locally">
  <div>
    <p class="home-eyebrow">What runs locally</p>
    <h2 class="home-h2">Know the boundary before you rely on a result.</h2>
  </div>
  <div>
    <p class="home-p">Supported paths run locally. Use Salesforce for hosted services, deployment, and final production validation.</p>
    <div class="home-support-preview" data-generated-support-preview aria-label="Apex capability preview">
      <div class="home-capability-list">
        <div class="home-capability-row"><code>Database.insert</code><span class="home-completion-status home-completion-status-supported">Runs locally</span></div>
        <div class="home-capability-row"><code>Schema.DescribeSObjectResult</code><span class="home-completion-status home-completion-status-limited">Runs with limits</span></div>
        <div class="home-capability-row"><code>Answers.findSimilar</code><span class="home-completion-status home-completion-status-salesforce">Requires Salesforce</span></div>
      </div>
      <a href="/guide/support-map">Check the versioned support map</a>
    </div>
  </div>
</section>

<section class="home-install-strip" aria-label="Install Glade">
  <code id="install-cmd" data-copy-text="curl -fsSL https://glade.sh/install.sh | sh&#10;glade doctor&#10;glade check --project .">curl -fsSL https://glade.sh/install.sh | sh
glade doctor
glade check --project .</code>
  <div>
    <p>Install, verify, then start your project’s first local check.</p>
    <button type="button" data-copy-target="install-cmd" aria-label="Copy install command">Copy install</button>
    <span class="home-copy-status" data-copy-status role="status" aria-live="polite"></span>
    <p class="home-trust-links"><a href="https://github.com/glade-sh/glade/releases">Release notes</a> · <a href="https://github.com/glade-sh/glade/blob/main/install.sh">Installer source</a> · <a href="/guide/security-trust#release-proof">Checksums, SBOM, and attestations</a> · <a href="/guide/security-trust">Security</a></p>
  </div>
</section>
