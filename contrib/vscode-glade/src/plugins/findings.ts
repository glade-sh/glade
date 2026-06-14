import {
  GladeFindingsV1,
  PluginArtifact,
  PluginFinding,
  PluginFindingSeverity,
} from "./model";

const normalizedSeverities = new Set<PluginFindingSeverity>(["error", "warning", "info", "hint"]);

export function parseFindingsOutput(stdout: string): GladeFindingsV1 {
  const parsed = JSON.parse(stdout) as unknown;
  if (!isRecord(parsed)) {
    throw new Error("glade findings output must be a JSON object");
  }
  if (parsed.kind !== "glade.findings.v1") {
    throw new Error("glade findings output must have kind glade.findings.v1");
  }

  return {
    kind: "glade.findings.v1",
    summary: parsed.summary,
    findings: arrayValue(parsed.findings).map(normalizeFinding),
    artifacts: arrayValue(parsed.artifacts).map(normalizeArtifact),
  };
}

export function normalizeSeverity(severity: unknown): PluginFindingSeverity {
  if (typeof severity !== "string") {
    return "warning";
  }
  const normalized = severity.toLowerCase();
  if (normalizedSeverities.has(normalized as PluginFindingSeverity)) {
    return normalized as PluginFindingSeverity;
  }
  return "warning";
}

function normalizeFinding(value: unknown): PluginFinding {
  if (!isRecord(value)) {
    return { severity: "warning", message: String(value) };
  }

  const finding: PluginFinding = {
    severity: normalizeSeverity(value.severity),
    message: stringValue(value.message),
  };
  if (typeof value.file === "string") {
    finding.file = value.file;
  }
  if (typeof value.line === "number") {
    finding.line = value.line;
  }
  if (typeof value.column === "number") {
    finding.column = value.column;
  }
  if (typeof value.ruleId === "string") {
    finding.ruleId = value.ruleId;
  }
  if (typeof value.source === "string") {
    finding.source = value.source;
  }
  return finding;
}

function normalizeArtifact(value: unknown): PluginArtifact {
  if (!isRecord(value)) {
    return {};
  }
  const artifact: PluginArtifact = {};
  if (typeof value.label === "string") {
    artifact.label = value.label;
  }
  if (typeof value.path === "string") {
    artifact.path = value.path;
  }
  if (typeof value.uri === "string") {
    artifact.uri = value.uri;
  }
  if (typeof value.kind === "string") {
    artifact.kind = value.kind;
  }
  if (typeof value.mimeType === "string") {
    artifact.mimeType = value.mimeType;
  }
  return artifact;
}

function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
