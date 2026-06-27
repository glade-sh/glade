export interface TextSource {
  text: string;
  selection?: {
    start: number;
    end: number;
  };
}

export interface GladeDebugConfig {
  type: "glade";
  request: "launch";
  name: string;
  project?: string;
  dbPath?: string;
  dryRun?: boolean;
  className?: string;
  methodName?: string;
  source: string;
}

export interface GladeDebugSessionOptions {
  suppressSaveBeforeStart: true;
}

export function apexSourceFromDocument(source: TextSource): string {
	const start = source.selection?.start ?? 0;
	const end = source.selection?.end ?? 0;
	if (end > start) {
		return source.text.slice(start, end);
	}
	return source.text;
}

export function editorAnonymousSource(source: TextSource): string | undefined {
	const text = apexSourceFromDocument(source).trim();
	return text || undefined;
}

export function editorSoqlSource(source: TextSource): string | undefined {
	const text = apexSourceFromDocument(source).trim();
	return text || undefined;
}

export function execAnonymousArgs(source: string, projectRoot?: string, dbPath?: string): string[] {
  const args = ["exec", "--debug-log", "-"];
  if (projectRoot) {
    args.push("--project", projectRoot);
  }
  if (dbPath) {
    args.push("--db", dbPath);
  }
  args.push(source);
  return args;
}

export function debugReplayArgs(logPath: string, projectRoot: string, entryIndex?: number): string[] {
  const args = ["debug", "replay", "--log", logPath, "--project", projectRoot];
  if (entryIndex !== undefined) {
    args.push("--entry-index", String(entryIndex));
  }
  args.push("--json");
  return args;
}

export function debugAnonymousConfig(project: string | undefined, source: string, dbPath?: string): GladeDebugConfig {
  const config: GladeDebugConfig = {
    type: "glade",
    request: "launch",
    name: "Glade: Debug Anonymous Apex",
    project,
    source,
  };
  if (dbPath) {
    config.dbPath = dbPath;
  }
  return config;
}

export function debugReplayConfig(project: string | undefined, source: string, dbPath?: string): GladeDebugConfig {
  const config: GladeDebugConfig = {
    type: "glade",
    request: "launch",
    name: "Glade: Replay Apex Log",
    project,
    source,
    dryRun: true,
  };
  if (dbPath) {
    config.dbPath = dbPath;
  }
  return config;
}

export function debugAnonymousSessionOptions(): GladeDebugSessionOptions {
  return { suppressSaveBeforeStart: true };
}

export function debugTestConfig(project: string, className: string, methodName: string | undefined, dbPath?: string): GladeDebugConfig {
  const source = methodName ? `${methodName}();` : `${className}();`;
  const config: GladeDebugConfig = {
    type: "glade",
    request: "launch",
    name: methodName ? `Glade: Debug ${className}.${methodName}` : `Glade: Debug ${className}`,
    project,
    source,
  };
  if (methodName) {
    config.className = className;
    config.methodName = methodName;
  }
  if (dbPath) {
    config.dbPath = dbPath;
  }
  return config;
}
