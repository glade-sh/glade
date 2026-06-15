import { ExecSummaryResult, SoqlQueryResult, parseExecJsonSummary, parseSoqlJsonResult } from "./model";

export function execArgs(projectRoot: string, dbPath: string, body: string): string[] {
  return ["exec", "--json", "--project", projectRoot, "--db", dbPath, body];
}

export function queryArgs(projectRoot: string, dbPath: string, soql: string, limit?: number): string[] {
  const args = ["db", "query", "--json", "--project", projectRoot, "--db", dbPath];
  if (limit !== undefined) {
    if (!Number.isInteger(limit) || limit < 1) {
      throw new Error("SOQL limit must be a positive integer");
    }
    args.push("--limit", String(limit));
  }
  args.push(soql);
  return args;
}

export function describeArgs(projectRoot: string, dbPath: string, objectName?: string): string[] {
  const args = ["db", "describe", "--json", "--project", projectRoot, "--db", dbPath];
  if (objectName && objectName.trim()) {
    args.push(objectName.trim());
  }
  return args;
}

export function parseExecOutput(stdout: string): ExecSummaryResult {
  return parseExecJsonSummary(stdout);
}

export function parseQueryOutput(stdout: string): SoqlQueryResult {
  return parseSoqlJsonResult(stdout);
}
