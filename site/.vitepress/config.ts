import { defineConfig } from 'vitepress'
import routeManifest from '../routes.json'

const tunnelAllowedHosts = [
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

const noindexRoutes = new Set(
  routeManifest.routes
    .filter((entry) => entry.classification === 'noindex' || entry.classification === 'redirect')
    .map((entry) => entry.route)
)
const buildCommit = process.env.CF_PAGES_COMMIT_SHA || process.env.GITHUB_SHA || 'local-preview'

const descriptions: Record<string, string> = {
  'index.md': 'Check source, run supported Apex tests, and debug in your Salesforce DX project on your own machine. Keep Salesforce as the final validation gate.',
  'guide/index.md': 'Install Glade, run a small local Apex test, or choose a workflow for your Salesforce DX project. Find support boundaries and recovery steps.',
  'guide/installation.md': 'Choose a Glade release for macOS or Linux, review the installer, verify downloads, and resolve PATH or platform setup problems.',
  'guide/quickstart.md': 'Choose a small sample, a local browser example, or your own Salesforce DX project. Check the named test and executed count before moving on.',
  'guide/workflows.md': 'Choose a local Glade workflow for Apex tests, debugging, local data, UI previews, or CI.',
  'guide/support-map.md': 'Read supported local Apex paths, deterministic models, named limitations, and behaviors that still need Salesforce validation.',
  'guide/workbench.md': 'Explore local capability boundaries and replay prepared workflow output. This website does not execute Apex; run Glade locally for execution.',
  'guide/security-trust.md': 'Review local execution boundaries, private vulnerability reporting, and fail-closed download verification before using a Glade release.',
  'reference/cli.md': 'Look up Glade command behavior, flags, output formats, configuration, and local Salesforce compatibility.',
  'help/index.md': 'Diagnose a Glade project, test, editor, data or CI problem and follow a safe recovery path.',
  'help/troubleshooting.md': 'Diagnose project discovery, installation, test selection and editor problems. Follow scoped recovery steps and report a sanitized reproduction.',
  'maintainer/index.md': 'Improve a focused local workflow, documentation or accessibility. Find the right repository and distinguish implementation from compatibility evidence.'
}

function routeFor(relativePath: string) {
  if (relativePath === 'index.md') return '/'
  return `/${relativePath.replace(/(?:^|\/)index\.md$/, '').replace(/\.md$/, '')}${relativePath.endsWith('/index.md') ? '/' : ''}`
}

function descriptionFor(relativePath: string, title: string) {
  if (descriptions[relativePath]) return descriptions[relativePath]
  const sentence = /[.?!]$/.test(title) ? title : `${title}.`
  if (relativePath.startsWith('help/')) {
    return relativePath === 'help/troubleshooting.md'
      ? `${sentence} Follow the shortest diagnostic and recovery path.`
      : `${sentence} Follow a focused task guide with expected Glade results.`
  }
  if (relativePath.startsWith('reference/')) {
    return `${sentence} Find exact Glade behavior, supported local paths, and the boundary with Salesforce.`
  }
  if (relativePath.startsWith('guide/workflows/')) {
    return `${sentence} Follow the local workflow with commands, expected results, and Salesforce boundaries.`
  }
  if (relativePath.startsWith('maintainer/')) {
    return `${sentence} Use the checked Glade maintainer runbook and repository commands.`
  }
  return `${sentence} Understand the Glade task, complete the local work, and identify when Salesforce is required.`
}

function resolvedDescription(pageData: { relativePath: string; title: string; frontmatter?: Record<string, unknown> }) {
  const explicit = pageData.frontmatter?.description
  return typeof explicit === 'string' && explicit.trim()
    ? explicit.trim()
    : descriptionFor(pageData.relativePath, pageData.title)
}

export default defineConfig({
  title: 'Glade',
  description: 'Run and test supported Salesforce Apex locally from a Salesforce DX project.',
  base: '/',
  srcDir: 'docs-src',
  outDir: '.vitepress/dist',
  cleanUrls: true,
  appearance: true,
  // Keep reference catalogs and demo code off unrelated reading routes.
  router: { prefetchLinks: false },
  sitemap: {
    hostname: 'https://glade.sh',
    transformItems: (items) => items.filter((item) => !noindexRoutes.has(new URL(item.url, 'https://glade.sh').pathname))
  },
  transformPageData(pageData) {
    return { description: resolvedDescription(pageData) }
  },
  transformHead(ctx) {
    const route = routeFor(ctx.pageData.relativePath)
    const canonical = `https://glade.sh${route}`
    const title = route === '/' ? 'Glade — Local Apex Runtime for Salesforce Developers' : `${ctx.pageData.title} | Glade`
    const description = resolvedDescription(ctx.pageData)
    const type = route === '/' ? 'website' : 'article'
    const head: [string, Record<string, string>][] = [
      ['link', { rel: 'canonical', href: canonical }],
      ['meta', { property: 'og:title', content: title }],
      ['meta', { property: 'og:description', content: description }],
      ['meta', { property: 'og:type', content: type }],
      ['meta', { property: 'og:url', content: canonical }],
      ['meta', { name: 'twitter:title', content: title }],
      ['meta', { name: 'twitter:description', content: description }]
    ]
    if (ctx.pageData.isNotFound || noindexRoutes.has(route)) {
      head.push(['meta', { name: 'robots', content: 'noindex' }])
    }
    return head
  },
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
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo-mark-topo.svg' }],
    ['meta', { name: 'theme-color', content: '#0b1119' }],
    ['meta', { property: 'og:site_name', content: 'Glade' }],
    ['meta', { property: 'og:image', content: 'https://glade.sh/social-card.png' }],
    ['meta', { property: 'og:image:secure_url', content: 'https://glade.sh/social-card.png' }],
    ['meta', { property: 'og:image:type', content: 'image/png' }],
    ['meta', { property: 'og:image:width', content: '1200' }],
    ['meta', { property: 'og:image:height', content: '630' }],
    ['meta', { property: 'og:image:alt', content: 'Glade: run and debug supported Apex locally, with Salesforce as the final validation gate' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:image', content: 'https://glade.sh/social-card.png' }],
    ['meta', { name: 'twitter:image:alt', content: 'Glade: run and debug supported Apex locally, with Salesforce as the final validation gate' }],
    ['meta', { name: 'glade:commit', content: buildCommit }]
  ],
  themeConfig: {
    siteTitle: 'glade.sh',
    logo: { light: '/logo-mark-topo-light.svg', dark: '/logo-mark-topo.svg' },
    search: { provider: 'local' },
    nav: [
      { text: 'Docs', link: '/guide/', activeMatch: '^/guide/(?!workflows(?:/|$)|support-map$|workbench$|installation$)' },
      { text: 'Guides', link: '/guide/workflows', activeMatch: '^/guide/workflows(?:/|$)' },
      { text: 'Reference', link: '/reference/cli', activeMatch: '^/reference/' },
      { text: 'Compatibility', link: '/guide/support-map', activeMatch: '^/guide/(?:support-map|workbench)$' },
      { text: 'Help', link: '/help/', activeMatch: '^/help/' },
      { text: 'Install', link: '/guide/installation' }
    ],
    sidebar: {
      '/guide/': [
      {
        text: 'Start',
        items: [
          { text: 'Documentation home', link: '/guide/' },
          { text: 'What is Glade?', link: '/guide/overview' },
          { text: 'Install', link: '/guide/installation' },
          { text: 'First local check', link: '/guide/quickstart' },
          { text: 'Choose a workflow', link: '/guide/workflows' },
          { text: 'What runs locally', link: '/guide/support-map' }
        ]
      },
      {
        text: 'Guides',
        collapsed: true,
        items: [
          { text: 'Run Apex tests', link: '/guide/workflows/apex-tests' },
          { text: 'Debug Apex', link: '/guide/workflows/debug-apex' },
          { text: 'Execute anonymous Apex and SOQL', link: '/guide/playground' },
          { text: 'Work with local data', link: '/guide/workflows/local-data' },
          { text: 'Preview LWC', link: '/guide/workflows/lwc-preview' },
          { text: 'Preview Visualforce', link: '/guide/workflows/visualforce-preview' },
          { text: 'Add Glade to CI', link: '/guide/workflows/ci' },
          { text: 'Use VS Code', link: '/guide/editor' }
        ]
      },
      {
        text: 'How Glade works',
        collapsed: true,
        items: [
          { text: 'Architecture and capabilities', link: '/guide/modules' },
          { text: 'Capability explorer', link: '/guide/workbench' }
        ]
      },
      {
        text: 'Trust & adoption',
        collapsed: true,
        items: [
          { text: 'Security & trust', link: '/guide/security-trust' },
          { text: 'Pilot Glade on a real project', link: '/guide/tester-field-guide' }
        ]
      },
      {
        text: 'Advanced',
        collapsed: true,
        items: [
          { text: 'Output modes', link: '/guide/cli-output' },
          { text: 'Automation and JSON', link: '/guide/automation' },
          { text: 'Affected tests', link: '/guide/affected-tests' },
          { text: 'Test startup cache', link: '/guide/test-startup-cache' },
          { text: 'Reports and package artifacts', link: '/guide/rich-local-workflows' },
          { text: 'Built-in examples', link: '/guide/examples' },
          { text: 'Build from source', link: '/guide/build-from-source' },
          { text: 'Local API routes', link: '/guide/local-api-server' },
          { text: 'Use Glade as a local sf target', link: '/guide/glade-orgs' },
          { text: 'CI artifacts', link: '/guide/ci-artifacts' },
          { text: 'Plugins', link: '/guide/plugins' },
          { text: 'Plugin install and manage', link: '/guide/plugins/install-manage' },
          { text: 'Plugin lock files and CI', link: '/guide/plugins/lock-ci' },
          { text: 'First-party plugins', link: '/guide/plugins/first-party' },
          { text: 'AI-assisted Apex', link: '/guide/ai-assisted-apex' }
        ]
      }
      ],
      '/reference/': [
      {
        text: 'Reference',
        items: [
          { text: 'CLI reference', link: '/reference/cli' },
          { text: 'Configuration', link: '/reference/config' },
          { text: 'LSP reference', link: '/reference/lsp' },
          { text: 'DAP reference', link: '/reference/dap' },
          { text: 'Error codes', link: '/reference/errors' },
          { text: 'JSON envelope', link: '/reference/json-schema' },
          { text: 'Apex language compatibility', link: '/reference/apex-language-compatibility' },
          { text: 'Apex support map', link: '/reference/apex-support' },
          { text: 'LWC support matrix', link: '/reference/lwc-support' },
          { text: 'Visualforce support matrix', link: '/reference/visualforce-support' },
          { text: 'Local API routes', link: '/reference/local-api-routes' }
        ]
      }
      ],
      '/help/': [
      {
        text: 'Help',
        items: [
          { text: 'Help home', link: '/help/' },
          { text: 'Fix a problem', link: '/help/troubleshooting' }
        ]
      },
      {
        text: 'Illustrated walkthroughs',
        collapsed: true,
        items: [
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
      }
      ],
      '/maintainer/': [
      {
        text: 'Contributors',
        items: [
          { text: 'Contributors home', link: '/maintainer/' },
          { text: 'Extend runtime support', link: '/maintainer/extend-runtime' },
          { text: 'Release runbook', link: '/maintainer/release' },
          { text: 'Develop the VS Code extension', link: '/maintainer/editor-extension' },
          { text: 'glade-tools', link: '/maintainer/glade-tools' },
          { text: 'Plugin runtime', link: '/maintainer/plugin-runtime' }
        ]
      }
      ]
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/glade-sh/glade' }
    ],
    footer: {
      message: 'Glade is an independent project and is not affiliated with, sponsored by, or endorsed by Salesforce. Salesforce and Apex are trademarks of Salesforce, Inc. · <a href="/maintainer/">Contributors</a> · <a href="/guide/security-trust">Security</a> · <a href="https://github.com/glade-sh/glade/releases">Releases</a> · <a href="https://github.com/glade-sh/glade/blob/main/LICENSE">Apache-2.0</a>',
      copyright: 'Copyright 2026 Matt Simonis.'
    }
  }
})
