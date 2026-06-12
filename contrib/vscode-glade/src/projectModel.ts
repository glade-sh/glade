import * as path from "path";

export interface GladeProjectContext {
  workspaceFolder: string;
  projectRoot: string;
  configPath?: string;
  configFound: boolean;
  namespace?: string;
  sourceApiVersion?: string;
  packageDirs: string[];
  salesforceExtensions: SalesforceExtensionState;
}

export interface SalesforceExtensionState {
  apex: boolean;
  apexTesting: boolean;
  apexLanguageServerTypescript: boolean;
}

export interface ConfigShowInfo {
  configPath?: string;
  configFound: boolean;
  projectRoot: string;
  namespace?: string;
  sourceApiVersion?: string;
  packageDirs?: string[];
}

export function nearestSfdxRoot(startPath: string, sfdxProjectFiles: string[]): string | undefined {
  let dir = path.resolve(startPath);
  const roots = new Set(sfdxProjectFiles.map((file) => path.dirname(path.resolve(file))));
  if (path.basename(dir) !== "sfdx-project.json") {
    dir = path.dirname(dir);
  }
  while (true) {
    if (roots.has(path.resolve(dir))) {
      return path.resolve(dir);
    }
    const parent = path.dirname(dir);
    if (parent === dir) {
      return undefined;
    }
    dir = parent;
  }
}

export function parseConfigShowInfo(
  info: ConfigShowInfo,
  workspaceFolder: string | undefined,
  salesforceExtensions: SalesforceExtensionState,
): GladeProjectContext {
  return {
    workspaceFolder: workspaceFolder || info.projectRoot,
    projectRoot: info.projectRoot,
    configFound: info.configFound,
    configPath: info.configPath,
    namespace: info.namespace,
    sourceApiVersion: info.sourceApiVersion,
    packageDirs: info.packageDirs || [],
    salesforceExtensions,
  };
}

export function detectSalesforceExtensions(extensionIds: string[]): SalesforceExtensionState {
  const ids = new Set(extensionIds);
  return {
    apex: ids.has("salesforce.salesforcedx-vscode-apex"),
    apexTesting: ids.has("salesforce.salesforcedx-vscode-apex-testing"),
    apexLanguageServerTypescript: ids.has("salesforce.apex-language-server-extension"),
  };
}
