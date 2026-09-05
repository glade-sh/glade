<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { withBase } from 'vitepress'
import { EditorState } from '@codemirror/state'
import { EditorView, basicSetup } from 'codemirror'
import { autocompletion } from '@codemirror/autocomplete'
import { syntaxHighlighting } from '@codemirror/language'
import { editorSupportCatalog } from './generated/editorSupport'
import { apexLanguage, gladeHighlight } from './editor/apexLanguage'
import { createApexCompletions, maybeOpenReceiverCompletion } from './editor/apexCompletions'

const editorHost = ref<HTMLElement | null>(null)
let editorView: EditorView | null = null
const apexCompletions = createApexCompletions(editorSupportCatalog)

const startDoc = `public with sharing class RenewalDesk {
  @AuraEnabled(cacheable=true)
  public static String rebuild(Id businessHoursId) {
    Account account = new Account(Name = 'Acme', BillingCity = 'Twin Lakes');
    List<Account> accounts = new List<Account>{ account };
    Savepoint marker = Database.setSavepoint();

    Database.SaveResult[] results = Database.insert(accounts, false);
    if (!results[0].isSuccess()) {
      Database.rollback(marker);
      return JSON.serialize(results[0].getErrors());
    }

    Schema.DescribeSObjectResult describe = Account.SObjectType.getDescribe();
    Map<String, Schema.SObjectField> fieldMap = describe.fields.getMap();
    Datetime nextWindow = BusinessHours.nextStartDate(businessHoursId, Datetime.now());

    describe
  }
}`

onMounted(() => {
  if (!editorHost.value) return
  editorView = new EditorView({
    parent: editorHost.value,
    state: EditorState.create({
      doc: startDoc,
      extensions: [
        basicSetup,
        apexLanguage,
        syntaxHighlighting(gladeHighlight),
        EditorView.contentAttributes.of({
          'aria-labelledby': 'apex-editor-heading',
          'aria-describedby': 'apex-editor-instructions'
        }),
        autocompletion({ override: [apexCompletions], activateOnTyping: false }),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) maybeOpenReceiverCompletion(update.view, () => editorView)
        }),
        EditorView.theme({
          '&': {
            backgroundColor: 'var(--glade-code)',
            color: 'var(--glade-text)'
          },
          '.cm-content': {
            caretColor: 'var(--glade-text)',
            fontFamily: 'var(--vp-font-family-mono)',
            fontSize: '13px',
            lineHeight: '1.68',
            padding: '18px 0'
          },
          '.cm-cursor, .cm-dropCursor': {
            borderLeftColor: 'var(--glade-focus)'
          },
          '.cm-gutters': {
            backgroundColor: 'var(--glade-rail)',
            borderRight: '1px solid var(--glade-border)',
            color: 'var(--glade-muted)'
          },
          '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
            backgroundColor: 'var(--glade-active)'
          },
          '.cm-activeLine, .cm-activeLineGutter': {
            backgroundColor: 'var(--glade-active)'
          },
          '.cm-tooltip': {
            border: '1px solid var(--glade-control)',
            backgroundColor: 'var(--glade-elevated)',
            color: 'var(--glade-text)'
          },
          '.cm-tooltip-autocomplete ul li[aria-selected]': {
            backgroundColor: 'var(--glade-active)',
            color: 'var(--glade-text)'
          }
        })
      ]
    })
  })
  editorView.scrollDOM.tabIndex = 0
  editorView.scrollDOM.setAttribute('aria-label', 'Apex editor scroll area')
})

onBeforeUnmount(() => {
  editorView?.destroy()
  editorView = null
})
</script>

<template>
  <section class="glade-cm-workbench" data-codemirror-workbench aria-label="Interactive Apex editor">
    <div class="glade-cm-head">
      <div>
        <p class="home-eyebrow">Browser capability lookup</p>
        <h2 id="apex-editor-heading" class="home-h2">Try capability-backed autocomplete.</h2>
      </div>
      <a :href="withBase('/guide/support-map')">What runs locally</a>
    </div>
    <div id="apex-editor-instructions" class="glade-cm-support">
      <p>Edits stay in this browser. This editor looks up capability labels; it does not execute Apex or send your source.</p>
      <span>Type a dot after the final describe, Account, Database, BusinessHours, Schema, describe.fields, results[0], or fieldMap.</span>
      <div>
        <code>Account.</code>
        <code>Database.</code>
        <code>BusinessHours.</code>
        <code>Schema.</code>
        <code>describe.</code>
        <code>describe.fields.</code>
        <code>results[0].</code>
        <code>fieldMap.</code>
      </div>
    </div>
    <div ref="editorHost" class="glade-cm-editor"></div>
    <div class="glade-cm-proof" aria-label="CodeMirror editor capabilities">
      <span><strong>Apex syntax</strong> annotations, SOQL, SObjects, and platform classes</span>
      <span><strong>Autocomplete</strong> describe, Database, Schema fields, DML results, maps, and local context</span>
      <span><strong>Boundary labels</strong> Runs locally, Runs locally with limits, Requires Salesforce</span>
    </div>
  </section>
</template>
