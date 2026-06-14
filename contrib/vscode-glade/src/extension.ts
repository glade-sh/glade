import * as fs from "fs";
import * as path from "path";
import * as vscode from "vscode";
import { adapterExecutable, GladeDebugConfiguration, resolveGladeConfiguration } from "./adapter";
import { GladeCodeLensProvider, LocalTestTarget } from "./codeLens";
import { debugTestConfig } from "./commandModel";
import { registerGladeCommands } from "./commands";
import { addEnvironment, clonedEnvironment, removeEnvironment, settingsValue } from "./environmentActions";
import { environmentNameFromInput, GladeEnvironment } from "./environments";
import { GladeLspClient } from "./lsp";
import {
  configuredActiveEnvironment,
  configuredEnvironments,
  dbExportArgs,
  dbResetArgs,
  dbSeedArgs,
  inspectLocalOrg,
  sendLocalOrgTerminal,
  terminalCommand,
} from "./localOrg";
import { summaryFromInspect } from "./localOrgModel";
import { GladeOutput } from "./output";
import { PreviewController } from "./preview/controller";
import { PluginController, PluginDiagnosticEntry, pluginArtifactRows } from "./plugins/controller";
import { PluginAvailableContexts, PluginEditorAction, PluginFindingSeverity } from "./plugins/model";
import { findProjectContext } from "./projectContext";
import { GladeProjectContext } from "./projectModel";
import { StartHereState } from "./startHereState";
import { GladeStatus } from "./status";
import { GladeTestController } from "./tests/controller";
import { currentApexTestAtOffset } from "./tests/discovery";
import { apexTestArgs, changedTestArgs, runApexTest, runChangedTests, startHereSummary } from "./tests/runner";
import { GladeTestWatch } from "./tests/watch";
import { DebugView } from "./views/debugView";
import { EnvironmentsView } from "./views/environmentsView";
import { LocalOrgView } from "./views/localOrgView";
import { PreviewView } from "./views/previewView";
import { PluginsView } from "./views/pluginsView";
import { RunsView } from "./views/runsView";
import { StartHereView } from "./views/startHereView";

class GladeDebugAdapterFactory implements vscode.DebugAdapterDescriptorFactory {
  createDebugAdapterDescriptor(session: vscode.DebugSession): vscode.ProviderResult<vscode.DebugAdapterDescriptor> {
    return adapterExecutable(session.configuration as GladeDebugConfiguration);
  }
}

export function activate(context: vscode.ExtensionContext): void {
  const output = new GladeOutput();
  context.subscriptions.push(output);
  const status = new GladeStatus(context);
  const startHereState = new StartHereState();
  const startHereView = new StartHereView(startHereState);
  const runsView = new RunsView();
  const environmentsView = new EnvironmentsView();
  const localOrgView = new LocalOrgView();
  const previewController = new PreviewController(output.logs);
  const previewView = new PreviewView(previewController);
  const debugView = new DebugView();
  const pluginsView = new PluginsView();
  const pluginDiagnostics = vscode.languages.createDiagnosticCollection("glade-plugins");
  context.subscriptions.push(pluginDiagnostics);
  let currentProject: GladeProjectContext | undefined;
  const plugins = new PluginController({
    project: findProjectContext,
    activeFile: () => vscode.window.activeTextEditor?.document.uri.fsPath,
    activeDb: () => currentProject ? configuredActiveEnvironment(currentProject).dbPath : undefined,
    inputBox: (options) => Promise.resolve(vscode.window.showInputBox(options)),
    quickPick: (items, options) => Promise.resolve(vscode.window.showQuickPick(items, options)),
    openDialog: async (options) => {
      const picked = await vscode.window.showOpenDialog(options);
      return picked?.map((uri) => uri.fsPath);
    },
    diagnostics: {
      set(entries) {
        publishPluginDiagnostics(pluginDiagnostics, entries);
      },
      clear() {
        pluginDiagnostics.clear();
      },
    },
    log(message) {
      output.logs.appendLine(message);
    },
    executeCommand: (command) => Promise.resolve(vscode.commands.executeCommand(command)).then(() => undefined),
  });
  const tests = new GladeTestController(context, output.tests, (summary) => {
    startHereState.setLastRun(summary);
    status.setLastRun({ failed: summary.failed, durationMs: summary.durationMs });
    startHereView.refresh();
  });
  const watch = new GladeTestWatch(output.tests, (running) => {
    startHereState.setWatchRunning(running);
    startHereView.refresh();
  });
  const lsp = new GladeLspClient(output.logs);
  context.subscriptions.push(lsp, watch, previewController, previewView);

  function pluginContexts(): PluginAvailableContexts {
    const activeFile = vscode.window.activeTextEditor?.document.uri.fsPath;
    return {
      project: currentProject !== undefined,
      activeApexFile: activeFile ? /\.(cls|trigger)$/i.test(activeFile) : false,
      activeDebugLog: activeFile ? /\.log$/i.test(activeFile) : false,
      activeDataEnvironment: currentProject !== undefined,
      lastLocalRun: startHereState.snapshot().lastRun !== undefined,
    };
  }

  function syncPluginViews(): void {
    const contexts = pluginContexts();
    startHereView.setPluginActions(plugins.actionRowsForView("startHere", contexts));
    runsView.setPluginActions(plugins.actionRowsForView("runs", contexts));
    localOrgView.setPluginActions(plugins.actionRowsForView("localOrg", contexts));
    debugView.setPluginActions(plugins.actionRowsForView("debug", contexts));
    pluginsView.setState(
      plugins.plugins(),
      plugins.actionRowsForView("plugins", contexts),
      pluginArtifactRows(plugins.latestArtifacts()),
    );
  }

  async function refreshPlugins(): Promise<void> {
    await plugins.refresh();
    syncPluginViews();
  }

  async function refreshProject(): Promise<void> {
    try {
      const project = await findProjectContext();
      currentProject = project;
      const testExplorerEnabled = vscode.workspace.getConfiguration("glade").get<boolean>("enableTestExplorer", true);
      const environment = project ? configuredActiveEnvironment(project) : undefined;
      const missingDb = environment ? !fs.existsSync(environment.dbPath) : false;
      status.setProject(project);
      status.setMissingDb(missingDb);
      startHereState.setMissingDb(missingDb);
      startHereView.setProject(project);
      tests.setProject(testExplorerEnabled ? project : undefined);
      environmentsView.setProject(project);
      localOrgView.setProject(project);
      previewController.setProject(project);
      debugView.setProject(project);
      syncPluginViews();
      await lsp.sync(project);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      currentProject = undefined;
      status.setProject(undefined);
      status.setMissingDb(false);
      startHereState.setMissingDb(undefined);
      startHereView.setProject(undefined);
      tests.setProject(undefined);
      environmentsView.setProject(undefined);
      localOrgView.setProject(undefined);
      previewController.setProject(undefined);
      debugView.setProject(undefined);
      syncPluginViews();
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
    vscode.window.registerTreeDataProvider("glade.project", startHereView),
    vscode.window.registerTreeDataProvider("glade.recommendedRuns", runsView),
    vscode.window.registerTreeDataProvider("glade.environments", environmentsView),
    vscode.window.registerTreeDataProvider("glade.localOrg", localOrgView),
    vscode.window.registerTreeDataProvider("glade.preview", previewView),
    vscode.window.registerTreeDataProvider("glade.debugLogs", debugView),
    vscode.window.registerTreeDataProvider("glade.plugins", pluginsView),
    vscode.commands.registerCommand("glade.refresh", async () => {
      runsView.refresh();
      startHereView.refresh();
      environmentsView.refresh();
      localOrgView.refresh();
      previewView.refresh();
      debugView.refresh();
      pluginsView.refresh();
      await refreshProject();
      await refreshPlugins();
    }),
    vscode.commands.registerCommand("glade.refreshPlugins", () => refreshPlugins()),
    vscode.commands.registerCommand("glade.managePlugins", async () => {
      await plugins.managePlugins();
      syncPluginViews();
    }),
    vscode.commands.registerCommand("glade.linkLocalPlugin", async () => {
      await plugins.linkLocalPlugin();
      syncPluginViews();
    }),
    vscode.commands.registerCommand("glade.installPluginArchive", async () => {
      await plugins.installPluginArchive();
      syncPluginViews();
    }),
    vscode.commands.registerCommand("glade.runPluginAction", async (action?: PluginEditorAction) => {
      let target = action;
      if (!target) {
        const rows = plugins.actionRowsForView("plugins", pluginContexts());
        const picked = await vscode.window.showQuickPick(
          rows.map((row) => ({ label: row.label, description: row.description, action: row.action })),
          { title: "Run Plugin Action" },
        );
        target = picked?.action;
      }
      if (!target) {
        return;
      }
      await plugins.runAction(target);
      syncPluginViews();
    }),
    vscode.commands.registerCommand("glade.refreshPreview", async () => {
      try {
        await previewController.refresh();
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(`Glade preview refresh failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("glade.startLWCPreview", async () => {
      try {
        await previewController.startLWC();
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(`Glade LWC preview failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("glade.stopLWCPreview", () => previewController.stopLWC()),
    vscode.commands.registerCommand("glade.startVFPreview", async () => {
      try {
        await previewController.startVF();
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(`Glade Visualforce preview failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("glade.stopVFPreview", () => previewController.stopVF()),
    vscode.commands.registerCommand("glade.openPreviewRoute", async (url?: string) => {
      if (!url) {
        void vscode.window.showErrorMessage("Glade preview route is missing a URL.");
        return;
      }
      try {
        await vscode.env.openExternal(vscode.Uri.parse(url));
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(`Glade preview route failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("glade.installToolchain", async () => {
      try {
        await previewController.installToolchain();
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(`Glade toolchain install failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("glade.runChangedTests", () => tests.runChanged()),
    vscode.commands.registerCommand("glade.runFailedTests", () => tests.runFailed()),
    vscode.commands.registerCommand("glade.runLocalProof", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const changedSince = vscode.workspace.getConfiguration("glade").get<string>("changedSince") || "origin/main";
      const environment = configuredActiveEnvironment(project);
      const args = changedTestArgs(project, changedSince);
      const command = `glade ${args.join(" ")}`;
      status.setBusy("running");
      output.tests.show(true);
      output.tests.appendLine(`Environment: ${environment.name} (${environment.dbPath})`);
      output.tests.appendLine(`$ ${command}`);
      try {
        const run = await runChangedTests(project, changedSince);
        const runSummary = startHereSummary("Changed tests", run);
        startHereState.setLastRun(runSummary);
        status.setLastRun({ failed: runSummary.failed, durationMs: runSummary.durationMs }, command);
        syncPluginViews();
        const inspect = await inspectLocalOrg(project, environment);
        const localSummary = summaryFromInspect(inspect);
        startHereState.setLocalOrgSummary(localSummary);
        startHereState.setMissingDb(false);
        status.setMissingDb(false);
        status.setChangedRecords(undefined);
        localOrgView.setInspect(inspect, environment);
        output.logs.appendLine("glade db inspect:");
        output.logs.appendLine(JSON.stringify(inspect, null, 2));
        await tests.discover();
        startHereView.refresh();
        localOrgView.refresh();
        debugView.refresh();
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        status.setLastRun({ failed: 1 }, command);
        output.tests.appendLine(message);
        void vscode.window.showErrorMessage(`Glade local proof failed: ${message}`, "Show Output")
          .then((picked) => {
            if (picked === "Show Output") {
              output.tests.show(true);
            }
          });
      }
    }),
    vscode.commands.registerCommand("glade.debugTestItem", (item?: vscode.TestItem) => tests.debugTestItem(item)),
    vscode.commands.registerCommand("glade.runLocalTestFromCodeLens", async (target: LocalTestTarget) => {
      const project = await projectOrWarn();
      if (!project || !target?.className) {
        return;
      }
      output.tests.show(true);
      const environment = configuredActiveEnvironment(project);
      output.tests.appendLine(`Environment: ${environment.name} (${environment.dbPath})`);
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
        const next = addEnvironment(current, {
          name,
          dbPath: path.join(project.projectRoot, ".glade", "envs", `${name}.sqlite`),
        });
        await config.update("environments", settingsValue(next, project.projectRoot), vscode.ConfigurationTarget.Workspace);
      }
      await config.update("activeEnvironment", name, vscode.ConfigurationTarget.Workspace);
      startHereState.setLocalOrgSummary(undefined);
      environmentsView.refresh();
      localOrgView.refresh();
      startHereView.refresh();
      debugView.refresh();
    }),
    vscode.commands.registerCommand("glade.switchEnvironment", async (environment?: GladeEnvironment) => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      let pickedName = environment?.name;
      if (!pickedName) {
        const picked = await vscode.window.showQuickPick(
          configuredEnvironments(project).map((entry) => ({
            label: entry.name,
            description: entry.dbPath,
          })),
          { title: "Switch Local Data Environment" },
        );
        pickedName = picked?.label;
      }
      if (!pickedName) {
        return;
      }
      await vscode.workspace.getConfiguration("glade").update(
        "activeEnvironment",
        pickedName,
        vscode.ConfigurationTarget.Workspace,
      );
      startHereState.setLocalOrgSummary(undefined);
      environmentsView.refresh();
      localOrgView.refresh();
      startHereView.refresh();
      debugView.refresh();
      status.setProject(project);
    }),
    vscode.commands.registerCommand("glade.cloneEnvironment", async (environment?: GladeEnvironment) => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      try {
        const source = environment || configuredActiveEnvironment(project);
        const current = configuredEnvironments(project);
        const clone = clonedEnvironment(source, project.projectRoot, current);
        await fs.promises.mkdir(path.dirname(clone.dbPath), { recursive: true });
        if (fs.existsSync(source.dbPath)) {
          await fs.promises.copyFile(source.dbPath, clone.dbPath);
        }
        const next = addEnvironment(current, clone);
        const config = vscode.workspace.getConfiguration("glade");
        await config.update("environments", settingsValue(next, project.projectRoot), vscode.ConfigurationTarget.Workspace);
        await config.update("activeEnvironment", clone.name, vscode.ConfigurationTarget.Workspace);
        startHereState.setLocalOrgSummary(undefined);
        startHereState.setMissingDb(!fs.existsSync(clone.dbPath));
        environmentsView.refresh();
        localOrgView.refresh();
        startHereView.refresh();
        debugView.refresh();
        status.setProject(project);
        status.setMissingDb(!fs.existsSync(clone.dbPath));
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(`Glade clone environment failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("glade.deleteEnvironment", async (environment?: GladeEnvironment) => {
      const project = await projectOrWarn();
      if (!project || !environment) {
        return;
      }
      const confirmed = await vscode.window.showWarningMessage(
        `Delete local data environment ${environment.name}?`,
        { modal: true },
        "Delete",
      );
      if (confirmed !== "Delete") {
        return;
      }
      try {
        const config = vscode.workspace.getConfiguration("glade");
        const active = configuredActiveEnvironment(project);
        const next = removeEnvironment(configuredEnvironments(project), environment.name);
        await config.update("environments", settingsValue(next, project.projectRoot), vscode.ConfigurationTarget.Workspace);
        if (active.name === environment.name) {
          await config.update("activeEnvironment", "dev", vscode.ConfigurationTarget.Workspace);
        }
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(`Glade delete environment failed: ${message}`);
        return;
      }
      startHereState.setLocalOrgSummary(undefined);
      environmentsView.refresh();
      localOrgView.refresh();
      startHereView.refresh();
      debugView.refresh();
      status.setProject(project);
    }),
    vscode.commands.registerCommand("glade.revealEnvironmentDb", async (environment?: GladeEnvironment) => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const target = environment || configuredActiveEnvironment(project);
      await vscode.commands.executeCommand("revealFileInOS", vscode.Uri.file(target.dbPath));
    }),
    vscode.commands.registerCommand("glade.inspectEnvironment", async (environment?: GladeEnvironment) => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const target = environment || configuredActiveEnvironment(project);
      const result = await inspectLocalOrg(project, target);
      const localSummary = summaryFromInspect(result);
      startHereState.setLocalOrgSummary(localSummary);
      startHereState.setMissingDb(false);
      status.setMissingDb(false);
      status.setChangedRecords(undefined);
      localOrgView.setInspect(result, target);
      startHereView.refresh();
      localOrgView.refresh();
      output.logs.appendLine("glade db inspect:");
      output.logs.appendLine(JSON.stringify(result, null, 2));
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
      sendLocalOrgTerminal(terminalCommand(["glade", ...dbSeedArgs(project, environment, fixture)]));
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
      sendLocalOrgTerminal(terminalCommand(["glade", ...dbResetArgs(project, environment)]));
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
        terminalCommand(["glade", ...dbExportArgs(project, environment)], target.fsPath),
      );
    }),
    vscode.commands.registerCommand("glade.inspectLocalOrg", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const environment = configuredActiveEnvironment(project);
      const result = await inspectLocalOrg(project, environment);
      const localSummary = summaryFromInspect(result);
      startHereState.setLocalOrgSummary(localSummary);
      startHereState.setMissingDb(false);
      status.setMissingDb(false);
      status.setChangedRecords(undefined);
      localOrgView.setInspect(result, environment);
      startHereView.refresh();
      output.logs.appendLine("glade db inspect:");
      output.logs.appendLine(JSON.stringify(result, null, 2));
    }),
    vscode.commands.registerCommand("glade.statusQuickPick", async () => {
      const picked = await vscode.window.showQuickPick(
        [
          { label: "Switch Local Data Environment", command: "glade.switchEnvironment" },
          { label: "Inspect Active Local Data", command: "glade.inspectLocalOrg" },
          { label: "Run Local Proof", command: "glade.runLocalProof" },
          { label: "Open Glade Output", command: "glade.openOutput" },
        ],
        { placeHolder: "Glade local workflow" },
      );
      if (picked) {
        await vscode.commands.executeCommand(picked.command);
      }
    }),
    vscode.commands.registerCommand("glade.openOutput", () => {
      output.logs.show(true);
    }),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration("glade.enableLsp")) {
        void refreshProject();
      }
      if (event.affectsConfiguration("glade.enableTestExplorer")) {
        void refreshProject();
      }
      if (event.affectsConfiguration("glade.environments") || event.affectsConfiguration("glade.activeEnvironment")) {
        startHereState.setLocalOrgSummary(undefined);
        startHereState.setMissingDb(undefined);
        environmentsView.refresh();
        localOrgView.refresh();
        startHereView.refresh();
        debugView.refresh();
        void refreshProject();
      }
    }),
    vscode.window.onDidChangeActiveTextEditor(() => syncPluginViews()),
    vscode.debug.onDidChangeBreakpoints(() => debugView.refresh()),
    vscode.languages.registerCodeLensProvider({ language: "apex", scheme: "file" }, new GladeCodeLensProvider()),
  );
  void refreshProject();
  void refreshPlugins();

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

function publishPluginDiagnostics(
  collection: vscode.DiagnosticCollection,
  entries: PluginDiagnosticEntry[],
): void {
  collection.clear();
  const grouped = new Map<string, vscode.Diagnostic[]>();
  for (const entry of entries) {
    const line = Math.max((entry.line || 1) - 1, 0);
    const column = Math.max((entry.column || 1) - 1, 0);
    const diagnostic = new vscode.Diagnostic(
      new vscode.Range(line, column, line, column + 1),
      entry.message,
      diagnosticSeverity(entry.severity),
    );
    diagnostic.source = entry.source || "glade-plugins";
    diagnostic.code = entry.ruleId;
    const diagnostics = grouped.get(entry.file) || [];
    diagnostics.push(diagnostic);
    grouped.set(entry.file, diagnostics);
  }
  for (const [file, diagnostics] of grouped) {
    collection.set(vscode.Uri.file(file), diagnostics);
  }
}

function diagnosticSeverity(severity: PluginFindingSeverity): vscode.DiagnosticSeverity {
  switch (severity) {
    case "error":
      return vscode.DiagnosticSeverity.Error;
    case "info":
      return vscode.DiagnosticSeverity.Information;
    case "hint":
      return vscode.DiagnosticSeverity.Hint;
    case "warning":
    default:
      return vscode.DiagnosticSeverity.Warning;
  }
}
