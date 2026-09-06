import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme-without-fonts'
import { defineAsyncComponent, h } from 'vue'
import { useData } from 'vitepress'
import '@fontsource-variable/inter/wght.css'
import '@fontsource/ibm-plex-mono/400.css'
import '@fontsource/ibm-plex-mono/500.css'
import './custom.css'
import './styles/tokens.css'
import './styles/reading.css'
import ArticleContext from './ArticleContext.vue'
import DocsEnhancer from './DocsEnhancer.vue'
import GladeNotFound from './GladeNotFound.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('GladeEditorWorkbench', defineAsyncComponent(() => import('./GladeEditorWorkbench.vue')))
    app.component('GladeSupportExplorer', defineAsyncComponent(() => import('./GladeSupportExplorer.vue')))
  },
  Layout() {
    const { frontmatter } = useData()
    // Keep VitePress layout watchers mounted across home/docs navigation.
    // The homepage's layout:false uses its native bare-Content branch.
    return h(DefaultTheme.Layout, { class: frontmatter.value.gladeHomepage ? undefined : 'glade-docs' }, {
      'doc-before': () => h(ArticleContext),
      'not-found': () => h(GladeNotFound),
      'layout-bottom': () => h(DocsEnhancer)
    })
  }
} satisfies Theme
