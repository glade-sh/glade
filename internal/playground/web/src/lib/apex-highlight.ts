const APEX_KEYWORDS = new Set([
  "abstract",
  "after",
  "as",
  "before",
  "break",
  "catch",
  "class",
  "commit",
  "continue",
  "delete",
  "do",
  "else",
  "enum",
  "extends",
  "final",
  "finally",
  "for",
  "get",
  "global",
  "if",
  "implements",
  "in",
  "inherited",
  "insert",
  "instanceof",
  "interface",
  "merge",
  "new",
  "override",
  "private",
  "protected",
  "public",
  "return",
  "set",
  "sharing",
  "static",
  "super",
  "switch",
  "testmethod",
  "this",
  "throw",
  "transaction",
  "transient",
  "trigger",
  "try",
  "undelete",
  "update",
  "upsert",
  "virtual",
  "void",
  "webservice",
  "when",
  "while",
  "with",
  "without",
])

const APEX_CONSTANTS = new Set(["false", "null", "true"])

const SYSTEM_TYPES = new Set([
  "blob",
  "boolean",
  "date",
  "datetime",
  "decimal",
  "double",
  "exception",
  "id",
  "integer",
  "list",
  "long",
  "map",
  "object",
  "set",
  "sobject",
  "string",
  "time",
])

const DECLARATION_KEYWORDS = new Set(["class", "enum", "interface", "trigger"])

const SOQL_KEYWORDS = new Set([
  "ALL",
  "AND",
  "ASC",
  "BY",
  "CUSTOM",
  "DESC",
  "ELSE",
  "END",
  "FIELDS",
  "FIRST",
  "FOR",
  "FROM",
  "GROUP",
  "HAVING",
  "IN",
  "INCLUDES",
  "LAST",
  "LIKE",
  "LIMIT",
  "NOT",
  "NULL",
  "NULLS",
  "OFFSET",
  "OR",
  "ORDER",
  "SELECT",
  "STANDARD",
  "SYSTEM_MODE",
  "THEN",
  "TYPEOF",
  "UPDATE",
  "USER_MODE",
  "USING",
  "VIEW",
  "WHEN",
  "WHERE",
  "WITH",
])

type SignificantToken = {
  kind: string
  text: string
}

export function highlightApex(source: string): string {
  let index = 0
  let html = ""
  let expectTypeDeclaration = false
  let pendingAnnotationCall = false
  let parenDepth = 0
  const annotationParens = new Set<number>()
  let lastSignificant: SignificantToken | null = null

  const push = (className: string, text: string) => {
    html += `<span class="token ${className}">${escapeHtml(text)}</span>`
  }

  const pushSignificant = (kind: string, text: string) => {
    lastSignificant = { kind, text }
  }

  while (index < source.length) {
    const char = source[index]
    const next = source[index + 1]

    if (isWhitespace(char)) {
      html += char
      index++
      continue
    }

    if (char === "/" && next === "/") {
      const end = readUntilLineEnd(source, index)
      push("comment", source.slice(index, end))
      index = end
      continue
    }

    if (char === "/" && next === "*") {
      const end = source.indexOf("*/", index + 2)
      const stop = end === -1 ? source.length : end + 2
      push(source[index + 2] === "*" ? "doc-comment" : "comment", source.slice(index, stop))
      index = stop
      continue
    }

    if (char === "'") {
      const end = readApexString(source, index)
      push("string", source.slice(index, end))
      index = end
      pushSignificant("string", "")
      continue
    }

    if (char === "[" && startsSoql(source, index + 1)) {
      const end = findMatchingBracket(source, index)
      if (end > index) {
        push("bracket", "[")
        html += highlightSoql(source.slice(index + 1, end))
        push("bracket", "]")
        index = end + 1
        pushSignificant("soql", "")
        continue
      }
    }

    if (char === "@") {
      const end = readIdentifier(source, index + 1)
      if (end > index + 1) {
        const token = source.slice(index, end)
        push("annotation-name", token)
        index = end
        pendingAnnotationCall = true
        pushSignificant("annotation", token)
        continue
      }
    }

    if (isIdentifierStart(char)) {
      const end = readIdentifier(source, index)
      const word = source.slice(index, end)
      const lower = word.toLowerCase()
      const nextNonSpace = source[nextNonWhitespace(source, end)]
      const previous = lastSignificant
      let kind = "identifier"

      if (isInAnnotation(annotationParens, parenDepth) && nextNonSpace === "=") {
        kind = "annotation-attr"
      } else if (expectTypeDeclaration) {
        kind = "type-declaration"
        expectTypeDeclaration = false
      } else if (APEX_KEYWORDS.has(lower)) {
        kind = "keyword"
        expectTypeDeclaration = DECLARATION_KEYWORDS.has(lower)
      } else if (APEX_CONSTANTS.has(lower)) {
        kind = "constant"
      } else if (SYSTEM_TYPES.has(lower)) {
        kind = "system-type"
      } else if (nextNonSpace === "=" && isApexSObjectFieldName(word)) {
        kind = "sobject-field"
      } else if (nextNonSpace === "(" && word[0] === word[0]?.toUpperCase()) {
        kind = "class-name"
      } else if (nextNonSpace === "(" && !isDot(previous)) {
        kind = isMethodDeclaration(previous) ? "method-declaration" : "method-call"
      } else if (word[0] === word[0]?.toUpperCase()) {
        kind = "class-name"
      }

      push(kind, word)
      index = end
      pushSignificant(kind, word)
      continue
    }

    if (isDigit(char)) {
      const end = readNumber(source, index)
      push("number", source.slice(index, end))
      index = end
      pushSignificant("number", "")
      continue
    }

    if (char === "(") {
      parenDepth++
      if (pendingAnnotationCall) {
        annotationParens.add(parenDepth)
      }
      pendingAnnotationCall = false
      push("paren", char)
      index++
      pushSignificant("paren", char)
      continue
    }

    if (char === ")") {
      annotationParens.delete(parenDepth)
      parenDepth = Math.max(0, parenDepth - 1)
      pendingAnnotationCall = false
      push("paren", char)
      index++
      pushSignificant("paren", char)
      continue
    }

    pendingAnnotationCall = false

    if (char === ".") {
      push("dot", char)
      index++
      pushSignificant("dot", char)
      continue
    }

    if ("{}[]".includes(char)) {
      push("bracket", char)
      index++
      pushSignificant("bracket", char)
      continue
    }

    if (",;".includes(char)) {
      push("punctuation", char)
      index++
      pushSignificant("punctuation", char)
      continue
    }

    const operator = readOperator(source, index)
    if (operator) {
      push("operator", operator)
      index += operator.length
      pushSignificant("operator", operator)
      continue
    }

    html += escapeHtml(char)
    index++
  }

  return html
}

function highlightSoql(source: string): string {
  let index = 0
  let html = ""
  let expectObjectName = false
  let sawFrom = false

  const push = (className: string, text: string) => {
    html += `<span class="token ${className}">${escapeHtml(text)}</span>`
  }

  while (index < source.length) {
    const char = source[index]
    const next = source[index + 1]

    if (isWhitespace(char)) {
      html += char
      index++
      continue
    }

    if (char === "/" && next === "/") {
      const end = readUntilLineEnd(source, index)
      push("comment", source.slice(index, end))
      index = end
      continue
    }

    if (char === "/" && next === "*") {
      const end = source.indexOf("*/", index + 2)
      const stop = end === -1 ? source.length : end + 2
      push("comment", source.slice(index, stop))
      index = stop
      continue
    }

    if (char === "'") {
      const end = readApexString(source, index)
      push("string", source.slice(index, end))
      index = end
      continue
    }

    if (char === ":" && isIdentifierStart(next)) {
      const end = readIdentifier(source, index + 1)
      push("bind-variable", source.slice(index, end))
      index = end
      continue
    }

    if (isIdentifierStart(char)) {
      const end = readIdentifier(source, index)
      const word = source.slice(index, end)
      const upper = word.toUpperCase()
      if (SOQL_KEYWORDS.has(upper)) {
        push("soql-keyword", word)
        expectObjectName = upper === "FROM"
        sawFrom = sawFrom || upper === "FROM"
      } else if (expectObjectName) {
        push("sobject-name", word)
        expectObjectName = false
      } else if (!sawFrom || isLikelySoqlField(source, end)) {
        push("sobject-field", word)
      } else {
        push("sobject-field", word)
      }
      index = end
      continue
    }

    if (isDigit(char)) {
      const end = readNumber(source, index)
      push("number", source.slice(index, end))
      index = end
      continue
    }

    if (char === ".") {
      push("dot", char)
      index++
      continue
    }

    if (",()".includes(char)) {
      push("punctuation", char)
      index++
      continue
    }

    const operator = readOperator(source, index)
    if (operator) {
      push("operator", operator)
      index += operator.length
      continue
    }

    html += escapeHtml(char)
    index++
  }

  return `<span class="token soql">${html}</span>`
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
}

function startsSoql(source: string, start: number): boolean {
  const next = nextNonWhitespace(source, start)
  return /^SELECT\b|^FIND\b/i.test(source.slice(next, next + 8))
}

function findMatchingBracket(source: string, start: number): number {
  let depth = 0
  let index = start
  while (index < source.length) {
    const char = source[index]
    const next = source[index + 1]
    if (char === "'") {
      index = readApexString(source, index)
      continue
    }
    if (char === "/" && next === "/") {
      index = readUntilLineEnd(source, index)
      continue
    }
    if (char === "/" && next === "*") {
      const end = source.indexOf("*/", index + 2)
      index = end === -1 ? source.length : end + 2
      continue
    }
    if (char === "[") depth++
    if (char === "]") {
      depth--
      if (depth === 0) return index
    }
    index++
  }
  return -1
}

function readUntilLineEnd(source: string, start: number): number {
  const end = source.indexOf("\n", start)
  return end === -1 ? source.length : end
}

function readApexString(source: string, start: number): number {
  let index = start + 1
  while (index < source.length) {
    if (source[index] === "\\" && index + 1 < source.length) {
      index += 2
      continue
    }
    if (source[index] === "'") return index + 1
    index++
  }
  return source.length
}

function readIdentifier(source: string, start: number): number {
  let index = start
  while (index < source.length && isIdentifierPart(source[index])) {
    index++
  }
  return index
}

function readNumber(source: string, start: number): number {
  let index = start
  while (index < source.length && /[0-9_]/.test(source[index])) {
    index++
  }
  if (source[index] === "." && isDigit(source[index + 1])) {
    index++
    while (index < source.length && /[0-9_]/.test(source[index])) {
      index++
    }
  }
  return index
}

function readOperator(source: string, start: number): string {
  const three = source.slice(start, start + 3)
  if (["===", "!==", ">>>", "<<=", ">>="].includes(three)) return three
  const two = source.slice(start, start + 2)
  if (["&&", "||", "++", "--", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", "=>", "?.", "??"].includes(two)) {
    return two
  }
  const one = source[start]
  return "+-*/%=!<>?:&|^~".includes(one) ? one : ""
}

function nextNonWhitespace(source: string, start: number): number {
  let index = start
  while (index < source.length && isWhitespace(source[index])) {
    index++
  }
  return index
}

function isWhitespace(char: string | undefined): boolean {
  return char === " " || char === "\t" || char === "\n" || char === "\r"
}

function isIdentifierStart(char: string | undefined): boolean {
  return !!char && /[A-Za-z_]/.test(char)
}

function isIdentifierPart(char: string | undefined): boolean {
  return !!char && /[A-Za-z0-9_]/.test(char)
}

function isDigit(char: string | undefined): boolean {
  return !!char && /[0-9]/.test(char)
}

function isApexSObjectFieldName(word: string): boolean {
  return /^[A-Z][A-Za-z0-9_]*(?:__c|__r)?$/.test(word)
}

function isInAnnotation(annotationParens: Set<number>, parenDepth: number): boolean {
  for (const depth of annotationParens) {
    if (parenDepth >= depth) return true
  }
  return false
}

function isMethodDeclaration(previous: SignificantToken | null): boolean {
  if (!previous) return false
  if (["class-name", "system-type", "type-declaration"].includes(previous.kind)) return true
  if (previous.kind === "operator" && previous.text === ">") return true
  return previous.kind === "keyword" && previous.text.toLowerCase() === "void"
}

function isDot(previous: SignificantToken | null): boolean {
  return previous?.kind === "dot"
}

function isLikelySoqlField(source: string, end: number): boolean {
  const next = source[nextNonWhitespace(source, end)]
  return next === "." || next === "," || next === ")" || next === undefined
}
