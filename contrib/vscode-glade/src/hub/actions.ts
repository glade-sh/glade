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

export type HubClientMessage = HubReadyMessage | HubRunCommandMessage | HubSelectTabMessage;

export const allowedHubCommands = new Set<string>([
  "vscode.openFolder",
  "glade.refresh",
  "glade.runChangedTests",
  "glade.runFailedTests",
  "glade.runLocalProof",
  "glade.createProjectOrg",
  "glade.startProjectOrg",
  "glade.stopProjectOrg",
  "glade.projectOrgStatus",
  "glade.startWatch",
  "glade.stopWatch",
  "glade.createEnvironment",
  "glade.switchEnvironment",
  "glade.cloneEnvironment",
  "glade.seedLocalOrg",
  "glade.resetLocalOrg",
  "glade.exportLocalOrg",
  "glade.inspectLocalOrg",
  "glade.debugCurrentTest",
  "glade.openOutput",
  "glade.workbench.newAnonymousApex",
  "glade.workbench.newSoql",
  "glade.managePlugins",
  "glade.runPluginAction",
  "glade.schemaImportDescribe",
  "glade.salesforceTargetStatus",
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

  if (record.type === "runCommand" && typeof record.command === "string") {
    if (!isHubCommand(record.command)) {
      throw new Error(`hub command is not allowed: ${record.command}`);
    }
    return { type: "runCommand", command: record.command };
  }

  throw new Error("unsupported hub message");
}
