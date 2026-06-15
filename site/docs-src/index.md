---
layout: home
---

<section class="home-hero-shell" aria-label="Glade homepage hero">
  <div class="home-hero-copy">
    <p class="home-type-eyebrow">Local Apex runtime for Salesforce teams</p>
    <h1>Run Apex checks and focused tests locally before you deploy.</h1>
    <p class="home-lead">Glade catches supported Apex, SOQL, DML, trigger, and test failures before the Salesforce org gate.</p>
    <div class="home-cta-row">
      <a class="home-cta primary" href="/guide/quickstart" data-demo-link>Start a 10-minute pilot</a>
      <a class="home-cta link" href="/guide/support-map">View capability map</a>
    </div>
    <p class="home-local-line">No org login for supported local work.</p>
    <p class="home-boundary-line">Salesforce CLI runs Apex tests in an org. Glade runs supported checks and tests locally first.</p>
  </div>

  <div class="home-loop-visual" data-home-loop aria-label="Animated local Apex development loop">
    <div class="home-loop-top">
      <strong>Project loop</strong>
      <span data-home-loop-label>AI edit check</span>
    </div>
    <div class="home-loop-stage">
      <svg viewBox="0 0 560 214" role="img" aria-labelledby="home-loop-title home-loop-desc">
        <title id="home-loop-title">Glade local Apex development loop</title>
        <desc id="home-loop-desc">A loop from save to Glade local runtime to proof.</desc>
        <defs>
          <filter id="home-loop-glow">
            <feGaussianBlur stdDeviation="4" result="blur"></feGaussianBlur>
            <feMerge>
              <feMergeNode in="blur"></feMergeNode>
              <feMergeNode in="SourceGraphic"></feMergeNode>
            </feMerge>
          </filter>
          <linearGradient id="home-loop-path-gradient" x1="0" x2="1" y1="0" y2="0">
            <stop offset="0%" stop-color="rgba(155,232,112,0.2)"></stop>
            <stop offset="52%" stop-color="#9be870"></stop>
            <stop offset="100%" stop-color="rgba(125,183,255,0.28)"></stop>
          </linearGradient>
        </defs>
        <path data-home-loop-path d="M 70 112 C 120 34 220 34 280 96 C 340 158 440 158 490 84" fill="none" stroke="rgba(155,232,112,0.22)" stroke-width="9" stroke-linecap="round"></path>
        <path d="M 70 112 C 120 34 220 34 280 96 C 340 158 440 158 490 84" fill="none" stroke="url(#home-loop-path-gradient)" stroke-width="2" stroke-linecap="round"></path>
        <circle data-home-loop-shadow cx="70" cy="112" r="18" fill="rgba(155,232,112,0.08)"></circle>
        <circle data-home-loop-runner cx="70" cy="112" r="8" fill="#9be870" filter="url(#home-loop-glow)"></circle>
        <g class="home-loop-node" data-home-loop-node="save" transform="translate(70 112)">
          <rect x="-54" y="-30" width="108" height="60" rx="10" fill="#08100d" stroke="#385c4a"></rect>
          <text class="home-loop-node-title" text-anchor="middle" y="-4">Save</text>
          <text class="home-loop-node-sub" text-anchor="middle" y="16">project</text>
        </g>
        <g class="home-loop-node" data-home-loop-node="runtime" transform="translate(280 96)">
          <rect x="-70" y="-35" width="140" height="70" rx="12" fill="#101a15" stroke="#4d8264"></rect>
          <text class="home-loop-node-title" text-anchor="middle" y="-6">Glade</text>
          <text class="home-loop-node-sub" text-anchor="middle" y="15">local runtime</text>
        </g>
        <g class="home-loop-node active" data-home-loop-node="proof" transform="translate(490 84)">
          <rect x="-54" y="-30" width="108" height="60" rx="10" fill="#08100d" stroke="#385c4a"></rect>
          <text class="home-loop-node-title" text-anchor="middle" y="-4">Proof</text>
          <text class="home-loop-node-sub" text-anchor="middle" y="16">now</text>
        </g>
      </svg>
    </div>
    <div class="home-loop-trace" aria-hidden="true">
      <span><i></i></span>
      <span><i></i></span>
      <span><i></i></span>
    </div>
    <div class="home-loop-terminal">
      <div class="home-loop-terminal-top" aria-hidden="true">
        <span></span>
        <span></span>
        <span></span>
      </div>
      <div class="home-loop-terminal-body">
        <div class="home-loop-state active" data-home-loop-state="check">
          <code>$ glade check --project . --no-progress</code>
          <div class="home-loop-result">
            <span class="home-loop-mark warn">!</span>
            <div>
              <strong>Variable not found</strong>
              <span>Cannot resolve variable renewalQuote in RenewalQuoteService.cls:42.</span>
            </div>
          </div>
          <div class="home-loop-metrics">
            <span><strong>1</strong>diagnostic</span>
            <span><strong>0</strong>deploys</span>
            <span><strong>412ms</strong>runtime</span>
          </div>
        </div>
        <div class="home-loop-state" data-home-loop-state="test">
          <code>$ glade test --class RenewalQuoteServiceTest --method updatesRenewalTotals</code>
          <div class="home-loop-result">
            <span class="home-loop-mark">✓</span>
            <div>
              <strong>2 passed · 336ms</strong>
              <span>Fix verified triggers, SOQL, DML, and limits locally.</span>
            </div>
          </div>
          <div class="home-loop-metrics">
            <span><strong>2</strong>tests</span>
            <span><strong>8</strong>SOQL</span>
            <span><strong>0</strong>org calls</span>
          </div>
        </div>
        <div class="home-loop-state" data-home-loop-state="exec">
          <code>$ glade exec --project . --file scripts/check-renewal-total.apex</code>
          <div class="home-loop-result">
            <span class="home-loop-mark">›</span>
            <div>
              <strong>Quote total verified</strong>
              <span>Anonymous Apex checked the renewal total against local data.</span>
            </div>
          </div>
          <div class="home-loop-metrics">
            <span><strong>12</strong>rows</span>
            <span><strong>0</strong>DML</span>
            <span><strong>0</strong>org calls</span>
          </div>
        </div>
      </div>
    </div>
    <div class="home-loop-tabs" aria-label="Local loop examples">
      <button type="button" data-home-loop-tab="check" aria-pressed="true">Check</button>
      <button type="button" data-home-loop-tab="test" aria-pressed="false">Test</button>
      <button type="button" data-home-loop-tab="exec" aria-pressed="false" aria-label="Anonymous Apex">Run</button>
    </div>
  </div>
</section>

<div class="home-proof-strip" aria-label="Main Glade strengths">
  <span>local checks</span>
  <span>focused tests</span>
  <span>SOQL/DML fixtures</span>
  <span>JSON/SARIF/JUnit</span>
  <span>VS Code + CI</span>
  <span>unsupported diagnostics</span>
</div>

<div class="home-support-preview" data-generated-support-preview aria-label="Apex autocomplete support preview">
  <p><strong>Capability map preview</strong><span>Generated from checked Glade support rows.</span></p>
  <div>
    <code>Database.insert</code><span class="home-completion-status home-completion-status-supported">Works well</span>
    <code>Schema.DescribeSObjectResult</code><span class="home-completion-status home-completion-status-limited">With limits</span>
    <code>Answers.findSimilar</code><span class="home-completion-status home-completion-status-salesforce">Needs Salesforce</span>
  </div>
  <a href="/guide/workbench">Open the local coverage workbench</a>
</div>

<section class="home-capability-section" aria-label="Where Glade fits">
  <div>
    <p class="home-eyebrow">Where Glade fits</p>
    <h2 class="home-h2">Move the first feedback loop onto the developer machine.</h2>
  </div>
  <div>
    <p class="home-p">Salesforce CLI runs Apex tests in an org. Glade runs supported Apex checks and tests locally first, so developers get fast feedback before the org gate.</p>
    <p class="home-capability-line">Edit Apex -&gt; glade check/test locally -&gt; fix fast -&gt; Salesforce org gate -&gt; deploy</p>
  </div>
</section>

<section class="home-capability-section" aria-label="What runs locally">
  <div>
    <p class="home-eyebrow">What runs locally</p>
    <h2 class="home-h2">Coverage is explicit before a team depends on it.</h2>
  </div>
  <div class="home-coverage-grid">
    <article>
      <h3>Runs locally</h3>
      <ul>
        <li>Apex parse + semantic checks</li>
        <li>Focused Apex tests</li>
        <li>SOQL, DML, triggers, and SObjects</li>
        <li>Anonymous Apex</li>
        <li>Local API routes</li>
        <li>JSON, SARIF, and JUnit output</li>
      </ul>
    </article>
    <article>
      <h3>Runs locally with limits</h3>
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

<section class="home-capability-section" aria-label="Developer and architect paths">
  <div>
    <p class="home-eyebrow">Two paths</p>
    <h2 class="home-h2">Developers get the short loop. Architects get a conservative gate.</h2>
  </div>
  <div>
    <div class="home-info-list" aria-label="Developer and architect details">
      <p><strong>Your daily Apex loop, without org wait.</strong><span>Run one class, one method, changed tests, anonymous Apex, SOQL/DML fixtures, and local debug traces from your machine.</span></p>
      <p><strong>Catch deploy-blockers early.</strong><span><code>glade check --project .</code> checks source before a deploy round trip.</span></p>
      <p><strong>Run the test you care about.</strong><span><code>glade test --class AccountServiceTest --method testCreatesAccount</code> keeps the loop focused.</span></p>
      <p><strong>A conservative pre-gate for Salesforce delivery.</strong><span>Add local Apex checks to CI with JSON, SARIF, JUnit, stable exit codes, affected-test selection, and explicit unsupported diagnostics.</span></p>
      <p><strong>Architect checklist.</strong><span>Check the project against the capability map, run a focused pilot, record unsupported diagnostics, compare with Salesforce org behavior, and keep Salesforce as the release gate.</span></p>
    </div>
  </div>
</section>

<section class="home-install-strip" aria-label="Install Glade">
  <code id="install-cmd">curl -fsSL https://glade.sh/install.sh | sh<br>glade doctor<br>glade check --project .</code>
  <div>
    <p>Supported paths run locally. Hosted Salesforce services fail fast with named diagnostics.</p>
    <button type="button" data-copy-target="install-cmd">Copy install</button>
    <span class="home-copy-status" data-copy-status role="status" aria-live="polite"></span>
  </div>
</section>
