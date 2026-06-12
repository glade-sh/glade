const APEX_KEYWORDS = new Set([
  "abstract", "after", "all", "array", "as", "asc", "before", "break",
  "bulk", "by", "catch", "category", "class", "commit", "continue",
  "cube", "custom", "data", "delete", "desc", "do", "else", "enum",
  "end", "excludes", "extends", "fields", "final", "finally", "first",
  "for", "from", "get", "global", "group", "having", "if", "implements",
  "in", "includes", "inherited", "insert", "instanceof", "interface",
  "last", "like", "limit", "merge", "new", "not", "nulls", "offset",
  "on", "order", "override", "package", "private", "protected", "public",
  "return", "rollback", "rollup", "rows", "savepoint", "security_enforced",
  "select", "set", "sharing", "sort", "standard", "static", "super",
  "switch", "then", "tracking", "testmethod", "this", "throw",
  "transaction", "transient", "trigger", "try", "typeof", "undelete",
  "update", "upsert", "using", "virtual", "void", "webservice", "where",
  "when", "while", "with", "without"
]);

const APEX_CONSTANTS = new Set(["false", "null", "true"]);
const SYSTEM_TYPES = new Set([
  "blob", "boolean", "date", "datetime", "decimal", "double",
  "exception", "id", "integer", "list", "long", "map", "string", "time"
]);
const DECLARATION_KEYWORDS = new Set(["class", "enum", "interface", "trigger"]);
const APEX_FUNCTIONS = new Set([
  "avg", "count", "count_distinct", "format", "grouping", "max", "min", "sum", "tolabel"
]);

function escapeHtml(value) {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function isWhitespace(char) {
  return char === " " || char === "\t" || char === "\n" || char === "\r";
}

function isIdentifierStart(char) {
  return !!char && /[A-Za-z_]/.test(char);
}

function isIdentifierPart(char) {
  return !!char && /[A-Za-z0-9_]/.test(char);
}

function isDigit(char) {
  return !!char && /[0-9]/.test(char);
}

function nextNonWhitespace(source, start) {
  let index = start;
  while (index < source.length && isWhitespace(source[index])) {
    index++;
  }
  return index;
}

function readUntilLineEnd(source, start) {
  const end = source.indexOf("\n", start);
  return end === -1 ? source.length : end;
}

function readApexString(source, start) {
  let index = start + 1;
  while (index < source.length) {
    if (source[index] === "'" && source[index + 1] === "'") {
      index += 2;
      continue;
    }
    if (source[index] === "\\" && index + 1 < source.length) {
      index += 2;
      continue;
    }
    if (source[index] === "'") {
      return index + 1;
    }
    index++;
  }
  return source.length;
}

function readIdentifier(source, start) {
  let index = start;
  while (index < source.length && isIdentifierPart(source[index])) {
    index++;
  }
  return index;
}

function readNumber(source, start) {
  let index = start;
  while (index < source.length && /[0-9_]/.test(source[index])) {
    index++;
  }
  if (source[index] === "." && isDigit(source[index + 1])) {
    index++;
    while (index < source.length && /[0-9_]/.test(source[index])) {
      index++;
    }
  }
  return index;
}

function readOperator(source, start) {
  const three = source.slice(start, start + 3);
  if (["===", "!==", ">>>", "<<=", ">>="].includes(three)) return three;
  const two = source.slice(start, start + 2);
  if (["&&", "||", "++", "--", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", "=>", "?.", "??"].includes(two)) return two;
  const one = source[start];
  return "+-*/%=!<>?:&|^~".includes(one) ? one : "";
}

function highlightApex(source) {
  let index = 0;
  let html = "";
  let expectTypeDeclaration = false;
  let lastSignificant = null;

  const push = (className, text) => {
    html += `<span class="token ${className}">${escapeHtml(text)}</span>`;
  };
  const pushSignificant = (kind, text) => {
    lastSignificant = { kind, text };
  };

  while (index < source.length) {
    const char = source[index];
    const next = source[index + 1];

    if (isWhitespace(char)) {
      html += char;
      index++;
      continue;
    }
    if (char === "/" && next === "/") {
      const end = readUntilLineEnd(source, index);
      push("comment", source.slice(index, end));
      index = end;
      continue;
    }
    if (char === "/" && next === "*") {
      const end = source.indexOf("*/", index + 2);
      const stop = end === -1 ? source.length : end + 2;
      push("comment", source.slice(index, stop));
      index = stop;
      continue;
    }
    if (char === "'") {
      const end = readApexString(source, index);
      push("string", source.slice(index, end));
      index = end;
      pushSignificant("string", "");
      continue;
    }
    if (isIdentifierStart(char)) {
      const end = readIdentifier(source, index);
      const word = source.slice(index, end);
      const lower = word.toLowerCase();
      const nextNonSpace = source[nextNonWhitespace(source, end)];
      const previous = lastSignificant;
      let kind = "identifier";

      if (expectTypeDeclaration) {
        kind = "type-declaration";
        expectTypeDeclaration = false;
      } else if (APEX_FUNCTIONS.has(lower) && nextNonSpace === "(") {
        kind = "method-call";
      } else if (APEX_KEYWORDS.has(lower)) {
        kind = lower === "select" || lower === "from" || lower === "where" ? "soql-keyword" : "keyword";
        expectTypeDeclaration = DECLARATION_KEYWORDS.has(lower);
      } else if (APEX_CONSTANTS.has(lower)) {
        kind = "constant";
      } else if (SYSTEM_TYPES.has(lower)) {
        kind = "system-type";
      } else if (/^[A-Za-z_][A-Za-z0-9_]*(?:__(?:c|r|x|mdt|e|b|kav))$/.test(word)) {
        kind = "sobject-field";
      } else if (previous?.text?.toLowerCase() === "from" && /^[A-Z][A-Za-z0-9_]*(?:__(?:c|mdt|e|b|kav))?$/.test(word)) {
        kind = "sobject-name";
      } else if (nextNonSpace === "(" && word[0] === word[0]?.toUpperCase()) {
        kind = "class-name";
      } else if (nextNonSpace === "(" && previous?.kind !== "dot") {
        kind = "method-call";
      } else if (word[0] === word[0]?.toUpperCase()) {
        kind = "class-name";
      }

      push(kind, word);
      index = end;
      pushSignificant(kind, word);
      continue;
    }
    if (isDigit(char)) {
      const end = readNumber(source, index);
      push("number", source.slice(index, end));
      index = end;
      pushSignificant("number", "");
      continue;
    }
    if (char === ".") {
      push("dot", char);
      index++;
      pushSignificant("dot", char);
      continue;
    }
    if ("{}[]()".includes(char)) {
      push("bracket", char);
      index++;
      pushSignificant("bracket", char);
      continue;
    }
    if (",;".includes(char)) {
      push("punctuation", char);
      index++;
      pushSignificant("punctuation", char);
      continue;
    }
    const operator = readOperator(source, index);
    if (operator) {
      push("operator", operator);
      index += operator.length;
      pushSignificant("operator", operator);
      continue;
    }
    html += escapeHtml(char);
    index++;
  }
  return html;
}

function highlightCodeBlock(codeElement) {
  const raw = codeElement.textContent;
  const highlighted = highlightApex(raw);
  const lines = highlighted.split("\n");
  const digits = String(lines.length).length;
  codeElement.innerHTML = lines.map((line, i) => {
    const num = String(i + 1).padStart(digits, "0");
    return `<span class="line"><span class="line-number">${num}</span> ${line}</span>`;
  }).join("");
}

window.gladeHighlightCodeBlock = highlightCodeBlock;

document.querySelectorAll("code.language-apex").forEach(highlightCodeBlock);
