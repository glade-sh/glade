---
layout: home
---

<section class="home-hero-shell" aria-label="Glade homepage hero">
  <div class="home-hero-copy">
    <p class="home-type-eyebrow">Local Apex runtime for Salesforce teams</p>
    <h1>Run Apex locally before you deploy.</h1>
    <p class="home-lead">Local Apex checks, tests, snippets, SOQL, DML, logs, and API routes on your machine.</p>
    <div class="home-cta-row">
      <a class="home-cta primary" href="/guide/workbench" data-demo-link>Explore support</a>
      <a class="home-cta" href="/guide/installation">Install Glade</a>
      <a class="home-cta link" href="/guide/support-map">Check support</a>
    </div>
    <p class="home-local-line">No org login for supported local work.</p>
    <p class="home-boundary-line">Salesforce stays in the path for hosted services.</p>
    <div class="home-proof-strip" aria-label="Main Glade strengths">
      <span>focused tests</span>
      <span>anonymous Apex</span>
      <span>local data</span>
      <span>no org wait</span>
    </div>
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

<div class="home-support-preview" data-generated-support-preview aria-label="Apex autocomplete support preview">
  <p><strong>Autocomplete preview</strong><span>Generated from checked Glade support rows.</span></p>
  <div>
    <code>Database.insert</code><span class="home-completion-status home-completion-status-supported">Works well</span>
    <code>Schema.DescribeSObjectResult</code><span class="home-completion-status home-completion-status-limited">With limits</span>
    <code>Answers.findSimilar</code><span class="home-completion-status home-completion-status-salesforce">Needs Salesforce</span>
  </div>
  <a href="/guide/workbench">Open the support showcase</a>
</div>

<section class="home-capability-section" aria-label="Glade local workflows">
  <div>
    <p class="home-eyebrow">What runs locally</p>
    <h2 class="home-h2">One local runtime for the daily Apex loop.</h2>
  </div>
  <div>
    <p class="home-p">Glade uses one Apex parser, semantic checker, VM, storage layer, and reporting format across the CLI, editor, tests, snippets, debug logs, and local API routes.</p>
    <p class="home-capability-line">check source · run tests · execute snippets · query local data · profile logs · serve local APIs · emit JSON</p>
    <div class="home-info-list" aria-label="Glade homepage detail">
      <p><strong>Local work</strong><span>Apex source checks, focused tests, Anonymous Apex snippets, SOQL against local data, DML-backed fixtures, saved debug-log profiling, and JSON output for automation.</span></p>
      <p><strong>Support boundary</strong><span>You supply org-specific metadata. Live Salesforce services stay visible instead of being silently faked.</span></p>
      <p><strong>Support showcase</strong><span><a href="/guide/workbench">Use the showcase</a> to try autocomplete, support labels, command outputs, JSON, traces, and workflow examples on one page.</span></p>
    </div>
    <p class="home-boundary-line">Visualforce pages and the LWC local shell remain preview features.</p>
  </div>
</section>

<section class="home-install-strip" aria-label="Install Glade">
  <code id="install-cmd">curl -fsSL https://glade.sh/install.sh | sh<br>glade doctor<br>glade check --project .</code>
  <div>
    <p>Works locally where Glade has evidence. Hosted Salesforce behavior stays marked.</p>
    <button type="button" data-copy-target="install-cmd">Copy install</button>
    <span class="home-copy-status" data-copy-status role="status" aria-live="polite"></span>
  </div>
</section>
