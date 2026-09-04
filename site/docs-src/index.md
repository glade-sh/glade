---
layout: home
title: Glade — Local Apex Runtime for Salesforce Developers
titleTemplate: false
head:
  - - link
    - rel: stylesheet
      href: /css/home.css
---

<script setup>
import releaseManifest from '../release-manifest.json'
</script>

<main>
<section class="home-hero-shell" aria-label="Glade homepage hero">
  <div class="home-hero-copy">
    <p class="home-type-eyebrow">Local Apex runtime</p>
    <h1>Apex feedback without the deploy wait.</h1>
    <p class="home-deck">Check source, run supported tests, and debug Apex against your Salesforce DX project—locally, without an org login. Salesforce remains the final validation gate.</p>
    <div class="home-cta-row">
      <a class="home-cta primary" href="/guide/quickstart" data-demo-link>Run your first local check</a>
      <a class="home-cta link" href="/guide/installation">Install Glade</a>
    </div>
    <p class="home-release-line"><a class="home-release-version" :href="`https://github.com/glade-sh/glade/releases/tag/${releaseManifest.version}`">{{ releaseManifest.version }}</a> · macOS and Linux · Apache-2.0</p>
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

<section class="home-capability-section home-steps-section" aria-label="Three step local loop">
  <div>
    <p class="home-eyebrow">One local loop</p>
    <h2 class="home-h2">Move from project to proof in three steps.</h2>
  </div>
  <ol class="home-step-list">
    <li><strong>Choose the project and task.</strong><span>Enter a Salesforce DX project and pick a check, test, debug, data, or CI job.</span></li>
    <li><strong>Run the supported path locally.</strong><span>Use the same runtime from the CLI, VS Code, local Playground, or automation.</span></li>
    <li><strong>Inspect concrete proof.</strong><span>Read the diagnostic, result, artifact, and exact Salesforce boundary.</span></li>
  </ol>
</section>

<section class="home-capability-section home-product-section" aria-label="Glade in VS Code">
  <div>
    <p class="home-eyebrow">Use the same proof from your tools</p>
    <h2 class="home-h2">Run local Apex tests from VS Code.</h2>
    <p class="home-p">Glade Home, Test Explorer, CodeLens, diagnostics, and debug actions use the same local project contract as the CLI.</p>
    <a class="home-section-link" href="/guide/editor">Use Glade in VS Code</a>
  </div>
  <figure class="home-product-figure">
    <img src="/help/screenshots/run-one-apex-test-02-codelens.png" alt="VS Code Apex editor showing Glade Run Local Test CodeLens actions">
    <figcaption>Checked help capture: run a local Apex test from CodeLens in VS Code.</figcaption>
  </figure>
</section>

<section class="home-capability-section home-workflow-section" aria-label="Start with the job you have">
  <div>
    <p class="home-eyebrow">Start with the job you have</p>
    <h2 class="home-h2">Choose the next local workflow.</h2>
  </div>
  <div class="home-workflow-grid">
    <a class="home-workflow-card" href="/guide/workflows/apex-tests"><strong>Run Apex tests</strong><span>Focus one class or method, run changed tests, or inspect failures.</span></a>
    <a class="home-workflow-card" href="/guide/workflows"><strong>Debug or execute Apex</strong><span>Choose breakpoints, profiling, anonymous Apex, or a SOQL query.</span></a>
    <a class="home-workflow-card" href="/guide/workflows/local-data"><strong>Work with local data</strong><span>Seed a named environment and use supported local API routes.</span></a>
    <a class="home-workflow-card" href="/guide/workflows/ci"><strong>Add Glade to CI</strong><span>Retain JSON, SARIF, JUnit, and stable exit-code evidence.</span></a>
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
        <div class="home-capability-row"><code>Schema.DescribeSObjectResult</code><span class="home-completion-status home-completion-status-limited">Runs locally with limits</span></div>
        <div class="home-capability-row"><code>ResetPasswordResult.getPassword</code><span class="home-completion-status home-completion-status-salesforce">Requires Salesforce</span></div>
      </div>
      <a href="/guide/support-map">Check the versioned support map</a>
    </div>
  </div>
</section>

<section class="home-install-strip" aria-label="Install Glade">
  <code id="install-cmd" data-copy-text="curl -fsSL https://glade.sh/install.sh | sh&#10;glade version">curl -fsSL https://glade.sh/install.sh | sh<br>glade version</code>
  <div>
    <p>Install and verify the binary, then continue inside your Salesforce DX project.</p>
    <button type="button" data-copy-target="install-cmd" aria-label="Copy install command">Copy install</button>
    <span class="home-copy-status" data-copy-status role="status" aria-live="polite"></span>
    <p class="home-trust-links"><a href="https://github.com/glade-sh/glade/releases">Release notes</a> · <a href="https://github.com/glade-sh/glade/blob/main/site/install.sh">Installer source</a> · <a href="/guide/security-trust#release-proof">Checksums, SBOM, and attestations</a> · <a href="/guide/security-trust">Security</a> · <a href="https://github.com/glade-sh/glade/issues/new/choose">Give feedback</a></p>
  </div>
</section>
</main>
