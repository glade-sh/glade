import * as vscode from "vscode";
import { runGladeJSON, runGladeJSONWithCodes } from "../gladeCli";
import { GladeProjectContext } from "../projectModel";
import { StartHereRunSummary } from "../startHereModel";
import { GladeTestRun } from "./model";

export function apexTestArgs(project: GladeProjectContext, className?: string, methodName?: string): string[] {
  const args = ["test", "--project", project.projectRoot, "--json"];
  if (className) {
    args.push("--class", className);
  }
  if (methodName) {
    args.push("--method", methodName);
  }
  return args;
}

export function changedTestArgs(project: GladeProjectContext, since = "origin/main"): string[] {
  return ["test", "changed", "--project", project.projectRoot, "--since", since, "--json"];
}

export async function runApexTest(
  project: GladeProjectContext,
  className?: string,
  methodName?: string,
): Promise<GladeTestRun> {
  return runGladeJSONWithCodes<GladeTestRun>(
    apexTestArgs(project, className, methodName),
    { cwd: project.projectRoot },
    "glade test",
    [0, 1],
  );
}

export async function runChangedTests(project: GladeProjectContext, since = "origin/main"): Promise<GladeTestRun> {
  return runGladeJSONWithCodes<GladeTestRun>(
    changedTestArgs(project, since),
    { cwd: project.projectRoot },
    "glade test changed",
    [0, 1],
  );
}

export function startHereSummary(label: string, run: GladeTestRun): StartHereRunSummary {
  return {
    label,
    passed: run.summary?.passed || 0,
    failed: run.summary?.failed || 0,
    durationMs: run.summary?.durationMs,
  };
}

export function testUri(file: string): vscode.Uri {
  return vscode.Uri.file(file);
}
