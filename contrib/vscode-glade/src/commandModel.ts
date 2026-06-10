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
  source: string;
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

export function execAnonymousArgs(source: string): string[] {
	return ["exec", "--debug-log", "-", source];
}

export function debugAnonymousConfig(project: string | undefined, source: string): GladeDebugConfig {
  return {
    type: "glade",
    request: "launch",
    name: "Glade: Debug Anonymous Apex",
    project,
    source,
  };
}
