import * as fs from "fs";
import * as path from "path";
import * as vscode from "vscode";
import { adapterExecutable, GladeDebugConfiguration, resolveGladeConfiguration } from "./adapter";
import { ApexLogAnalysisCache } from "./apexLog/cache";
import { ApexLogDiagnostics } from "./apexLog/diagnostics";
import { looksLikeApexLog } from "./apexLog/detection";
import { registerApexLogProviders } from "./apexLog/providers";
import { GladeCodeLensProvider, LocalTestTarget } from "./codeLens";
import { debugAnonymousSessionOptions, debugReplayArgs, debugReplayConfig, debugTestConfig, editorSoqlSource } from "./commandModel";
import { registerGladeCommands } from "./commands";
import { addEnvironment, clonedEnvironment, removeEnvironment, settingsValue } from "./environmentActions";
import { environmentNameFromInput, GladeEnvironment } from "./environments";
import { runGladeJSON } from "./gladeCli";
import { GladeHomeController } from "./hub/controller";
import { HubSnapshot, ProjectOrgState, SalesforceTargetState } from "./hub/model";
import { checkSalesforceTarget } from "./hub/salesforce";
import { GladeLspClient } from "./lsp";
import {
  checkProjectOrg,
  configuredActiveEnvironment,
  configuredEnvironments,
  createProjectOrg,
  dbExportArgs,
  dbResetArgs,
  dbSeedArgs,
  inspectLocalOrg,
  orgCreateArgs,
  orgStartArgs,
  schemaImportDescribeArgs,
  sendGladeTerminal,
  sendLocalOrgTerminal,
  terminalCommand,
} from "./localOrg";
import { summaryFromInspect } from "./localOrgModel";
import { GladeOutput } from "./output";
import { PluginController, PluginDiagnosticEntry, pluginArtifactRows } from "./plugins/controller";
import { isApexDebugLogEditor, isApexDebugLogPath, PluginAvailableContexts, PluginEditorAction, PluginFindingSeverity } from "./plugins/model";
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
import { PluginsView } from "./views/pluginsView";
import { RunsView } from "./views/runsView";
import { StartHereView } from "./views/startHereView";
import { WorkbenchView } from "./views/workbenchView";
import { WorkbenchController } from "./workbench/controller";

class GladeDebugAdapterFactory implements vscode.DebugAdapterDescriptorFactory {
  createDebugAdapterDescriptor(session: vscode.DebugSession): vscode.ProviderResult<vscode.DebugAdapterDescriptor> {
    return adapterExecutable(session.configuration as GladeDebugConfiguration);
  }
}

interface DebugReplayEnvelope {
  data?: {
    source?: string;
    warnings?: string[];
  };
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
  const workbenchView = new WorkbenchView();
  const debugView = new DebugView();
  const pluginsView = new PluginsView();
  const pluginDiagnostics = vscode.languages.createDiagnosticCollection("glade-plugins");
  context.subscriptions.push(pluginDiagnostics);
  const apexLogCache = new ApexLogAnalysisCache();
  const apexLogDiagnostics = new ApexLogDiagnostics();
  context.subscriptions.push(apexLogDiagnostics);
  let currentProject: GladeProjectContext | undefined;
  let salesforceTarget: SalesforceTargetState | undefined;
  let projectOrg: ProjectOrgState | undefined;
  let projectOrgTerminal: vscode.Terminal | undefined;
  let homeController: GladeHomeController | undefined;
  const projectOrgAlias = "my-glade-org";
  function updateHome(): void {
    homeController?.update();
  }
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
    runsView.setState({ lastRun: summary, watchRunning: startHereState.snapshot().watchRunning });
    status.setLastRun({ failed: summary.failed, durationMs: summary.durationMs });
    startHereView.refresh();
    updateHome();
  }, (failedTestCount) => {
    runsView.setState({ failedTestCount });
  });
  const watch = new GladeTestWatch(output.tests, (running) => {
    startHereState.setWatchRunning(running);
    runsView.setState({ lastRun: startHereState.snapshot().lastRun, watchRunning: running });
    startHereView.refresh();
    updateHome();
  });
  const lsp = new GladeLspClient(output.logs);
  context.subscriptions.push(lsp, watch);
  const workbench = new WorkbenchController(output.logs, (rows) => workbenchView.setRows(rows));

  function hubSnapshot(): HubSnapshot {
    const runtime = startHereState.snapshot();
    const config = vscode.workspace.getConfiguration("glade");
    const project = currentProject;
    return {
      project,
      activeEnvironment: project ? configuredActiveEnvironment(project) : undefined,
      localOrgSummary: runtime.localOrgSummary,
      projectOrg,
      projectOrgAlias,
      missingDb: runtime.missingDb,
      watchRunning: runtime.watchRunning,
      lastRun: runtime.lastRun,
      changedSince: config.get<string>("changedSince") || "origin/main",
      pluginActionCount: runtime.pluginActionCount,
      pluginFindingCount: plugins.latestFindingCount() || undefined,
      salesforceTarget,
    };
  }

  const home = new GladeHomeController(context, {
    snapshot: hubSnapshot,
    executeCommand: (command) => Promise.resolve(vscode.commands.executeCommand(command)),
    onError: (message) => output.logs.appendLine(message),
  });
  homeController = home;
  context.subscriptions.push(home);

  function pluginContexts(): PluginAvailableContexts {
    const activeEditor = vscode.window.activeTextEditor;
    const activeFile = activeEditor?.document.uri.fsPath;
    return {
      project: currentProject !== undefined,
      activeApexFile: activeFile ? /\.(cls|trigger)$/i.test(activeFile) : false,
      activeDebugLog: isApexDebugLogEditor(activeFile, activeEditor?.document.languageId),
      activeDataEnvironment: currentProject !== undefined,
      lastLocalRun: startHereState.snapshot().lastRun !== undefined,
    };
  }

  function syncPluginViews(): void {
    const contexts = pluginContexts();
    const startHereActions = plugins.actionRowsForView("startHere", contexts);
    startHereState.setPluginActionCount(startHereActions.length === 0 ? undefined : startHereActions.length);
    status.setPluginActionCount(plugins.latestFindingCount() || undefined);
    startHereView.setPluginActions(startHereActions);
    runsView.setPluginActions(plugins.actionRowsForView("runs", contexts));
    localOrgView.setPluginActions(plugins.actionRowsForView("localOrg", contexts));
    debugView.setPluginActions(plugins.actionRowsForView("debug", contexts));
    pluginsView.setState(
      plugins.plugins(),
      plugins.actionRowsForView("plugins", contexts),
      pluginArtifactRows(plugins.latestArtifacts()),
    );
    updateHome();
  }

  async function refreshPlugins(): Promise<void> {
    await plugins.refresh();
    syncPluginViews();
  }

  async function refreshProject(): Promise<void> {
    try {
      const previousProjectRoot = currentProject?.projectRoot;
      const project = await findProjectContext();
      currentProject = project;
      if (previousProjectRoot !== project?.projectRoot) {
        salesforceTarget = undefined;
        projectOrg = undefined;
      }
      const testExplorerEnabled = vscode.workspace.getConfiguration("glade").get<boolean>("enableTestExplorer", true);
      const environment = project ? configuredActiveEnvironment(project) : undefined;
      const missingDb = environment ? !fs.existsSync(environment.dbPath) : false;
      status.setProject(project);
      status.setMissingDb(missingDb);
      startHereState.setMissingDb(missingDb);
      startHereView.setProject(project);
      runsView.setState({ projectReady: project !== undefined });
      tests.setProject(testExplorerEnabled ? project : undefined);
      environmentsView.setProject(project);
      localOrgView.setProject(project);
      workbench.setProject(project);
      debugView.setProject(project);
      syncPluginViews();
      await lsp.sync(project);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      currentProject = undefined;
      salesforceTarget = undefined;
      projectOrg = undefined;
      status.setProject(undefined);
      status.setMissingDb(false);
      startHereState.setMissingDb(undefined);
      startHereView.setProject(undefined);
      runsView.setState({ projectReady: false });
      tests.setProject(undefined);
      environmentsView.setProject(undefined);
      localOrgView.setProject(undefined);
      workbench.setProject(undefined);
      debugView.setProject(undefined);
      syncPluginViews();
      await lsp.sync(undefined);
      output.logs.appendLine(`project detection failed: ${message}`);
    }
  }

  async function projectOrWarn(): Promise<GladeProjectContext | undefined> {
    const project = await findProjectContext();
    if (!project) {
      void vscode.window.showErrorMessage("Glade local data commands require a Salesforce DX project.");
      return undefined;
    }
    return project;
  }

  async function refreshProjectOrg(project: GladeProjectContext, detail = "checking"): Promise<void> {
    projectOrg = { alias: projectOrgAlias, state: "unknown", detail };
    updateHome();
    projectOrg = await checkProjectOrg(project, projectOrgAlias);
    updateHome();
  }

  async function ensureProjectOrg(project: GladeProjectContext): Promise<void> {
    const current = await checkProjectOrg(project, projectOrgAlias);
    if (current.state !== "missing") {
      projectOrg = current;
      updateHome();
      return;
    }
    output.logs.appendLine(`$ ${terminalCommand(["glade", ...orgCreateArgs({ projectRoot: "." }, projectOrgAlias)])}`);
    await createProjectOrg(project, projectOrgAlias);
    projectOrg = { alias: projectOrgAlias, state: "stopped", detail: "created local org" };
    updateHome();
  }

  function scheduleProjectOrgRefresh(project: GladeProjectContext): void {
    setTimeout(() => {
      void checkProjectOrg(project, projectOrgAlias)
        .then((state) => {
          projectOrg = state;
          updateHome();
        })
        .catch((error) => {
          const message = error instanceof Error ? error.message : String(error);
          projectOrg = { alias: projectOrgAlias, state: "unknown", detail: message };
          updateHome();
        });
    }, 1200);
  }

  async function openAnonymousApexScratch(): Promise<void> {
    const document = await vscode.workspace.openTextDocument({ language: "apex", content: "" });
    await vscode.window.showTextDocument(document, { preview: false });
  }

  async function openSoqlScratch(): Promise<void> {
    const document = await vscode.workspace.openTextDocument({ language: "soql", content: "" });
    await vscode.window.showTextDocument(document, { preview: false });
  }

  async function runSoqlScratch(): Promise<void> {
    const editor = vscode.window.activeTextEditor;
    if (!editor) {
      void vscode.window.showInformationMessage("Open a SOQL scratch buffer to run.");
      return;
    }
    const document = editor.document;
    const selection = editor.selection;
    const query = editorSoqlSource({
      text: document.getText(),
      selection: selection.isEmpty
        ? undefined
        : {
            start: document.offsetAt(selection.start),
            end: document.offsetAt(selection.end),
          },
    });
    if (!query) {
      void vscode.window.showInformationMessage("Open a SOQL scratch buffer or select a SOQL query to run.");
      return;
    }
    await runWorkbenchCommand(() => workbench.runSoqlText(query));
  }

  async function replayDebugLog(uri?: vscode.Uri, entryIndex?: number): Promise<void> {
    const logPath = uri?.fsPath || vscode.window.activeTextEditor?.document.uri.fsPath;
    if (!logPath || !isApexDebugLogPath(logPath)) {
      void vscode.window.showInformationMessage("Open an Apex debug log to replay.");
      return;
    }
    const project = await findProjectContext();
    if (!project) {
      void vscode.window.showErrorMessage("Glade replay requires a Salesforce DX project.");
      return;
    }
    const environment = configuredActiveEnvironment(project);
    const args = debugReplayArgs(logPath, project.projectRoot, entryIndex);
    output.logs.appendLine(`$ ${terminalCommand(["glade", ...args])}`);
    let replay: DebugReplayEnvelope;
    try {
      replay = await runGladeJSON<DebugReplayEnvelope>(args, { cwd: project.projectRoot }, "glade debug replay");
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      output.logs.appendLine(message);
      void vscode.window.showErrorMessage(`Glade replay failed: ${message}`, "Show Output")
        .then((picked) => {
          if (picked === "Show Output") {
            output.logs.show(true);
          }
        });
      return;
    }
    const source = replay.data?.source?.trim();
    if (!source) {
      void vscode.window.showErrorMessage("Glade replay did not return Apex source.");
      return;
    }
    for (const warning of replay.data?.warnings || []) {
      output.logs.appendLine(`Replay warning: ${warning}`);
    }
    const folder = vscode.workspace.getWorkspaceFolder(vscode.Uri.file(project.projectRoot));
    await vscode.debug.startDebugging(
      folder,
      debugReplayConfig(project.projectRoot, source, environment.dbPath),
      debugAnonymousSessionOptions(),
    );
  }

  async function refreshApexLogAnalysis(): Promise<void> {
    const editor = vscode.window.activeTextEditor;
    if (!editor || editor.document.languageId !== "apexlog") {
      void vscode.window.showInformationMessage("Open an Apex Log editor to refresh analysis.");
      return;
    }
    await analyzeApexLogDocument(editor.document, true);
  }

  async function analyzeApexLogDocument(document: vscode.TextDocument, force = false): Promise<void> {
    if (document.languageId !== "apexlog") {
      return;
    }
    if (force) {
      apexLogCache.clear(document);
    }
    const analysis = await apexLogCache.getAnalysis(document);
    apexLogDiagnostics.update(document, analysis?.diagnostics || apexLogCache.diagnosticsFor(document));
  }

  function refreshOpenApexLogDocuments(force = false): void {
    for (const document of vscode.workspace.textDocuments) {
      if (document.languageId === "apexlog") {
        if (force) {
          apexLogCache.clear(document);
        }
        void analyzeApexLogDocument(document, force);
      }
    }
  }

  async function treatCurrentFileAsApexLog(): Promise<void> {
    const editor = vscode.window.activeTextEditor;
    if (!editor) {
      return;
    }
    await vscode.languages.setTextDocumentLanguage(editor.document, "apexlog");
    await analyzeApexLogDocument(editor.document, true);
  }

  async function replayApexLogFrame(entryIndex?: number): Promise<void> {
    if (entryIndex === undefined) {
      void vscode.window.showInformationMessage("Replay from this frame is not available for this log entry.");
      return;
    }
    await replayDebugLog(undefined, entryIndex);
  }

  async function maybeDetectApexLog(editor?: vscode.TextEditor): Promise<void> {
    const target = editor || vscode.window.activeTextEditor;
    if (!target || target.document.languageId === "apexlog") {
      return;
    }
    const filePath = target.document.uri.fsPath;
    const uriKey = `apexLog.detectedLanguageMode:${target.document.uri.toString()}`;
    if (context.workspaceState.get(uriKey) !== undefined) {
      return;
    }
    if (!looksLikeApexLog(filePath, target.document.getText())) {
      return;
    }
    const picked = await vscode.window.showInformationMessage(
      "This looks like a Salesforce debug log. Use Apex Log language mode?",
      "Use Apex Log",
      "Not Now",
    );
    await context.workspaceState.update(uriKey, picked === "Use Apex Log");
    if (picked === "Use Apex Log") {
      await vscode.languages.setTextDocumentLanguage(target.document, "apexlog");
      await analyzeApexLogDocument(target.document, true);
    }
  }

  context.subscriptions.push(
    vscode.window.registerTreeDataProvider("glade.project", startHereView),
    vscode.window.registerTreeDataProvider("glade.recommendedRuns", runsView),
    vscode.window.registerTreeDataProvider("glade.environments", environmentsView),
    vscode.window.registerTreeDataProvider("glade.localOrg", localOrgView),
    vscode.window.registerTreeDataProvider("glade.workbench", workbenchView),
    vscode.window.registerTreeDataProvider("glade.debugLogs", debugView),
    vscode.window.registerTreeDataProvider("glade.plugins", pluginsView),
    vscode.commands.registerCommand("glade.openHome", () => home.open()),
    vscode.commands.registerCommand("glade.refresh", async () => {
      runsView.refresh();
      startHereView.refresh();
      environmentsView.refresh();
      localOrgView.refresh();
      workbenchView.refresh();
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
    vscode.commands.registerCommand("glade.workbench.newAnonymousApex", async () => {
      await openAnonymousApexScratch();
    }),
    vscode.commands.registerCommand("glade.workbench.newSoql", async () => {
      await openSoqlScratch();
    }),
    vscode.commands.registerCommand("glade.runSoql", async () => {
      await runSoqlScratch();
    }),
    vscode.commands.registerCommand("glade.workbench.runEntry", async (entryId?: string) => {
      await runWorkbenchCommand(() => workbench.runEntry(entryId));
    }),
    vscode.commands.registerCommand("glade.workbench.runLastSoql", async () => {
      await runWorkbenchCommand(() => workbench.runLast("soql"));
    }),
    vscode.commands.registerCommand("glade.workbench.describe", async () => {
      const objectName = await vscode.window.showInputBox({
        title: "Describe Local Object",
        prompt: "Object API name, or leave blank to list objects",
      });
      if (objectName === undefined) {
        return;
      }
      await runWorkbenchCommand(() => workbench.describe(objectName));
    }),
    vscode.commands.registerCommand("glade.workbench.openResult", () => workbench.openLastResult()),
    vscode.commands.registerCommand("glade.replayDebugLog", async (uri?: vscode.Uri) => {
      await replayDebugLog(uri);
    }),
    vscode.commands.registerCommand("glade.apexLog.refreshAnalysis", async () => {
      await refreshApexLogAnalysis();
    }),
    vscode.commands.registerCommand("glade.apexLog.treatAsApexLog", async () => {
      await treatCurrentFileAsApexLog();
    }),
    vscode.commands.registerCommand("glade.apexLog.replayFromFrame", async (entryIndex?: number) => {
      await replayApexLogFrame(entryIndex);
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
        runsView.setState({ lastRun: runSummary, watchRunning: startHereState.snapshot().watchRunning });
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
        updateHome();
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        const runSummary = { label: "Changed tests failed", passed: 0, failed: 1 };
        startHereState.setLastRun(runSummary);
        runsView.setState({ lastRun: runSummary, watchRunning: startHereState.snapshot().watchRunning });
        status.setLastRun({ failed: 1 }, command);
        output.tests.appendLine(message);
        updateHome();
        void vscode.window.showErrorMessage(`Glade changed tests failed: ${message}`, "Show Output")
          .then((picked) => {
            if (picked === "Show Output") {
              output.tests.show(true);
            }
          });
      }
    }),
    vscode.commands.registerCommand("glade.createProjectOrg", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      try {
        projectOrg = { alias: projectOrgAlias, state: "unknown", detail: "creating" };
        updateHome();
        output.logs.appendLine(`$ ${terminalCommand(["glade", ...orgCreateArgs({ projectRoot: "." }, projectOrgAlias)])}`);
        await createProjectOrg(project, projectOrgAlias);
        projectOrg = { alias: projectOrgAlias, state: "stopped", detail: "created local org" };
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        projectOrg = { alias: projectOrgAlias, state: "missing", detail: message };
        void vscode.window.showErrorMessage(`Glade org create failed: ${message}`);
      }
      updateHome();
    }),
    vscode.commands.registerCommand("glade.startProjectOrg", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      try {
        projectOrg = { alias: projectOrgAlias, state: "unknown", detail: "starting" };
        updateHome();
        await ensureProjectOrg(project);
        projectOrgTerminal?.dispose();
        const command = terminalCommand(["glade", ...orgStartArgs({ projectRoot: "." }, projectOrgAlias)]);
        output.logs.appendLine(`$ ${command}`);
        projectOrgTerminal = sendGladeTerminal(command, project.projectRoot);
        projectOrg = { alias: projectOrgAlias, state: "running", detail: command };
        scheduleProjectOrgRefresh(project);
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        projectOrg = { alias: projectOrgAlias, state: "missing", detail: message };
        void vscode.window.showErrorMessage(`Glade org start failed: ${message}`);
      }
      updateHome();
    }),
    vscode.commands.registerCommand("glade.stopProjectOrg", () => {
      if (projectOrgTerminal) {
        projectOrgTerminal.dispose();
        projectOrgTerminal = undefined;
        projectOrg = { alias: projectOrgAlias, state: "stopped", detail: "VS Code terminal stopped" };
      } else {
        projectOrg = { alias: projectOrgAlias, state: "unknown", detail: "No org terminal started by this window." };
      }
      updateHome();
    }),
    vscode.commands.registerCommand("glade.projectOrgStatus", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      try {
        await refreshProjectOrg(project);
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        projectOrg = { alias: projectOrgAlias, state: "unknown", detail: message };
        updateHome();
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
      workbench.reload();
      startHereView.refresh();
      debugView.refresh();
      updateHome();
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
      workbench.reload();
      startHereView.refresh();
      debugView.refresh();
      status.setProject(project);
      updateHome();
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
        workbench.reload();
        startHereView.refresh();
        debugView.refresh();
        status.setProject(project);
        status.setMissingDb(!fs.existsSync(clone.dbPath));
        updateHome();
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
      workbench.reload();
      startHereView.refresh();
      debugView.refresh();
      status.setProject(project);
      updateHome();
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
      updateHome();
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
      startHereState.setLocalOrgSummary(undefined);
      startHereState.setMissingDb(undefined);
      localOrgView.refresh();
      startHereView.refresh();
      updateHome();
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
      startHereState.setLocalOrgSummary(undefined);
      startHereState.setMissingDb(undefined);
      status.setChangedRecords(undefined);
      localOrgView.refresh();
      startHereView.refresh();
      updateHome();
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
    vscode.commands.registerCommand("glade.schemaImportDescribe", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const picked = await vscode.window.showOpenDialog({
        title: "Import Salesforce Describe JSON",
        filters: { JSON: ["json"] },
        canSelectMany: false,
      });
      const input = picked?.[0]?.fsPath;
      if (!input) {
        return;
      }
      sendGladeTerminal(terminalCommand(["glade", ...schemaImportDescribeArgs(project, input)]));
    }),
    vscode.commands.registerCommand("glade.salesforceTargetStatus", async () => {
      salesforceTarget = { label: "checking target", state: "unknown", detail: "sf org display --json" };
      updateHome();
      try {
        salesforceTarget = await checkSalesforceTarget(currentProject?.projectRoot);
        output.logs.appendLine(`Salesforce target: ${salesforceTarget.label} (${salesforceTarget.state})`);
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        salesforceTarget = { label: "target check failed", state: "missing", detail: message };
        output.logs.appendLine(`Salesforce target check failed: ${message}`);
        void vscode.window.showErrorMessage(`Glade Salesforce target check failed: ${message}`);
      }
      updateHome();
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
      updateHome();
      output.logs.appendLine("glade db inspect:");
      output.logs.appendLine(JSON.stringify(result, null, 2));
    }),
    vscode.commands.registerCommand("glade.statusQuickPick", async () => {
      const picked = await vscode.window.showQuickPick(
        [
          { label: "Open Glade Home", command: "glade.openHome" },
          { label: "Switch Local Data Environment", command: "glade.switchEnvironment" },
          { label: "Inspect Active Local Data", command: "glade.inspectLocalOrg" },
          { label: "Open Anonymous Apex Scratch", command: "glade.workbench.newAnonymousApex" },
          { label: "Run Last SOQL", command: "glade.workbench.runLastSoql" },
          { label: "Run Changed Tests", command: "glade.runLocalProof" },
          { label: "Manage Plugins", command: "glade.managePlugins" },
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
      if (
        event.affectsConfiguration("glade.apexLog.smartFeatures.enabled")
        || event.affectsConfiguration("glade.apexLog.maxAnalysisBytes")
      ) {
        apexLogDiagnostics.clearAll();
        refreshOpenApexLogDocuments(true);
      }
      if (event.affectsConfiguration("glade.environments") || event.affectsConfiguration("glade.activeEnvironment")) {
        startHereState.setLocalOrgSummary(undefined);
        startHereState.setMissingDb(undefined);
        environmentsView.refresh();
        localOrgView.refresh();
        workbench.reload();
        startHereView.refresh();
        debugView.refresh();
        updateHome();
        void refreshProject();
      }
    }),
    vscode.window.onDidChangeActiveTextEditor(() => syncPluginViews()),
    vscode.window.onDidChangeActiveTextEditor((editor) => {
      void maybeDetectApexLog(editor);
      if (editor?.document.languageId === "apexlog") {
        void analyzeApexLogDocument(editor.document);
      }
    }),
    vscode.workspace.onDidOpenTextDocument((document) => {
      if (document.languageId === "apexlog") {
        void analyzeApexLogDocument(document);
      }
    }),
    vscode.workspace.onDidSaveTextDocument((document) => {
      if (document.languageId === "apexlog") {
        void analyzeApexLogDocument(document, true);
      }
    }),
    vscode.workspace.onDidCloseTextDocument((document) => {
      if (document.languageId === "apexlog") {
        apexLogCache.clear(document);
        apexLogDiagnostics.clear(document);
      }
    }),
    vscode.window.onDidCloseTerminal((terminal) => {
      if (terminal === projectOrgTerminal) {
        projectOrgTerminal = undefined;
        if (projectOrg?.state === "running") {
          projectOrg = { alias: projectOrgAlias, state: "stopped", detail: "Terminal closed" };
          updateHome();
        }
      }
    }),
    vscode.debug.onDidChangeBreakpoints(() => debugView.refresh()),
    vscode.languages.registerCodeLensProvider({ language: "apex", scheme: "file" }, new GladeCodeLensProvider()),
  );
  registerApexLogProviders(context, { cache: apexLogCache });
  void maybeDetectApexLog();
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

async function runWorkbenchCommand(action: () => Promise<void>): Promise<void> {
  try {
    await action();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    void vscode.window.showErrorMessage(`Glade Workbench failed: ${message}`, "Show Output")
      .then((picked) => {
        if (picked === "Show Output") {
          void vscode.commands.executeCommand("glade.openOutput");
        }
      });
  }
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
