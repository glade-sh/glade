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
  let current: string | undefined;
  for (const method of testMethodPositions(source)) {
    if (method.offset > offset) {
      break;
    }
    current = method.name;
  }
  return current ? { className: discovered.className, methodName: current } : undefined;
}

function testMethodPositions(source: string): Array<{ name: string; offset: number }> {
  const methods: Array<{ name: string; offset: number }> = [];
  const isTestMethod = /@isTest(?:\s*\([^)]*\))?[\s\S]{0,240}?\bstatic\s+void\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/gi;
  for (const match of source.matchAll(isTestMethod)) {
    methods.push({ name: match[1], offset: methodNameOffset(match) });
  }
  const legacyMethod = /\btestMethod\s+static\s+void\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/gi;
  for (const match of source.matchAll(legacyMethod)) {
    methods.push({ name: match[1], offset: methodNameOffset(match) });
  }
  methods.sort((a, b) => a.offset - b.offset || a.name.localeCompare(b.name));
  return methods;
}

function methodNameOffset(match: RegExpMatchArray): number {
  const text = match[0] || "";
  const name = match[1] || "";
  return (match.index || 0) + Math.max(0, text.lastIndexOf(name));
}
