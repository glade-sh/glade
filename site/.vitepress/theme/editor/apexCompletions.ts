import { startCompletion, type Completion, type CompletionContext } from '@codemirror/autocomplete'
import type { EditorView } from '@codemirror/view'
import type { EditorCompletion, EditorSupportCatalog } from './editorSupportTypes'

export function createApexCompletions(catalog: EditorSupportCatalog) {
  const completionCatalog = Object.fromEntries(
    Object.entries(catalog.receivers).map(([receiver, support]) => [
      receiver,
      support.items.map((item) => toCodeMirrorCompletion(item))
    ])
  )
  const rootCompletions = catalog.rootCompletions.map((item) => toCodeMirrorCompletion(item))

  function indexedReceiverType(type: string, hasIndexAccess: boolean) {
    if (!hasIndexAccess) return type
    if (type === 'Database.SaveResult[]') return 'Database.SaveResult'
    if (type === 'List<Account>') return 'Account'
    return type
  }

  function inferReceiverType(doc: string, receiver: string) {
    const hasIndexAccess = /\[[^\]]+\]/.test(receiver)
    const normalized = receiver.replace(/\[[^\]]*\]/g, '')
    if (completionCatalog[receiver]) return receiver
    if (completionCatalog[normalized]) return normalized

    if (normalized.endsWith('.fields')) {
      const owner = normalized.slice(0, -'.fields'.length)
      if (inferReceiverType(doc, owner) === 'Schema.DescribeSObjectResult') {
        return 'Schema.DescribeSObjectResult.fields'
      }
    }

    const variableName = normalized.split('.').pop() || normalized
    const declarations = [
      { pattern: /Database\.SaveResult\[\]\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'Database.SaveResult[]' },
      { pattern: /Database\.Error\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'Database.Error' },
      { pattern: /Schema\.DescribeSObjectResult\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'Schema.DescribeSObjectResult' },
      { pattern: /Schema\.SObjectType\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'Schema.SObjectType' },
      { pattern: /Schema\.SObjectField\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'Schema.SObjectField' },
      { pattern: /Schema\.DescribeFieldResult\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'Schema.DescribeFieldResult' },
      { pattern: /Map\s*<\s*String\s*,\s*Schema\.SObjectField\s*>\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'Map<String, Schema.SObjectField>' },
      { pattern: /List\s*<\s*Account\s*>\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'List<Account>' },
      { pattern: /\bAccount\s+([A-Za-z_][A-Za-z0-9_]*)/g, type: 'Account' }
    ]

    for (const declaration of declarations) {
      declaration.pattern.lastIndex = 0
      let match: RegExpExecArray | null
      while ((match = declaration.pattern.exec(doc))) {
        if (match[1] === variableName) {
          return indexedReceiverType(declaration.type, hasIndexAccess)
        }
      }
    }

    if (/\.SObjectType$/.test(normalized)) return 'Schema.SObjectType'
    if (catalog.demoReceivers[variableName]) return indexedReceiverType(catalog.demoReceivers[variableName], hasIndexAccess)
    return completionCatalog[variableName] ? variableName : ''
  }

  function apexCompletions(context: CompletionContext) {
    const receiver = context.matchBefore(/([A-Za-z_][A-Za-z0-9_]*(?:\[[^\]]+\])?(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\.([A-Za-z0-9_]*)?$/)
    if (receiver) {
      const match = /^([A-Za-z_][A-Za-z0-9_]*(?:\[[^\]]+\])?(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\.([A-Za-z0-9_]*)?$/.exec(receiver.text)
      const receiverName = match?.[1] || ''
      const prefix = match?.[2] || ''
      const receiverType = inferReceiverType(context.state.doc.toString(), receiverName)
      const options = receiverType ? completionCatalog[receiverType] : undefined
      if (!options) return null
      return {
        from: context.pos - prefix.length,
        options,
        validFor: /^[A-Za-z0-9_]*$/
      }
    }

    const word = context.matchBefore(/[A-Za-z_][A-Za-z0-9_]*/)
    if (!word && !context.explicit) return null
    return {
      from: word ? word.from : context.pos,
      options: rootCompletions,
      validFor: /^[A-Za-z0-9_]*$/
    }
  }

  return apexCompletions
}

export function maybeOpenReceiverCompletion(view: EditorView, currentView: () => EditorView | null) {
  const cursor = view.state.selection.main.head
  const beforeCursor = view.state.sliceDoc(Math.max(0, cursor - 120), cursor)
  if (!/[A-Za-z_][A-Za-z0-9_]*(?:\[[^\]]+\])?(?:\.[A-Za-z_][A-Za-z0-9_]*)*\.$/.test(beforeCursor)) return

  window.requestAnimationFrame(() => {
    if (currentView() === view) startCompletion(view)
  })
}

function toCodeMirrorCompletion(item: EditorCompletion): Completion {
  return {
    label: item.label,
    apply: item.apply || item.label,
    type: item.type,
    detail: item.detail,
    info: () => completionInfo(item)
  }
}

function completionInfo(completion: EditorCompletion) {
  const root = document.createElement('div')
  root.className = 'glade-completion-info'

  const status = document.createElement('span')
  status.className = `glade-completion-status glade-completion-status-${completion.status}`
  status.textContent = completion.statusLabel || completion.status

  const detail = document.createElement('strong')
  detail.textContent = completion.signature || completion.detail || completion.label

  root.append(status, detail)

  if (completion.info) {
    const note = document.createElement('p')
    note.textContent = completion.info
    root.append(note)
  }

  return root
}
