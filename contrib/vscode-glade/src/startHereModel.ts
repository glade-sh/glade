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
        label: "Open an SFDX project",
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
  return [
    {
      id: "ready",
      label: "Ready for local Apex",
      description: project.configFound ? "project config loaded" : "using SFDX defaults",
      tooltip: project.projectRoot,
      contextValue: "gladeStartHereStatus",
    },
    {
      id: "project",
      label: shortPath(project.projectRoot),
      description: `API ${project.sourceApiVersion || "unknown"}`,
      tooltip: project.projectRoot,
      contextValue: "gladeStartHereStatus",
    },
    {
      id: "environment",
      label: `Data env: ${environment?.name || "dev"}`,
      description: records,
      tooltip: environment?.dbPath || "No active local DB path.",
      command: "glade.inspectLocalOrg",
      contextValue: "gladeStartHereAction",
    },
    {
      id: "local-proof",
      label: "Run local proof",
      description: `changed since ${snapshot.changedSince}`,
      tooltip: "Run changed local Apex tests, inspect the active DB, and update this panel.",
      command: "glade.runLocalProof",
      contextValue: "gladeStartHereAction",
    },
    {
      id: "last-run",
      label: lastRun ? lastRun.label : "No local run yet",
      description: lastRun ? `${lastRun.passed} passed, ${lastRun.failed} failed` : "run local proof",
      tooltip: lastRun
        ? `Last local run: ${lastRun.passed} passed, ${lastRun.failed} failed.`
        : "No local test run has been recorded in this window.",
      contextValue: "gladeStartHereStatus",
    },
    {
      id: "watch",
      label: snapshot.watchRunning ? "Watch running" : "Watch stopped",
      description: snapshot.watchRunning ? "local daemon active" : "click to start",
      tooltip: "Start or stop the local Apex watch loop.",
      command: snapshot.watchRunning ? "glade.stopWatch" : "glade.startWatch",
      contextValue: "gladeStartHereAction",
    },
  ];
}

function shortPath(file: string): string {
  const parts = file.split(/[\\/]+/).filter(Boolean);
  return parts[parts.length - 1] || file;
}
