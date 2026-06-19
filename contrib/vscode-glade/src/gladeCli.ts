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

export function runCommand(command: string, args: string[], options: GladeRunOptions = {}): Promise<GladeRunResult> {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
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

export function runGlade(args: string[], options: GladeRunOptions = {}): Promise<GladeRunResult> {
  return runCommand("glade", args, options);
}

export function parseJSONOutput<T>(stdout: string, label: string): T {
  try {
    return JSON.parse(stdout) as T;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`${label} produced invalid JSON: ${message}`);
  }
}

export function parseJSONRunResult<T>(result: GladeRunResult, label: string, allowedCodes: Array<number | null> = [0]): T {
  if (!allowedCodes.includes(result.code)) {
    const detail = result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`;
    throw new Error(`${label} failed: ${detail}`);
  }
  return parseJSONOutput<T>(result.stdout, label);
}

export async function runGladeJSON<T>(
  args: string[],
  options: GladeRunOptions = {},
  label = "glade",
): Promise<T> {
  const result = await runGlade(args, options);
  return parseJSONRunResult<T>(result, label);
}

export async function runGladeJSONWithCodes<T>(
  args: string[],
  options: GladeRunOptions = {},
  label = "glade",
  allowedCodes: Array<number | null> = [0],
): Promise<T> {
  const result = await runGlade(args, options);
  return parseJSONRunResult<T>(result, label, allowedCodes);
}
