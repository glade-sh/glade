import * as fs from "fs";
import * as path from "path";

import { WorkbenchEntry, WorkbenchEntryKind, assertWorkbenchEntryKind } from "./model";

interface WorkbenchStoreFile {
  version: 1;
  entries: WorkbenchEntry[];
}

const fileNames: Record<WorkbenchEntryKind, string> = {
  anonymousApex: "snippets.json",
  soql: "queries.json",
};

export function workbenchStorePath(projectRoot: string, kind: string): string {
  assertWorkbenchEntryKind(kind);
  return path.join(projectRoot, ".glade", "workbench", fileNames[kind]);
}

export function readWorkbenchEntries(projectRoot: string, kind: string): WorkbenchEntry[] {
  const filePath = workbenchStorePath(projectRoot, kind);
  if (!fs.existsSync(filePath)) {
    return [];
  }
  const parsed = JSON.parse(fs.readFileSync(filePath, "utf8")) as Partial<WorkbenchStoreFile>;
  if (parsed.version !== 1) {
    throw new Error(`unsupported workbench store version: ${String(parsed.version)}`);
  }
  if (!Array.isArray(parsed.entries)) {
    throw new Error("workbench store entries must be an array");
  }
  return parsed.entries.map((entry, index) => normalizeEntry(entry, kind as WorkbenchEntryKind, index));
}

export function writeWorkbenchEntries(projectRoot: string, kind: string, entries: WorkbenchEntry[]): void {
  const filePath = workbenchStorePath(projectRoot, kind);
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  const document: WorkbenchStoreFile = {
    version: 1,
    entries: entries.map((entry, index) => normalizeEntry(entry, kind as WorkbenchEntryKind, index)),
  };
  fs.writeFileSync(filePath, `${JSON.stringify(document, null, 2)}\n`, "utf8");
}

function normalizeEntry(value: unknown, kind: WorkbenchEntryKind, index: number): WorkbenchEntry {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`workbench entry ${index} must be an object`);
  }
  const entry = value as Partial<WorkbenchEntry>;
  if (entry.kind !== kind) {
    throw new Error(`workbench entry ${index} kind must be ${kind}`);
  }
  const id = requiredString(entry.id, index, "id");
  const name = requiredString(entry.name, index, "name");
  const body = requiredString(entry.body, index, "body");
  const createdAt = requiredString(entry.createdAt, index, "createdAt");
  const updatedAt = requiredString(entry.updatedAt, index, "updatedAt");
  if (entry.lastRunAt !== undefined && typeof entry.lastRunAt !== "string") {
    throw new Error(`workbench entry ${index} lastRunAt must be a string`);
  }
  return {
    id,
    kind,
    name,
    body,
    createdAt,
    updatedAt,
    ...(entry.lastRunAt ? { lastRunAt: entry.lastRunAt } : {}),
  };
}

function requiredString(value: unknown, index: number, field: string): string {
  if (typeof value !== "string" || !value) {
    throw new Error(`workbench entry ${index} ${field} is required`);
  }
  return value;
}
