import * as vscode from "vscode";
import { adapterExecutable, GladeDebugConfiguration, resolveGladeConfiguration } from "./adapter";
import { GladeCodeLensProvider, LocalTestTarget } from "./codeLens";
import { debugTestConfig } from "./commandModel";
import { registerGladeCommands } from "./commands";
import { environmentNameFromInput } from "./environments";
import { GladeLspClient } from "./lsp";
import {
  configuredActiveEnvironment,
  configuredEnvironments,
  defaultEnvironmentEntry,
  inspectLocalOrg,
  sendLocalOrgTerminal,
  terminalCommand,
} from "./localOrg";
import { GladeOutput } from "./output";
import { findProjectContext } from "./projectContext";
import { GladeProjectContext } from "./projectModel";
import { GladeStatus } from "./status";
import { GladeTestController } from "./tests/controller";
import { currentApexTestAtOffset } from "./tests/discovery";
import { apexTestArgs, runApexTest } from "./tests/runner";
import { GladeTestWatch } from "./tests/watch";
import { DebugView } from "./views/debugView";
import { EnvironmentsView } from "./views/environmentsView";
import { LocalOrgView } from "./views/localOrgView";
import { ProjectView } from "./views/projectView";
import { ApexTestsView, RunsView } from "./views/runsView";

class GladeDebugAdapterFactory implements vscode.DebugAdapterDescriptorFactory {
  createDebugAdapterDescriptor(session: vscode.DebugSession): vscode.ProviderResult<vscode.DebugAdapterDescriptor> {
    return adapterExecutable(session.configuration as GladeDebugConfiguration);
  }
}

export function activate(context: vscode.ExtensionContext): void {
  const output = new GladeOutput();
  context.subscriptions.push(output);
  const status = new GladeStatus(context);
  const projectView = new ProjectView();
  const runsView = new RunsView();
  const apexTestsView = new ApexTestsView();
  const environmentsView = new EnvironmentsView();
  const localOrgView = new LocalOrgView();
  const debugView = new DebugView();
  const tests = new GladeTestController(context, output.tests);
  const watch = new GladeTestWatch(output.tests);
  const lsp = new GladeLspClient(output.logs);
  context.subscriptions.push(lsp, watch);

  async function refreshProject(): Promise<void> {
    try {
      const project = await findProjectContext();
      const testExplorerEnabled = vscode.workspace.getConfiguration("glade").get<boolean>("enableTestExplorer", true);
      status.setProject(project);
      projectView.setProject(project);
      tests.setProject(testExplorerEnabled ? project : undefined);
      environmentsView.setProject(project);
      await lsp.sync(project);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      status.setProject(undefined);
      projectView.setProject(undefined);
      tests.setProject(undefined);
      environmentsView.setProject(undefined);
      await lsp.sync(undefined);
      output.logs.appendLine(`project detection failed: ${message}`);
    }
  }

  async function projectOrWarn(): Promise<GladeProjectContext | undefined> {
    const project = await findProjectContext();
    if (!project) {
      void vscode.window.showErrorMessage("Glade local data commands require an SFDX project.");
      return undefined;
    }
    return project;
  }

  context.subscriptions.push(
    vscode.window.registerTreeDataProvider("glade.project", projectView),
    vscode.window.registerTreeDataProvider("glade.recommendedRuns", runsView),
    vscode.window.registerTreeDataProvider("glade.apexTests", apexTestsView),
    vscode.window.registerTreeDataProvider("glade.environments", environmentsView),
    vscode.window.registerTreeDataProvider("glade.localOrg", localOrgView),
    vscode.window.registerTreeDataProvider("glade.debugLogs", debugView),
    vscode.commands.registerCommand("glade.refresh", async () => {
      runsView.refresh();
      apexTestsView.refresh();
      environmentsView.refresh();
      localOrgView.refresh();
      debugView.refresh();
      await refreshProject();
    }),
    vscode.commands.registerCommand("glade.runChangedTests", () => tests.runChanged()),
    vscode.commands.registerCommand("glade.runFailedTests", () => tests.runFailed()),
    vscode.commands.registerCommand("glade.debugTestItem", (item?: vscode.TestItem) => tests.debugTestItem(item)),
    vscode.commands.registerCommand("glade.runLocalTestFromCodeLens", async (target: LocalTestTarget) => {
      const project = await projectOrWarn();
      if (!project || !target?.className) {
        return;
      }
      output.tests.show(true);
      output.tests.appendLine(`$ glade ${apexTestArgs(project, target.className, target.methodName).join(" ")}`);
      try {
        const result = await runApexTest(project, target.className, target.methodName);
        output.tests.appendLine(JSON.stringify(result.summary, null, 2));
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        output.tests.appendLine(message);
        void vscode.window.showErrorMessage(`Glade local test failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("glade.debugLocalTestFromCodeLens", async (target: LocalTestTarget) => {
      const project = await projectOrWarn();
      if (!project || !target?.className) {
        return;
      }
      const environment = configuredActiveEnvironment(project);
      const folder = vscode.workspace.getWorkspaceFolder(vscode.Uri.file(project.projectRoot));
      await vscode.debug.startDebugging(
        folder,
        debugTestConfig(project.projectRoot, target.className, target.methodName, environment.dbPath),
      );
    }),
    vscode.commands.registerCommand("glade.debugCurrentTest", async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) {
        void vscode.window.showInformationMessage("Open a local Apex test method to debug.");
        return;
      }
      const target = currentApexTestAtOffset(
        editor.document.uri.fsPath,
        editor.document.getText(),
        editor.document.offsetAt(editor.selection.active),
      );
      if (!target) {
        void vscode.window.showInformationMessage("Place the cursor inside a local Apex test method.");
        return;
      }
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const environment = configuredActiveEnvironment(project);
      const folder = vscode.workspace.getWorkspaceFolder(vscode.Uri.file(project.projectRoot));
      await vscode.debug.startDebugging(
        folder,
        debugTestConfig(project.projectRoot, target.className, target.methodName, environment.dbPath),
      );
    }),
    vscode.commands.registerCommand("glade.startWatch", async () => {
      const project = await projectOrWarn();
      if (project) {
        watch.start(project);
      }
    }),
    vscode.commands.registerCommand("glade.stopWatch", () => watch.stop()),
    vscode.commands.registerCommand("glade.createEnvironment", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const entered = await vscode.window.showInputBox({
        title: "Create Local Data Environment",
        prompt: "Environment name",
      });
      if (!entered) {
        return;
      }
      let name: string;
      try {
        name = environmentNameFromInput(entered);
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(message);
        return;
      }
      const config = vscode.workspace.getConfiguration("glade");
      const current = configuredEnvironments(project);
      if (!current.some((entry) => entry.name === name)) {
        const raw = config.get<unknown[]>("environments") || [];
        const next = raw.length > 0 ? raw : [defaultEnvironmentEntry("dev")];
        await config.update("environments", [...next, defaultEnvironmentEntry(name)], vscode.ConfigurationTarget.Workspace);
      }
      await config.update("activeEnvironment", name, vscode.ConfigurationTarget.Workspace);
      environmentsView.refresh();
      localOrgView.refresh();
    }),
    vscode.commands.registerCommand("glade.switchEnvironment", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const picked = await vscode.window.showQuickPick(
        configuredEnvironments(project).map((environment) => ({
          label: environment.name,
          description: environment.dbPath,
        })),
        { title: "Switch Local Data Environment" },
      );
      if (!picked) {
        return;
      }
      await vscode.workspace.getConfiguration("glade").update(
        "activeEnvironment",
        picked.label,
        vscode.ConfigurationTarget.Workspace,
      );
      environmentsView.refresh();
      localOrgView.refresh();
    }),
    vscode.commands.registerCommand("glade.seedLocalOrg", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const picked = await vscode.window.showOpenDialog({
        title: "Seed Local Data Environment",
        filters: { JSON: ["json"] },
        canSelectMany: false,
      });
      const fixture = picked?.[0]?.fsPath;
      if (!fixture) {
        return;
      }
      const environment = configuredActiveEnvironment(project);
      sendLocalOrgTerminal(terminalCommand([
        "glade",
        "db",
        "seed",
        "--db",
        environment.dbPath,
        "--project",
        project.projectRoot,
        "--json",
        fixture,
      ]));
    }),
    vscode.commands.registerCommand("glade.resetLocalOrg", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const environment = configuredActiveEnvironment(project);
      const confirmed = await vscode.window.showWarningMessage(
        `Reset local data environment ${environment.name}?`,
        { modal: true },
        "Reset",
      );
      if (confirmed !== "Reset") {
        return;
      }
      sendLocalOrgTerminal(terminalCommand([
        "glade",
        "db",
        "reset",
        "--db",
        environment.dbPath,
        "--project",
        project.projectRoot,
        "--json",
      ]));
    }),
    vscode.commands.registerCommand("glade.exportLocalOrg", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const target = await vscode.window.showSaveDialog({
        title: "Export Local Data Environment",
        filters: { JSON: ["json"] },
      });
      if (!target) {
        return;
      }
      const environment = configuredActiveEnvironment(project);
      sendLocalOrgTerminal(
        terminalCommand([
          "glade",
          "db",
          "export",
          "--db",
          environment.dbPath,
          "--project",
          project.projectRoot,
        ], target.fsPath),
      );
    }),
    vscode.commands.registerCommand("glade.inspectLocalOrg", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const result = await inspectLocalOrg(project);
      localOrgView.setInspect(result);
      output.logs.appendLine("glade db inspect:");
      output.logs.appendLine(JSON.stringify(result, null, 2));
    }),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration("glade.enableLsp")) {
        void refreshProject();
      }
      if (event.affectsConfiguration("glade.enableTestExplorer")) {
        void refreshProject();
      }
      if (event.affectsConfiguration("glade.environments") || event.affectsConfiguration("glade.activeEnvironment")) {
        environmentsView.refresh();
        localOrgView.refresh();
      }
    }),
    vscode.debug.onDidChangeBreakpoints(() => debugView.refresh()),
    vscode.languages.registerCodeLensProvider({ language: "apex", scheme: "file" }, new GladeCodeLensProvider()),
  );
  void refreshProject();

  registerGladeCommands(context);
  context.subscriptions.push(
    vscode.debug.registerDebugConfigurationProvider("glade", {
      resolveDebugConfiguration(folder, config) {
        return resolveGladeConfiguration(folder, config as GladeDebugConfiguration);
      },
    }),
  );
  context.subscriptions.push(
    vscode.debug.registerDebugAdapterDescriptorFactory("glade", new GladeDebugAdapterFactory()),
  );
}

export function deactivate(): void {}
