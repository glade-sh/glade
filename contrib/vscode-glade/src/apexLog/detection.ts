import * as path from "path";

const eventPattern = /\|[A-Z_]+(?:_[A-Z]+)*\|/g;
const signaturePattern = /\b(CODE_UNIT_STARTED|METHOD_ENTRY|SOQL_EXECUTE_BEGIN|USER_DEBUG)\b/;

export function looksLikeApexLog(filePath: string, text: string): boolean {
  const ext = path.extname(filePath).toLowerCase();
  if (ext !== ".log" && ext !== ".txt" && ext !== ".apexlog") {
    return false;
  }
  if (ext === ".apexlog") {
    return true;
  }
  const firstLines = text.split(/\r?\n/, 200).join("\n");
  const events = firstLines.match(eventPattern) || [];
  return events.length >= 5 && signaturePattern.test(firstLines);
}
