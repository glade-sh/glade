import * as vscode from "vscode";
import { PreviewController, PreviewRuntime } from "../preview/controller";
import { PreviewRoute } from "../preview/model";
import { commandItem, GladeTreeItem } from "./tree";

export class PreviewView implements vscode.TreeDataProvider<GladeTreeItem>, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  private readonly subscription: vscode.Disposable;
  readonly onDidChangeTreeData = this.changed.event;

  constructor(private readonly controller: PreviewController) {
    this.subscription = controller.onDidChange(() => this.refresh());
  }

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    const snapshot = this.controller.snapshot();
    return [
      commandItem("Refresh preview", "glade.refreshPreview", "Refresh Local Preview state.", new vscode.ThemeIcon("refresh")),
      toolchainItem(snapshot.toolchain),
      commandItem("Install toolchain", "glade.installToolchain", "Install the LWC preview toolchain.", new vscode.ThemeIcon("cloud-download")),
      commandItem("Start LWC", "glade.startLWCPreview", "Start the local LWC preview server.", new vscode.ThemeIcon("play")),
      commandItem("Start Visualforce", "glade.startVFPreview", "Start the local Visualforce preview server.", new vscode.ThemeIcon("play")),
      serverItem("LWC", snapshot.lwc),
      ...routeItems(snapshot.lwc, "LWC"),
      commandItem("Stop LWC", "glade.stopLWCPreview", "Stop the local LWC preview server.", new vscode.ThemeIcon("debug-stop")),
      serverItem("Visualforce", snapshot.visualforce),
      ...routeItems(snapshot.visualforce, "Visualforce"),
      commandItem("Stop Visualforce", "glade.stopVFPreview", "Stop the local Visualforce preview server.", new vscode.ThemeIcon("debug-stop")),
    ];
  }

  dispose(): void {
    this.subscription.dispose();
    this.changed.dispose();
  }
}

function toolchainItem(toolchain: ReturnType<PreviewController["snapshot"]>["toolchain"]): GladeTreeItem {
  const item = new GladeTreeItem("Toolchain");
  item.description = toolchain ? (toolchain.ok ? "ready" : "not ready") : "not checked";
  item.tooltip = toolchain?.path || toolchain?.detail || "Run Refresh preview.";
  item.iconPath = new vscode.ThemeIcon(toolchain?.ok ? "check" : "warning");
  return item;
}

function serverItem(label: string, runtime: PreviewRuntime): GladeTreeItem {
  const item = new GladeTreeItem(`${label} server`);
  item.description = runtime.running ? (runtime.server?.addr || "starting") : "stopped";
  item.tooltip = runtime.server?.url || item.description;
  item.iconPath = new vscode.ThemeIcon(runtime.running ? "radio-tower" : "circle-slash");
  return item;
}

function routeItems(runtime: PreviewRuntime, label: string): GladeTreeItem[] {
  if (!runtime.server) {
    return [];
  }
  return runtime.server.routes.map((route) => routeItem(runtime.server!.url, route, label));
}

function routeItem(baseURL: string, route: PreviewRoute, group: string): GladeTreeItem {
  const fullURL = new URL(route.path, baseURL).toString();
  const item = new GladeTreeItem(route.label);
  item.description = group;
  item.tooltip = route.sourcePath ? `${route.sourcePath} -> ${fullURL}` : fullURL;
  item.iconPath = new vscode.ThemeIcon("link-external");
  item.command = {
    command: "glade.openPreviewRoute",
    title: "Open preview route",
    arguments: [fullURL],
  };
  return item;
}
