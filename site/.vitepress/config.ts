import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Glade',
  description: 'Orgless Apex runtime for local development and testing.',
  base: '/',
  srcDir: 'docs-src',
  outDir: '.vitepress/dist',
  cleanUrls: true,
  appearance: 'dark',
  lastUpdated: true,
  head: [
    ['meta', { name: 'theme-color', content: '#080b12' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
    ['link', { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&family=Space+Grotesk:wght@400;500;600;700&display=swap' }]
  ],
  themeConfig: {
    logo: '/logo-mark.svg',
    search: { provider: 'local' },
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Docs', link: '/guide/installation' },
      { text: 'Playground', link: 'https://play.glade.sh/playground/' },
      { text: 'GitHub', link: 'https://github.com/glade-sh/glade' }
    ],
    sidebar: [
      {
        text: 'Start',
        items: [
          { text: 'Installation', link: '/guide/installation' },
          { text: 'Project Configuration', link: '/guide/configuration' },
          { text: 'CLI Reference', link: '/guide/cli-reference' }
        ]
      },
      {
        text: 'Workflows',
        items: [
          { text: 'Local Testing', link: '/guide/local-testing' },
          { text: 'Test Startup Cache', link: '/guide/test-startup-cache' },
          { text: 'Affected-Test Selection', link: '/guide/affected-tests' },
          { text: 'CI And Artifacts', link: '/guide/ci-artifacts' },
          { text: 'Rich Local Workflows', link: '/guide/rich-local-workflows' },
          { text: 'Editor, LSP, and DAP', link: '/guide/editor' },
          { text: 'Local API Server', link: '/guide/local-api-server' },
          { text: 'Playground', link: '/guide/playground' }
        ]
      },
      {
        text: 'Project Status',
        items: [
          { text: 'Support Map', link: '/guide/support-map' },
          { text: 'Compatibility', link: '/guide/compatibility' },
          { text: 'Compatibility Dashboard', link: '/guide/compatibility-dashboard' }
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/glade-sh/glade' }
    ],
    footer: {
      message: '127.0.0.1 is a fine place to test Apex.',
      copyright: 'Released by the Glade project.'
    }
  }
})
