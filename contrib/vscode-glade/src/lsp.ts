import * as vscode from "vscode";
import { LanguageClient, LanguageClientOptions, ServerOptions } from "vscode-languageclient/node";
import { GladeProjectContext, SalesforceExtensionState } from "./projectModel";

export class GladeLspClient implements vscode.Disposable {
  private client: LanguageClient | undefined;
  private projectRoot: string | undefined;
  private idleKey: string | undefined;

  constructor(private readonly output: vscode.OutputChannel) {}

  async sync(project: GladeProjectContext | undefined): Promise<void> {
    if (!project) {
      await this.stop();
      this.projectRoot = undefined;
      this.idleKey = undefined;
      return;
    }

    const enabled = vscode.workspace.getConfiguration("glade").get<boolean>("enableLsp", false);
    if (!enabled) {
      await this.stop();
      this.projectRoot = undefined;
      this.logIdle(project);
      return;
    }

    if (this.client && this.projectRoot === project.projectRoot) {
      return;
    }

    await this.stop();
    this.projectRoot = project.projectRoot;

    const serverOptions: ServerOptions = {
      command: "glade",
      args: ["lsp", "--project", project.projectRoot],
      options: { cwd: project.projectRoot },
    };
    const clientOptions: LanguageClientOptions = {
      documentSelector: [{ scheme: "file", language: "apex" }],
      outputChannel: this.output,
    };
    const client = new LanguageClient("gladeLsp", "Glade Local Apex", serverOptions, clientOptions);
    this.client = client;
    this.output.appendLine(`Glade LSP starting: glade lsp --project ${project.projectRoot}`);
    await client.start();
  }

  async stop(): Promise<void> {
    const client = this.client;
    this.client = undefined;
    if (client) {
      await client.stop();
      this.output.appendLine("Glade LSP stopped.");
    }
  }

  dispose(): void {
    void this.stop();
  }

  private logIdle(project: GladeProjectContext): void {
    if (!hasSalesforceApexLanguageSupport(project.salesforceExtensions)) {
      return;
    }
    const key = `${project.projectRoot}:salesforce`;
    if (this.idleKey === key) {
      return;
    }
    this.idleKey = key;
    this.output.appendLine(
      "Glade LSP idle: Salesforce Apex language support is installed; set glade.enableLsp=true to start local diagnostics.",
    );
  }
}

function hasSalesforceApexLanguageSupport(extensions: SalesforceExtensionState): boolean {
  return extensions.apex || extensions.apexLanguageServerTypescript;
}
