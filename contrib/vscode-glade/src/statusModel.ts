export interface StatusRunSummary {
  failed: number;
  durationMs?: number;
}

export interface GladeStatusSnapshot {
  projectReady: boolean;
  projectRoot?: string;
  activeEnvironment?: string;
  dbPath?: string;
  lastRun?: StatusRunSummary;
  changedRecords?: number;
  missingDb?: boolean;
  busyLabel?: string;
  lastCommand?: string;
}

export function buildStatusText(snapshot: GladeStatusSnapshot): string {
  if (!snapshot.projectReady) {
    return "Glade: no SFDX root";
  }
  if (snapshot.busyLabel) {
    return `Glade: ${snapshot.busyLabel}`;
  }
  const environment = snapshot.activeEnvironment || "dev";
  if (snapshot.missingDb) {
    return `Glade: ${environment} no DB`;
  }
  if (snapshot.lastRun && snapshot.lastRun.failed > 0) {
    return `Glade: ${environment} ${snapshot.lastRun.failed} fail`;
  }
  if (snapshot.lastRun && snapshot.lastRun.durationMs !== undefined) {
    return `Glade: ${environment} ${snapshot.lastRun.durationMs}ms`;
  }
  if (snapshot.changedRecords && snapshot.changedRecords > 0) {
    return `Glade: ${environment} ${snapshot.changedRecords} changed`;
  }
  return `Glade: ${environment}`;
}

export function buildStatusTooltip(snapshot: GladeStatusSnapshot): string {
  if (!snapshot.projectReady) {
    return "Open a Salesforce DX project with sfdx-project.json.";
  }
  const lines = [
    snapshot.projectRoot ? `Project: ${snapshot.projectRoot}` : undefined,
    `Environment: ${snapshot.activeEnvironment || "dev"}`,
    snapshot.dbPath ? `DB: ${snapshot.dbPath}` : undefined,
    snapshot.lastCommand ? `Last command: ${snapshot.lastCommand}` : undefined,
  ].filter((line): line is string => Boolean(line));
  return lines.join("\n");
}
