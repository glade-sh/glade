export interface HubReadyMessage {
  type: "ready";
}

export interface HubRunCommandMessage {
  type: "runCommand";
  command: string;
}

export interface HubSelectTabMessage {
  type: "selectTab";
  tab: "home" | "state";
}

export interface HubSelectLaneMessage {
  type: "selectLane";
  scope: "home" | "state";
  lane: string;
}

export type HubClientMessage = HubReadyMessage | HubRunCommandMessage | HubSelectTabMessage | HubSelectLaneMessage;

export const allowedHubCommands = new Set<string>([
  "vscode.openFolder",
  "glade.runLocalProof",
  "glade.runFailedTests",
  "glade.startWatch",
  "glade.stopWatch",
  "glade.createProjectOrg",
  "glade.startProjectOrg",
  "glade.stopProjectOrg",
  "glade.projectOrgStatus",
  "glade.salesforceTargetStatus",
  "glade.schemaImportDescribe",
  "glade.createEnvironment",
  "glade.switchEnvironment",
  "glade.inspectLocalOrg",
  "glade.seedLocalOrg",
  "glade.resetLocalOrg",
  "glade.exportLocalOrg",
  "glade.workbench.newAnonymousApex",
  "glade.workbench.newSoql",
]);

export function isHubCommand(command: string): boolean {
  return allowedHubCommands.has(command);
}

export function parseHubMessage(value: unknown): HubClientMessage {
  if (!value || typeof value !== "object") {
    throw new Error("unsupported hub message");
  }

  const record = value as Record<string, unknown>;
  if (record.type === "ready") {
    return { type: "ready" };
  }

  if (record.type === "selectTab" && (record.tab === "home" || record.tab === "state")) {
    return { type: "selectTab", tab: record.tab };
  }

  if (
    record.type === "selectLane" &&
    (record.scope === "home" || record.scope === "state") &&
    typeof record.lane === "string" &&
    /^[a-z][a-z0-9-]*$/.test(record.lane)
  ) {
    return { type: "selectLane", scope: record.scope, lane: record.lane };
  }

  if (record.type === "runCommand" && typeof record.command === "string") {
    if (!isHubCommand(record.command)) {
      throw new Error(`hub command is not allowed: ${record.command}`);
    }
    return { type: "runCommand", command: record.command };
  }

  throw new Error("unsupported hub message");
}
