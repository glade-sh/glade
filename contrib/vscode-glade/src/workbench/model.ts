export type WorkbenchEntryKind = "soql";

export interface WorkbenchEntry {
  id: string;
  kind: WorkbenchEntryKind;
  name: string;
  body: string;
  createdAt: string;
  updatedAt: string;
  lastRunAt?: string;
}

export interface WorkbenchTreeRow {
  id: string;
  type: "environment" | "group" | "entry";
  label: string;
  description?: string;
  kind?: WorkbenchEntryKind;
  count?: number;
  entryId?: string;
}

export interface SoqlQueryResult {
  totalSize: number;
  done: boolean;
  records: Array<Record<string, unknown>>;
  columns: string[];
}

const labels: Record<WorkbenchEntryKind, string> = {
  soql: "SOQL",
};
const savedWorkbenchKinds: WorkbenchEntryKind[] = ["soql"];

export function isWorkbenchEntryKind(kind: string): kind is WorkbenchEntryKind {
  return kind === "soql";
}

export function assertWorkbenchEntryKind(kind: string): asserts kind is WorkbenchEntryKind {
  if (!isWorkbenchEntryKind(kind)) {
    throw new Error(`unsupported workbench entry kind: ${kind}`);
  }
}

export function entryFromInput(kind: string, name: string, body: string, now: string, id?: string): WorkbenchEntry {
  assertWorkbenchEntryKind(kind);
  const trimmedName = name.trim();
  const trimmedBody = body.trim();
  if (!trimmedName) {
    throw new Error("workbench entry name is required");
  }
  if (!trimmedBody) {
    throw new Error("workbench entry body is required");
  }
  return {
    id: id?.trim() || generatedEntryId(kind, trimmedName, now),
    kind,
    name: trimmedName,
    body: trimmedBody,
    createdAt: now,
    updatedAt: now,
  };
}

export function sortEntries(entries: WorkbenchEntry[]): WorkbenchEntry[] {
  return [...entries].sort((a, b) => {
    const aRun = timestamp(a.lastRunAt);
    const bRun = timestamp(b.lastRunAt);
    if (aRun !== bRun) {
      return bRun - aRun;
    }
    const aUpdated = timestamp(a.updatedAt);
    const bUpdated = timestamp(b.updatedAt);
    if (aUpdated !== bUpdated) {
      return bUpdated - aUpdated;
    }
    return a.name.localeCompare(b.name);
  });
}

export function workbenchTreeRows(entries: WorkbenchEntry[], activeEnvironmentName?: string): WorkbenchTreeRow[] {
  const rows: WorkbenchTreeRow[] = [];
  if (activeEnvironmentName && activeEnvironmentName.trim()) {
    rows.push({
      id: "environment",
      type: "environment",
      label: "Environment",
      description: activeEnvironmentName.trim(),
    });
  }
  for (const kind of savedWorkbenchKinds) {
    const groupEntries = sortEntries(entries.filter((entry) => entry.kind === kind));
    rows.push({
      id: kind,
      type: "group",
      kind,
      label: labels[kind],
      count: groupEntries.length,
    });
    for (const entry of groupEntries) {
      rows.push({
        id: `${kind}:${entry.id}`,
        type: "entry",
        kind,
        entryId: entry.id,
        label: entry.name,
        description: entry.lastRunAt ? `Last run ${entry.lastRunAt}` : `Updated ${entry.updatedAt}`,
      });
    }
  }
  return rows;
}

export function parseSoqlJsonResult(stdout: string): SoqlQueryResult {
  const parsed = parseJSONRecord(stdout, "invalid SOQL JSON");
  const rawRecords = parsed.records;
  if (!Array.isArray(rawRecords)) {
    throw new Error("SOQL JSON records must be an array");
  }
  const records = rawRecords.map((record, index) => recordValue(record, `SOQL JSON records[${index}] must be an object`));
  return {
    totalSize: numberValue(parsed.totalSize, records.length),
    done: booleanValue(parsed.done, true),
    records,
    columns: columnsFromResult(parsed, records),
  };
}

function generatedEntryId(kind: WorkbenchEntryKind, name: string, now: string): string {
  const slug = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48);
  const stamp = now.replace(/[^0-9a-z]+/gi, "").toLowerCase();
  return [kind, stamp, slug || "entry"].join("-");
}

function timestamp(value: string | undefined): number {
  if (!value) {
    return 0;
  }
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function parseJSONRecord(stdout: string, message: string): Record<string, unknown> {
  try {
    return recordValue(JSON.parse(stdout), message);
  } catch (error) {
    if (error instanceof SyntaxError) {
      throw new Error(`${message}: ${error.message}`);
    }
    throw error;
  }
}

function recordValue(value: unknown, message: string): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(message);
  }
  return value as Record<string, unknown>;
}

function numberValue(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function booleanValue(value: unknown, fallback: boolean): boolean {
  return typeof value === "boolean" ? value : fallback;
}

function columnsFromResult(parsed: Record<string, unknown>, records: Array<Record<string, unknown>>): string[] {
  if (Array.isArray(parsed.columns)) {
    return parsed.columns.filter((column): column is string => typeof column === "string");
  }
  const columns: string[] = [];
  const seen = new Set<string>();
  for (const record of records) {
    for (const column of Object.keys(record)) {
      if (column === "attributes" || seen.has(column)) {
        continue;
      }
      seen.add(column);
      columns.push(column);
    }
  }
  return columns;
}
