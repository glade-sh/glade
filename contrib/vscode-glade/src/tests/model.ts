export interface GladeRunSummary {
  total: number;
  passed: number;
  failed: number;
  skipped: number;
  compileErrors: number;
  runtimeErrors: number;
  unsupported: number;
  errors: number;
  durationMs: number;
}

export interface GladeTestCase {
  name?: string;
  className?: string;
  methodName?: string;
  status: "pass" | "fail" | "skipped" | "compile_error" | "runtime_error" | "unsupported";
  durationMs?: number;
  problem?: {
    type?: string;
    message: string;
    detail?: string;
    stack?: Array<{ symbol?: string; file?: string; line?: number; column?: number }>;
  };
}

export interface GladeTestSuite {
  name: string;
  durationMs?: number;
  cases: GladeTestCase[];
}

export interface GladeTestRun {
  name?: string;
  durationMs?: number;
  summary: GladeRunSummary;
  suites: GladeTestSuite[];
}
