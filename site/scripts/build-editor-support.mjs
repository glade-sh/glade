#!/usr/bin/env node
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = resolve(siteRoot, "..");
const coveragePath = resolve(repoRoot, "docs/STDLIB_COVERAGE.md");
const tsOut = resolve(siteRoot, ".vitepress/theme/generated/editorSupport.ts");
const jsonOut = resolve(siteRoot, "docs-src/public/data/editor-support.json");
const check = process.argv.includes("--check");

const statusLabels = {
  supported: "Runs locally",
  partial: "Runs locally with limits",
  stub: "Runs locally with limits",
  unsupported: "Requires Salesforce",
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
  "Schema.DescribeSObjectResult": {
    label: "Schema.DescribeSObjectResult",
    detail: "Object describe metadata",
    items: [
      item("getRecordTypeInfosByDeveloperName", "method", "Record type metadata", "supported", "Local metadata", "getRecordTypeInfosByDeveloperName()"),
      item("getChildRelationships", "method", "Relationship metadata", "supported", "Local metadata", "getChildRelationships()"),
      item("fields.getMap", "method", "Field describe map", "partial", "Local metadata with named limits", "fields.getMap()"),
      item("getLabel", "method", "Object label", "supported", "Local metadata", "getLabel()"),
      item("getName", "method", "Object API name", "supported", "Local metadata", "getName()")
    ]
  },
  "Schema.DescribeSObjectResult.fields": {
    label: "Schema.DescribeSObjectResult.fields",
    detail: "Field describe namespace",
    items: [
      item("getMap", "method", "Map<String, Schema.SObjectField>", "partial", "Local metadata with named limits", "getMap()")
    ]
  },
  "Schema.SObjectType": {
    label: "Schema.SObjectType",
    detail: "Object token",
    items: [
      item("getDescribe", "method", "Describe object", "supported", "Local metadata", "getDescribe()"),
      item("newSObject", "method", "Construct SObject", "supported", "Local SObject construction", "newSObject()")
    ]
  },
  "Schema.SObjectField": {
    label: "Schema.SObjectField",
    detail: "Field token",
    items: [
      item("getDescribe", "method", "Field metadata", "supported", "Local metadata", "getDescribe()")
    ]
  },
  "Schema.DescribeFieldResult": {
    label: "Schema.DescribeFieldResult",
    detail: "Field describe metadata",
    items: [
      item("getLabel", "method", "Field label", "supported", "Local metadata", "getLabel()"),
      item("getType", "method", "Display type", "supported", "Local metadata", "getType()"),
      item("isCreateable", "method", "CRUD metadata", "partial", "Local metadata with named limits", "isCreateable()"),
      item("isUpdateable", "method", "CRUD metadata", "partial", "Local metadata with named limits", "isUpdateable()"),
      item("getReferenceTo", "method", "Lookup targets", "partial", "Local metadata with named limits", "getReferenceTo()")
    ]
  },
  Limits: {
    label: "Limits",
    detail: "Governor counters",
    items: [
      item("getDmlRows", "method", "Governor counter", "supported", "Local counters", "getDmlRows()"),
      item("getQueries", "method", "SOQL counter", "supported", "Local counters", "getQueries()"),
      item("getLimitDmlRows", "method", "Governor ceiling", "supported", "Local counters", "getLimitDmlRows()")
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

function defaultApply(label, type) {
  return type === "method" ? `${label}()` : label;
}

function item(label, type, detail, status, info, apply) {
  return {
    label,
    apply: apply || defaultApply(label, type),
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
    addReceiverItem(receiver, parsed.item);
    receivers[parsed.receiver] = receiver;
  }
  return receivers;
}

function addReceiverItem(receiver, nextItem) {
  const current = receiver.items.find((entry) => entry.label === nextItem.label);
  if (!current) {
    receiver.items.push(nextItem);
    return;
  }

  const signatures = new Set([
    ...(current.signatures || []),
    current.signature,
    nextItem.signature,
    ...(nextItem.signatures || [])
  ].filter(Boolean));
  if (signatures.size > 1) current.signatures = [...signatures];
}

function parseApi(api, area, status, notes) {
  const normalized = api.replace(/\([^)]*\)$/, "");
  const dot = normalized.indexOf(".");
  if (dot < 0) return null;
  const receiver = normalized.slice(0, dot);
  const label = normalized.slice(dot + 1);
  if (!receiver || !label || label.includes(" ")) return null;
  const type = label[0] === label[0]?.toUpperCase() ? "class" : "method";
  const signature = api.slice(receiver.length + 1);
  return {
    receiver,
    item: {
      label,
      apply: defaultApply(label, type),
      type,
      detail: `${area} API`,
      status,
      statusLabel: statusLabels[status] || "Not measured",
      info: notes,
      signature,
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
      if (!seen.has(entry.label)) {
        current.items.push(entry);
        seen.add(entry.label);
      }
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
    summary: summarizeCoverage(markdown),
    statusLabels,
    receivers: mergeReceivers(parseCoverage(markdown), curatedReceivers),
    rootCompletions: [
      item("Account", "class", "SObject", "supported", "Schema-backed local SObject", "Account"),
      item("Database", "class", "DML and SOQL", "supported", "Partial-success DML and dynamic query paths", "Database"),
      item("Schema", "class", "Metadata", "partial", "Configured metadata with named limits", "Schema"),
      item("Limits", "class", "Governor counters", "supported", "Local counters for SOQL, DML, CPU, heap, and async", "Limits"),
      item("JSON", "class", "Serialization", "supported", "Local JSON helpers", "JSON"),
      item("UserInfo", "class", "User context", "supported", "Local identity helpers", "UserInfo"),
      item("Answers", "class", "Hosted API", "unsupported", "Requires Salesforce", "Answers")
    ],
    demoReceivers
  };
}

function summarizeCoverage(markdown) {
  const summary = { supported: 0, partial: 0, stub: 0, unsupported: 0, unknown: 0 };
  for (const line of markdown.split("\n")) {
    const match = /^\| [^|]+ \| `[^`]+` \| `([^`]+)` \| [^|]+ \|$/.exec(line);
    if (match && Object.hasOwn(summary, match[1])) summary[match[1]] += 1;
  }
  return summary;
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

  await Promise.all([mkdir(dirname(jsonOut), { recursive: true }), mkdir(dirname(tsOut), { recursive: true })]);
  await Promise.all([writeFile(jsonOut, json), writeFile(tsOut, ts)]);
}

main().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
