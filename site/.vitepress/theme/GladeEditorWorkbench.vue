<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
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
        autocompletion({ override: [apexCompletions], activateOnTyping: false }),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) maybeOpenReceiverCompletion(update.view, () => editorView)
        }),
        EditorView.theme({
          '&': {
            backgroundColor: '#05090b',
            color: '#f3f7f5'
          },
          '.cm-content': {
            caretColor: '#f3f7f5',
            fontFamily: 'var(--vp-font-family-mono)',
            fontSize: '13px',
            lineHeight: '1.68',
            padding: '18px 0'
          },
          '.cm-cursor, .cm-dropCursor': {
            borderLeftColor: '#9be870'
          },
          '.cm-gutters': {
            backgroundColor: '#081014',
            borderRight: '1px solid #26363d',
            color: '#53676f'
          },
          '.cm-activeLine, .cm-activeLineGutter': {
            backgroundColor: 'rgba(155, 232, 112, 0.08)'
          },
          '.cm-tooltip': {
            border: '1px solid rgba(155, 232, 112, 0.42)',
            backgroundColor: '#10191e',
            color: '#f3f7f5'
          },
          '.cm-tooltip-autocomplete ul li[aria-selected]': {
            backgroundColor: 'rgba(155, 232, 112, 0.18)',
            color: '#f3f7f5'
          }
        })
      ]
    })
  })
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
        <p class="home-eyebrow">Interactive Editor</p>
        <h2 class="home-h2">Try support-backed autocomplete.</h2>
      </div>
      <a href="/guide/support-map">Check support</a>
    </div>
    <div class="glade-cm-support" aria-label="Autocomplete surfaces to try">
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
    <div ref="editorHost" class="glade-cm-editor" aria-label="CodeMirror Apex editor"></div>
    <div class="glade-cm-proof" aria-label="CodeMirror editor capabilities">
      <span><strong>Apex syntax</strong> annotations, SOQL, SObjects, and platform classes</span>
      <span><strong>Autocomplete</strong> describe, Database, Schema fields, DML results, maps, and local context</span>
      <span><strong>Boundary labels</strong> Works well, With limits, Needs Salesforce</span>
    </div>
  </section>
</template>
