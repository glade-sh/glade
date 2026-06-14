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
      { text: 'Support map', link: '/guide/support-map' },
      { text: 'Docs', link: '/guide/overview' },
      { text: 'GitHub', link: 'https://github.com/glade-sh/glade' },
      { text: 'Install', link: '/guide/installation' }
    ],
    sidebar: [
      {
        text: 'Start',
        items: [
          { text: 'What is Glade?', link: '/guide/overview' },
          { text: 'Install', link: '/guide/installation' },
          { text: 'First local check', link: '/guide/quickstart' },
          { text: 'Support map', link: '/guide/support-map' },
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
          { text: 'Local API server', link: '/guide/local-api-server' },
          { text: 'CI', link: '/guide/ci-artifacts' },
          { text: 'VS Code', link: '/guide/editor' }
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
          { text: 'Plugins', link: '/guide/plugins' }
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
