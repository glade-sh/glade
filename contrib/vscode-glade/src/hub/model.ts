import type { GladeEnvironment } from "../environments";
import type { LocalOrgSummary } from "../localOrgModel";
import type { GladeProjectContext } from "../projectModel";
import type { StartHereRunSummary } from "../startHereModel";

export type HubTone = "ok" | "warn" | "error" | "muted";
export type HubTaskId = "data" | "run" | "org" | "scratch" | "salesforce";
export type HubStateId = "project" | "local-org" | "data" | "salesforce" | "tests" | "plugins";

export interface SalesforceTargetState {
  label: string;
  state: "ready" | "stale" | "missing" | "unknown";
  detail?: string;
}

export interface ProjectOrgState {
  alias: string;
  state: "running" | "stopped" | "missing" | "unknown";
  detail?: string;
}

export interface HubSnapshot {
  project?: GladeProjectContext;
  activeEnvironment?: GladeEnvironment;
  localOrgSummary?: LocalOrgSummary;
  projectOrg?: ProjectOrgState;
  projectOrgAlias?: string;
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
        id: "org",
        title: "Open project",
        summary: "Open a Salesforce DX project before using Glade.",
        status: { label: "No Salesforce DX project", tone: "warn" },
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
  const org = projectOrg(snapshot);
  const target = salesforceTarget(snapshot);

  return [
    {
      id: "data",
      title: "Data browser",
      summary: "Inspect the active SQLite data environment and manage local data.",
      status: dataStatusFor(snapshot),
      primary: { id: "inspect-db", label: "Inspect data", command: "glade.inspectLocalOrg", primary: true },
      actions: [
        { id: "switch-env", label: "Switch environment", command: "glade.switchEnvironment" },
        { id: "seed", label: "Seed data", command: "glade.seedLocalOrg" },
        { id: "reset", label: "Reset data", command: "glade.resetLocalOrg" },
        { id: "export", label: "Export data", command: "glade.exportLocalOrg" },
        ...(snapshot.missingDb ? [{ id: "create-env", label: "Create environment", command: "glade.createEnvironment" }] : []),
      ],
    },
    {
      id: "run",
      title: "Local tests",
      summary: "Run changed Apex tests and control the local watch loop.",
      status: {
        label: lastRun ? `${lastRun.passed} pass, ${lastRun.failed} fail` : "No local run",
        detail: `${snapshot.watchRunning ? "watch running" : "watch stopped"}; changed since ${snapshot.changedSince}`,
        tone: failed > 0 ? "error" : lastRun ? "ok" : "muted",
      },
      primary: { id: "run-proof", label: "Run changed tests", command: "glade.runLocalProof", primary: true },
      actions: [
        ...(failed > 0 ? [{ id: "failed", label: "Failed tests", command: "glade.runFailedTests" }] : []),
        {
          id: "watch",
          label: snapshot.watchRunning ? "Stop watch" : "Start watch",
          command: snapshot.watchRunning ? "glade.stopWatch" : "glade.startWatch",
        },
      ],
    },
    {
      id: "org",
      title: "Glade org",
      summary: "Start or check the local Salesforce-shaped API.",
      status: { label: `${org.alias} ${org.state}`, detail: org.detail, tone: projectOrgTone(org) },
      primary:
        org.state === "missing"
          ? { id: "create-org", label: "Create org", command: "glade.createProjectOrg", primary: true }
          : org.state === "running"
          ? { id: "stop-org", label: "Stop org", command: "glade.stopProjectOrg", primary: true }
          : { id: "start-org", label: "Start org", command: "glade.startProjectOrg", primary: true },
      actions: [
        { id: "org-state", label: "Check org state", command: "glade.projectOrgStatus" },
      ],
    },
    {
      id: "scratch",
      title: "Scratch editors",
      summary: "Open untitled Apex and SOQL editors for quick local work.",
      status: { label: "Editor scoped", detail: "uses the active project", tone: "muted" },
      primary: { id: "apex", label: "Anonymous Apex", command: "glade.workbench.newAnonymousApex", primary: true },
      actions: [
        { id: "soql", label: "SOQL", command: "glade.workbench.newSoql" },
      ],
    },
    {
      id: "salesforce",
      title: "Salesforce",
      summary: "Check the default org and import describe data.",
      status: { label: target.label, detail: target.detail, tone: salesforceTone(target) },
      primary: { id: "sf-target", label: "Check Salesforce org", command: "glade.salesforceTargetStatus", primary: true },
      actions: [
        { id: "schema", label: "Import schema", command: "glade.schemaImportDescribe" },
      ],
    },
  ];
}

export function buildHubState(snapshot: HubSnapshot): HubStateSection[] {
  const project = snapshot.project;
  const env = snapshot.activeEnvironment;
  const summary = snapshot.localOrgSummary;
  const target = salesforceTarget(snapshot);
  const org = projectOrg(snapshot);

  return [
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
      id: "local-org",
      title: "Glade org",
      tone: projectOrgTone(org),
      rows: [
        { label: "Alias", value: org.alias },
        { label: "State", value: org.state, detail: org.detail },
        { label: "Project config", value: project?.configFound ? "loaded" : "defaults" },
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
      id: "salesforce",
      title: "Salesforce",
      tone: salesforceTone(target),
      rows: [
        { label: "Target", value: target.label },
        { label: "State", value: target.state, detail: target.detail },
      ],
    },
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
      id: "plugins",
      title: "Plugins",
      tone: snapshot.pluginFindingCount ? "warn" : "muted",
      rows: [
        { label: "Actions", value: String(snapshot.pluginActionCount || 0) },
        { label: "Reports", value: String(snapshot.pluginFindingCount || 0) },
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

function projectOrg(snapshot: HubSnapshot): ProjectOrgState {
  return snapshot.projectOrg || { alias: snapshot.projectOrgAlias || "my-glade-org", state: "unknown", detail: "not checked" };
}

function projectOrgTone(org: ProjectOrgState): HubTone {
  if (org.state === "running") {
    return "ok";
  }
  if (org.state === "missing") {
    return "error";
  }
  if (org.state === "stopped") {
    return "warn";
  }
  return "muted";
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
