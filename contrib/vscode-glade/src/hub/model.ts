import type { GladeEnvironment } from "../environments";
import type { LocalOrgSummary } from "../localOrgModel";
import type { GladeProjectContext } from "../projectModel";
import type { StartHereRunSummary } from "../startHereModel";

export type HubTone = "ok" | "warn" | "error" | "muted";
export type HubTaskId = "run" | "data" | "debug" | "salesforce" | "ship";
export type HubStateId = "project" | "local-org" | "data" | "salesforce" | "tests" | "plugins";

export interface SalesforceTargetState {
  label: string;
  state: "ready" | "stale" | "missing" | "unknown";
  detail?: string;
}

export interface HubSnapshot {
  project?: GladeProjectContext;
  activeEnvironment?: GladeEnvironment;
  localOrgSummary?: LocalOrgSummary;
  missingDb?: boolean;
  watchRunning?: boolean;
  lastRun?: StartHereRunSummary;
  changedSince: string;
  pluginActionCount?: number;
  pluginFindingCount?: number;
  salesforceTarget?: SalesforceTargetState;
}

export interface HubStatus {
  label: string;
  detail?: string;
  tone: HubTone;
}

export interface HubAction {
  id: string;
  label: string;
  command: string;
  description?: string;
  primary?: boolean;
  disabledReason?: string;
}

export interface HubTaskGroup {
  id: HubTaskId;
  title: string;
  summary: string;
  status: HubStatus;
  primary: HubAction;
  actions: HubAction[];
}

export interface HubStateRow {
  label: string;
  value: string;
  detail?: string;
}

export interface HubStateSection {
  id: HubStateId;
  title: string;
  tone: HubTone;
  rows: HubStateRow[];
}

export function buildHubHome(snapshot: HubSnapshot): HubTaskGroup[] {
  if (!snapshot.project) {
    return [
      {
        id: "run",
        title: "Open project",
        summary: "Open a Salesforce DX project before using Glade.",
        status: { label: "No SFDX project", tone: "warn" },
        primary: {
          id: "open-project",
          label: "Open folder",
          command: "vscode.openFolder",
          primary: true,
        },
        actions: [],
      },
    ];
  }

  const lastRun = snapshot.lastRun;
  const failed = lastRun?.failed || 0;
  const target = salesforceTarget(snapshot);

  return [
    {
      id: "run",
      title: "Run",
      summary: "Run the local Apex loop from the current branch.",
      status: {
        label: lastRun ? `${lastRun.passed} pass, ${lastRun.failed} fail` : "No local run",
        detail: `changed since ${snapshot.changedSince}`,
        tone: failed > 0 ? "error" : lastRun ? "ok" : "muted",
      },
      primary: { id: "run-proof", label: "Run proof", command: "glade.runLocalProof", primary: true },
      actions: [
        { id: "changed", label: "Changed tests", command: "glade.runChangedTests" },
        { id: "failed", label: "Failed tests", command: "glade.runFailedTests" },
        {
          id: "watch",
          label: snapshot.watchRunning ? "Stop watch" : "Start watch",
          command: snapshot.watchRunning ? "glade.stopWatch" : "glade.startWatch",
        },
      ],
    },
    {
      id: "data",
      title: "Data",
      summary: "Work with the active local data environment.",
      status: dataStatusFor(snapshot),
      primary: { id: "inspect-db", label: "Inspect DB", command: "glade.inspectLocalOrg", primary: true },
      actions: [
        { id: "switch-env", label: "Switch env", command: "glade.switchEnvironment" },
        { id: "create-env", label: "Create env", command: "glade.createEnvironment" },
        { id: "clone-env", label: "Clone env", command: "glade.cloneEnvironment" },
        { id: "seed", label: "Seed", command: "glade.seedLocalOrg" },
        { id: "reset", label: "Reset", command: "glade.resetLocalOrg" },
        { id: "export", label: "Export", command: "glade.exportLocalOrg" },
        { id: "soql", label: "SOQL scratch", command: "glade.workbench.newSoql" },
        { id: "apex", label: "Apex scratch", command: "glade.workbench.newAnonymousApex" },
      ],
    },
    {
      id: "debug",
      title: "Debug",
      summary: "Start local debug work from the current editor.",
      status: { label: "Editor scoped", detail: "uses active Apex context", tone: "muted" },
      primary: { id: "debug-current", label: "Debug current test", command: "glade.debugCurrentTest", primary: true },
      actions: [
        { id: "apex-scratch", label: "Apex scratch", command: "glade.workbench.newAnonymousApex" },
        { id: "output", label: "Open output", command: "glade.openOutput" },
      ],
    },
    {
      id: "salesforce",
      title: "Salesforce",
      summary: "Check the org-backed target and import describe data.",
      status: { label: target.label, detail: target.detail, tone: salesforceTone(target) },
      primary: { id: "sf-target", label: "Check target", command: "glade.salesforceTargetStatus", primary: true },
      actions: [
        { id: "schema", label: "Import schema", command: "glade.schemaImportDescribe" },
        {
          id: "capture",
          label: "Capture fixture",
          command: "glade.runPluginAction",
          description: "Runs a plugin action when installed.",
        },
        { id: "plugins", label: "Manage plugins", command: "glade.managePlugins" },
      ],
    },
    {
      id: "ship",
      title: "Ship",
      summary: "Gather local proof and findings before pushing.",
      status: {
        label: shipLabel(snapshot),
        detail: lastRun ? lastRun.label : "run proof first",
        tone: failed > 0 ? "warn" : lastRun ? "ok" : "muted",
      },
      primary: { id: "ship-proof", label: "Run proof", command: "glade.runLocalProof", primary: true },
      actions: [
        { id: "output", label: "Open output", command: "glade.openOutput" },
        { id: "plugins", label: "Plugin findings", command: "glade.managePlugins" },
        { id: "refresh", label: "Refresh", command: "glade.refresh" },
      ],
    },
  ];
}

export function buildHubState(snapshot: HubSnapshot): HubStateSection[] {
  const project = snapshot.project;
  const env = snapshot.activeEnvironment;
  const summary = snapshot.localOrgSummary;
  const target = salesforceTarget(snapshot);

  return [
    {
      id: "project",
      title: "Project",
      tone: project ? "ok" : "warn",
      rows: [
        { label: "Root", value: project?.projectRoot || "none" },
        { label: "API", value: project?.sourceApiVersion || "unknown" },
        { label: "Package dirs", value: project?.packageDirs?.join(", ") || "none" },
      ],
    },
    {
      id: "local-org",
      title: "Glade org",
      tone: project ? "ok" : "muted",
      rows: [
        { label: "Endpoint", value: "127.0.0.1:17911", detail: "default local org port" },
        { label: "Project config", value: project?.configFound ? "loaded" : "defaults" },
      ],
    },
    {
      id: "data",
      title: "Data environment",
      tone: snapshot.missingDb ? "warn" : env ? "ok" : "muted",
      rows: [
        { label: "Active", value: env?.name || "dev" },
        { label: "DB", value: env?.dbPath || "not configured" },
        { label: "Records", value: snapshot.missingDb ? "no DB" : String(summary?.records || 0) },
        { label: "Objects", value: snapshot.missingDb ? "no DB" : String(summary?.objects || 0) },
      ],
    },
    {
      id: "salesforce",
      title: "Salesforce target",
      tone: salesforceTone(target),
      rows: [
        { label: "Target", value: target.label },
        { label: "State", value: target.state, detail: target.detail },
      ],
    },
    {
      id: "tests",
      title: "Tests",
      tone: snapshot.lastRun?.failed ? "error" : snapshot.lastRun ? "ok" : "muted",
      rows: [
        { label: "Watch", value: snapshot.watchRunning ? "running" : "stopped" },
        { label: "Changed since", value: snapshot.changedSince },
        {
          label: "Last run",
          value: snapshot.lastRun ? `${snapshot.lastRun.passed} pass, ${snapshot.lastRun.failed} fail` : "none",
        },
      ],
    },
    {
      id: "plugins",
      title: "Plugins",
      tone: snapshot.pluginFindingCount ? "warn" : "muted",
      rows: [
        { label: "Actions", value: String(snapshot.pluginActionCount || 0) },
        { label: "Findings", value: String(snapshot.pluginFindingCount || 0) },
      ],
    },
  ];
}

function dataStatusFor(snapshot: HubSnapshot): HubStatus {
  const env = snapshot.activeEnvironment;
  if (snapshot.missingDb) {
    return { label: `${env?.name || "dev"} has no DB`, detail: env?.dbPath, tone: "warn" };
  }
  if (snapshot.localOrgSummary) {
    return {
      label: `${snapshot.localOrgSummary.records} records`,
      detail: env?.name || "dev",
      tone: "ok",
    };
  }
  return { label: env?.name || "dev", detail: "not inspected", tone: "muted" };
}

function salesforceTarget(snapshot: HubSnapshot): SalesforceTargetState {
  return snapshot.salesforceTarget || { label: "default target", state: "unknown", detail: "not checked" };
}

function salesforceTone(target: SalesforceTargetState): HubTone {
  if (target.state === "ready") {
    return "ok";
  }
  if (target.state === "missing") {
    return "error";
  }
  if (target.state === "stale") {
    return "warn";
  }
  return "muted";
}

function shipLabel(snapshot: HubSnapshot): string {
  const findings = snapshot.pluginFindingCount || 0;
  if (findings > 0) {
    return `${findings} findings`;
  }
  return "No plugin findings";
}
