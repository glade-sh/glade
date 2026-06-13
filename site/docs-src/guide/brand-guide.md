---
title: Brand Guide
description: Glade identity, color, typography, logo, component, and accessibility guidance.
aside: false
---

# Brand Guide

<div class="brand-guide">
  <section class="brand-guide-intro">
    <p class="brand-guide-kicker">identity</p>
    <div class="brand-guide-logo-lockup">
      <img class="brand-guide-logo-open" src="/logo-mark-open.svg" alt="Glade open contour logo" />
      <div class="brand-guide-logo-copy">
        <h2>Glade</h2>
        <p>Apex feedback before you deploy.</p>
        <code>glade check --project .</code>
      </div>
    </div>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">mark</p>
      <h2>Use the contour mark with room around it.</h2>
    </div>
    <div class="brand-logo-grid">
      <article class="brand-logo-card">
        <span>boxed mark</span>
        <img class="brand-logo-swatch brand-logo-swatch-boxed" src="/logo-mark.svg" alt="Glade boxed contour logo" />
        <strong>Use it in tight chrome.</strong>
        <p>Navbar, favicon, app icon, avatar, package metadata, and any place where the mark has to hold a square.</p>
      </article>
      <article class="brand-logo-card">
        <span>open mark</span>
        <img class="brand-logo-swatch brand-logo-swatch-open" src="/logo-mark-open.svg" alt="Glade open contour logo" />
        <strong>Use it where it can breathe.</strong>
        <p>Hero lockups, docs art, slides, social cards, and larger layouts on controlled dark backgrounds.</p>
      </article>
    </div>
    <div class="brand-rule-grid">
      <article class="brand-rule-card">
        <span>clearspace</span>
        <strong>Give the mark room to read as terrain, not texture.</strong>
        <p>Keep at least 25% of the mark width clear on every side.</p>
      </article>
      <article class="brand-rule-card">
        <span>minimum size</span>
        <strong>Keep the full lockup at 120px or wider.</strong>
        <p>Use the mark alone at 24px and above. Use the boxed mark for tighter chrome.</p>
      </article>
      <article class="brand-rule-card">
        <span>tiny use</span>
        <strong>Use a padded square mark at 16px.</strong>
        <p>The open contour lines collapse when they are squeezed too far.</p>
      </article>
      <article class="brand-rule-card">
        <span>background</span>
        <strong>Prefer near-black and raised surfaces.</strong>
        <p>Avoid noisy images and mid-tone backgrounds unless the mark sits in a controlled dark container.</p>
      </article>
      <article class="brand-rule-card">
        <span>misuse</span>
        <strong>Do not stretch, rotate, crop, or recolor the mark.</strong>
        <p>Do not place it where the contour strokes lose contrast.</p>
      </article>
      <article class="brand-rule-card">
        <span>assets</span>
        <strong>Ship named exports.</strong>
        <p><code>glade-mark.svg</code>, <code>glade-lockup.svg</code>, <code>glade-favicon.svg</code>, and <code>glade-social-card.png</code>.</p>
      </article>
    </div>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">color</p>
      <h2>Host Signal uses green, dark surfaces, and named states.</h2>
    </div>
    <div class="brand-color-grid">
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #9BE870"></span>
        <strong>Glade green</strong>
        <code>#9BE870</code>
        <p>Primary action, selected state, local support, and success.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #B7FF8A"></span>
        <strong>Glade strong</strong>
        <code>#B7FF8A</code>
        <p>Hover, focus, and strong active borders.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #070B0D"></span>
        <strong>Background</strong>
        <code>#070B0D</code>
        <p>Primary page background. Keep it calm and developer-native.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #10191E"></span>
        <strong>Surface</strong>
        <code>#10191E</code>
        <p>Workbench, cards, nav, and docs panels.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #152229"></span>
        <strong>Raised surface</strong>
        <code>#152229</code>
        <p>Active controls and lifted workbench panels.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #26363D"></span>
        <strong>Line</strong>
        <code>#26363D</code>
        <p>Panel edges, table rules, and quiet dividers.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #F5C95F"></span>
        <strong>Warning</strong>
        <code>#F5C95F</code>
        <p>Partial support, warning states, and degraded behavior.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #FF6B61"></span>
        <strong>Danger</strong>
        <code>#FF6B61</code>
        <p>Failures, invalid inputs, and error diagnostics.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #7DB7FF"></span>
        <strong>Info</strong>
        <code>#7DB7FF</code>
        <p>Requires-org state, neutral runtime details, and informational status.</p>
      </article>
    </div>
    <pre class="brand-token-block"><code>:root &#123;
  --bg: #070B0D;
  --surface: #10191E;
  --surface-raised: #14232A;
  --terminal: #05090B;
  --line: #26363D;
  --line-strong: #38505A;
  --text: #F3F7F5;
  --text-muted: #9AABA5;
  --text-soft: #6F817B;
  --text-inverse: #061009;
  --glade: #9BE870;
  --glade-strong: #B7FF8A;
  --glade-muted: rgba(155,232,112,0.14);
  --success: #9BE870;
  --warning: #F5C95F;
  --danger: #FF6B61;
  --info: #7DB7FF;
  --focus: #B7FF8A;
&#125;</code></pre>
    <p class="brand-guide-note">Warning and danger colors exist for product clarity, not brand decoration. Use them only for CLI output, form states, docs callouts, and runtime diagnostics.</p>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">type</p>
      <h2>Host Signal keeps the interface legible and direct.</h2>
    </div>
    <div class="brand-stack-grid">
      <article class="brand-stack-card brand-stack-recommended">
        <span>recommended stack</span>
        <h3>Glade</h3>
        <p class="brand-stack-line">Apex feedback before you deploy.</p>
        <p class="brand-stack-copy">Host Grotesk carries headings, UI, and body copy. Monaspace Argon carries commands, terminal output, JSON, traces, and code. Keep tracking at zero and let size, weight, and spacing do the work.</p>
        <code>glade test changed --since HEAD</code>
        <dl>
          <div><dt>sans</dt><dd>Host Grotesk</dd></div>
          <div><dt>mono</dt><dd>Monaspace Argon</dd></div>
          <div><dt>accent</dt><dd>None by default</dd></div>
        </dl>
      </article>
    </div>
    <div class="brand-rule-grid">
      <article class="brand-rule-card">
        <span>Hero display</span>
        <strong>Host Grotesk, clamp(3.05rem, 6.2vw, 4.625rem).</strong>
        <p>Use 700 weight and zero tracking. Adjust wording before forcing cramped lines.</p>
      </article>
      <article class="brand-rule-card">
        <span>Page H1</span>
        <strong>Host Grotesk, 44-56px desktop.</strong>
        <p>Use it for brand and docs openings. Keep the line height close to 1.04.</p>
      </article>
      <article class="brand-rule-card">
        <span>Section headline</span>
        <strong>Host Grotesk, 30-38px desktop.</strong>
        <p>Keep it clear, but not hero-sized inside compact panels.</p>
      </article>
      <article class="brand-rule-card">
        <span>Body</span>
        <strong>Host Grotesk, 16-17px.</strong>
        <p>Use line height 1.6-1.7 and zero tracking for docs and product copy.</p>
      </article>
      <article class="brand-rule-card">
        <span>Code</span>
        <strong>Monaspace Argon, 13.5-14px.</strong>
        <p>Use line height 1.55 for CLI, snippets, and output panels.</p>
      </article>
      <article class="brand-rule-card">
        <span>Eyebrow</span>
        <strong>Host Grotesk, 11-12px uppercase.</strong>
        <p>Keep it short and keep tracking at zero.</p>
      </article>
    </div>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">layout</p>
      <h2>Use a short spacing scale and steady containers.</h2>
    </div>
    <pre class="brand-token-block"><code>:root &#123;
  --container-page: 1120px;
  --container-reading: 760px;
  --container-wide: 1280px;
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-5: 24px;
  --space-6: 32px;
  --space-7: 48px;
  --space-8: 64px;
  --space-9: 96px;
  --space-10: 128px;
  --radius-sm: 6px;
  --radius-md: 8px;
  --radius-pill: 999px;
&#125;</code></pre>
    <div class="brand-rule-grid">
      <article class="brand-rule-card">
        <span>major sections</span>
        <strong>Use 96-128px between large bands.</strong>
        <p>Mobile can tighten to 64-80px.</p>
      </article>
      <article class="brand-rule-card">
        <span>card gap</span>
        <strong>Use 16-24px between repeated cards.</strong>
        <p>Inside cards, use 20-28px padding.</p>
      </article>
      <article class="brand-rule-card">
        <span>chrome</span>
        <strong>Give sticky nav and sidebar clear room.</strong>
        <p>Content must not tuck under fixed bars at any scroll position.</p>
      </article>
    </div>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">components</p>
      <h2>Define states, not just shapes.</h2>
    </div>
    <div class="brand-rule-grid">
      <article class="brand-rule-card">
        <span>Primary button</span>
        <strong>Filled Glade green, inverse text, visible focus.</strong>
        <p>Use for Install Glade and the single main action on a page.</p>
      </article>
      <article class="brand-rule-card">
        <span>Secondary button</span>
        <strong>Subtle raised surface with a firm border.</strong>
        <p>Use for Playground, Docs, and lateral navigation.</p>
      </article>
      <article class="brand-rule-card">
        <span>Tertiary button</span>
        <strong>Text-like, no heavy fill.</strong>
        <p>Use only after a primary and secondary action already exist.</p>
      </article>
      <article class="brand-rule-card">
        <span>Command block</span>
        <strong>Prompt marker, copy button, long-command overflow.</strong>
        <p>Keep install details visible near the block.</p>
      </article>
      <article class="brand-rule-card">
        <span>Status badge</span>
        <strong>Pair color with labels.</strong>
        <p>Use ready, running, pass, warning, fail, and idle labels.</p>
      </article>
      <article class="brand-rule-card">
        <span>Runtime output</span>
        <strong>Show status, timing, log, and local state.</strong>
        <p>Make the product proof concrete.</p>
      </article>
      <article class="brand-rule-card">
        <span>Docs nav</span>
        <strong>Keep active and focus states stronger than hover.</strong>
        <p>On mobile, collapse navigation into a menu.</p>
      </article>
      <article class="brand-rule-card">
        <span>Search</span>
        <strong>Use the same focus ring as buttons and nav.</strong>
        <p>Show keyboard hints, empty states, and stable spacing.</p>
      </article>
    </div>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">background</p>
      <h2>Let the terrain sit behind the work.</h2>
    </div>
    <p class="brand-guide-note">The grid is structure. The contour lines are terrain. Neither should compete with content.</p>
    <div class="brand-rule-grid">
      <article class="brand-rule-card">
        <span>grid</span>
        <strong>Keep it 4-8% visible over the page background.</strong>
        <p>It should read as structure, not decoration.</p>
      </article>
      <article class="brand-rule-card">
        <span>contours</span>
        <strong>Use 10-18% opacity depending on nearby text.</strong>
        <p>Reduce opacity behind headlines and dense copy.</p>
      </article>
      <article class="brand-rule-card">
        <span>motion</span>
        <strong>Motion may drift, but it must stop on request.</strong>
        <p>Respect reduced-motion settings everywhere.</p>
      </article>
    </div>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">CLI</p>
      <h2>Terminal color follows the site palette.</h2>
    </div>
    <div class="brand-rule-grid">
      <article class="brand-rule-card">
        <span>primary</span>
        <strong>Use Glade green for progress and active details.</strong>
        <p>ANSI truecolor: <code>38;2;155;232;112</code>.</p>
      </article>
      <article class="brand-rule-card">
        <span>success</span>
        <strong>Use strong Glade green for pass emphasis.</strong>
        <p>ANSI truecolor: <code>38;2;183;255;138</code>.</p>
      </article>
      <article class="brand-rule-card">
        <span>info</span>
        <strong>Use info blue for requires-org and neutral details.</strong>
        <p>ANSI truecolor: <code>38;2;125;183;255</code>.</p>
      </article>
      <article class="brand-rule-card">
        <span>warning</span>
        <strong>Use warning only when action may be needed.</strong>
        <p>ANSI truecolor: <code>38;2;245;201;95</code>.</p>
      </article>
      <article class="brand-rule-card">
        <span>danger</span>
        <strong>Use danger for failed commands and invalid states.</strong>
        <p>ANSI truecolor: <code>38;2;255;107;97</code>.</p>
      </article>
    </div>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">copy</p>
      <h2>Keep the language plain and local.</h2>
    </div>
    <div class="brand-rule-grid">
      <article class="brand-rule-card">
        <span>tagline</span>
        <strong>Apex feedback before you deploy.</strong>
        <p>Use this in metadata, hero copy, and compact product descriptions.</p>
      </article>
      <article class="brand-rule-card">
        <span>say</span>
        <strong>Check Apex locally before deploys fail.</strong>
        <p>Use product nouns before mood words.</p>
      </article>
      <article class="brand-rule-card">
        <span>say</span>
        <strong>One Go binary. Local state. Fast feedback.</strong>
        <p>Say what runs on the machine.</p>
      </article>
      <article class="brand-rule-card">
        <span>say</span>
        <strong>Known gaps stay visible.</strong>
        <p>Do not imply full platform simulation where support is incomplete.</p>
      </article>
      <article class="brand-rule-card">
        <span>avoid</span>
        <strong>No magic, blazing, revolutionary, or seamless.</strong>
        <p>The brand works best when the facts carry the weight.</p>
      </article>
    </div>
  </section>
</div>
