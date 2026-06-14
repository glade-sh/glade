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
  toolchainReady?: boolean;
  toolchainDetail?: string;
  lwcRouteCount?: number;
  vfRouteCount?: number;
  pluginActionCount?: number;
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
    toolchainRow(snapshot),
    {
      id: "lwc-preview",
      label: "LWC preview",
      description:
        snapshot.lwcRouteCount && snapshot.lwcRouteCount > 0 ? `${snapshot.lwcRouteCount} ${plural("route", snapshot.lwcRouteCount)}` : "stopped",
      tooltip:
        snapshot.lwcRouteCount && snapshot.lwcRouteCount > 0
          ? `${snapshot.lwcRouteCount} LWC preview ${plural("route", snapshot.lwcRouteCount)} discovered.`
          : "LWC local preview is stopped.",
      contextValue: "gladeStartHereStatus",
    },
    {
      id: "vf-preview",
      label: "Visualforce preview",
      description:
        snapshot.vfRouteCount && snapshot.vfRouteCount > 0 ? `${snapshot.vfRouteCount} ${plural("page", snapshot.vfRouteCount)}` : "stopped",
      tooltip:
        snapshot.vfRouteCount && snapshot.vfRouteCount > 0
          ? `${snapshot.vfRouteCount} Visualforce preview ${plural("page", snapshot.vfRouteCount)} discovered.`
          : "Visualforce local preview is stopped.",
      contextValue: "gladeStartHereStatus",
    },
    {
      id: "plugin-actions",
      label: "Plugin actions",
      description:
        snapshot.pluginActionCount && snapshot.pluginActionCount > 0
          ? `${snapshot.pluginActionCount} ${plural("finding", snapshot.pluginActionCount)}`
          : "absent",
      tooltip:
        snapshot.pluginActionCount && snapshot.pluginActionCount > 0
          ? `${snapshot.pluginActionCount} plugin ${plural("finding", snapshot.pluginActionCount)} ready.`
          : "No plugin actions are available.",
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

function toolchainRow(snapshot: StartHereSnapshot): StartHereRow {
  if (snapshot.toolchainReady === true) {
    return {
      id: "toolchain",
      label: "Toolchain ready",
      description: snapshot.toolchainDetail || "local preview ready",
      tooltip: snapshot.toolchainDetail || "The local preview toolchain is ready.",
      contextValue: "gladeStartHereStatus",
    };
  }
  if (snapshot.toolchainReady === false) {
    return {
      id: "toolchain",
      label: "Toolchain install required",
      description: snapshot.toolchainDetail || "run install",
      tooltip: snapshot.toolchainDetail || "Run Glade toolchain install before local preview.",
      contextValue: "gladeStartHereStatus",
    };
  }
  return {
    id: "toolchain",
    label: "Toolchain unknown",
    description: snapshot.toolchainDetail || "not checked",
    tooltip: snapshot.toolchainDetail || "Toolchain status has not been checked.",
    contextValue: "gladeStartHereStatus",
  };
}

function shortPath(file: string): string {
  const parts = file.split(/[\\/]+/).filter(Boolean);
  return parts[parts.length - 1] || file;
}

function plural(word: string, count: number): string {
  return count === 1 ? word : `${word}s`;
}
