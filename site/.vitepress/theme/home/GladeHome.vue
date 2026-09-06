<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { withBase } from 'vitepress'
import releaseManifest from '../../../release-manifest.json'
import { INSTALL_COMMAND, commands, tokenize } from './home-demo'

type Scenario = 'tests' | 'debug' | 'check'
const scenario = ref<Scenario>('tests')
const tool = ref('vscode')
const state = reactive({ failing: false, fixed: false, debugCompleted: false, testsPending: false, checkPending: false })
const busy = ref(false)
const hydrated = ref(false)
const reducedMotion = ref(false)
const menuOpen = ref(false)
const scrolled = ref(false)
const header = ref<HTMLElement>()
const menuToggle = ref<HTMLButtonElement>()
const installCommand = ref<HTMLElement>()
const copied = ref(false)
const toast = ref('')
let runTimer: ReturnType<typeof setTimeout> | undefined
let copyTimer: ReturnType<typeof setTimeout> | undefined
let toastTimer: ReturnType<typeof setTimeout> | undefined
let alive = false
const code = computed(() => scenario.value === 'tests' ? [
 '@IsTest', 'private class AccountServiceTest {', '  @IsTest static void createsAccount() {',
 "    Account acc = new Account(Name = 'Acme');", '    insert acc;',
 `    Assert.areEqual('${state.failing ? 'Other' : 'Acme'}', acc.Name);`, '  }', '}'
] : scenario.value === 'debug' ? [
 'public class AccountService {', '  public static Account create() {', '    Account acc = new Account(',
 "      Name = 'Acme');", '    insert acc;', '    return acc;', '  }', '}'
] : [
 'public class AccountService {', '  public static String accountName() {', "    String name = 'Acme';",
 `    return ${state.fixed ? 'name' : 'acountName'};`, '  }', '}', '', '// Local feedback, before a deploy.'
])
function lineClass(index: number) {
 if (scenario.value === 'tests') return index === 5 && state.failing ? 'line-error' : ''
 if (scenario.value === 'debug') return index === 4 && !state.debugCompleted ? 'highlighted' : ''
 return index === 3 ? state.fixed ? 'highlighted' : 'line-error' : ''
}
const output = computed(() => {
 const result = (kind: string, message: string, note: string, pill = '') => ({ kind, message, note, pill })
 if (busy.value) return result('neutral', 'Running the illustrative example…', 'This preview does not execute Apex.')
 if (scenario.value === 'tests') {
  if (state.testsPending) return result('neutral', state.failing ? "Expected name changed to 'Other'." : "Expected name restored to 'Acme'.", 'Run the example to see the new test result.')
  return state.failing ? result('failure', 'createsAccount · assertion failed', 'Line 6 — Expected: Other, Actual: Acme') : result('success', 'AccountServiceTest.createsAccount', '1 test executed · 1 passed · 0 failed', 'Passed')
 }
 if (scenario.value === 'debug') return state.debugCompleted ? result('success', 'Local debug session completed', 'AccountService.create() returned an Account.') : result('warning', 'Paused at breakpoint · line 5', 'acc.Name = "Acme" · inspect before insert')
 if (state.checkPending) return result('neutral', state.fixed ? 'Source updated: return name;' : 'Sample source error restored.', 'Run the check to see the updated diagnostic.')
 return state.fixed ? result('success', 'No source diagnostics', 'The sample now returns the declared variable.') : result('failure', 'Cannot resolve variable acountName', 'AccountService.cls:4 · 1 diagnostic')
})
const runLabel = computed(() => scenario.value === 'tests' ? 'Run example' : scenario.value === 'debug' ? state.debugCompleted ? 'Restart' : 'Continue' : 'Run check')
const editLabel = computed(() => scenario.value === 'tests' ? state.failing ? 'Restore passing test' : 'Try a failing test' : scenario.value === 'debug' ? 'Restart debugging' : state.fixed ? 'Restore the diagnostic' : 'Apply the sample fix')
function selectScenario(value: Scenario) { clearTimeout(runTimer); busy.value = false; scenario.value = value }
function run() {
 if (busy.value) return
 if (scenario.value === 'debug' && state.debugCompleted) { state.debugCompleted = false; return }
 busy.value = true
 const selected = scenario.value
 runTimer = setTimeout(() => {
  if (!alive || scenario.value !== selected) return
  busy.value = false
  if (selected === 'debug') state.debugCompleted = true
  if (selected === 'tests') state.testsPending = false
  if (selected === 'check') state.checkPending = false
 }, reducedMotion.value ? 0 : 750)
}
function edit() {
 if (busy.value) return
 if (scenario.value === 'tests') { state.failing = !state.failing; state.testsPending = true }
 else if (scenario.value === 'check') { state.fixed = !state.fixed; state.checkPending = true }
 else state.debugCompleted = false
}
function tabKey(event: KeyboardEvent) {
 const tabs = Array.from((event.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('[role="tab"]'))
 const index = tabs.indexOf(event.target as HTMLButtonElement)
 if (index < 0) return
 const next = event.key === 'ArrowRight' ? (index + 1) % tabs.length : event.key === 'ArrowLeft' ? (index - 1 + tabs.length) % tabs.length : event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : -1
 if (next < 0) return
 event.preventDefault(); tabs[next].focus(); tabs[next].click()
}
function closeMenu(restore = false) { menuOpen.value = false; if (restore) menuToggle.value?.focus() }
function closeForNavigation(event: MouseEvent) {
 if ((event.target as Element).closest('a')) closeMenu()
}
function focusHash(event: MouseEvent) {
 const link = (event.target as Element).closest('a')
 const hash = link?.getAttribute('href')
 if (!hash?.startsWith('#')) return
 const destination = document.getElementById(hash.slice(1))
 if (destination) { destination.setAttribute('tabindex', '-1'); destination.focus({ preventScroll: true }) }
}
function onScroll() { scrolled.value = window.scrollY > 8 }
function onKey(event: KeyboardEvent) { if (event.key === 'Escape' && menuOpen.value) closeMenu(true) }
function outside(event: MouseEvent) { if (menuOpen.value && !header.value?.contains(event.target as Node)) closeMenu() }
function announce(message: string) { clearTimeout(toastTimer); toast.value = message; toastTimer = setTimeout(() => { toast.value = '' }, 3200) }
async function copyInstall() {
 let success = false
 try { await navigator.clipboard.writeText(INSTALL_COMMAND); success = true } catch { /* Select the visible command below; never claim success after denial. */ }
 if (!alive) return
 clearTimeout(copyTimer)
 if (success) {
  copied.value = true
  announce('Install command copied. Nothing has been installed.')
  copyTimer = setTimeout(() => { copied.value = false }, 2400)
 } else {
  copied.value = false
  if (installCommand.value) {
   const range = document.createRange(); range.selectNodeContents(installCommand.value)
   const selection = window.getSelection(); selection?.removeAllRanges(); selection?.addRange(range)
  }
  announce('Command selected. Use Copy, or press ⌘C / Ctrl+C to copy.')
 }
}
let wide: MediaQueryList | undefined
function onWide() { if (wide?.matches && menuOpen.value) closeMenu(header.value?.contains(document.activeElement) ?? false) }
onMounted(() => {
 alive = true; hydrated.value = true
 reducedMotion.value = window.matchMedia('(prefers-reduced-motion: reduce)').matches
 wide = window.matchMedia('(min-width:761px)'); wide.addEventListener('change', onWide)
 window.addEventListener('scroll', onScroll, { passive: true }); onScroll()
 document.addEventListener('keydown', onKey); document.addEventListener('click', outside)
})
onUnmounted(() => {
 alive = false
 clearTimeout(runTimer); clearTimeout(copyTimer); clearTimeout(toastTimer)
 wide?.removeEventListener('change', onWide)
 window.removeEventListener('scroll', onScroll)
 document.removeEventListener('keydown', onKey); document.removeEventListener('click', outside)
})

</script>
<template>
<div class="glade-home" id="top" @click="focusHash">
<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" style="position:absolute;width:0;height:0;overflow:hidden"><defs><linearGradient id="topo-color" x1="11" y1="6" x2="33" y2="47" gradientUnits="userSpaceOnUse"><stop stop-color="#78ddea"/><stop offset=".46" stop-color="#829eff"/><stop offset="1" stop-color="#a080ff"/></linearGradient></defs><symbol id="i-arrow" viewBox="0 0 24 24"><path d="M4 12h15m-6-6 6 6-6 6"/></symbol><symbol id="i-arrow-up" viewBox="0 0 24 24"><path d="M6 18 18 6M6 6h12v12"/></symbol><symbol id="i-chevron" viewBox="0 0 24 24"><path d="m9 5 7 7-7 7"/></symbol><symbol id="i-github" viewBox="0 0 24 24"><path d="M9 19c-4.5 1.4-4.5-2.2-6.3-2.6M15.5 22v-3.5a3 3 0 0 0-.9-2.3c3-.3 6.2-1.5 6.2-6.7a5.2 5.2 0 0 0-1.4-3.7 4.9 4.9 0 0 0-.1-3.6s-1.1-.4-3.8 1.4a13 13 0 0 0-6.9 0C5.9 1.8 4.8 2.2 4.8 2.2a4.9 4.9 0 0 0-.1 3.6 5.2 5.2 0 0 0-1.4 3.7c0 5.2 3.1 6.4 6.1 6.7a3 3 0 0 0-.9 2.3V22"/></symbol><symbol id="i-play" viewBox="0 0 24 24"><path d="m8 5 11 7-11 7z"/></symbol><symbol id="i-check" viewBox="0 0 24 24"><path d="m5 12 4 4L19 6"/></symbol><symbol id="i-check-circle" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="m8 12 3 3 5-6"/></symbol><symbol id="i-close" viewBox="0 0 24 24"><path d="m6 6 12 12M6 18 18 6"/></symbol><symbol id="i-code" viewBox="0 0 24 24"><path d="m7 6-6 6 6 6m10-12 6 6-6 6M14 3l-4 18"/></symbol><symbol id="i-file" viewBox="0 0 24 24"><path d="M14 2H5v20h14V7zM14 2v6h5M8 12h8m-8 4h6"/></symbol><symbol id="i-folder" viewBox="0 0 24 24"><path d="M3 6h6l2 2h10v12H3zM3 6V4h6l2 2"/></symbol><symbol id="i-branch" viewBox="0 0 24 24"><circle cx="6" cy="4" r="2"/><circle cx="6" cy="20" r="2"/><circle cx="18" cy="6" r="2"/><path d="M6 6v12m0-3c0-5 12-1 12-7"/></symbol><symbol id="i-copy" viewBox="0 0 24 24"><rect x="8" y="8" width="12" height="13" rx="2"/><path d="M15 8V3H3v13h5"/></symbol><symbol id="i-spark" viewBox="0 0 24 24"><path d="m12 3 2.3 6.7L21 12l-6.7 2.3L12 21l-2.3-6.7L3 12l6.7-2.3zM20 2v4m-2-2h4"/></symbol><symbol id="i-bug" viewBox="0 0 24 24"><rect x="7" y="7" width="10" height="14" rx="5"/><path d="m8 3 2 4m6-4-2 4M7 11H3m18 0h-4M7 16H2m20 0h-5M12 7v13M3 4l4 4m14-4-4 4"/></symbol><symbol id="i-breakpoint" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><circle cx="12" cy="12" r="3"/></symbol><symbol id="i-bolt" viewBox="0 0 24 24"><path d="m13 2-9 12h7l-1 8 10-13h-8z"/></symbol><symbol id="i-editor" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 9h18M9 9v11m4-7 2 2-2 2m4 0h2"/></symbol><symbol id="i-monitor" viewBox="0 0 24 24"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8m-4-4v4"/></symbol><symbol id="i-box" viewBox="0 0 24 24"><path d="m12 2 9 5v10l-9 5-9-5V7zM3 7l9 5 9-5m-9 5v10M7.5 4.5l9 5"/></symbol><symbol id="i-sliders" viewBox="0 0 24 24"><path d="M4 21v-7m0-4V3m8 18v-4m0-4V3m8 18V10m0-4V3M1 10h6m2 7h6m2-11h6"/></symbol><symbol id="i-plug" viewBox="0 0 24 24"><path d="M8 3v5m8-5v5M6 8h12v4a6 6 0 0 1-12 0zM12 18v4"/></symbol><symbol id="i-cloud" viewBox="0 0 24 24"><path d="M7 18H6a4 4 0 0 1-1-7.9A7 7 0 0 1 18.5 9 4.5 4.5 0 0 1-.5 9H7"/></symbol><symbol id="i-lock" viewBox="0 0 24 24"><rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V6a4 4 0 0 1 8 0v4m-4 5v2"/></symbol><symbol id="i-shield" viewBox="0 0 24 24"><path d="m12 2 9 4v6c0 6-9 10-9 10S3 18 3 12V6z"/><path d="m8 12 3 3 5-6"/></symbol><symbol id="i-menu" viewBox="0 0 24 24"><path d="M4 6h16M4 12h16M4 18h16"/></symbol><symbol id="i-loader" viewBox="0 0 24 24"><path d="M21 12a9 9 0 1 1-9-9"/></symbol><symbol id="i-pause" viewBox="0 0 24 24"><path d="M9 5v14M15 5v14"/></symbol><symbol id="i-cursor" viewBox="0 0 24 24"><path d="m4 2 6 18 3-7 7-3z"/></symbol><symbol id="i-braces" viewBox="0 0 24 24"><path d="M8 3H6v6l-3 3 3 3v6h2m8-18h2v6l3 3-3 3v6h-2"/></symbol><symbol id="i-git-pull" viewBox="0 0 24 24"><circle cx="6" cy="5" r="2"/><circle cx="6" cy="19" r="2"/><circle cx="18" cy="19" r="2"/><path d="M6 7v10m12 0V8a3 3 0 0 0-3-3h-3m2-3-3 3 3 3"/></symbol><symbol id="i-cube" viewBox="0 0 24 24"><path d="m12 3 8 4.5v9L12 21l-8-4.5v-9zM4 7.5l8 4.5 8-4.5M12 12v9"/></symbol><symbol id="i-performance" viewBox="0 0 24 24"><path d="M4 19a9 9 0 1 1 16 0M12 13l5-5M7 18h10"/><circle cx="12" cy="13" r="1.5"/></symbol><symbol id="i-external" viewBox="0 0 24 24"><path d="M14 3h7v7m0-7L10 14M10 3H3v18h18v-7"/></symbol></svg>
<a class="skip-link" href="#main">Skip to content</a>
<header class="site-header" :class="{ scrolled }" id="site-header" ref="header">
 <div class="container header-inner">
  <a class="brand" href="#top" aria-label="Glade home"><img class="brand-mark" :src="withBase('/logo-mark-topo.svg')" alt=""><span>glade<span class="brand-domain">.sh</span></span></a>
  <nav class="nav-links" aria-label="Main navigation">
   <a href="#features">Features</a><a href="#workflow">Workflow</a><a href="#extend">Extend</a><a :href="withBase('/guide/')">Docs</a>
  </nav>
  <div class="nav-actions">
   <a class="github-icon" href="https://github.com/glade-sh/glade" aria-label="Glade on GitHub"><svg class="icon " aria-hidden="true"><use href="#i-github"/></svg></a>
   <a class="button button-primary button-small" href="#get-started">Get started <svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a>
   <button class="menu-toggle" id="menu-toggle" ref="menuToggle" :disabled="!hydrated" :aria-label="menuOpen ? 'Close navigation' : 'Open navigation'" aria-controls="mobile-nav" :aria-expanded="menuOpen" @click="menuOpen = !menuOpen"><svg class="icon " aria-hidden="true"><use :href="menuOpen ? '#i-close' : '#i-menu'"/></svg></button>
  </div>
 </div>
 <nav class="mobile-nav" id="mobile-nav" aria-label="Mobile navigation" :hidden="!menuOpen" @click="closeForNavigation">
  <a href="#features">Features</a><a href="#workflow">Workflow</a><a href="#extend">Extend</a><a :href="withBase('/guide/')">Documentation</a><a href="https://github.com/glade-sh/glade">GitHub</a>
 </nav>
</header>
<main id="main" tabindex="-1">
 <section class="hero" aria-labelledby="hero-title">
  <div class="hero-topo" aria-hidden="true"><img :src="withBase('/home/contours.svg')" alt=""></div>
  <div class="container hero-grid">
   <div class="hero-copy">
    <p class="eyebrow"><span class="eyebrow-dot"></span>A little more local. A lot less waiting.</p>
    <h1 id="hero-title"><span class="headline-first" style="color:var(--gh-text)">Run Apex locally.</span> <span>Keep your</span> <span>momentum.</span></h1>
    <p class="hero-description">Check your project, run supported tests, and debug Apex on your machine—without waiting for a deploy.</p>
    <div class="hero-cta">
     <a class="button button-primary" href="#get-started">Get started <svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a>
     <a class="button button-secondary" href="https://github.com/glade-sh/glade"><svg class="icon " aria-hidden="true"><use href="#i-github"/></svg> View on GitHub</a>
    </div>
    <div class="hero-meta"><span><svg class="icon " aria-hidden="true"><use href="#i-monitor"/></svg> macOS &amp; Linux</span><a href="https://github.com/glade-sh/glade/blob/main/LICENSE"><svg class="icon " aria-hidden="true"><use href="#i-code"/></svg> Open source · Apache-2.0</a></div>
   </div>
   <div class="demo-wrap">
    <div class="demo-prelabel"><span>Your project. Your machine.</span><span class="interactive-tag"><svg class="icon " aria-hidden="true"><use href="#i-cursor"/></svg> Try the workflow</span></div>
    <div class="editor" id="hero-demo">
     <div class="editor-toolbar"><div class="window-dots" aria-hidden="true"><i></i><i></i><i></i></div><span class="editor-title">acme-salesforce / local workspace</span><svg class="icon " aria-hidden="true"><use href="#i-sliders"/></svg></div>
     <div class="demo-tabs" @keydown="tabKey($event)" role="tablist" aria-label="Local workflow preview">
      <button class="demo-tab" id="demo-tab-tests" role="tab" :aria-selected="scenario === 'tests'" aria-controls="demo-panel" :tabindex="scenario === 'tests' ? 0 : -1" data-demo="tests" :disabled="!hydrated" @click="selectScenario('tests')"><svg class="icon " aria-hidden="true"><use href="#i-check-circle"/></svg> Run tests</button>
      <button class="demo-tab" id="demo-tab-debug" role="tab" :aria-selected="scenario === 'debug'" aria-controls="demo-panel" :tabindex="scenario === 'debug' ? 0 : -1" data-demo="debug" :disabled="!hydrated" @click="selectScenario('debug')"><svg class="icon " aria-hidden="true"><use href="#i-bug"/></svg> Debug Apex</button>
      <button class="demo-tab" id="demo-tab-check" role="tab" :aria-selected="scenario === 'check'" aria-controls="demo-panel" :tabindex="scenario === 'check' ? 0 : -1" data-demo="check" :disabled="!hydrated" @click="selectScenario('check')"><svg class="icon " aria-hidden="true"><use href="#i-code"/></svg> Check source</button>
     </div>
     <div id="demo-panel" role="tabpanel" :aria-labelledby="`demo-tab-${scenario}`" :aria-busy="busy" tabindex="0">
      <div class="file-row"><svg class="icon " aria-hidden="true"><use href="#i-file"/></svg><span id="demo-filename">{{ scenario === 'tests' ? 'AccountServiceTest.cls' : 'AccountService.cls' }}</span><span class="lang-tag">APEX</span></div>
      <div class="code-pane" id="demo-code" aria-label="Illustrative Apex source" tabindex="0"><div v-for="(line, index) in code" :key="index" class="code-line" :class="lineClass(index)"><span class="line-no" aria-hidden="true">{{ index + 1 }}</span><span class="line-code"><span v-for="(token, i) in tokenize(line)" :key="i" :class="token.kind">{{ token.text }}</span></span></div></div>
      <div class="demo-console">
       <div class="console-header"><span class="console-heading"><svg class="icon " aria-hidden="true"><use href="#i-editor"/></svg><span id="console-title">{{ scenario === 'tests' ? 'Test results' : scenario === 'debug' ? 'Debug console' : 'Diagnostics' }}</span></span><button class="run-button" id="run-demo" :disabled="!hydrated || busy" :aria-label="`${runLabel} — simulated ${scenario} preview`" @click="run"><svg class="icon " aria-hidden="true"><use href="#i-play"/></svg><span>{{ busy ? 'Running…' : runLabel }}</span></button></div>
       <p class="console-command"><span class="prompt">$</span><span id="demo-command">{{ commands[scenario] }}</span></p>
       <div class="console-output" id="demo-output" aria-live="polite" aria-atomic="true"><div class="output-line" :class="output.kind"><svg class="icon" aria-hidden="true"><use :href="`#i-${output.kind === 'success' ? 'check-circle' : output.kind === 'failure' ? 'close' : output.kind === 'warning' ? 'pause' : 'code'}`"/></svg><span>{{ output.message }}</span><span v-if="output.pill" class="result-pill">{{ output.pill }}</span></div><div class="output-note">{{ output.note }}</div></div>
      </div>
     </div>
     <div class="demo-footer"><span><svg class="icon " aria-hidden="true"><use href="#i-branch"/></svg> main</span><span><svg class="icon " aria-hidden="true"><use href="#i-monitor"/></svg> Local runtime</span><span>Salesforce DX</span></div>
    </div>
    <div class="demo-caption"><span>Interactive preview · simulated output</span><button id="demo-change" class="demo-change" :disabled="!hydrated || busy" @click="edit">{{ editLabel }} ↗</button></div>
   </div>
  </div>
 </section>
 <section class="container capabilities" id="features" tabindex="-1" aria-label="What you can do with Glade">
  <div class="capability"><svg class="icon " aria-hidden="true"><use href="#i-bolt"/></svg><h2>Test without the wait</h2><p>Run a focused Apex test locally, then keep iterating.</p></div>
  <div class="capability"><svg class="icon " aria-hidden="true"><use href="#i-breakpoint"/></svg><h2>See what’s happening</h2><p>Set a breakpoint. Inspect variables. Find the why.</p></div>
  <div class="capability"><svg class="icon " aria-hidden="true"><use href="#i-editor"/></svg><h2>Keep your favorite tools</h2><p>Your Salesforce DX project, in your editor or terminal.</p></div>
  <div class="capability"><svg class="icon " aria-hidden="true"><use href="#i-git-pull"/></svg><h2>Automate the loop</h2><p>Local checks for your agents and CI workflows.</p></div>
 </section>
 <section class="section container" id="workflow" tabindex="-1" aria-labelledby="workflow-title">
  <div class="workflow-layout">
   <div class="workflow-copy">
    <p class="eyebrow">Less context switching</p>
    <h2 class="section-title" id="workflow-title">Stay in the code.<br>Not the deploy queue.</h2>
    <p class="section-description">A local feedback loop that fits the way you already work. No new project format. No org login for supported local checks.</p>
    <div class="workflow-list">
     <div class="workflow-item"><span class="workflow-number">01</span><div><strong>Start with your project.</strong><p>Open your Salesforce DX workspace.</p></div></div>
     <div class="workflow-item"><span class="workflow-number">02</span><div><strong>Make a change. See the result.</strong><p>Check, test, and debug supported Apex locally.</p></div></div>
     <div class="workflow-item"><span class="workflow-number">03</span><div><strong>Validate when you’re ready.</strong><p>Keep Salesforce as your final validation gate.</p></div></div>
    </div>
    <a class="text-link" :href="withBase('/guide/editor')">Explore the editor workflow<svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a>
   </div>
   <div class="workbench-wrap">
    <div class="tool-tabs" @keydown="tabKey($event)" role="tablist" aria-label="Explore Glade with your tools">
     <button class="tool-tab" role="tab" id="tool-vscode" :aria-selected="tool === 'vscode'" aria-controls="panel-vscode" :tabindex="tool === 'vscode' ? 0 : -1" data-tool="vscode" :disabled="!hydrated" @click="tool = 'vscode'"><svg class="icon " aria-hidden="true"><use href="#i-editor"/></svg> VS Code</button>
     <button class="tool-tab" role="tab" id="tool-terminal" :aria-selected="tool === 'terminal'" aria-controls="panel-terminal" :tabindex="tool === 'terminal' ? 0 : -1" data-tool="terminal" :disabled="!hydrated" @click="tool = 'terminal'"><svg class="icon " aria-hidden="true"><use href="#i-code"/></svg> Terminal</button>
     <button class="tool-tab" role="tab" id="tool-agent" :aria-selected="tool === 'agent'" aria-controls="panel-agent" :tabindex="tool === 'agent' ? 0 : -1" data-tool="agent" :disabled="!hydrated" @click="tool = 'agent'"><svg class="icon " aria-hidden="true"><use href="#i-spark"/></svg> AI workflow</button>
    </div>
    <div class="workbench">
     <div class="tool-panel" id="panel-vscode" role="tabpanel" aria-labelledby="tool-vscode" tabindex="0" :hidden="tool !== 'vscode'">
      <div class="workbench-head">acme-salesforce — Visual Studio Code</div>
      <div class="workbench-body">
       <div class="workbench-sidebar">
        <p class="sidebar-label">Explorer</p>
        <div class="sidebar-row"><svg class="icon " aria-hidden="true"><use href="#i-folder"/></svg> force-app</div>
        <div class="sidebar-row indented">⌄ main / default</div>
        <div class="sidebar-row indented">⌄ classes</div>
        <div class="sidebar-row selected active-file"><svg class="icon " aria-hidden="true"><use href="#i-file"/></svg> AccountService</div>
        <div class="sidebar-row"><svg class="icon " aria-hidden="true"><use href="#i-file"/></svg> AccountServiceTest</div>
        <div class="sidebar-row"><svg class="icon " aria-hidden="true"><use href="#i-sliders"/></svg> glade.yml</div>
       </div>
       <div class="workbench-content">
        <div class="workbench-file"><svg class="icon " aria-hidden="true"><use href="#i-file"/></svg> AccountService.cls</div>
        <div class="debug-code" role="region" aria-label="Illustrative debug source" tabindex="0">
         <div class="code-line"><span class="line-no" aria-hidden="true">1</span><span class="line-code"><span class="t-key">public class</span> <span class="t-type">AccountService</span> {</span></div>
         <div class="code-line"><span class="line-no" aria-hidden="true">2</span><span class="line-code">  <span class="t-key">public static</span> <span class="t-type">Account</span> <span class="t-fn">create</span>() {</span></div>
         <div class="code-line"><span class="line-no" aria-hidden="true">3</span><span class="line-code">    <span class="t-type">Account</span> acc = <span class="t-key">new</span> <span class="t-type">Account</span>(</span></div>
         <div class="code-line"><span class="line-no" aria-hidden="true">4</span><span class="line-code">      Name = <span class="t-string">'Acme'</span>);</span></div>
         <div class="code-line breakpoint-row"><span class="line-no" aria-hidden="true"><span class="breakpoint-dot"></span>5</span><span class="line-code">    <span class="t-key">insert</span> acc;</span></div>
         <div class="code-line"><span class="line-no" aria-hidden="true">6</span><span class="line-code">    <span class="t-key">return</span> acc;</span></div>
         <div class="code-line"><span class="line-no" aria-hidden="true">7</span><span class="line-code">  }</span></div>
         <div class="code-line"><span class="line-no" aria-hidden="true">8</span><span class="line-code">}</span></div>
        </div>
        <div class="inspector"><div class="inspector-head"><svg class="icon " aria-hidden="true"><use href="#i-pause"/></svg> Paused at breakpoint · line 5</div><div class="variable">acc.Name <span>"Acme"</span></div></div>
       </div>
      </div>
      <div class="workbench-foot"><span><svg class="icon " aria-hidden="true"><use href="#i-branch"/></svg> main</span><span><svg class="icon " aria-hidden="true"><use href="#i-breakpoint"/></svg> Glade · local debug</span></div>
     </div>
     <div class="tool-panel" id="panel-terminal" role="tabpanel" aria-labelledby="tool-terminal" tabindex="0" :hidden="tool !== 'terminal'">
      <div class="workbench-head">acme-salesforce — terminal</div>
      <div class="terminal-body">
       <p class="term-line"><span class="term-prompt">$</span>glade doctor --project .</p><p class="term-success">✓ Project found. Ready.</p>
       <p class="term-line"><span class="term-prompt">$</span>glade check --project .</p><p class="term-success">✓ No source diagnostics.</p>
       <p class="term-line"><span class="term-prompt">$</span>glade test --project .<br>&nbsp; --class AccountServiceTest</p><p class="term-success">✓ createsAccount · 1 test passed</p>
      </div>
      <div class="workbench-foot"><span><svg class="icon " aria-hidden="true"><use href="#i-branch"/></svg> main</span><span><svg class="icon " aria-hidden="true"><use href="#i-monitor"/></svg> Local project</span></div>
     </div>
     <div class="tool-panel" id="panel-agent" role="tabpanel" aria-labelledby="tool-agent" tabindex="0" :hidden="tool !== 'agent'">
      <div class="workbench-head">An agent workflow using the Glade CLI</div>
      <div class="agent-body">
       <div class="agent-instruction">“Add a regression test. Prove it fails, fix the source, and rerun with Glade.”</div>
       <div class="agent-step"><svg class="icon " aria-hidden="true"><use href="#i-check-circle"/></svg><div><strong>Write a focused failing test</strong><p>AccountServiceTest.createsAccount</p></div></div>
       <div class="agent-step"><svg class="icon " aria-hidden="true"><use href="#i-check-circle"/></svg><div><strong>Use the local result to make the fix</strong><p>glade test --project . --class ... --json</p></div></div>
       <div class="agent-step"><svg class="icon " aria-hidden="true"><use href="#i-check-circle"/></svg><div><strong>Rerun and report the evidence</strong><p>Named test · diagnostic · exit status</p></div></div>
      </div>
      <div class="workbench-foot"><span>Bring your own agent</span><span>Glade provides the local feedback</span></div>
     </div>
    </div>
    <p class="workbench-note">Illustrative workflow previews · initialized Salesforce DX project. <a class="text-link" :href="withBase('/guide/workflows')">Read the guides<svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a></p>
   </div>
  </div>
 </section>
 <section class="extensions" id="extend" tabindex="-1" aria-labelledby="extend-title">
  <div class="container">
   <div class="section-heading-row"><div><p class="eyebrow">A small tool. Room to grow.</p><h2 class="section-title" id="extend-title">Make the workflow yours.</h2></div><p>Start with the local runtime. Connect it to the tools and workflows that help you build.</p></div>
   <div class="extension-grid">
    <article class="extension-card">
     <div class="extension-art" aria-hidden="true"><div class="agent-flow"><div class="flow-node"><svg class="icon " aria-hidden="true"><use href="#i-spark"/></svg>Agent</div><div class="flow-arrow"></div><div class="flow-node"><svg class="icon " aria-hidden="true"><use href="#i-code"/></svg>Glade</div><div class="flow-arrow"></div><div class="flow-node"><svg class="icon " aria-hidden="true"><use href="#i-check"/></svg>Feedback</div></div></div>
     <div class="extension-text"><h3>Give your agent a feedback loop.</h3><p>Let your coding agent check its work with local tests and concrete diagnostics—not guesswork.</p><a class="text-link" :href="withBase('/guide/ai-assisted-apex')">Build an AI-assisted workflow<svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a></div>
    </article>
    <article class="extension-card">
     <div class="extension-art" aria-hidden="true"><div class="ci-mini"><div class="ci-row"><svg class="icon " aria-hidden="true"><use href="#i-check-circle"/></svg>local-checks<span class="ci-label">Passed</span></div><div class="report-row"><span class="report-tag"><svg class="icon " aria-hidden="true"><use href="#i-file"/></svg>JSON</span><span class="report-tag"><svg class="icon " aria-hidden="true"><use href="#i-file"/></svg>SARIF</span><span class="report-tag"><svg class="icon " aria-hidden="true"><use href="#i-file"/></svg>JUnit</span></div></div></div>
     <div class="extension-text"><h3>Take local checks into CI.</h3><p>Run checks and affected tests on pull requests. Keep machine-readable reports with your build.</p><a class="text-link" :href="withBase('/guide/workflows/ci')">Add Glade to your pipeline<svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a></div>
    </article>
    <article class="extension-card">
     <div class="extension-art" aria-hidden="true"><div class="plugin-mini"><div class="plugin-item"><svg class="icon " aria-hidden="true"><use href="#i-performance"/></svg>@glade/performance<span class="plus">+</span></div><div class="plugin-item"><svg class="icon " aria-hidden="true"><use href="#i-box"/></svg>@glade/orgpackage<span class="plus">+</span></div></div></div>
     <div class="extension-text"><h3>Add only what you need.</h3><p>Extend Glade with optional first-party plugins for advisory project scans and org-backed package artifact capture.</p><a class="text-link" :href="withBase('/guide/plugins')">Explore plugins<svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a></div>
    </article>
   </div>
  </div>
 </section>
 <section class="compatibility container" aria-labelledby="compat-title">
  <div class="compatibility-grid">
   <div><p class="eyebrow">Built on clear boundaries</p><h2 class="section-title" id="compat-title">Local first.<br>Not a Salesforce replacement.</h2><p class="section-description">Get useful feedback earlier. Glade runs supported paths locally; Salesforce stays the final check for your production environment.</p><a class="text-link" :href="withBase('/guide/support-map')">See what runs locally<svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a></div>
   <div class="compatibility-table" role="table" aria-label="Local workflow boundaries">
    <div class="compatibility-table-head" role="row"><span role="columnheader">Workflow</span><span role="columnheader">Where it runs</span></div>
    <div class="compat-row" role="row"><div role="cell"><strong>Apex checks &amp; supported tests</strong><small>Inside your Salesforce DX project</small></div><span class="status" role="cell"><svg class="icon " aria-hidden="true"><use href="#i-check-circle"/></svg>Local</span></div>
    <div class="compat-row" role="row"><div role="cell"><strong>Supported SOQL &amp; DML</strong><small>Against local data</small></div><span class="status" role="cell"><svg class="icon " aria-hidden="true"><use href="#i-check-circle"/></svg>Local</span></div>
    <div class="compat-row" role="row"><div role="cell"><strong>Hosted services &amp; final validation</strong><small>Against the real platform</small></div><span class="status salesforce" role="cell"><svg class="icon " aria-hidden="true"><use href="#i-cloud"/></svg>Salesforce</span></div>
   </div>
  </div>
 </section>
 <section class="get-started" id="get-started" tabindex="-1" aria-labelledby="start-title">
  <div class="footer-topo" aria-hidden="true"><img :src="withBase('/home/contours.svg')" alt=""></div><div class="footer-topo right" aria-hidden="true"><img :src="withBase('/home/contours.svg')" alt=""></div>
  <div class="container">
   <p class="eyebrow">Go from waiting to making.</p>
   <h2 id="start-title">Your next test.<br>One less deploy.</h2>
   <p>Install Glade, open your project, and start your local loop.</p>
   <div class="install-box" id="install-cmd"><code><span class="install-prompt" aria-hidden="true">$</span><span id="install-command" ref="installCommand">{{ INSTALL_COMMAND }}</span></code><button class="copy-button" id="copy-install" :disabled="!hydrated" :aria-label="copied ? 'Install command copied' : 'Copy Glade install command'" @click="copyInstall"><svg class="icon " aria-hidden="true"><use href="#i-copy"/></svg><span class="copy-label">{{ copied ? 'Copied' : 'Copy' }}</span></button></div>
   <div class="install-links"><a class="text-link" :href="withBase('/guide/quickstart')">Follow the quickstart<svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a><span class="platform-note">macOS &amp; Linux</span><a class="text-link" :href="withBase('/guide/installation')">Installation options<svg class="icon " aria-hidden="true"><use href="#i-arrow"/></svg></a></div>
  </div>
 </section>
</main>
<footer class="site-footer">
 <div class="container">
  <div class="footer-main"><div><a class="brand" href="#top" aria-label="Glade home"><img class="brand-mark" :src="withBase('/logo-mark-topo.svg')" alt=""><span>glade<span class="brand-domain">.sh</span></span></a><p class="footer-tagline">Local Apex. More momentum.</p></div><nav class="footer-links" aria-label="Footer navigation"><a :href="withBase('/guide/')">Docs</a><a :href="withBase('/guide/support-map')">Compatibility</a><a href="https://github.com/glade-sh/glade/releases">Releases</a><a :href="withBase('/guide/security-trust')">Security</a><a href="https://github.com/glade-sh/glade" aria-label="Glade on GitHub"><svg class="icon " aria-hidden="true"><use href="#i-github"/></svg></a><a :href="withBase('/help/')">Help</a><a :href="withBase('/maintainer/')">Contributors</a></nav></div>
  <div class="home-trust-links"><a class="home-release-version" :href="`https://github.com/glade-sh/glade/releases/tag/${releaseManifest.version}`">{{ releaseManifest.version }}</a><a href="https://github.com/glade-sh/glade/blob/main/site/install.sh">Installer source</a><a :href="withBase('/guide/security-trust#release-proof')">Checksums, SBOM, and attestations</a><a href="https://github.com/glade-sh/glade/issues">Issue feedback</a></div>
  <div class="footer-bottom"><p>Glade is an independent project, not affiliated with, sponsored by, or endorsed by Salesforce. Salesforce and Apex are trademarks of Salesforce, Inc. Copyright 2026 Matt Simonis.</p><a href="https://github.com/glade-sh/glade/blob/main/LICENSE">Open source · Apache-2.0</a></div>
 </div>
</footer>
<div class="toast" :class="{ show: toast }" id="toast" role="status" aria-live="polite">{{ toast }}</div>
<noscript>Enable JavaScript to use the simulated editor previews and copy controls. All documentation links work without it.</noscript>

</div>
</template>
<style scoped src="./home.css"></style>
