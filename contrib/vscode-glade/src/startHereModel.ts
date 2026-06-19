import { GladeEnvironment } from "./environments";
import { LocalOrgSummary } from "./localOrgModel";
import { GladeProjectContext } from "./projectModel";

export interface StartHereRunSummary {
  label: string;
  passed: number;
  failed: number;
  durationMs?: number;
}

export interface StartHereSnapshot {
  project?: GladeProjectContext;
  activeEnvironment?: GladeEnvironment;
  localOrgSummary?: LocalOrgSummary;
  missingDb?: boolean;
  watchRunning?: boolean;
  lastRun?: StartHereRunSummary;
  changedSince: string;
}

export interface StartHereRow {
  id: string;
  label: string;
  description?: string;
  tooltip?: string;
  command?: string;
  contextValue?: string;
}

export function buildStartHereRows(snapshot: StartHereSnapshot): StartHereRow[] {
  if (!snapshot.project) {
    return [
      {
        id: "open-project",
        label: "Open a Salesforce DX project",
        description: "sfdx-project.json required",
        tooltip: "Open a folder containing sfdx-project.json.",
        command: "vscode.openFolder",
        contextValue: "gladeStartHereAction",
      },
    ];
  }

  const project = snapshot.project;
  const environment = snapshot.activeEnvironment;
  const summary = snapshot.localOrgSummary;
  const records = snapshot.missingDb ? "no DB" : summary ? `${summary.records} records` : "not inspected";
  const lastRun = snapshot.lastRun;
  const rows: StartHereRow[] = [
    {
      id: "home",
      label: "Glade Home",
      description: "daily developer hub",
      tooltip: "Open the task-first Glade Home webview.",
      command: "glade.openHome",
      contextValue: "gladeStartHereAction",
    },
    {
      id: "environment",
      label: "Data environment",
      description: `${environment?.name || "dev"} - ${records}`,
      tooltip: [
        `${project.configFound ? "Project config loaded" : "Using project defaults"} at ${project.projectRoot}.`,
        `API ${project.sourceApiVersion || "unknown"}.`,
        environment?.dbPath ? `DB ${environment.dbPath}.` : "No active local DB path.",
      ].join("\n"),
      command: "glade.inspectLocalOrg",
      contextValue: "gladeStartHereAction",
    },
    {
      id: "local-proof",
      label: "Run changed tests",
      description: `changed since ${snapshot.changedSince}`,
      tooltip: "Run changed local Apex tests, inspect the active DB, and update this panel.",
      command: "glade.runLocalProof",
      contextValue: "gladeStartHereAction",
    },
  ];
  if (lastRun) {
    rows.push({
      id: "last-run",
      label: lastRun.label,
      description: `${lastRun.passed} passed, ${lastRun.failed} failed`,
      tooltip: `Last local run: ${lastRun.passed} passed, ${lastRun.failed} failed.`,
      contextValue: "gladeStartHereStatus",
    });
  }
  if (snapshot.watchRunning) {
    rows.push({
      id: "watch",
      label: "Watch running",
      description: "click to stop",
      tooltip: "Stop the local Apex watch loop.",
      command: "glade.stopWatch",
      contextValue: "gladeStartHereAction",
    });
  }
  return rows;
}
