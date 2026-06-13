import * as path from "path";
import { GladeEnvironment } from "./environments";

export function addEnvironment(existing: GladeEnvironment[], next: GladeEnvironment): GladeEnvironment[] {
  if (existing.some((entry) => entry.name === next.name)) {
    throw new Error(`environment "${next.name}" already exists`);
  }
  return [...existing, next];
}

export function removeEnvironment(existing: GladeEnvironment[], name: string): GladeEnvironment[] {
  if (name === "dev") {
    throw new Error("cannot delete the dev environment");
  }
  const next = existing.filter((entry) => entry.name !== name);
  if (next.length === existing.length) {
    throw new Error(`environment "${name}" does not exist`);
  }
  return next;
}

export function cloneName(name: string): string {
  return name.endsWith("-copy") ? `${name}-2` : `${name}-copy`;
}

export function nextCloneName(name: string, existing: GladeEnvironment[]): string {
  const names = new Set(existing.map((entry) => entry.name));
  let candidate = cloneName(name);
  let suffix = 2;
  while (names.has(candidate)) {
    candidate = `${cloneName(name)}-${suffix}`;
    suffix += 1;
  }
  return candidate;
}

export function clonedEnvironment(
  source: GladeEnvironment,
  projectRoot: string,
  existing: GladeEnvironment[] = [],
): GladeEnvironment {
  const name = existing.length > 0 ? nextCloneName(source.name, existing) : cloneName(source.name);
  return {
    name,
    dbPath: path.join(projectRoot, ".glade", "envs", `${name}.sqlite`),
    fixturePath: source.fixturePath,
  };
}

export function settingsValue(environments: GladeEnvironment[], projectRoot: string): GladeEnvironment[] {
  return environments.map((entry) => ({
    name: entry.name,
    dbPath: path.isAbsolute(entry.dbPath) ? path.relative(projectRoot, entry.dbPath) : entry.dbPath,
    fixturePath: entry.fixturePath
      ? path.isAbsolute(entry.fixturePath)
        ? path.relative(projectRoot, entry.fixturePath)
        : entry.fixturePath
      : undefined,
  }));
}
