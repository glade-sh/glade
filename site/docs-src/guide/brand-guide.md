---
title: Brand Guide
description: Glade identity, color, typography, logo, and CLI color guidance.
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
        <p>Support color. Use for the boxed logo frame, small labels, contours, and success-adjacent UI.</p>
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
        <p>Light-mode primary accent. It is darker than the dark-mode blue for contrast.</p>
      </article>
      <article class="brand-color-card">
        <span class="brand-color-swatch" style="--swatch: #6E7650"></span>
        <strong>Light lichen</strong>
        <code>#6E7650</code>
        <p>Light-mode support color. Use for small labels and restrained natural cues.</p>
      </article>
    </div>
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
        <span>tone</span>
        <strong>Concrete before poetic.</strong>
        <p>Say parse, check, test, query, debug, and local state. Let the glade idea stay in the visual system.</p>
      </article>
      <article class="brand-rule-card">
        <span>restraint</span>
        <strong>Few shapes. Few colors. No glow for its own sake.</strong>
        <p>The mark should feel small-tool friendly, not decorative.</p>
      </article>
    </div>
  </section>
</div>
