import { spawn } from "child_process";

export interface GladeRunOptions {
  cwd?: string;
  env?: NodeJS.ProcessEnv;
}

export interface GladeRunResult {
  code: number | null;
  stdout: string;
  stderr: string;
}

export function buildGladeArgs(command: string, args: string[]): string[] {
  return [command, ...args];
}

export function runGlade(args: string[], options: GladeRunOptions = {}): Promise<GladeRunResult> {
  return new Promise((resolve, reject) => {
    const child = spawn("glade", args, {
      cwd: options.cwd,
      env: { ...process.env, ...options.env },
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk: Buffer) => {
      stdout += chunk.toString();
    });
    child.stderr.on("data", (chunk: Buffer) => {
      stderr += chunk.toString();
    });
    child.on("error", reject);
    child.on("close", (code) => resolve({ code, stdout, stderr }));
  });
}

export function parseJSONOutput<T>(stdout: string, label: string): T {
  try {
    return JSON.parse(stdout) as T;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`${label} produced invalid JSON: ${message}`);
  }
}

export async function runGladeJSON<T>(
  args: string[],
  options: GladeRunOptions = {},
  label = "glade",
): Promise<T> {
  const result = await runGlade(args, options);
  if (result.code !== 0) {
    const detail = result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`;
    throw new Error(`${label} failed: ${detail}`);
  }
  return parseJSONOutput<T>(result.stdout, label);
}
