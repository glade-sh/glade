import * as fs from "fs";
import * as path from "path";
import * as vscode from "vscode";
import { configuredActiveEnvironment } from "../localOrg";
import { GladeProjectContext } from "../projectModel";
import { runGlade } from "../gladeCli";
import { describeArgs, parseQueryOutput, queryArgs } from "./cli";
import {
  SoqlQueryResult,
  WorkbenchEntry,
  entryFromInput,
  sortEntries,
  workbenchTreeRows,
} from "./model";
import { readWorkbenchEntries, writeWorkbenchEntries } from "./store";

export class WorkbenchController {
  private project?: GladeProjectContext;
  private entries: WorkbenchEntry[] = [];
  private lastResultPath?: string;

  constructor(
    private readonly output: vscode.OutputChannel,
    private readonly onDidChange: (rows: ReturnType<typeof workbenchTreeRows>) => void,
  ) {}

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    this.reload();
  }

  reload(): void {
    if (!this.project) {
      this.entries = [];
      this.onDidChange([]);
      return;
    }
    try {
      this.entries = readWorkbenchEntries(this.project.projectRoot, "soql");
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.output.appendLine(`Glade Workbench store failed: ${message}`);
      this.entries = [];
    }
    this.emitRows();
  }

  async createEntry(kind: "soql", name: string, body: string): Promise<WorkbenchEntry | undefined> {
    const project = this.project;
    if (!project) {
      void vscode.window.showErrorMessage("Glade Workbench requires an SFDX project.");
      return undefined;
    }
    const now = new Date().toISOString();
    const entry = entryFromInput(kind, name, body, now);
    const next = [...this.entries.filter((existing) => existing.kind !== kind), ...readWorkbenchEntries(project.projectRoot, kind), entry];
    writeWorkbenchEntries(project.projectRoot, kind, sortEntries(next.filter((existing) => existing.kind === kind)));
    this.reload();
    return entry;
  }

  async runEntry(entryId?: string): Promise<void> {
    const entry = entryId ? this.entries.find((candidate) => candidate.id === entryId) : await this.pickEntry();
    if (!entry) {
      return;
    }
    await this.runSoql(entry);
  }

  async runLast(kind: "soql"): Promise<void> {
    const entry = sortEntries(this.entries.filter((candidate) => candidate.kind === kind))[0];
    if (!entry) {
      void vscode.window.showInformationMessage("No SOQL queries saved.");
      return;
    }
    await this.runEntry(entry.id);
  }

  async runSoqlText(queryText: string): Promise<void> {
    const query = queryText.trim();
    if (!query) {
      void vscode.window.showInformationMessage("Open a SOQL scratch buffer or select a SOQL query to run.");
      return;
    }
    await this.runSoqlQuery("Scratch", "scratch-soql", query, false);
  }

  async describe(objectName?: string): Promise<void> {
    const project = this.project;
    if (!project) {
      void vscode.window.showErrorMessage("Glade Workbench requires an SFDX project.");
      return;
    }
    const environment = configuredActiveEnvironment(project);
    const args = describeArgs(project.projectRoot, environment.dbPath, objectName);
    this.output.show(true);
    this.output.appendLine(`$ glade ${args.map(shellQuote).join(" ")}`);
    const result = await runGlade(args, { cwd: project.projectRoot });
    if (result.code !== 0) {
      throw new Error(result.stderr.trim() || result.stdout.trim() || `glade db describe exited with code ${result.code}`);
    }
    const filePath = this.writeResultFile("describe", result.stdout, "json");
    this.lastResultPath = filePath;
    await vscode.commands.executeCommand("vscode.open", vscode.Uri.file(filePath));
  }

  async openLastResult(): Promise<void> {
    if (!this.lastResultPath) {
      void vscode.window.showInformationMessage("No Glade Workbench result yet.");
      return;
    }
    await vscode.commands.executeCommand("vscode.open", vscode.Uri.file(this.lastResultPath));
  }

  private async runSoql(entry: WorkbenchEntry): Promise<void> {
    await this.runSoqlQuery(entry.name, entry.id, entry.body, true, entry);
  }

  private async runSoqlQuery(label: string, resultBaseName: string, queryText: string, markSaved: boolean, entry?: WorkbenchEntry): Promise<void> {
    const project = this.requireProject();
    const environment = configuredActiveEnvironment(project);
    const args = queryArgs(project.projectRoot, environment.dbPath, queryText);
    this.output.show(true);
    this.output.appendLine(`$ glade ${args.map(shellQuote).join(" ")}`);
    const result = await runGlade(args, { cwd: project.projectRoot });
    if (result.code !== 0) {
      throw new Error(result.stderr.trim() || result.stdout.trim() || `glade db query exited with code ${result.code}`);
    }
    const query = parseQueryOutput(result.stdout);
    this.output.appendLine(`SOQL ${label}: ${query.totalSize} row(s)`);
    const filePath = this.writeResultFile(resultBaseName, soqlResultText(query), "csv");
    this.lastResultPath = filePath;
    if (markSaved && entry) {
      this.markRun(entry);
    }
    await vscode.commands.executeCommand("vscode.open", vscode.Uri.file(filePath));
  }

  private async pickEntry(): Promise<WorkbenchEntry | undefined> {
    const picked = await vscode.window.showQuickPick(
      sortEntries(this.entries).map((entry) => ({
        label: entry.name,
        description: "SOQL",
        entry,
      })),
      { title: "Run Glade Workbench Entry" },
    );
    return picked?.entry;
  }

  private requireProject(): GladeProjectContext {
    if (!this.project) {
      throw new Error("Glade Workbench requires an SFDX project.");
    }
    return this.project;
  }

  private markRun(entry: WorkbenchEntry): void {
    const project = this.requireProject();
    const updated = { ...entry, lastRunAt: new Date().toISOString(), updatedAt: new Date().toISOString() };
    const sameKind = this.entries.map((candidate) => candidate.id === entry.id ? updated : candidate);
    writeWorkbenchEntries(project.projectRoot, entry.kind, sortEntries(sameKind));
    this.reload();
  }

  private emitRows(): void {
    const environment = this.project ? configuredActiveEnvironment(this.project).name : undefined;
    this.onDidChange(workbenchTreeRows(this.entries, environment));
  }

  private writeResultFile(baseName: string, body: string, ext: "csv" | "json"): string {
    const project = this.requireProject();
    const dir = path.join(project.projectRoot, ".glade", "workbench", "results");
    fs.mkdirSync(dir, { recursive: true });
    const filePath = path.join(dir, `${safeFileName(baseName)}-${Date.now()}.${ext}`);
    fs.writeFileSync(filePath, body.endsWith("\n") ? body : `${body}\n`, "utf8");
    return filePath;
  }
}

function soqlResultText(result: SoqlQueryResult): string {
  const columns = result.columns.length > 0 ? result.columns : columnsFromRecords(result.records);
  const lines = [columns.map(csvCell).join(",")];
  for (const record of result.records) {
    lines.push(columns.map((column) => csvCell(record[column])).join(","));
  }
  return `${lines.join("\n")}\n`;
}

function columnsFromRecords(records: Array<Record<string, unknown>>): string[] {
  const seen = new Set<string>();
  const columns: string[] = [];
  for (const record of records) {
    for (const key of Object.keys(record)) {
      if (key === "attributes" || seen.has(key)) {
        continue;
      }
      seen.add(key);
      columns.push(key);
    }
  }
  return columns;
}

function csvCell(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  const text = typeof value === "object" ? JSON.stringify(value) : String(value);
  return /[",\n\r]/.test(text) ? `"${text.replace(/"/g, "\"\"")}"` : text;
}

function safeFileName(value: string): string {
  return value.replace(/[^A-Za-z0-9_.-]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 64) || "result";
}

function shellQuote(value: string): string {
  if (/^[A-Za-z0-9_./:=+-]+$/.test(value)) {
    return value;
  }
  return `'${value.replace(/'/g, "'\\''")}'`;
}
