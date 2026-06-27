const assert = require("assert");

const { looksLikeApexLog } = require("../out/apexLog/detection");

const apexLog = [
  "64.0 APEX_CODE,FINEST;DB,FINE;SYSTEM,DEBUG",
  "00:00:00.001 (1)|EXECUTION_STARTED",
  "00:00:00.002 (2)|CODE_UNIT_STARTED|[EXTERNAL]|Test.run",
  "00:00:00.003 (3)|METHOD_ENTRY|[2]|01p|Test.run()",
  "00:00:00.004 (4)|USER_DEBUG|[3]|DEBUG|hello",
  "00:00:00.005 (5)|SOQL_EXECUTE_BEGIN|[4]|Aggregations:0|SELECT Id FROM Account",
  "00:00:00.006 (6)|METHOD_EXIT|[2]|Test.run()",
].join("\n");

assert.strictEqual(looksLikeApexLog("/tmp/apex.log", apexLog), true);
assert.strictEqual(looksLikeApexLog("/tmp/apex.txt", apexLog), true);
assert.strictEqual(looksLikeApexLog("/tmp/apex.apexlog", apexLog), true);
assert.strictEqual(looksLikeApexLog("/tmp/build.log", "INFO startup\nWARN done\n"), false);
assert.strictEqual(looksLikeApexLog("/tmp/build.json", apexLog), false);
