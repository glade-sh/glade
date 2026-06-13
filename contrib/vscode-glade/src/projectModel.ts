import * as path from "path";

export interface GladeProjectContext {
  workspaceFolder: string;
  projectRoot: string;
  configPath?: string;
  configFound: boolean;
  namespace?: string;
  sourceApiVersion?: string;
  packageDirs: string[];
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
): GladeProjectContext {
  return {
    workspaceFolder: workspaceFolder || info.projectRoot,
    projectRoot: info.projectRoot,
    configFound: info.configFound,
    configPath: info.configPath,
    namespace: info.namespace,
    sourceApiVersion: info.sourceApiVersion,
    packageDirs: info.packageDirs || [],
  };
}
