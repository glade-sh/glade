import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Glade - Local Workbench for Salesforce Apex',
  description: 'Glade is a local Apex workbench for checking, testing, debugging, and exercising Salesforce-shaped APIs from one Go binary.',
  base: '/',
  srcDir: 'docs-src',
  outDir: '.vitepress/dist',
  cleanUrls: true,
  appearance: 'dark',
  lastUpdated: true,
  vite: {
    server: {
      allowedHosts: ['apollo.local']
    }
  },
  head: [
    ['meta', { name: 'theme-color', content: '#060a0d' }],
    ['meta', { name: 'description', content: 'Glade is a local Apex workbench for checking, testing, debugging, and exercising Salesforce-shaped APIs from one Go binary.' }],
    ['meta', { property: 'og:title', content: 'Glade: The local workbench for Apex.' }],
    ['meta', { property: 'og:description', content: 'Build and test Apex workflows locally before Salesforce gets involved.' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
    ['link', { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Atkinson+Hyperlegible+Next:wght@400;500;600;700&family=Fraunces:ital,opsz,wght@1,9..144,400..700&family=IBM+Plex+Mono:wght@400;500;600&family=IBM+Plex+Sans:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&family=Literata:ital,opsz,wght@0,7..72,400..700;1,7..72,400..700&family=Lora:ital,wght@0,400..700;1,400..700&family=Mona+Sans:wght@400;500;600;700;800&family=Newsreader:ital,opsz,wght@0,6..72,400..700;1,6..72,400..700&family=Source+Serif+4:ital,opsz,wght@0,8..60,400..700;1,8..60,400..700&family=Space+Grotesk:wght@400;500;600;700&display=swap' }]
  ],
  themeConfig: {
    siteTitle: 'Glade',
    logo: '/logo-mark.svg',
    search: { provider: 'local' },
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Docs', link: '/guide/installation' },
      { text: 'Playground', link: '/guide/playground' },
      { text: 'GitHub', link: 'https://github.com/glade-sh/glade' }
    ],
    sidebar: [
      {
        text: 'Start',
        items: [
          { text: 'Installation', link: '/guide/installation' },
          { text: 'Project Configuration', link: '/guide/configuration' },
          { text: 'CLI Reference', link: '/guide/cli-reference' },
          { text: 'Brand Guide', link: '/guide/brand-guide' }
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
        text: 'Plugins',
        items: [
          { text: 'Overview', link: '/guide/plugins' },
          { text: 'First-Party Plugins', link: '/guide/plugins/first-party' },
          { text: 'Marketplace', link: '/guide/plugins/marketplace' },
          { text: 'Install And Manage', link: '/guide/plugins/install-manage' },
          { text: 'Build A Plugin', link: '/guide/plugins/build' },
          { text: 'Publish A Plugin', link: '/guide/plugins/publish' },
          { text: 'Manifest Reference', link: '/guide/plugins/manifest' },
          { text: 'Lock Files And CI', link: '/guide/plugins/lock-ci' }
        ]
      },
      {
        text: 'Project Status',
        items: [
          { text: 'Apex and Salesforce Support', link: '/guide/support-map' },
          { text: 'Compatibility Policy', link: '/guide/compatibility' },
          { text: 'Developer Reports', link: '/guide/compatibility-dashboard' }
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
