import * as path from "path";

export interface GladeEnvironment {
  name: string;
  dbPath: string;
  fixturePath?: string;
}

export function defaultEnvironment(projectRoot: string): GladeEnvironment {
  return { name: "dev", dbPath: path.join(projectRoot, ".glade", "envs", "dev.sqlite") };
}

export function normalizeEnvironments(raw: GladeEnvironment[] | undefined, projectRoot: string): GladeEnvironment[] {
  const source = raw && raw.length > 0 ? raw : [defaultEnvironment(projectRoot)];
  return source.map((entry) => {
    const normalized: GladeEnvironment = {
      name: environmentNameFromInput(entry.name),
      dbPath: absolutePath(entry.dbPath, projectRoot),
    };
    if (entry.fixturePath) {
      normalized.fixturePath = absolutePath(entry.fixturePath, projectRoot);
    }
    return normalized;
  });
}

export function activeEnvironment(activeName: string | undefined, environments: GladeEnvironment[]): GladeEnvironment {
  const wanted = activeName || "dev";
  return environments.find((entry) => entry.name === wanted) || environments[0] || defaultEnvironment(".");
}

export function environmentNameFromInput(input: string): string {
  const name = input.trim().replace(/[^A-Za-z0-9_.-]+/g, "-").replace(/^-+|-+$/g, "");
  if (!name) {
    throw new Error("environment name is required");
  }
  return name;
}

function absolutePath(value: string, projectRoot: string): string {
  return path.isAbsolute(value) ? value : path.join(projectRoot, value);
}
