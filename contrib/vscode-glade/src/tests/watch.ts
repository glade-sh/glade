import { ChildProcessWithoutNullStreams, spawn } from "child_process";
import * as vscode from "vscode";
import { GladeProjectContext } from "../projectModel";
import { watchArgs } from "./watchModel";

export class GladeTestWatch implements vscode.Disposable {
  private child?: ChildProcessWithoutNullStreams;

  constructor(
    private readonly output: vscode.OutputChannel,
    private readonly onStateChange?: (running: boolean) => void,
  ) {}

  get running(): boolean {
    return !!this.child;
  }

  start(project: GladeProjectContext): void {
    if (this.child) {
      void vscode.window.showInformationMessage("Glade local watch is already running.");
      return;
    }
    const args = watchArgs(project);
    this.output.show(true);
    this.output.appendLine(`$ glade ${args.join(" ")}`);
    const child = spawn("glade", args, { cwd: project.projectRoot });
    this.child = child;
    this.onStateChange?.(true);
    child.stdout.on("data", (chunk: Buffer) => this.output.append(chunk.toString()));
    child.stderr.on("data", (chunk: Buffer) => this.output.append(chunk.toString()));
    child.on("error", (error: Error) => {
      this.child = undefined;
      this.onStateChange?.(false);
      void vscode.window.showErrorMessage(`glade test watch failed: ${error.message}`);
    });
    child.on("close", (code: number | null, signal: NodeJS.Signals | null) => {
      this.child = undefined;
      this.onStateChange?.(false);
      this.output.appendLine(`glade test watch stopped${code === null ? "" : ` with code ${code}`}${signal ? ` signal ${signal}` : ""}`);
    });
  }

  stop(): void {
    const child = this.child;
    if (!child) {
      void vscode.window.showInformationMessage("Glade local watch is not running.");
      return;
    }
    child.kill();
  }

  dispose(): void {
    this.child?.kill();
    this.child = undefined;
    this.onStateChange?.(false);
  }
}
