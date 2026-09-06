<template>
  <main class="glade-not-found" aria-labelledby="not-found-heading">
    <p class="glade-page-type">404</p>
    <h1 id="not-found-heading">Page not found</h1>
    <p>That address doesn’t point to a Glade page. Browse the documentation or return home to find your next step.</p>
    <nav aria-label="Page recovery">
      <a class="VPButton medium brand" href="/guide/">Browse documentation</a>
      <a class="VPButton medium alt" href="/">Return home</a>
    </nav>
  </main>
</template>

<style scoped>
.glade-not-found { max-width: 640px; margin: 0 auto; padding: 80px 24px 120px; text-align: center; }
h1 { margin: 16px 0; font-size: clamp(32px, 5vw, 44px); font-weight: 550; letter-spacing: -1px; line-height: 1.2; }
main > p:not(.glade-page-type) { color: var(--glade-muted); font-size: 16px; line-height: 1.75; }
nav { display: flex; justify-content: center; flex-wrap: wrap; gap: 12px; margin-top: 28px; }
a { display: inline-flex; align-items: center; justify-content: center; padding: 10px 20px; font-size: 14px; }
</style>
