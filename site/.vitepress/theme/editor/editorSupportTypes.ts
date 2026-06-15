export type EditorSupportStatus = 'supported' | 'partial' | 'stub' | 'unsupported' | 'unknown'

export type EditorCompletion = {
  readonly label: string
  readonly apply?: string
  readonly type?: string
  readonly detail?: string
  readonly status: EditorSupportStatus
  readonly statusLabel?: string
  readonly info?: string
  readonly source?: string
  readonly signature?: string
  readonly signatures?: readonly string[]
}

export type EditorReceiver = {
  readonly label: string
  readonly detail?: string
  readonly items: readonly EditorCompletion[]
}

export type EditorSupportCatalog = {
  readonly schemaVersion: number
  readonly generatedFrom: string
  readonly statusLabels: Readonly<Record<EditorSupportStatus, string>>
  readonly receivers: Readonly<Record<string, EditorReceiver>>
  readonly rootCompletions: readonly EditorCompletion[]
  readonly demoReceivers: Readonly<Record<string, string>>
}
