import * as vscode from "vscode";
import { runGladeJSON } from "../gladeCli";
import { GladeProjectContext } from "../projectModel";
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
  return runGladeJSON<GladeTestRun>(apexTestArgs(project, className, methodName), { cwd: project.projectRoot }, "glade test");
}

export async function runChangedTests(project: GladeProjectContext, since = "origin/main"): Promise<GladeTestRun> {
  return runGladeJSON<GladeTestRun>(
    changedTestArgs(project, since),
    { cwd: project.projectRoot },
    "glade test changed",
  );
}

export function testUri(file: string): vscode.Uri {
  return vscode.Uri.file(file);
}
