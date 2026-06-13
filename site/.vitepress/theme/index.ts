import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme-without-fonts'
import { h } from 'vue'
import '@fontsource-variable/host-grotesk'
import '@fontsource/monaspace-argon/400.css'
import '@fontsource/monaspace-argon/600.css'
import './custom.css'
import DocsEnhancer from './DocsEnhancer.vue'
import DocsNavTitleSuffix from './DocsNavTitleSuffix.vue'

export default {
  extends: DefaultTheme,
  Layout() {
    return h(DefaultTheme.Layout, null, {
      'layout-bottom': () => h(DocsEnhancer),
      'nav-bar-title-after': () => h(DocsNavTitleSuffix)
    })
  }
} satisfies Theme
