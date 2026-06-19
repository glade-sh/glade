import * as vscode from "vscode";
import { parseHubMessage } from "./actions";
import { renderHubHtml } from "./html";
import { HubSnapshot } from "./model";

export interface GladeHomeControllerOptions {
  snapshot: () => HubSnapshot;
  executeCommand: (command: string) => Thenable<unknown>;
  onError?: (message: string) => void;
}

export class GladeHomeController implements vscode.Disposable {
  private panel?: vscode.WebviewPanel;
  private panelDisposables: vscode.Disposable[] = [];
  private readonly disposables: vscode.Disposable[] = [];
  private activeTab: "home" | "state" = "home";

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly options: GladeHomeControllerOptions,
  ) {}

  open(): void {
    if (this.panel) {
      this.panel.reveal(vscode.ViewColumn.One);
      this.render();
      return;
    }

    const panel = vscode.window.createWebviewPanel(
      "glade.home",
      "Glade Home",
      vscode.ViewColumn.One,
      {
        enableScripts: true,
        retainContextWhenHidden: true,
      },
    );
    this.panel = panel;
    this.panelDisposables = [
      panel.onDidDispose(() => {
        this.panel = undefined;
        this.activeTab = "home";
        for (const disposable of this.panelDisposables.splice(0)) {
          disposable.dispose();
        }
      }),
      panel.webview.onDidReceiveMessage((message) => this.handleMessage(message)),
    ];
    this.render();
  }

  update(): void {
    if (this.panel) {
      this.render();
    }
  }

  dispose(): void {
    for (const disposable of this.disposables.splice(0)) {
      disposable.dispose();
    }
    for (const disposable of this.panelDisposables.splice(0)) {
      disposable.dispose();
    }
    this.panel?.dispose();
    this.panel = undefined;
    this.activeTab = "home";
  }

  private render(): void {
    const panel = this.panel;
    if (!panel) {
      return;
    }
    panel.webview.html = renderHubHtml(this.options.snapshot(), {
      cspSource: panel.webview.cspSource,
      nonce: nonce(),
      initialTab: this.activeTab,
    });
  }

  private async handleMessage(message: unknown): Promise<void> {
    try {
      const parsed = parseHubMessage(message);
      if (parsed.type === "ready") {
        return;
      }
      if (parsed.type === "selectTab") {
        this.activeTab = parsed.tab;
        return;
      }
      await this.options.executeCommand(parsed.command);
      this.update();
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      this.options.onError?.(`Glade Home message failed: ${detail}`);
    }
  }
}

function nonce(): string {
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let value = "";
  for (let index = 0; index < 24; index += 1) {
    value += chars[Math.floor(Math.random() * chars.length)];
  }
  return value;
}
