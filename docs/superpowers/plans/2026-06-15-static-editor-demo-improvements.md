# Static Editor Demo Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the static VitePress editor demo feel much closer to Glade without running a live LSP, local server, or project daemon.

**Architecture:** Keep the public site static. Generate a small editor-support catalog from checked Glade support reports, layer a tiny curated demo fixture on top, and make the CodeMirror workbench consume that generated catalog. The home page stays focused on the local Apex loop, with only a compact generated autocomplete preview and a link into the workbench.

**Tech Stack:** VitePress, Vue 3, CodeMirror 6, Node test runner, checked Markdown support reports, generated TypeScript/JSON assets.

---

## Current state

The workbench editor uses real CodeMirror 6, but the language behavior is local to `site/.vitepress/theme/GladeEditorWorkbench.vue`.

- `completionCatalog` is hand-written in the Vue file.
- `inferReceiverType()` uses regex scanning, not Glade's parser.
- Syntax highlighting uses a custom `StreamLanguage`.
- The homepage currently avoids a full editor and focuses on the local loop.
- The checked source of support truth available in this repo is `docs/STDLIB_COVERAGE.md`.

This plan improves option 1 only. It does not add a local server, LSP bridge, or WebSocket transport.

## File structure

- Create `site/scripts/build-editor-support.mjs`
  - Parses `docs/STDLIB_COVERAGE.md`.
  - Merges a small curated demo fixture.
  - Writes generated static assets.
  - Supports `--check` for stale-output detection.

- Create `site/.vitepress/theme/generated/editorSupport.ts`
  - Checked-in generated TypeScript module consumed by Vue.
  - Exports a typed plain object.

- Create `site/docs-src/public/data/editor-support.json`
  - Checked-in generated JSON for inspection and optional future browser demos.

- Create `site/.vitepress/theme/editor/editorSupportTypes.ts`
  - Defines `EditorSupportCatalog`, `EditorCompletion`, `EditorSupportStatus`, and demo receiver types.

- Create `site/.vitepress/theme/editor/apexLanguage.ts`
  - Moves current CodeMirror Apex stream tokenizer out of the component.

- Create `site/.vitepress/theme/editor/apexCompletions.ts`
  - Resolves receivers against the generated catalog.
  - Keeps the same dot-trigger behavior.

- Modify `site/.vitepress/theme/GladeEditorWorkbench.vue`
  - Shrinks it to rendering, CodeMirror setup, and layout.
  - Imports generated catalog and editor modules.

- Modify `site/docs-src/index.md`
  - Add a compact generated support preview below the local-loop proof, not a full editor.

- Modify `site/tests/theme.test.mjs`
  - Add stale generation checks.
  - Assert the Vue component no longer owns a large hard-coded completion catalog.
  - Assert important completions come from generated data.

- Modify `site/package.json`
  - Add `generate:editor-support`.
  - Run the generator in `--check` mode before tests and builds.

---

### Task 1: Pin the static-editor contract with failing tests

**Files:**
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add generated asset reads**

Add these reads near the existing top-level file reads:

```js
const editorSupportTs = await readFile(new URL("../.vitepress/theme/generated/editorSupport.ts", import.meta.url), "utf8").catch(() => "");
const editorSupportJsonText = await readFile(new URL("../docs-src/public/data/editor-support.json", import.meta.url), "utf8").catch(() => "{}");
const editorSupportJson = JSON.parse(editorSupportJsonText || "{}");
const buildEditorSupportScript = await readFile(new URL("../scripts/build-editor-support.mjs", import.meta.url), "utf8").catch(() => "");
```

- [ ] **Step 2: Add a failing generated-catalog test**

Add this test after `site copy is task-first and names local support plainly`:

```js
test("editor support catalog is generated from checked Glade support data", () => {
  assert.match(buildEditorSupportScript, /docs\/STDLIB_COVERAGE\.md/);
  assert.match(buildEditorSupportScript, /--check/);
  assert.match(editorSupportTs, /export const editorSupportCatalog/);
  assert.equal(editorSupportJson.schemaVersion, 1);
  assert.equal(editorSupportJson.generatedFrom, "docs/STDLIB_COVERAGE.md");

  const database = editorSupportJson.receivers?.Database?.items || [];
  assert.ok(database.some((item) => item.label === "insert" && item.status === "supported"));
  assert.ok(database.some((item) => item.label === "rollback" && item.status === "supported"));
  assert.ok(database.some((item) => item.label === "setSavepoint" && item.status === "supported"));

  const answers = editorSupportJson.receivers?.Answers?.items || [];
  assert.ok(answers.some((item) => item.label === "findSimilar" && item.status === "unsupported"));

  const describe = editorSupportJson.receivers?.["Schema.DescribeSObjectResult"]?.items || [];
  assert.ok(describe.some((item) => item.label === "getChildRelationships" && item.status === "supported"));
  assert.ok(describe.some((item) => item.label === "fields.getMap" && item.status === "partial"));
});
```

- [ ] **Step 3: Add a failing component split test**

Add this test after the generated-catalog test:

```js
test("workbench imports generated editor support instead of owning a static catalog", () => {
  assert.match(codeMirrorWorkbench, /editorSupportCatalog/);
  assert.match(codeMirrorWorkbench, /createApexCompletions/);
  assert.match(codeMirrorWorkbench, /apexLanguage/);
  assert.doesNotMatch(codeMirrorWorkbench, /const completionCatalog: Record/);
  assert.doesNotMatch(codeMirrorWorkbench, /const rootCompletions: Completion\[\]/);
  assert.doesNotMatch(codeMirrorWorkbench, /const DEMO_RECEIVER_TYPES/);
});
```

- [ ] **Step 4: Run tests and verify failure**

Run:

```bash
npm --prefix site test
```

Expected: FAIL. The generated files and imports do not exist yet.

- [ ] **Step 5: Commit**

Do not commit if this is part of a larger uncommitted prototype branch. If executing as a clean branch:

```bash
git add site/tests/theme.test.mjs
git commit -m "test: pin generated editor support contract"
```

---

### Task 2: Add the generated editor-support catalog

**Files:**
- Create: `site/scripts/build-editor-support.mjs`
- Create: `site/.vitepress/theme/generated/editorSupport.ts`
- Create: `site/docs-src/public/data/editor-support.json`
- Modify: `site/package.json`

- [ ] **Step 1: Create the generator script**

Create `site/scripts/build-editor-support.mjs`:

```js
#!/usr/bin/env node
import { readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = resolve(siteRoot, "..");
const coveragePath = resolve(repoRoot, "docs/STDLIB_COVERAGE.md");
const tsOut = resolve(siteRoot, ".vitepress/theme/generated/editorSupport.ts");
const jsonOut = resolve(siteRoot, "docs-src/public/data/editor-support.json");
const check = process.argv.includes("--check");

const statusLabels = {
  supported: "Works well",
  partial: "With limits",
  stub: "With limits",
  unsupported: "Needs Salesforce",
  unknown: "Not measured"
};

const curatedReceivers = {
  Account: {
    label: "Account",
    detail: "Schema-backed local SObject",
    items: [
      item("Name", "property", "Account field", "supported", "Schema-backed local field"),
      item("Id", "property", "Standard field", "supported", "Schema-backed local field"),
      item("BillingCity", "property", "Standard field", "supported", "Schema-backed local field"),
      item("OwnerId", "property", "Lookup field", "supported", "Schema-backed local field"),
      item("AnnualRevenue", "property", "Currency field", "supported", "Schema-backed local field"),
      item("SObjectType", "property", "Describe token", "partial", "Local metadata with named limits"),
      item("get", "method", "Dynamic field read", "supported", "Local SObject dynamic field read", "get('Name')"),
      item("put", "method", "Dynamic field write", "supported", "Local SObject dynamic field write", "put('Name', value)"),
      item("addError", "method", "Validation failure", "supported", "Local validation error", "addError('message')")
    ]
  },
  "List<Account>": {
    label: "List<Account>",
    detail: "Collection",
    items: [
      item("size", "method", "List method", "supported", "Collection support", "size()"),
      item("isEmpty", "method", "List method", "supported", "Collection support", "isEmpty()"),
      item("get", "method", "Indexed Account", "supported", "Collection support", "get(0)"),
      item("add", "method", "Collection mutation", "supported", "Collection support", "add(account)"),
      item("clone", "method", "Collection clone", "partial", "Collection support with named limits", "clone()")
    ]
  },
  "Database.SaveResult[]": {
    label: "Database.SaveResult[]",
    detail: "DML results",
    items: [
      item("size", "method", "Result count", "supported", "Collection support", "size()"),
      item("isEmpty", "method", "Result list state", "supported", "Collection support", "isEmpty()"),
      item("get", "method", "Single DML result", "supported", "Collection support", "get(0)")
    ]
  },
  "Database.SaveResult": {
    label: "Database.SaveResult",
    detail: "DML result",
    items: [
      item("isSuccess", "method", "DML result", "supported", "Local DML result", "isSuccess()"),
      item("getId", "method", "Inserted record Id", "supported", "Local DML result", "getId()"),
      item("getErrors", "method", "Database.Error[]", "supported", "Local DML errors", "getErrors()")
    ]
  },
  "Schema.DescribeSObjectResult.fields": {
    label: "Schema.DescribeSObjectResult.fields",
    detail: "Field describe namespace",
    items: [
      item("getMap", "method", "Map<String, Schema.SObjectField>", "partial", "Local metadata with named limits", "getMap()")
    ]
  },
  "Map<String, Schema.SObjectField>": {
    label: "Map<String, Schema.SObjectField>",
    detail: "Field token map",
    items: [
      item("get", "method", "Schema.SObjectField", "partial", "Local metadata with named limits", "get('Name')"),
      item("containsKey", "method", "Field presence", "supported", "Map support", "containsKey('Name')"),
      item("keySet", "method", "Field API names", "supported", "Map support", "keySet()"),
      item("values", "method", "Field tokens", "supported", "Map support", "values()")
    ]
  }
};

const demoReceivers = {
  account: "Account",
  accounts: "List<Account>",
  describe: "Schema.DescribeSObjectResult",
  fieldMap: "Map<String, Schema.SObjectField>",
  results: "Database.SaveResult[]"
};

function item(label, type, detail, status, info, apply = `${label}()`) {
  return {
    label,
    apply,
    type,
    detail,
    status,
    statusLabel: statusLabels[status] || "Not measured",
    info
  };
}

function parseCoverage(markdown) {
  const receivers = {};
  for (const line of markdown.split("\n")) {
    const match = /^\| ([^|]+) \| `([^`]+)` \| `([^`]+)` \| ([^|]+) \|$/.exec(line);
    if (!match) continue;
    const [, area, api, status, notes] = match;
    const parsed = parseApi(api.trim(), area.trim(), status.trim(), notes.trim());
    if (!parsed) continue;
    const receiver = receivers[parsed.receiver] || {
      label: parsed.receiver,
      detail: `${parsed.receiver} support`,
      items: []
    };
    receiver.items.push(parsed.item);
    receivers[parsed.receiver] = receiver;
  }
  return receivers;
}

function parseApi(api, area, status, notes) {
  const normalized = api.replace(/\([^)]*\)$/, "");
  const dot = normalized.indexOf(".");
  if (dot < 0) return null;
  const receiver = normalized.slice(0, dot);
  const label = normalized.slice(dot + 1);
  if (!receiver || !label || label.includes(" ")) return null;
  return {
    receiver,
    item: {
      label,
      apply: api.slice(receiver.length + 1),
      type: label[0] === label[0]?.toUpperCase() ? "class" : "method",
      detail: `${area} API`,
      status,
      statusLabel: statusLabels[status] || "Not measured",
      info: notes,
      source: "docs/STDLIB_COVERAGE.md"
    }
  };
}

function mergeReceivers(generated, curated) {
  const merged = { ...generated };
  for (const [name, receiver] of Object.entries(curated)) {
    const current = merged[name] || { label: receiver.label, detail: receiver.detail, items: [] };
    const seen = new Set(current.items.map((entry) => entry.label));
    for (const entry of receiver.items) {
      if (!seen.has(entry.label)) current.items.push(entry);
    }
    current.detail = receiver.detail || current.detail;
    merged[name] = current;
  }
  return Object.fromEntries(Object.entries(merged).sort(([a], [b]) => a.localeCompare(b)));
}

function buildCatalog(markdown) {
  return {
    schemaVersion: 1,
    generatedFrom: "docs/STDLIB_COVERAGE.md",
    statusLabels,
    receivers: mergeReceivers(parseCoverage(markdown), curatedReceivers),
    rootCompletions: [
      item("Account", "class", "SObject", "supported", "Schema-backed local SObject", "Account"),
      item("Database", "class", "DML and SOQL", "supported", "Partial-success DML and dynamic query paths", "Database"),
      item("Schema", "class", "Metadata", "partial", "Configured metadata with named limits", "Schema"),
      item("Limits", "class", "Governor counters", "supported", "Local counters for SOQL, DML, CPU, heap, and async", "Limits"),
      item("JSON", "class", "Serialization", "supported", "Local JSON helpers", "JSON"),
      item("UserInfo", "class", "User context", "supported", "Local identity helpers", "UserInfo"),
      item("Answers", "class", "Hosted API", "unsupported", "Needs Salesforce", "Answers")
    ],
    demoReceivers
  };
}

function renderTs(catalog) {
  return [
    "// Generated by site/scripts/build-editor-support.mjs. Do not edit by hand.",
    "import type { EditorSupportCatalog } from '../editor/editorSupportTypes'",
    "",
    `export const editorSupportCatalog = ${JSON.stringify(catalog, null, 2)} as const satisfies EditorSupportCatalog`,
    ""
  ].join("\n");
}

async function main() {
  const markdown = await readFile(coveragePath, "utf8");
  const catalog = buildCatalog(markdown);
  const json = `${JSON.stringify(catalog, null, 2)}\n`;
  const ts = renderTs(catalog);

  if (check) {
    const [oldJson, oldTs] = await Promise.all([
      readFile(jsonOut, "utf8").catch(() => ""),
      readFile(tsOut, "utf8").catch(() => "")
    ]);
    if (oldJson !== json || oldTs !== ts) {
      throw new Error("editor support catalog is stale; run npm --prefix site run generate:editor-support");
    }
    return;
  }

  await writeFile(jsonOut, json);
  await writeFile(tsOut, ts);
}

main().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
```

- [ ] **Step 2: Generate the initial files**

Run:

```bash
npm --prefix site exec -- node scripts/build-editor-support.mjs
```

Expected: `site/.vitepress/theme/generated/editorSupport.ts` and `site/docs-src/public/data/editor-support.json` are created.

- [ ] **Step 3: Add npm scripts**

Modify `site/package.json` scripts:

```json
{
  "scripts": {
    "generate:editor-support": "node scripts/build-editor-support.mjs",
    "dev": "vitepress dev .",
    "build": "npm run generate:editor-support -- --check && vitepress build .",
    "test": "npm run generate:editor-support -- --check && node --test tests/*.test.mjs",
    "preview": "vitepress preview ."
  }
}
```

- [ ] **Step 4: Run tests and verify partial failure**

Run:

```bash
npm --prefix site test
```

Expected: generator contract passes. Component split test still fails because the Vue component has not been split yet.

- [ ] **Step 5: Commit**

```bash
git add site/scripts/build-editor-support.mjs site/.vitepress/theme/generated/editorSupport.ts site/docs-src/public/data/editor-support.json site/package.json site/package-lock.json
git commit -m "feat: generate static editor support catalog"
```

---

### Task 3: Split editor logic out of the Vue component

**Files:**
- Create: `site/.vitepress/theme/editor/editorSupportTypes.ts`
- Create: `site/.vitepress/theme/editor/apexLanguage.ts`
- Create: `site/.vitepress/theme/editor/apexCompletions.ts`
- Modify: `site/.vitepress/theme/GladeEditorWorkbench.vue`

- [ ] **Step 1: Create support types**

Create `site/.vitepress/theme/editor/editorSupportTypes.ts`:

```ts
import type { Completion } from '@codemirror/autocomplete'

export type EditorSupportStatus = 'supported' | 'partial' | 'stub' | 'unsupported' | 'unknown'

export type EditorCompletion = Completion & {
  status: EditorSupportStatus
  statusLabel: string
  info: string
  source?: string
}

export type EditorReceiver = {
  label: string
  detail: string
  items: readonly EditorCompletion[]
}

export type EditorSupportCatalog = {
  schemaVersion: 1
  generatedFrom: string
  statusLabels: Record<EditorSupportStatus, string>
  receivers: Record<string, EditorReceiver>
  rootCompletions: readonly EditorCompletion[]
  demoReceivers: Record<string, string>
}
```

- [ ] **Step 2: Move language code**

Create `site/.vitepress/theme/editor/apexLanguage.ts` by moving these pieces out of `GladeEditorWorkbench.vue`:

```ts
import { HighlightStyle, StreamLanguage, type StringStream } from '@codemirror/language'
import { tags } from '@lezer/highlight'
```

Export these names:

```ts
export const apexLanguage = StreamLanguage.define<ApexModeState>({ ... })
export const gladeHighlight = HighlightStyle.define([ ... ])
```

Use the existing sets and helper functions unchanged in the first pass:

- `APEX_KEYWORDS`
- `APEX_CONSTANTS`
- `APEX_ANNOTATIONS`
- `APEX_ANNOTATION_ATTRIBUTES`
- `SYSTEM_TYPES`
- `PLATFORM_TYPES`
- `SOQL_FUNCTIONS`
- `DECLARATION_KEYWORDS`
- `remember`
- `readApexString`
- `readAnnotation`
- `readApexIdentifier`

- [ ] **Step 3: Move completion code**

Create `site/.vitepress/theme/editor/apexCompletions.ts`:

```ts
import { startCompletion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete'
import type { EditorView } from '@codemirror/view'
import type { EditorSupportCatalog } from './editorSupportTypes'

function indexedReceiverType(type: string, hasIndexAccess: boolean) {
  if (!hasIndexAccess) return type
  if (type === 'Database.SaveResult[]') return 'Database.SaveResult'
  if (type === 'List<Account>') return 'Account'
  return type
}

function inferReceiverType(catalog: EditorSupportCatalog, doc: string, receiver: string) {
  const hasIndexAccess = /\[[^\]]+\]/.test(receiver)
  const normalized = receiver.replace(/\[[^\]]*\]/g, '')
  if (catalog.receivers[receiver]) return receiver
  if (catalog.receivers[normalized]) return normalized

  if (normalized.endsWith('.fields')) {
    const owner = normalized.slice(0, -'.fields'.length)
    if (inferReceiverType(catalog, doc, owner) === 'Schema.DescribeSObjectResult') {
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
  return catalog.receivers[variableName] ? variableName : ''
}

export function createApexCompletions(catalog: EditorSupportCatalog) {
  return function apexCompletions(context: CompletionContext): CompletionResult | null {
    const receiver = context.matchBefore(/([A-Za-z_][A-Za-z0-9_]*(?:\[[^\]]+\])?(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\.([A-Za-z0-9_]*)?$/)
    if (receiver) {
      const match = /^([A-Za-z_][A-Za-z0-9_]*(?:\[[^\]]+\])?(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\.([A-Za-z0-9_]*)?$/.exec(receiver.text)
      const receiverName = match?.[1] || ''
      const prefix = match?.[2] || ''
      const receiverType = inferReceiverType(catalog, context.state.doc.toString(), receiverName)
      const options = receiverType ? catalog.receivers[receiverType]?.items : undefined
      if (!options) return null
      return { from: context.pos - prefix.length, options: [...options], validFor: /^[A-Za-z0-9_]*$/ }
    }

    const word = context.matchBefore(/[A-Za-z_][A-Za-z0-9_]*/)
    if (!word && !context.explicit) return null
    return { from: word ? word.from : context.pos, options: [...catalog.rootCompletions], validFor: /^[A-Za-z0-9_]*$/ }
  }
}

export function maybeOpenReceiverCompletion(view: EditorView, currentView: EditorView | null) {
  const cursor = view.state.selection.main.head
  const beforeCursor = view.state.sliceDoc(Math.max(0, cursor - 120), cursor)
  if (!/[A-Za-z_][A-Za-z0-9_]*(?:\[[^\]]+\])?(?:\.[A-Za-z_][A-Za-z0-9_]*)*\.$/.test(beforeCursor)) return

  window.requestAnimationFrame(() => {
    if (currentView === view) startCompletion(view)
  })
}
```

- [ ] **Step 4: Shrink the Vue component**

Modify `site/.vitepress/theme/GladeEditorWorkbench.vue` imports:

```ts
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { EditorState } from '@codemirror/state'
import { EditorView, basicSetup } from 'codemirror'
import { autocompletion } from '@codemirror/autocomplete'
import { syntaxHighlighting } from '@codemirror/language'
import { editorSupportCatalog } from './generated/editorSupport'
import { apexLanguage, gladeHighlight } from './editor/apexLanguage'
import { createApexCompletions, maybeOpenReceiverCompletion } from './editor/apexCompletions'
```

Delete the hard-coded `completionCatalog`, `rootCompletions`, `DEMO_RECEIVER_TYPES`, Apex language sets, and receiver helper functions from the Vue file.

Add this near `startDoc`:

```ts
const apexCompletions = createApexCompletions(editorSupportCatalog)
```

Change the update listener:

```ts
EditorView.updateListener.of((update) => {
  if (update.docChanged) maybeOpenReceiverCompletion(update.view, editorView)
})
```

- [ ] **Step 5: Run tests**

Run:

```bash
npm --prefix site test
```

Expected: PASS for the generated-catalog and component split tests. Existing workbench tests may fail if they still search for old inline constants; update those assertions to check imported modules and generated catalog contents instead.

- [ ] **Step 6: Commit**

```bash
git add site/.vitepress/theme/GladeEditorWorkbench.vue site/.vitepress/theme/editor site/tests/theme.test.mjs
git commit -m "refactor: feed workbench completions from generated support catalog"
```

---

### Task 4: Improve the demo source so completions feel richer

**Files:**
- Modify: `site/.vitepress/theme/GladeEditorWorkbench.vue`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add failing assertions for richer receivers**

In `workbench page mounts a real CodeMirror editor`, add:

```js
assert.match(codeMirrorWorkbench, /Savepoint marker = Database\.setSavepoint\(\)/);
assert.match(codeMirrorWorkbench, /Database\.rollback\(marker\)/);
assert.match(codeMirrorWorkbench, /JSON\.serialize\(results\)/);
assert.match(codeMirrorWorkbench, /BusinessHours\.nextStartDate/);
assert.match(codeMirrorWorkbench, /Answers\.findSimilar/);
```

- [ ] **Step 2: Replace `startDoc` with a stronger sample**

Use this source in `GladeEditorWorkbench.vue`:

```ts
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
```

Do not include `Answers.findSimilar()` in the executable sample. Keep it as a completion suggestion only, so the unsupported label is visible without making the source look like broken code.

- [ ] **Step 3: Ensure generated catalog covers new root receivers**

Regenerate:

```bash
npm --prefix site run generate:editor-support
```

Expected: `BusinessHours`, `Database`, `JSON`, and `Answers` receiver entries exist because they come from `docs/STDLIB_COVERAGE.md`.

- [ ] **Step 4: Run tests**

Run:

```bash
npm --prefix site test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add site/.vitepress/theme/GladeEditorWorkbench.vue site/.vitepress/theme/generated/editorSupport.ts site/docs-src/public/data/editor-support.json site/tests/theme.test.mjs
git commit -m "feat: show richer static Apex editor sample"
```

---

### Task 5: Add better completion labels and info panels

**Files:**
- Modify: `site/.vitepress/theme/editor/apexCompletions.ts`
- Modify: `site/.vitepress/theme/custom.css`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add failing tests for status markup**

Add:

```js
test("workbench completion info exposes support status labels", () => {
  assert.match(codeMirrorWorkbench, /completionInfo/);
  assert.match(css, /\.glade-completion-info/);
  assert.match(css, /\.glade-completion-status-supported/);
  assert.match(css, /\.glade-completion-status-partial/);
  assert.match(css, /\.glade-completion-status-unsupported/);
});
```

- [ ] **Step 2: Add `completionInfo` helper**

In `site/.vitepress/theme/editor/apexCompletions.ts`, add:

```ts
import type { EditorCompletion } from './editorSupportTypes'

function completionInfo(completion: EditorCompletion) {
  const root = document.createElement('div')
  root.className = 'glade-completion-info'

  const status = document.createElement('span')
  status.className = `glade-completion-status glade-completion-status-${completion.status}`
  status.textContent = completion.statusLabel

  const detail = document.createElement('strong')
  detail.textContent = completion.detail || completion.label

  const note = document.createElement('p')
  note.textContent = completion.info || ''

  root.append(status, detail, note)
  return root
}
```

When returning options, map them:

```ts
const options = [...receiver.items].map((entry) => ({ ...entry, info: completionInfo(entry) }))
```

For root completions:

```ts
const options = [...catalog.rootCompletions].map((entry) => ({ ...entry, info: completionInfo(entry) }))
```

- [ ] **Step 3: Add CSS**

Add to `site/.vitepress/theme/custom.css` near the CodeMirror workbench styles:

```css
.glade-completion-info {
  display: grid;
  gap: 6px;
  max-width: 320px;
  padding: 10px;
}

.glade-completion-info strong {
  color: var(--vp-c-text-1);
  font-size: 12px;
}

.glade-completion-info p {
  margin: 0;
  color: var(--vp-c-text-2);
  font-size: 12px;
  line-height: 1.45;
}

.glade-completion-status {
  width: max-content;
  border-radius: 999px;
  padding: 2px 7px;
  font-size: 11px;
  font-weight: 700;
}

.glade-completion-status-supported {
  background: rgba(155, 232, 112, 0.16);
  color: var(--glade-green);
}

.glade-completion-status-partial,
.glade-completion-status-stub {
  background: rgba(245, 201, 95, 0.16);
  color: #f5c95f;
}

.glade-completion-status-unsupported {
  background: rgba(255, 132, 112, 0.16);
  color: #ff967f;
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
npm --prefix site test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add site/.vitepress/theme/editor/apexCompletions.ts site/.vitepress/theme/custom.css site/tests/theme.test.mjs
git commit -m "feat: show support status in completion info"
```

---

### Task 6: Add a compact generated support preview to the homepage

**Files:**
- Modify: `site/docs-src/index.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add failing home-page assertions**

Update `home page uses a polished local loop proof without experiment tabs` so it still forbids a full editor, but allows a compact generated preview:

```js
assert.doesNotMatch(index, /class="[^"]*\bhome-editor-demo\b[^"]*"/);
assert.doesNotMatch(index, /data-apex-editor/);
assert.doesNotMatch(index, /<textarea/);
assert.match(index, /class="home-support-preview"/);
assert.match(index, /data-generated-support-preview/);
assert.match(index, /Database\.insert/);
assert.match(index, /Schema\.DescribeSObjectResult/);
assert.match(index, /Answers\.findSimilar/);
```

- [ ] **Step 2: Add the preview below the local-loop visual**

In `site/docs-src/index.md`, add one compact block after the existing local-loop proof and before the broader feature copy:

```html
<div class="home-support-preview" data-generated-support-preview aria-label="Apex autocomplete support preview">
  <p><strong>Autocomplete preview</strong><span>Generated from checked Glade support rows.</span></p>
  <div>
    <code>Database.insert</code><span class="home-completion-status home-completion-status-supported">Works well</span>
    <code>Schema.DescribeSObjectResult</code><span class="home-completion-status home-completion-status-limited">With limits</span>
    <code>Answers.findSimilar</code><span class="home-completion-status home-completion-status-salesforce">Needs Salesforce</span>
  </div>
  <a href="/guide/workbench">Open the interactive workbench</a>
</div>
```

This is intentionally not an editor. It is a trail marker. The workbench carries the editor.

- [ ] **Step 3: Add CSS if needed**

If no existing styles fit, add:

```css
.home-support-preview {
  display: grid;
  gap: 12px;
  border: 1px solid var(--glade-border);
  background: rgba(5, 9, 11, 0.72);
  padding: 14px;
}

.home-support-preview p {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 10px;
  margin: 0;
}

.home-support-preview p span {
  color: var(--vp-c-text-2);
}

.home-support-preview > div {
  display: grid;
  grid-template-columns: minmax(0, 1fr) max-content;
  gap: 8px 10px;
  align-items: center;
}

.home-support-preview code {
  overflow-wrap: anywhere;
}

@media (max-width: 640px) {
  .home-support-preview > div {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
npm --prefix site test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add site/docs-src/index.md site/.vitepress/theme/custom.css site/tests/theme.test.mjs
git commit -m "feat: add compact generated support preview"
```

---

### Task 7: Browser verification and polish

**Files:**
- Modify as needed from browser findings.

- [ ] **Step 1: Run full site gates**

Run:

```bash
npm --prefix site test
npm --prefix site run build
git diff --check
```

Expected:

- Tests pass.
- VitePress build completes.
- `git diff --check` has no output.

- [ ] **Step 2: Start or reuse the dev server**

If no server is running:

```bash
npm --prefix site run dev -- --host 127.0.0.1 --port 5174
```

Expected: VitePress serves `http://127.0.0.1:5174/`.

- [ ] **Step 3: Browser-check home page**

Open:

```text
http://127.0.0.1:5174/
```

Check:

- The first viewport still reads as local Apex runtime, not IDE landing page.
- The support preview is compact.
- The preview labels are visible on mobile and desktop.
- No text overlaps.
- No horizontal overflow at 390px width.

- [ ] **Step 4: Browser-check workbench page**

Open:

```text
http://127.0.0.1:5174/guide/workbench
```

Check:

- Editor is not auto-focused on page load.
- Typing `Database.` opens generated completions.
- Typing `BusinessHours.` opens generated completions.
- Typing `Answers.` shows `findSimilar` with `Needs Salesforce`.
- Typing `describe.` shows describe completions.
- Typing `describe.fields.` shows `getMap`.
- Typing `results[0].` shows `isSuccess`, `getId`, and `getErrors`.
- The completion info panel does not cover the whole editor.
- CLI output highlighting still works.

- [ ] **Step 5: Commit browser polish**

If browser work required fixes:

```bash
git add site
git commit -m "fix: polish static editor demo rendering"
```

If no fixes were required, skip this commit.

---

## Self-review

Spec coverage:

- Static site only: covered by generator and VitePress assets.
- No live LSP: preserved. No server, WebSocket, daemon, or `glade lsp` bridge added.
- Closer to Glade: generated data comes from `docs/STDLIB_COVERAGE.md`.
- More convincing autocomplete: workbench uses generated receiver catalog, richer sample, and status info panels.
- Home page restraint: only compact preview goes on home; the full editor stays on `/guide/workbench`.
- Testing: generator check, text tests, build, diff check, and browser smoke are specified.

Placeholder scan:

- No placeholder steps remain.
- Every code-changing task includes concrete code or exact edits.
- Every verification step has commands and expected results.

Type consistency:

- `EditorSupportCatalog`, `EditorReceiver`, `EditorCompletion`, and `EditorSupportStatus` are defined before use.
- `createApexCompletions()` consumes `EditorSupportCatalog`.
- Generated `editorSupportCatalog` satisfies `EditorSupportCatalog`.
- `maybeOpenReceiverCompletion()` takes the current `EditorView | null` so the Vue component does not close over module state.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-15-static-editor-demo-improvements.md`.

Two execution options:

1. **Subagent-Driven (recommended)** - dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** - execute tasks in this session using executing-plans, with checkpoints after each task.
