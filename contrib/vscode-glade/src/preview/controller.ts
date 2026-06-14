import { ChildProcessWithoutNullStreams, spawn } from "child_process";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import * as vscode from "vscode";
import { runGlade, runGladeJSONWithCodes } from "../gladeCli";
import { GladeProjectContext } from "../projectModel";
import { devLWCArgs, devVFArgs, toolchainInstallArgs, toolchainStatusAllowedCodes, toolchainStatusArgs } from "./cli";
import { formatPreviewStartFailure, parseLWCReadyFile, parseVFReadyFile, PreviewProcessFailure, PreviewServer, stoppedPreviewServer, ToolchainStatus } from "./model";

type PreviewKind = "lwc" | "visualforce";

export interface PreviewRuntime {
  running: boolean;
  server?: PreviewServer;
}

export interface PreviewSnapshot {
  project?: GladeProjectContext;
  toolchain?: ToolchainStatus;
  lwc: PreviewRuntime;
  visualforce: PreviewRuntime;
}

export class PreviewController implements vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<void>();
  private project?: GladeProjectContext;
  private toolchain?: ToolchainStatus;
  private lwc: PreviewRuntime = { running: false };
  private visualforce: PreviewRuntime = { running: false };
  private lwcChild?: ChildProcessWithoutNullStreams;
  private vfChild?: ChildProcessWithoutNullStreams;
  readonly onDidChange = this.changed.event;

  constructor(private readonly output?: vscode.OutputChannel) {}

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    if (!project) {
      this.toolchain = undefined;
      this.stopLWC();
      this.stopVF();
    }
    this.changed.fire();
  }

  snapshot(): PreviewSnapshot {
    return {
      project: this.project,
      toolchain: this.toolchain,
      lwc: this.lwc,
      visualforce: this.visualforce,
    };
  }

  async refresh(): Promise<void> {
    if (!this.project) {
      this.toolchain = undefined;
      this.changed.fire();
      return;
    }
    this.toolchain = await runGladeJSONWithCodes<ToolchainStatus>(
      toolchainStatusArgs(),
      { cwd: this.project.projectRoot },
      "glade toolchain status",
      toolchainStatusAllowedCodes(),
    );
    this.changed.fire();
  }

  async installToolchain(): Promise<void> {
    const result = await runGlade(toolchainInstallArgs(), { cwd: this.project?.projectRoot });
    if (result.code !== 0) {
      const detail = result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`;
      throw new Error(detail);
    }
    await this.refresh();
  }

  async startLWC(): Promise<void> {
    await this.start("lwc");
  }

  stopLWC(): void {
    this.stop("lwc");
  }

  async startVF(): Promise<void> {
    await this.start("visualforce");
  }

  stopVF(): void {
    this.stop("visualforce");
  }

  dispose(): void {
    this.lwcChild?.kill();
    this.vfChild?.kill();
    this.lwcChild = undefined;
    this.vfChild = undefined;
    this.changed.dispose();
  }

  private async start(kind: PreviewKind): Promise<void> {
    const project = this.requireProject();
    if (this.child(kind)) {
      return;
    }
    const readyFile = await readyFilePath(kind);
    const args = kind === "lwc"
      ? devLWCArgs(project.projectRoot, "127.0.0.1:0", readyFile)
      : devVFArgs(project.projectRoot, "127.0.0.1:0", readyFile);
    this.output?.appendLine(`$ glade ${args.join(" ")}`);
    const child = spawn("glade", args, { cwd: project.projectRoot });
    this.setChild(kind, child);
    this.setRuntime(kind, { ...this.runtime(kind), running: true });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk: Buffer) => {
      const text = chunk.toString();
      stdout += text;
      this.output?.append(text);
    });
    child.stderr.on("data", (chunk: Buffer) => {
      const text = chunk.toString();
      stderr += text;
      this.output?.append(text);
    });
    child.on("error", (error: Error) => {
      if (this.child(kind) === child) {
        this.setChild(kind, undefined);
        this.markStopped(kind);
      }
      void vscode.window.showErrorMessage(`glade dev ${kindCommand(kind)} failed: ${error.message}`);
    });
    child.on("close", () => {
      if (this.child(kind) === child) {
        this.setChild(kind, undefined);
        this.markStopped(kind);
      }
    });
    try {
      const raw = await waitForReadyFile(readyFile, child, () => ({ stdout, stderr }));
      const server = kind === "lwc" ? parseLWCReadyFile(raw) : parseVFReadyFile(raw);
      this.setRuntime(kind, { running: true, server });
    } catch (error) {
      child.kill();
      if (this.child(kind) === child) {
        this.setChild(kind, undefined);
        this.markStopped(kind);
      }
      throw error;
    } finally {
      void fs.promises.rm(path.dirname(readyFile), { recursive: true, force: true });
    }
  }

  private stop(kind: PreviewKind): void {
    const child = this.child(kind);
    if (!child) {
      this.markStopped(kind);
      return;
    }
    this.setChild(kind, undefined);
    child.kill();
    this.markStopped(kind);
  }

  private markStopped(kind: PreviewKind): void {
    const runtime = this.runtime(kind);
    this.setRuntime(kind, {
      running: false,
      server: runtime.server ? stoppedPreviewServer(runtime.server) : undefined,
    });
  }

  private runtime(kind: PreviewKind): PreviewRuntime {
    return kind === "lwc" ? this.lwc : this.visualforce;
  }

  private setRuntime(kind: PreviewKind, runtime: PreviewRuntime): void {
    if (kind === "lwc") {
      this.lwc = runtime;
    } else {
      this.visualforce = runtime;
    }
    this.changed.fire();
  }

  private child(kind: PreviewKind): ChildProcessWithoutNullStreams | undefined {
    return kind === "lwc" ? this.lwcChild : this.vfChild;
  }

  private setChild(kind: PreviewKind, child: ChildProcessWithoutNullStreams | undefined): void {
    if (kind === "lwc") {
      this.lwcChild = child;
    } else {
      this.vfChild = child;
    }
  }

  private requireProject(): GladeProjectContext {
    if (!this.project) {
      throw new Error("Glade Local Preview requires an SFDX project.");
    }
    return this.project;
  }
}

async function readyFilePath(kind: PreviewKind): Promise<string> {
  const dir = await fs.promises.mkdtemp(path.join(os.tmpdir(), `glade-${kind}-preview-`));
  return path.join(dir, "ready.json");
}

function waitForReadyFile(
  readyFile: string,
  child: ChildProcessWithoutNullStreams,
  output: () => PreviewProcessFailure,
): Promise<string> {
  const deadline = Date.now() + 30000;
  return new Promise((resolve, reject) => {
    let settled = false;
    const finish = (callback: () => void): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearInterval(timer);
      child.off("close", onClose);
      child.off("error", onError);
      callback();
    };
    const onClose = (code: number | null, signal: NodeJS.Signals | null): void => finish(() => reject(new Error(
      formatPreviewStartFailure("glade dev exited before writing the ready file", { ...output(), code, signal }),
    )));
    const onError = (error: Error): void => finish(() => reject(error));
    const timer = setInterval(() => {
      if (Date.now() > deadline) {
        finish(() => reject(new Error(formatPreviewStartFailure("timed out waiting for glade dev ready file", output()))));
        return;
      }
      if (!fs.existsSync(readyFile)) {
        return;
      }
      try {
        const raw = fs.readFileSync(readyFile, "utf8");
        finish(() => resolve(raw));
      } catch (error) {
        finish(() => reject(error instanceof Error ? error : new Error(String(error))));
      }
    }, 100);
    child.once("close", onClose);
    child.once("error", onError);
  });
}

function kindCommand(kind: PreviewKind): string {
  return kind === "lwc" ? "lwc" : "vf";
}
