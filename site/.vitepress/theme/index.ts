import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme-without-fonts'
import { defineAsyncComponent, h } from 'vue'
import '@fontsource-variable/host-grotesk'
import '@fontsource/monaspace-argon/400.css'
import '@fontsource/monaspace-argon/600.css'
import './custom.css'
import './styles/tokens.css'
import DocsEnhancer from './DocsEnhancer.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('GladeEditorWorkbench', defineAsyncComponent(() => import('./GladeEditorWorkbench.vue')))
  },
  Layout() {
    return h(DefaultTheme.Layout, null, {
      'layout-bottom': () => h(DocsEnhancer)
    })
  }
} satisfies Theme
