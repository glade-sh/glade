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

export function salesforceTargetStatusArgs(): string[] {
  return ["org", "display", "--json"];
}

export async function checkSalesforceTarget(cwd?: string): Promise<SalesforceTargetState> {
  return salesforceTargetStateFromRun(await runCommand("sf", salesforceTargetStatusArgs(), { cwd }));
}

export function salesforceTargetStateFromRun(result: GladeRunResult): SalesforceTargetState {
  if (result.code !== 0) {
    return {
      label: "no default target",
      state: "missing",
      detail: result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`,
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
