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
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:url', content: 'https://glade.sh/' }],
    ['meta', { property: 'og:site_name', content: 'Glade' }],
    ['meta', { property: 'og:image', content: 'https://glade.sh/social-card.png' }],
    ['meta', { property: 'og:image:secure_url', content: 'https://glade.sh/social-card.png' }],
    ['meta', { property: 'og:image:type', content: 'image/png' }],
    ['meta', { property: 'og:image:width', content: '1200' }],
    ['meta', { property: 'og:image:height', content: '630' }],
    ['meta', { property: 'og:image:alt', content: 'Glade local Apex runtime social preview' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:title', content: 'Glade — Local Apex runtime for SFDX projects' }],
    ['meta', { name: 'twitter:description', content: 'Run supported Apex checks before the Salesforce round trip.' }],
    ['meta', { name: 'twitter:image', content: 'https://glade.sh/social-card.png' }],
    ['meta', { name: 'twitter:image:alt', content: 'Glade local Apex runtime social preview' }]
  ],
  themeConfig: {
    siteTitle: 'Glade',
    logo: '/logo-mark.svg',
    search: { provider: 'local' },
    nav: [
      { text: 'Install', link: '/guide/installation' },
      { text: 'Workflows', link: '/guide/workflows' },
      { text: 'Product areas', link: '/guide/modules' },
      { text: 'What runs locally', link: '/guide/support-map' },
      { text: 'Reference', link: '/reference/cli' },
      { text: 'Help', link: '/help/' },
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
          { text: 'Choose a workflow', link: '/guide/workflows' },
          { text: 'What runs locally', link: '/guide/support-map' },
          { text: 'Security & Trust', link: '/guide/security-trust' },
          { text: 'Playground', link: '/guide/playground' }
        ]
      },
      {
        text: 'Workflows',
        collapsed: false,
        items: [
          { text: 'Run Apex tests', link: '/guide/workflows/apex-tests' },
          { text: 'Debug Apex', link: '/guide/workflows/debug-apex' },
          { text: 'Execute anonymous Apex and SOQL', link: '/guide/workbench' },
          { text: 'Work with local data', link: '/guide/workflows/local-data' },
          { text: 'Preview LWC', link: '/guide/workflows/lwc-preview' },
          { text: 'Preview Visualforce', link: '/guide/workflows/visualforce-preview' },
          { text: 'Add Glade to CI', link: '/guide/workflows/ci' },
          { text: 'Use VS Code', link: '/guide/editor' },
          { text: 'Use plugins', link: '/guide/plugins' }
        ]
      },
      {
        text: 'Product areas',
        collapsed: false,
        items: [
          { text: 'Product area overview', link: '/guide/modules' },
          { text: 'Apex runtime', link: '/guide/modules/apex-runtime' },
          { text: 'Test runner', link: '/guide/modules/test-runner' },
          { text: 'Local org and data', link: '/guide/modules/local-org-data' },
          { text: 'LWC preview', link: '/guide/modules/lwc-preview' },
          { text: 'Visualforce preview', link: '/guide/modules/visualforce-preview' },
          { text: 'Debug and profile', link: '/guide/modules/debug-profile' },
          { text: 'Editor and workbench', link: '/guide/modules/editor' },
          { text: 'Plugins', link: '/guide/modules/plugins' }
        ]
      },
      {
        text: 'Reference',
        collapsed: true,
        items: [
          { text: 'CLI reference', link: '/reference/cli' },
          { text: 'Output modes', link: '/guide/cli-output' },
          { text: 'Config reference', link: '/reference/config' },
          { text: 'JSON envelope', link: '/reference/json-schema' },
          { text: 'Automation and JSON', link: '/guide/automation' },
          { text: 'Exit codes', link: '/guide/exit-codes' },
          { text: 'Error codes', link: '/reference/errors' },
          { text: 'Apex language compatibility', link: '/reference/apex-language-compatibility' },
          { text: 'Apex support map', link: '/reference/apex-support' },
          { text: 'LWC support matrix', link: '/reference/lwc-support' },
          { text: 'Visualforce support matrix', link: '/reference/visualforce-support' },
          { text: 'Local API routes', link: '/reference/local-api-routes' }
        ]
      },
      {
        text: 'Guided help',
        collapsed: true,
        items: [
          { text: 'Help overview', link: '/help/' },
          { text: 'First local check', link: '/help/first-local-check' },
          { text: 'Run one Apex test', link: '/help/run-one-apex-test' },
          { text: 'Debug with breakpoints', link: '/help/debug-apex-vscode' },
          { text: 'Anonymous Apex scratch', link: '/help/anonymous-apex-scratch' },
          { text: 'Local data environments', link: '/help/local-data-environments' },
          { text: 'Changed tests before a PR', link: '/help/changed-tests-before-pr' },
          { text: 'Glade org data import', link: '/help/glade-org-sf-data-import' },
          { text: 'Profile a debug log', link: '/help/profile-apex-debug-log' },
          { text: 'CI setup', link: '/help/ci-setup' }
        ]
      },
      {
        text: 'Advanced',
        collapsed: true,
        items: [
          { text: 'Enterprise projects', link: '/guide/enterprise-workflows' },
          { text: 'Affected tests', link: '/guide/affected-tests' },
          { text: 'Test startup cache', link: '/guide/test-startup-cache' },
          { text: 'Reports and package artifacts', link: '/guide/rich-local-workflows' },
          { text: 'Built-in examples', link: '/guide/examples' },
          { text: 'Local API routes', link: '/guide/local-api-server' },
          { text: 'sf target orgs', link: '/guide/glade-orgs' },
          { text: 'CI artifacts', link: '/guide/ci-artifacts' },
          { text: 'Plugin install and manage', link: '/guide/plugins/install-manage' },
          { text: 'Plugin lock files and CI', link: '/guide/plugins/lock-ci' },
          { text: 'First-party plugins', link: '/guide/plugins/first-party' },
          { text: 'AI-assisted Apex', link: '/guide/ai-assisted-apex' }
        ]
      },
      {
        text: 'Maintainer',
        collapsed: true,
        items: [
          { text: 'Maintainer home', link: '/maintainer/' },
          { text: 'Extend runtime support', link: '/maintainer/extend-runtime' },
          { text: 'Release runbook', link: '/maintainer/release' },
          { text: 'glade-tools', link: '/maintainer/glade-tools' },
          { text: 'Plugin runtime', link: '/maintainer/plugin-runtime' }
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
