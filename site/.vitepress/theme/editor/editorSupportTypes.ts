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

export type SupportLedgerRow = {
  readonly id: string
  readonly area: string
  readonly api: string
  readonly status: EditorSupportStatus
  readonly notes: string
}

export type EditorSupportCatalog = {
  readonly schemaVersion: number
  readonly generatedFrom: string
  readonly summary: Readonly<Record<EditorSupportStatus, number>>
  readonly statusLabels: Readonly<Record<EditorSupportStatus, string>>
  readonly rows: readonly SupportLedgerRow[]
  readonly receivers: Readonly<Record<string, EditorReceiver>>
  readonly rootCompletions: readonly EditorCompletion[]
  readonly demoReceivers: Readonly<Record<string, string>>
}
