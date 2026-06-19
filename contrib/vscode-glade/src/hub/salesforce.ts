import { GladeRunResult, runCommand } from "../gladeCli";
import { SalesforceTargetState } from "./model";

interface SalesforceOrgDisplay {
  result?: {
    alias?: string;
    username?: string;
    orgId?: string;
    instanceUrl?: string;
    connectedStatus?: string;
  };
}

interface SalesforceErrorDisplay {
  name?: string;
  code?: string;
  message?: string;
}

export function salesforceTargetStatusArgs(): string[] {
  return ["org", "display", "--json"];
}

export async function checkSalesforceTarget(cwd?: string): Promise<SalesforceTargetState> {
  return salesforceTargetStateFromRun(await runCommand("sf", salesforceTargetStatusArgs(), { cwd }));
}

export function salesforceTargetStateFromRun(result: GladeRunResult): SalesforceTargetState {
  if (result.code !== 0) {
    const error = parseSalesforceError(result.stdout) || parseSalesforceError(result.stderr);
    if (error) {
      return salesforceTargetStateFromError(error);
    }
    return {
      label: "no default target",
      state: "missing",
      detail: firstLine(result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`),
    };
  }
  try {
    return salesforceTargetStateFromDisplay(JSON.parse(result.stdout) as SalesforceOrgDisplay);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return { label: "default target", state: "unknown", detail: `invalid sf JSON: ${message}` };
  }
}

export function salesforceTargetStateFromDisplay(display: SalesforceOrgDisplay): SalesforceTargetState {
  const result = display.result || {};
  const label = result.alias || result.username || result.orgId || "default target";
  const status = result.connectedStatus || "unknown";
  if (status === "Connected") {
    return { label, state: "ready", detail: result.username || result.instanceUrl };
  }
  if (status !== "unknown") {
    return { label, state: "stale", detail: status };
  }
  return { label, state: "unknown", detail: result.username || "not checked" };
}

function parseSalesforceError(text: string): SalesforceErrorDisplay | undefined {
  const trimmed = text.trim();
  if (!trimmed.startsWith("{")) {
    return undefined;
  }
  try {
    const parsed = JSON.parse(trimmed) as SalesforceErrorDisplay;
    if (parsed && (parsed.message || parsed.name || parsed.code)) {
      return parsed;
    }
  } catch {
    return undefined;
  }
  return undefined;
}

function salesforceTargetStateFromError(error: SalesforceErrorDisplay): SalesforceTargetState {
  const marker = `${error.name || ""} ${error.code || ""} ${error.message || ""}`;
  if (/NoDefaultEnvError|No default environment|No default org/i.test(marker)) {
    return {
      label: "no default org",
      state: "missing",
      detail: "Set a default Salesforce org, then check again.",
    };
  }
  return {
    label: error.name || error.code || "target check failed",
    state: "missing",
    detail: firstLine(error.message || "sf org display failed"),
  };
}

function firstLine(value: string): string {
  return value.split(/\r?\n/, 1)[0] || value;
}
