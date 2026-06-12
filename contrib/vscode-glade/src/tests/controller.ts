import * as fs from "fs";
import * as vscode from "vscode";
import { debugTestConfig } from "../commandModel";
import { configuredActiveEnvironment } from "../localOrg";
import { flattenTestCases, FlatTestCaseResult } from "../testResults";
import { GladeProjectContext } from "../projectModel";
import { discoverApexTests } from "./discovery";
import { changedTestArgs, runApexTest, runChangedTests, apexTestArgs } from "./runner";

interface TestData {
  className: string;
  methodName?: string;
}

export class GladeTestController {
  readonly controller = vscode.tests.createTestController("gladeApexTests", "Glade Apex");
  private project?: GladeProjectContext;
  private readonly itemData = new WeakMap<vscode.TestItem, TestData>();
  private readonly failedItems = new Map<string, vscode.TestItem>();

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly output?: vscode.OutputChannel,
  ) {
    context.subscriptions.push(this.controller);
    this.controller.refreshHandler = () => this.discover();
    this.controller.createRunProfile("Run", vscode.TestRunProfileKind.Run, (request, token) => this.run(request, token), true);
    this.controller.createRunProfile("Debug", vscode.TestRunProfileKind.Debug, (request) => this.debug(request), true);
  }

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    void this.discover();
  }

  async discover(): Promise<void> {
    if (!this.project) {
      this.controller.items.replace([]);
      return;
    }
    const pattern = new vscode.RelativePattern(this.project.projectRoot, "**/*.cls");
    const files = await vscode.workspace.findFiles(pattern, "**/{node_modules,.sfdx,.sf,.glade}/**");
    const items: vscode.TestItem[] = [];
    for (const file of files) {
      const source = fs.readFileSync(file.fsPath, "utf8");
      const discovered = discoverApexTests(file.fsPath, source);
      if (!discovered) {
        continue;
      }
      const classItem = this.controller.createTestItem(discovered.className, discovered.className, file);
      classItem.canResolveChildren = false;
      this.itemData.set(classItem, { className: discovered.className });
      for (const method of discovered.methods) {
        const methodItem = this.controller.createTestItem(`${discovered.className}.${method}`, method, file);
        this.itemData.set(methodItem, { className: discovered.className, methodName: method });
        classItem.children.add(methodItem);
      }
      items.push(classItem);
    }
    this.controller.items.replace(items);
  }

  async run(request: vscode.TestRunRequest, token: vscode.CancellationToken): Promise<void> {
    if (!this.project) {
      return;
    }
    const run = this.controller.createTestRun(request);
    const queue = request.include && request.include.length > 0 ? request.include : collectionItems(this.controller.items);
    for (const item of queue) {
      if (token.isCancellationRequested) {
        break;
      }
      await this.runItem(run, item);
    }
    run.end();
  }

  async runFailed(): Promise<void> {
    const failed = [...this.failedItems.values()];
    if (failed.length === 0) {
      vscode.window.showInformationMessage("Glade has no failed local Apex tests to rerun.");
      return;
    }
    const source = new vscode.CancellationTokenSource();
    try {
      await this.run(new vscode.TestRunRequest(failed), source.token);
    } finally {
      source.dispose();
    }
  }

  async runChanged(): Promise<void> {
    if (!this.project) {
      return;
    }
    const since = vscode.workspace.getConfiguration("glade").get<string>("changedSince", "origin/main");
    const request = new vscode.TestRunRequest();
    const run = this.controller.createTestRun(request, `Glade changed tests since ${since}`);
    this.logCommand(changedTestArgs(this.project, since));
    try {
      const result = await runChangedTests(this.project, since);
      for (const testCase of flattenTestCases(result)) {
        const item = this.findTestItem(testCase);
        if (!item) {
          continue;
        }
        run.started(item);
        this.recordResult(run, item, testCase);
      }
    } catch (error) {
      const message = new vscode.TestMessage(error instanceof Error ? error.message : String(error));
      for (const item of collectionItems(this.controller.items)) {
        run.errored(item, message);
      }
    } finally {
      run.end();
    }
  }

  async debugTestItem(item: vscode.TestItem | undefined): Promise<void> {
    if (!item || !this.project) {
      return;
    }
    const data = this.itemData.get(item);
    if (!data) {
      return;
    }
    if (!data.methodName) {
      void vscode.window.showInformationMessage("Select a local Apex test method to debug.");
      return;
    }
    const environment = configuredActiveEnvironment(this.project);
    const folder = vscode.workspace.getWorkspaceFolder(vscode.Uri.file(this.project.projectRoot));
    await vscode.debug.startDebugging(
      folder,
      debugTestConfig(this.project.projectRoot, data.className, data.methodName, environment.dbPath),
    );
  }

  private async runItem(run: vscode.TestRun, item: vscode.TestItem): Promise<void> {
    const data = this.itemData.get(item);
    if (!data || !this.project) {
      return;
    }
    run.started(item);
    this.startChildren(run, item);
    this.logCommand(apexTestArgs(this.project, data.className, data.methodName));
    try {
      const result = await runApexTest(this.project, data.className, data.methodName);
      const flat = flattenTestCases(result);
      const childResults = flat.filter((testCase) => testCase.className === data.className);
      for (const testCase of childResults) {
        const child = this.findTestItem(testCase);
        if (child && child !== item) {
          this.recordResult(run, child, testCase);
        }
      }
      const failed = childResults.find((candidate) => candidate.status !== "pass");
      if (failed) {
        run.failed(item, testMessage(failed), result.summary.durationMs);
        this.failedItems.set(item.id, item);
      } else {
        run.passed(item, result.summary.durationMs);
        this.failedItems.delete(item.id);
      }
    } catch (error) {
      run.errored(item, new vscode.TestMessage(error instanceof Error ? error.message : String(error)));
    }
  }

  private startChildren(run: vscode.TestRun, item: vscode.TestItem): void {
    item.children.forEach((child) => run.started(child));
  }

  private recordResult(run: vscode.TestRun, item: vscode.TestItem, result: FlatTestCaseResult): void {
    if (result.status === "pass") {
      run.passed(item, result.durationMs);
      this.failedItems.delete(item.id);
      return;
    }
    run.failed(item, testMessage(result), result.durationMs);
    this.failedItems.set(item.id, item);
  }

  private findTestItem(result: FlatTestCaseResult): vscode.TestItem | undefined {
    const classItem = this.controller.items.get(result.className);
    if (!classItem || !result.methodName) {
      return classItem;
    }
    return classItem.children.get(result.id) || classItem;
  }

  private async debug(request: vscode.TestRunRequest): Promise<void> {
    const first = request.include?.[0];
    if (!first) {
      return;
    }
    await this.debugTestItem(first);
  }

  private logCommand(args: string[]): void {
    this.output?.appendLine(`$ glade ${args.join(" ")}`);
  }
}

function collectionItems(collection: vscode.TestItemCollection): vscode.TestItem[] {
  const items: vscode.TestItem[] = [];
  collection.forEach((item) => items.push(item));
  return items;
}

function testMessage(result: FlatTestCaseResult): vscode.TestMessage {
  const message = new vscode.TestMessage(result.message || result.status);
  if (result.file && result.line !== undefined) {
    const line = Math.max(0, result.line - 1);
    const column = Math.max(0, (result.column || 1) - 1);
    message.location = new vscode.Location(vscode.Uri.file(result.file), new vscode.Position(line, column));
  }
  return message;
}
