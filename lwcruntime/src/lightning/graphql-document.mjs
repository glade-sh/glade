// A document parser for the local UI GraphQL query surface. Syntax errors are
// construction errors; schema validation belongs to query execution.
export class GraphQLError extends Error {
  constructor(message) { super(message); this.name = "GraphQLError"; }
}

function tokens(source) {
  const result = [];
  let pos = 0;
  while (pos < source.length) {
    const ch = source[pos];
    if (/[\s,\uFEFF]/.test(ch)) { pos++; continue; }
    if (ch === "#") { while (pos < source.length && source[pos] !== "\n") pos++; continue; }
    const start = pos;
    if (source.startsWith("...", pos)) { result.push({ text: "...", start }); pos += 3; continue; }
    if (/[!$():=@\[\]{|}]/.test(ch)) { result.push({ text: ch, start }); pos++; continue; }
    if (ch === '"') {
      pos++;
      while (pos < source.length && source[pos] !== '"') {
        if (source[pos] === "\\") pos++;
        pos++;
      }
      if (pos >= source.length) throw new GraphQLError("Syntax Error: Unterminated string.");
      pos++;
      const text = source.slice(start, pos);
      try { result.push({ text, start, value: JSON.parse(text), type: "String" }); }
      catch { throw new GraphQLError("Syntax Error: Invalid string escape."); }
      continue;
    }
    if (ch === "-" || /\d/.test(ch)) {
      if (ch === "-" && !/\d/.test(source[pos + 1] || "")) {
        throw new GraphQLError(`Syntax Error: Invalid number, expected digit but got: ${JSON.stringify(source[pos + 1] || "<EOF>")}.`);
      }
      const match = /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/.exec(source.slice(pos));
      pos += match[0].length;
      if (/[._0-9A-Za-z]/.test(source[pos] || "")) {
        throw new GraphQLError(`Syntax Error: Invalid number, expected digit but got: ${JSON.stringify(source[pos])}.`);
      }
      result.push({ text: match[0], start, value: Number(match[0]), type: /[.eE]/.test(match[0]) ? "Float" : "Int" });
      continue;
    }
    const name = /^[_A-Za-z][_0-9A-Za-z]*/.exec(source.slice(pos));
    if (name) { result.push({ text: name[0], start, type: "Name" }); pos += name[0].length; continue; }
    throw new GraphQLError(`Syntax Error: Cannot parse the unexpected character ${JSON.stringify(ch)}.`);
  }
  result.push({ text: "<EOF>", start: source.length });
  return result;
}

export function parseDocument(source) {
  const stream = tokens(source);
  let index = 0;
  const peek = () => stream[index];
  const take = () => stream[index++];
  const accept = text => peek().text === text ? take() : null;
  const describe = token => token.text === "<EOF>" ? "<EOF>" : token.type === "Name" ? `Name ${JSON.stringify(token.text)}` : token.type === "Int" ? `Int ${JSON.stringify(token.text)}` : JSON.stringify(token.text);
  const expect = text => {
    if (peek().text !== text) throw new GraphQLError(`Syntax Error: Expected ${JSON.stringify(text)}, found ${describe(peek())}.`);
    return take();
  };
  const name = () => {
    if (peek().type !== "Name") throw new GraphQLError(`Syntax Error: Expected Name, found ${describe(peek())}.`);
    return take();
  };
  function value() {
    const token = take();
    if (token.text === "$") return { kind: "Variable", name: name().text };
    if (token.text === "[") {
      const values = [];
      while (!accept("]")) { if (peek().text === "<EOF>") expect("]"); values.push(value()); }
      return values;
    }
    if (token.text === "{") {
      const values = Object.create(null);
      while (!accept("}")) { const key = name().text; expect(":"); values[key] = value(); }
      return values;
    }
    if (token.type === "String" || token.type === "Int" || token.type === "Float") return token.value;
    if (token.text === "null") return null;
    if (token.text === "true" || token.text === "false") return token.text === "true";
    if (token.type === "Name") return { kind: "Enum", name: token.text };
    throw new GraphQLError(`Syntax Error: Unexpected ${describe(token)}.`);
  }
  function args() {
    const values = [];
    if (accept("(")) {
      do { const key = name(); expect(":"); values.push({ name: key.text, value: value(), start: key.start }); }
      while (!accept(")"));
    }
    return values;
  }
  function directives() {
    const values = [];
    while (accept("@")) { const key = name(); values.push({ name: key.text, args: args(), start: key.start - 1 }); }
    return values;
  }
  function selection() {
    expect("{");
    const values = [];
    do {
      if (accept("...")) {
        const start = stream[index - 1].start;
        if (accept("on")) values.push({ kind: "InlineFragment", type: name().text, directives: directives(), selection: selection(), start });
        else if (peek().text === "{" || peek().text === "@") values.push({ kind: "InlineFragment", type: null, directives: directives(), selection: selection(), start });
        else values.push({ kind: "FragmentSpread", name: name().text, directives: directives(), start });
      } else {
        const key = name();
        let field = key.text, alias = null;
        if (accept(":")) { alias = field; field = name().text; }
        const arguments_ = args(), decorations = directives();
        values.push({ kind: "Field", name: field, alias, args: arguments_, directives: decorations, selection: peek().text === "{" ? selection() : [], start: key.start });
      }
    } while (!accept("}"));
    return values;
  }
  function type() {
    let result = accept("[") ? "[" + type() + (expect("]"), "]") : name().text;
    if (accept("!")) result += "!";
    return result;
  }
  const definitions = [];
  while (peek().text !== "<EOF>") {
    const start = peek().start;
    if (peek().text === "{") { definitions.push({ kind: "OperationDefinition", operation: "query", name: null, variables: [], directives: [], selection: selection(), start }); continue; }
    const token = take();
    if (token.text === "fragment") {
      const key = name().text; expect("on");
      definitions.push({ kind: "FragmentDefinition", name: key, type: name().text, directives: directives(), selection: selection(), start });
      continue;
    }
    if (!["query", "mutation", "subscription"].includes(token.text)) throw new GraphQLError(`Syntax Error: Unexpected ${describe(token)}.`);
    const operationName = peek().type === "Name" ? take().text : null;
    const variables = [];
    if (accept("(")) {
      do {
        const variableStart = expect("$").start;
        const key = name().text; expect(":"); const variableType = type();
        const defaultValue = accept("=") ? value() : undefined;
        variables.push({ name: key, type: variableType, defaultValue, start: variableStart });
      } while (!accept(")"));
    }
    definitions.push({ kind: "OperationDefinition", operation: token.text, name: operationName, variables, directives: directives(), selection: selection(), start });
  }
  return { kind: "Document", definitions, source };
}

export function printValue(value) {
  if (value?.kind === "Variable") return "$" + value.name;
  if (value?.kind === "Enum") return value.name;
  if (Array.isArray(value)) return "[" + value.map(printValue).join(", ") + "]";
  if (value && typeof value === "object") return "{" + Object.entries(value).map(([key, val]) => key + ": " + printValue(val)).join(", ") + "}";
  return JSON.stringify(value);
}

export function printDocument(document) {
  const argument = arg => arg.name + ": " + printValue(arg.value);
  const directives = node => node.directives.map(d => " @" + d.name + (d.args.length ? "(" + d.args.map(argument).join(", ") + ")" : "")).join("");
  function selections(nodes, level) {
    return nodes.map(node => {
      const indent = "  ".repeat(level);
      let head;
      if (node.kind === "FragmentSpread") return indent + "..." + node.name + directives(node);
      if (node.kind === "InlineFragment") head = "..." + (node.type ? " on " + node.type : "") + directives(node);
      else {
        head = (node.alias ? node.alias + ": " : "") + node.name;
        if (node.args.length) {
          const text = node.args.map(argument).join(", ");
          head += head.length + text.length + 2 > 80 ? "(\n" + node.args.map(arg => indent + "  " + argument(arg)).join("\n") + "\n" + indent + ")" : "(" + text + ")";
        }
        head += directives(node);
      }
      return indent + head + (node.selection.length ? " {\n" + selections(node.selection, level + 1) + "\n" + indent + "}" : "");
    }).join("\n");
  }
  return document.definitions.map(node => {
    let head = node.kind === "FragmentDefinition" ? "fragment " + node.name + " on " + node.type : node.operation + (node.name ? " " + node.name : "");
    if (node.variables?.length) head += "(" + node.variables.map(v => "$" + v.name + ": " + v.type + (v.defaultValue !== undefined ? " = " + printValue(v.defaultValue) : "")).join(", ") + ")";
    if (node.kind === "OperationDefinition" && !node.name && !node.variables.length && !node.directives.length) head = "";
    // Captured validation locations retain two extra operation separator lines;
    // consecutive fragment definitions share the ordinary two-line separator.
    return head + directives(node) + (head ? " " : "") + "{\n" + selections(node.selection, 1) + "\n}" + (node.kind === "OperationDefinition" ? "\n\n" : "");
  }).join("\n\n").trimEnd();
}

export function resolvedValue(value, variables) {
  if (value?.kind === "Variable") return variables[value.name];
  if (value?.kind === "Enum") return value.name;
  if (Array.isArray(value)) return value.map(v => resolvedValue(v, variables));
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([key, val]) => [key, resolvedValue(val, variables)]));
  return value;
}
