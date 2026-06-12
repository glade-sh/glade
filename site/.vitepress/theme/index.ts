import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import { FlaskConical, PlayCircle, SearchCheck, ServerCog, SquareTerminal } from '@lucide/vue'
import './custom.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('IconSearchCheck', SearchCheck)
    app.component('IconFlaskConical', FlaskConical)
    app.component('IconPlayCircle', PlayCircle)
    app.component('IconSquareTerminal', SquareTerminal)
    app.component('IconServerCog', ServerCog)
  }
} satisfies Theme
