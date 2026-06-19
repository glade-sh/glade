import { defineConfig } from 'vitepress'

const tunnelAllowedHosts = [
  'apollo.local',
  '.trycloudflare.com',
  '.ngrok-free.app',
  '.ngrok.app',
  '.ngrok.io'
]

function isVueUsePureAnnotationWarning(warning: { code?: string; id?: string; message?: string }) {
  return (
    warning.code === 'INVALID_ANNOTATION' &&
    warning.id?.includes('@vueuse/core') &&
    warning.message?.includes('#__PURE__')
  )
}

export default defineConfig({
  title: 'Glade - Local Apex Runtime for SFDX Projects',
  description: 'Local Apex checks and focused tests before the Salesforce validation gate.',
  base: '/',
  srcDir: 'docs-src',
  outDir: '.vitepress/dist',
  cleanUrls: true,
  appearance: 'force-dark',
  lastUpdated: false,
  vite: {
    server: {
      allowedHosts: tunnelAllowedHosts
    },
    preview: {
      allowedHosts: tunnelAllowedHosts
    },
    build: {
      rollupOptions: {
        onwarn(warning, warn) {
          if (isVueUsePureAnnotationWarning(warning)) return
          warn(warning)
        }
      }
    }
  },
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo-mark.svg' }],
    ['script', { defer: true, src: '/js/highlight.js' }],
    ['script', { defer: true, src: '/js/home.js' }],
    ['meta', { name: 'theme-color', content: '#060a0d' }],
    ['meta', { name: 'description', content: 'Local Apex checks and focused tests before the Salesforce validation gate.' }],
    ['meta', { property: 'og:title', content: 'Glade — Local Apex runtime for SFDX projects' }],
    ['meta', { property: 'og:description', content: 'Run supported Apex checks before the Salesforce round trip.' }],
    ['meta', { property: 'og:type', content: 'website' }]
  ],
  themeConfig: {
    siteTitle: 'Glade',
    logo: '/logo-mark.svg',
    search: { provider: 'local' },
    nav: [
      { text: 'Install', link: '/guide/installation' },
      { text: 'What runs locally', link: '/guide/support-map' },
      { text: 'VS Code', link: '/guide/editor' },
      { text: 'Playground', link: '/guide/playground' },
      { text: 'Docs', link: '/guide/overview' },
      { text: 'GitHub', link: 'https://github.com/glade-sh/glade' }
    ],
    sidebar: [
      {
        text: 'Start',
        items: [
          { text: 'What is Glade?', link: '/guide/overview' },
          { text: 'Install', link: '/guide/installation' },
          { text: 'First local check', link: '/guide/quickstart' },
          { text: 'Tester field guide', link: '/guide/tester-field-guide' },
          { text: 'What runs locally', link: '/guide/support-map' },
          { text: 'Playground', link: '/guide/playground' }
        ]
      },
      {
        text: 'Workflows',
        collapsed: true,
        items: [
          { text: 'Check source', link: '/guide/quickstart#3-check-source' },
          { text: 'Run tests', link: '/guide/local-testing' },
          { text: 'Local LWC shell', link: '/guide/lwc-local-shell' },
          { text: 'Affected tests', link: '/guide/affected-tests' },
          { text: 'Local API routes', link: '/guide/local-api-server' },
          { text: 'sf target orgs', link: '/guide/glade-orgs' },
          { text: 'CI', link: '/guide/ci-artifacts' },
          { text: 'VS Code', link: '/guide/editor' },
          { text: 'Workbench', link: '/guide/workbench' }
        ]
      },
      {
        text: 'Reference',
        collapsed: true,
        items: [
          { text: 'CLI reference', link: '/guide/cli-reference' },
          { text: 'Output modes', link: '/guide/cli-output' },
          { text: 'Exit codes', link: '/guide/exit-codes' },
          { text: 'JSON envelope', link: '/reference/json-schema' },
          { text: 'Automation and JSON', link: '/guide/automation' },
          { text: 'Error codes and `glade explain`', link: '/guide/errors' }
        ]
      },
      {
        text: 'Advanced',
        collapsed: true,
        items: [
          { text: 'Enterprise projects', link: '/guide/enterprise-workflows' },
          { text: 'Test startup cache', link: '/guide/test-startup-cache' },
          { text: 'Reports and package artifacts', link: '/guide/rich-local-workflows' },
          { text: 'Built-in examples', link: '/guide/examples' },
          {
            text: 'Plugins',
            link: '/guide/plugins',
            collapsed: true,
            items: [
              { text: 'First-party plugins', link: '/guide/plugins/first-party' },
              { text: 'Install and manage', link: '/guide/plugins/install-manage' },
              { text: 'Lock files and CI', link: '/guide/plugins/lock-ci' },
              { text: 'Build a plugin', link: '/guide/plugins/build' },
              { text: 'Manifest reference', link: '/guide/plugins/manifest' },
              { text: 'Marketplace', link: '/guide/plugins/marketplace' },
              { text: 'Publish', link: '/guide/plugins/publish' }
            ]
          }
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
