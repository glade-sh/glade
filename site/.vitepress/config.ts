import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Glade — Local Apex Workbench',
  description: 'Run local Apex checks, focused tests, snippets, and debug-log profiling from one binary with visible runtime boundaries.',
  base: '/',
  srcDir: 'docs-src',
  outDir: '.vitepress/dist',
  cleanUrls: true,
  appearance: 'dark',
  lastUpdated: false,
  vite: {
    server: {
      allowedHosts: ['apollo.local', 'tract-rear-consumers-isa.trycloudflare.com']
    }
  },
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo-mark.svg' }],
    ['meta', { name: 'theme-color', content: '#060a0d' }],
    ['meta', { name: 'description', content: 'Run local Apex checks, focused tests, snippets, and debug-log profiling from one binary with visible runtime boundaries.' }],
    ['meta', { property: 'og:title', content: 'Glade — Apex feedback before you deploy' }],
    ['meta', { property: 'og:description', content: 'Local-first Apex tooling for checks, tests, snippets, debug-log profiling, and copyable CI commands.' }],
    ['meta', { property: 'og:type', content: 'website' }]
  ],
  themeConfig: {
    siteTitle: 'Glade',
    logo: '/logo-mark.svg',
    search: { provider: 'local' },
    nav: [
      { text: 'Playground', link: '/guide/playground' },
      { text: 'Support', link: '/guide/support-map' },
      { text: 'GitHub', link: 'https://github.com/glade-sh/glade' },
      { text: 'Install', link: '/guide/installation' }
    ],
    sidebar: [
      {
        text: 'Start',
        items: [
          { text: 'Overview', link: '/guide/overview' },
          { text: 'Quickstart', link: '/guide/quickstart' },
          { text: 'Tester Field Guide', link: '/guide/tester-field-guide' },
          { text: 'Installation', link: '/guide/installation' },
          { text: 'CLI Output Modes', link: '/guide/cli-output' },
          { text: 'Exit Codes', link: '/guide/exit-codes' },
          { text: 'Support map', link: '/guide/support-map' }
        ]
      },
      {
        text: 'Core Workflows',
        items: [
          { text: 'Configure a Glade project', link: '/guide/configuration' },
          { text: 'CLI Reference', link: '/guide/cli-reference' },
          { text: 'Run Apex tests locally', link: '/guide/local-testing' },
          { text: 'Run only affected tests', link: '/guide/affected-tests' },
          { text: 'Map enterprise projects', link: '/guide/enterprise-workflows' },
          { text: 'Use the local playground', link: '/guide/playground' },
          { text: 'Run a local Salesforce-shaped API', link: '/guide/local-api-server' },
          { text: 'Add Glade to CI', link: '/guide/ci-artifacts' },
          { text: 'Automation And JSON', link: '/guide/automation' },
          { text: 'Built-In Examples', link: '/guide/examples' },
          { text: 'Error Codes And Explain', link: '/guide/errors' }
        ]
      },
      {
        text: 'Reference',
        collapsed: true,
        items: [
          { text: 'JSON Envelope', link: '/reference/json-schema' }
        ]
      },
      {
        text: 'Advanced',
        items: [
          { text: 'Speed Up Test Startup', link: '/guide/test-startup-cache' },
          { text: 'VS Code Extension, LSP, and DAP', link: '/guide/editor' },
          { text: 'Progress, Wizards, and Package Artifacts', link: '/guide/rich-local-workflows' }
        ]
      },
      {
        text: 'Plugin Authors',
        collapsed: true,
        items: [
          { text: 'Use Plugins', link: '/guide/plugins' },
          { text: 'First-Party Plugins', link: '/guide/plugins/first-party' },
          { text: 'Marketplace And Trust', link: '/guide/plugins/marketplace' },
          { text: 'Install And Manage', link: '/guide/plugins/install-manage' },
          { text: 'Build A Plugin', link: '/guide/plugins/build' },
          { text: 'Manifest Reference', link: '/guide/plugins/manifest' },
          { text: 'Publish A Plugin', link: '/guide/plugins/publish' },
          { text: 'Plugin Lock Files And CI', link: '/guide/plugins/lock-ci' }
        ]
      },
      {
        text: 'Project',
        collapsed: true,
        items: [
          { text: 'Compatibility Policy', link: '/guide/compatibility' },
          { text: 'Maintainer Proof Reports', link: '/guide/compatibility-dashboard' },
          { text: 'Brand Guide', link: '/guide/brand-guide' }
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/glade-sh/glade' }
    ],
    footer: {
      message: 'Glade is local-first Apex tooling.',
      copyright: 'Released by the Glade project.'
    }
  }
})
