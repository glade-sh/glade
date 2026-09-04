import { HighlightStyle, StreamLanguage, type StringStream } from '@codemirror/language'
import { tags } from '@lezer/highlight'

type ApexModeState = {
  expectTypeDeclaration: boolean
  lastKind: string
  lastText: string
}

const APEX_KEYWORDS = new Set([
  'abstract', 'after', 'all', 'array', 'as', 'asc', 'before', 'break',
  'bulk', 'by', 'catch', 'category', 'class', 'commit', 'continue',
  'cube', 'custom', 'data', 'delete', 'desc', 'do', 'else', 'enum',
  'end', 'excludes', 'extends', 'fields', 'final', 'finally', 'first',
  'for', 'from', 'get', 'global', 'group', 'having', 'if', 'implements',
  'in', 'includes', 'inherited', 'insert', 'instanceof', 'interface',
  'last', 'like', 'limit', 'merge', 'new', 'not', 'nulls', 'offset',
  'on', 'order', 'override', 'package', 'private', 'protected', 'public',
  'return', 'rollback', 'rollup', 'rows', 'savepoint', 'security_enforced',
  'select', 'set', 'sharing', 'sort', 'standard', 'static', 'super',
  'switch', 'then', 'tracking', 'testmethod', 'this', 'throw',
  'transaction', 'transient', 'trigger', 'try', 'typeof', 'undelete',
  'update', 'upsert', 'using', 'virtual', 'void', 'webservice', 'where',
  'when', 'while', 'with', 'without'
])

const APEX_CONSTANTS = new Set(['false', 'null', 'true'])
const APEX_ANNOTATIONS = new Set([
  'auraenabled', 'critical', 'deprecated', 'future', 'httpdelete',
  'httpget', 'httppatch', 'httppost', 'httpput', 'invocablemethod',
  'invocablevariable', 'istest', 'jsonaccess', 'jsonproperty',
  'namespaceaccessible', 'remoteaction', 'restresource', 'testsetup',
  'testvisible'
])
const APEX_ANNOTATION_ATTRIBUTES = new Set(['cacheable', 'callout', 'description', 'label', 'required', 'seealldata', 'urlmapping'])
const SYSTEM_TYPES = new Set(['blob', 'boolean', 'date', 'datetime', 'decimal', 'double', 'exception', 'id', 'integer', 'list', 'long', 'map', 'set', 'string', 'time', 'void'])
const PLATFORM_TYPES = new Set(['Account', 'Database', 'DescribeSObjectResult', 'JSON', 'Limits', 'SaveResult', 'Schema', 'SObject', 'SObjectField', 'SObjectType', 'System'])
const SOQL_FUNCTIONS = new Set(['avg', 'calendar_month', 'count', 'count_distinct', 'format', 'grouping', 'max', 'min', 'sum', 'tolabel'])
const DECLARATION_KEYWORDS = new Set(['class', 'enum', 'interface', 'trigger'])

export const gladeHighlight = HighlightStyle.define([
  { tag: tags.keyword, color: '#cba6f7', fontWeight: '700' },
  { tag: tags.atom, color: '#fab387' },
  { tag: [tags.string, tags.character], color: '#a6e3a1' },
  { tag: [tags.number, tags.bool], color: '#fab387' },
  { tag: [tags.className, tags.typeName, tags.definition(tags.typeName)], color: '#f5c95f', fontWeight: '700' },
  { tag: tags.standard(tags.variableName), color: '#89b4fa' },
  { tag: [tags.propertyName, tags.attributeName], color: '#cdd6f4' },
  { tag: tags.meta, color: '#fab387' },
  { tag: tags.comment, color: '#7f849c', fontStyle: 'normal' },
  { tag: tags.operator, color: '#89dceb' },
  { tag: tags.punctuation, color: '#9399b2' }
])

export const apexLanguage = StreamLanguage.define<ApexModeState>({
  name: 'apex',
  startState() {
    return { expectTypeDeclaration: false, lastKind: '', lastText: '' }
  },
  token(stream, state) {
    if (stream.eatSpace()) return null
    if (stream.match('//')) {
      stream.skipToEnd()
      return remember(state, 'comment', '', 'comment')
    }
    if (stream.match('/*')) {
      while (!stream.eol()) {
        if (stream.match('*/')) break
        stream.next()
      }
      return remember(state, 'comment', '', 'comment')
    }

    const char = stream.peek()
    if (char === "'") return readApexString(stream, state)
    if (char === '@') return readAnnotation(stream, state)
    if (/[A-Za-z_]/.test(char || '')) return readApexIdentifier(stream, state)
    if (/[0-9]/.test(char || '')) {
      stream.eatWhile(/[0-9_]/)
      if (stream.peek() === '.') {
        stream.next()
        stream.eatWhile(/[0-9_]/)
      }
      return remember(state, 'number', '', 'number')
    }
    if ('{}[]();,'.includes(char || '')) {
      stream.next()
      return remember(state, 'punctuation', char || '', 'punctuation')
    }
    stream.next()
    return remember(state, 'operator', char || '', 'operator')
  }
})

function remember(state: ApexModeState, kind: string, text: string, style: string) {
  state.lastKind = kind
  state.lastText = text
  return style
}

function readApexString(stream: StringStream, state: ApexModeState) {
  stream.next()
  while (!stream.eol()) {
    const char = stream.next()
    if (char === "'" && stream.peek() === "'") {
      stream.next()
      continue
    }
    if (char === "'") break
  }
  return remember(state, 'string', '', 'string')
}

function readAnnotation(stream: StringStream, state: ApexModeState) {
  const start = stream.pos
  stream.next()
  stream.eatWhile(/[A-Za-z0-9_]/)
  return remember(state, 'annotation-name', stream.string.slice(start, stream.pos), 'meta')
}

function readApexIdentifier(stream: StringStream, state: ApexModeState) {
  const start = stream.pos
  stream.eat(/[A-Za-z_]/)
  stream.eatWhile(/[A-Za-z0-9_]/)
  const word = stream.string.slice(start, stream.pos)
  const lower = word.toLowerCase()
  const next = nextNonWhitespace(stream.string, stream.pos)
  const nextChar = stream.string[next]
  let kind = 'identifier'
  let style = 'variableName'

  if (state.expectTypeDeclaration) {
    kind = 'type-declaration'
    style = 'typeName'
    state.expectTypeDeclaration = false
  } else if (APEX_ANNOTATIONS.has(lower)) {
    kind = 'annotation-name'
    style = 'meta'
  } else if (APEX_ANNOTATION_ATTRIBUTES.has(lower) && isAnnotationAttribute(state, nextChar)) {
    kind = 'annotation-attr'
    style = 'attributeName'
  } else if (state.lastText === '.' && nextChar === '(') {
    kind = 'method-call'
    style = 'builtin'
  } else if (SOQL_FUNCTIONS.has(lower) && nextChar === '(') {
    kind = 'method-call'
    style = 'builtin'
  } else if (APEX_KEYWORDS.has(lower)) {
    kind = 'keyword'
    style = 'keyword'
    state.expectTypeDeclaration = DECLARATION_KEYWORDS.has(lower)
  } else if (APEX_CONSTANTS.has(lower)) {
    kind = 'constant'
    style = 'atom'
  } else if (PLATFORM_TYPES.has(word) || SYSTEM_TYPES.has(lower) || /^[A-Z][A-Za-z0-9_]*(?:__(?:c|mdt|e|b|kav))?$/.test(word)) {
    kind = PLATFORM_TYPES.has(word) ? 'platform-type' : 'class-name'
    style = 'typeName'
  } else if (/^[A-Za-z_][A-Za-z0-9_]*(?:__(?:c|r|x|mdt|e|b|kav))$/.test(word)) {
    kind = 'sobject-field'
    style = 'propertyName'
  } else if (nextChar === '(' && state.lastKind !== 'operator') {
    kind = isMethodDeclaration(state) ? 'method-declaration' : 'method-call'
    style = 'builtin'
  }

  return remember(state, kind, word, style)
}

function nextNonWhitespace(source: string, start: number) {
  let index = start
  while (index < source.length && /\s/.test(source[index])) index += 1
  return index
}

function isAnnotationAttribute(state: ApexModeState, nextChar: string | undefined) {
  return nextChar === '=' || state.lastKind === 'annotation-name' || state.lastText === '(' || state.lastText === ','
}

function isMethodDeclaration(state: ApexModeState) {
  return ['class-name', 'platform-type', 'type-declaration'].includes(state.lastKind) || state.lastText.toLowerCase() === 'void'
}
