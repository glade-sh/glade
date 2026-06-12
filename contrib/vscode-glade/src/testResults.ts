import { GladeTestRun } from "./tests/model";

export interface FlatTestCaseResult {
  id: string;
  className: string;
  methodName: string;
  status: string;
  durationMs: number;
  message?: string;
  file?: string;
  line?: number;
  column?: number;
}

export function flattenTestCases(run: GladeTestRun): FlatTestCaseResult[] {
  const cases: FlatTestCaseResult[] = [];
  for (const suite of run.suites || []) {
    for (const testCase of suite.cases || []) {
      const className = testCase.className || suite.name;
      const methodName = testCase.methodName || testCase.name || "";
      const frame = testCase.problem?.stack?.find((candidate) => candidate.file) || testCase.problem?.stack?.[0];
      cases.push({
        id: methodName ? `${className}.${methodName}` : className,
        className,
        methodName,
        status: testCase.status,
        durationMs: testCase.durationMs || 0,
        message: testCase.problem?.message,
        file: frame?.file,
        line: frame?.line,
        column: frame?.column,
      });
    }
  }
  return cases;
}
