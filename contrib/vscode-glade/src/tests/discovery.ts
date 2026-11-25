export interface DiscoveredApexTestClass {
  className: string;
  methods: string[];
}

export interface ApexTestTarget {
  className: string;
  methodName: string;
}

export function discoverApexTests(_file: string, source: string): DiscoveredApexTestClass | undefined {
  if (!/@isTest\b/i.test(source) && !/\btestMethod\b/i.test(source)) {
    return undefined;
  }
  const classMatch = source.match(/\bclass\s+([A-Za-z_][A-Za-z0-9_]*)\b/);
  if (!classMatch) {
    return undefined;
  }
  const methods = new Set<string>();
  for (const method of testMethodPositions(source)) {
    methods.add(method.name);
  }
  if (methods.size === 0) {
    return undefined;
  }
  return { className: classMatch[1], methods: [...methods].sort() };
}

export function currentApexTestAtOffset(file: string, source: string, offset: number): ApexTestTarget | undefined {
  const discovered = discoverApexTests(file, source);
  if (!discovered) {
    return undefined;
  }
  const current = testMethodPositions(source).find((method) => offset >= method.offset && offset <= method.end);
  return current ? { className: discovered.className, methodName: current.name } : undefined;
}

function testMethodPositions(source: string): Array<{ name: string; offset: number; end: number }> {
  // Mask comments and strings without changing offsets or interpreting their braces.
  const code = source.replace(/\/\*[\s\S]*?\*\/|\/\/[^\r\n]*|'(?:\\[\s\S]|[^'\\])*'/g, (text) => text.replace(/[^\r\n]/g, " "));
  const methods: Array<{ name: string; offset: number; end: number }> = [];
  const declaration = /\bstatic\s+void\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\s*\{/gi;
  for (const match of code.matchAll(declaration)) {
    const start = match.index || 0;
    const boundary = Math.max(code.lastIndexOf("{", start - 1), code.lastIndexOf("}", start - 1), code.lastIndexOf(";", start - 1));
    const modifiers = code.slice(boundary + 1, start);
    if (!/@isTest\b/i.test(modifiers) && !/\btestMethod\b/i.test(modifiers)) {
      continue;
    }
    const body = start + match[0].length - 1;
    let depth = 1;
    let end = body + 1;
    while (end < code.length && depth > 0) {
      if (code[end] === "{") depth++;
      if (code[end] === "}") depth--;
      end++;
    }
    methods.push({ name: match[1], offset: start, end: end - 1 });
  }
  return methods;
}
