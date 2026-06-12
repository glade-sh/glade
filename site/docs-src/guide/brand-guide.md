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
        <p>The local workbench for Apex.</p>
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
        <strong>Prefer ink water and tarn surface.</strong>
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
      <h2>Deep tarn, lichen, and near-black water.</h2>
    </div>
    <div class="brand-color-grid">
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #7897B8"></span>
        <strong>Deep tarn</strong>
        <code>#7897B8</code>
        <p>Primary dark-mode accent. Use for links, icons, progress, active state, and logo contours.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #B7C68F"></span>
        <strong>Lichen</strong>
        <code>#B7C68F</code>
        <p>Support and success color. Use for pass states and small support accents.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #060A0D"></span>
        <strong>Ink water</strong>
        <code>#060A0D</code>
        <p>Primary dark background. Keep it deep enough for the serif and contour field to carry.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #0E171D"></span>
        <strong>Tarn surface</strong>
        <code>#0E171D</code>
        <p>Card and nav surface. Use it as a quiet layer over the grid.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #435F7C"></span>
        <strong>Light tarn</strong>
        <code>#435F7C</code>
        <p>Light-mode primary accent. In dark mode, keep it to borders and large UI.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #6E7650"></span>
        <strong>Light lichen</strong>
        <code>#6E7650</code>
        <p>Light-mode support. In dark mode, use it for quiet accents, not core copy.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #B6CADF"></span>
        <strong>Pale tarn</strong>
        <code>#B6CADF</code>
        <p>High-contrast interaction color. Use for primary buttons, focus rings, and diagnostic codes.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #D8B36C"></span>
        <strong>Warning</strong>
        <code>#D8B36C</code>
        <p>Operational token for warnings, caution callouts, and partial runtime states.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #D48178"></span>
        <strong>Danger</strong>
        <code>#D48178</code>
        <p>Operational token for failures, invalid inputs, and error diagnostics.</p>
      </article>
    </div>
    <pre class="brand-token-block"><code>:root &#123;
  --color-ink-water: #060A0D;
  --color-tarn-surface: #0E171D;
  --color-deep-tarn: #7897B8;
  --color-lichen: #B7C68F;
  --color-light-tarn: #435F7C;
  --color-light-lichen: #6E7650;
  --text-primary: #EDF3F6;
  --text-secondary: #B7C2C8;
  --text-muted: #8D9AA2;
  --surface-page: var(--color-ink-water);
  --surface-card: var(--color-tarn-surface);
  --surface-code: #101B21;
  --border-subtle: rgba(120, 151, 184, 0.18);
  --border-strong: rgba(120, 151, 184, 0.38);
  --focus-ring: #B6CADF;
  --button-primary-bg: #B6CADF;
  --button-primary-text: #060A0D;
  --status-success: #B7C68F;
  --status-warning: #D8B36C;
  --status-danger: #D48178;
&#125;</code></pre>
    <p class="brand-guide-note">Warning and danger colors exist for product clarity, not brand decoration. Use them only for CLI output, form states, docs callouts, and runtime diagnostics.</p>
  </section>

  <section class="brand-guide-section">
    <div class="brand-guide-heading">
      <p class="brand-guide-kicker">type</p>
      <h2>Editorial head, quiet UI, clear code.</h2>
    </div>
    <div class="brand-stack-grid">
      <article class="brand-stack-card brand-stack-recommended">
        <span>recommended stack</span>
        <h3>Glade</h3>
        <p class="brand-stack-line">The local workbench for Apex.</p>
        <p class="brand-stack-copy">Newsreader Italic gives the page its field-note voice. Mona Sans keeps controls and docs text crisp. Monaspace Neon is the preferred code face, with JetBrains Mono as the stable fallback.</p>
        <code>glade test changed --since HEAD</code>
        <dl>
          <div><dt>serif</dt><dd>Newsreader Italic</dd></div>
          <div><dt>sans</dt><dd>Mona Sans</dd></div>
          <div><dt>mono</dt><dd>Monaspace Neon / JetBrains Mono</dd></div>
        </dl>
      </article>
    </div>
    <div class="brand-rule-grid">
      <article class="brand-rule-card">
        <span>Hero display</span>
        <strong>Newsreader Italic, 72-104px desktop.</strong>
        <p>Use it only for major page openings. Mobile range is 44-56px.</p>
      </article>
      <article class="brand-rule-card">
        <span>Page H1</span>
        <strong>Newsreader Italic, 44-56px desktop.</strong>
        <p>Use it for brand and docs openings. Keep line height near 1.04.</p>
      </article>
      <article class="brand-rule-card">
        <span>Section headline</span>
        <strong>Newsreader Italic, 30-38px desktop.</strong>
        <p>Keep it elegant, but not hero-sized inside compact panels.</p>
      </article>
      <article class="brand-rule-card">
        <span>Body</span>
        <strong>Mona Sans, 15-16px.</strong>
        <p>Use line height 1.55-1.65 and brighter secondary text for product copy.</p>
      </article>
      <article class="brand-rule-card">
        <span>Code</span>
        <strong>Monaspace Neon, 12.5-13.5px.</strong>
        <p>Use line height 1.45-1.55 for CLI, snippets, and output panels.</p>
      </article>
      <article class="brand-rule-card">
        <span>Eyebrow</span>
        <strong>Mona Sans, 11-12px uppercase.</strong>
        <p>Use 0.06-0.1em tracking. Keep it short.</p>
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
        <strong>Filled pale tarn, ink text, visible focus.</strong>
        <p>Use for Install Glade and the single main action on a page.</p>
      </article>
      <article class="brand-rule-card">
        <span>Secondary button</span>
        <strong>Subtle tarn surface with a firm border.</strong>
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
        <strong>Keep it 4-8% visible over ink water.</strong>
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
        <strong>Use deep tarn for progress and active details.</strong>
        <p>ANSI truecolor: <code>38;2;120;151;184</code>.</p>
      </article>
      <article class="brand-rule-card">
        <span>success</span>
        <strong>Use lichen for pass states.</strong>
        <p>ANSI truecolor: <code>38;2;183;198;143</code>.</p>
      </article>
      <article class="brand-rule-card">
        <span>support</span>
        <strong>Use pale tarn for diagnostic codes.</strong>
        <p>ANSI truecolor: <code>38;2;182;202;223</code>.</p>
      </article>
      <article class="brand-rule-card">
        <span>warning</span>
        <strong>Use warning only when action may be needed.</strong>
        <p>ANSI truecolor: <code>38;2;216;179;108</code>.</p>
      </article>
      <article class="brand-rule-card">
        <span>danger</span>
        <strong>Use danger for failed commands and invalid states.</strong>
        <p>ANSI truecolor: <code>38;2;212;129;120</code>.</p>
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
        <strong>The local workbench for Apex.</strong>
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
