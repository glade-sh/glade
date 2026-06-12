const assert = require("assert");
const results = require("../out/testResults");

const run = {
  summary: {
    total: 1,
    passed: 0,
    failed: 1,
    skipped: 0,
    compileErrors: 0,
    runtimeErrors: 0,
    unsupported: 0,
    errors: 1,
    durationMs: 97,
  },
  suites: [
    {
      name: "AccountServiceTest",
      cases: [
        {
          className: "AccountServiceTest",
          methodName: "createsContact",
          status: "fail",
          durationMs: 97,
          problem: {
            message: "expected Pond Supply, actual Pond Supply Primary",
            stack: [{ file: "/repo/force-app/classes/AccountServiceTest.cls", line: 8, column: 5 }],
          },
        },
      ],
    },
  ],
};

const flattened = results.flattenTestCases(run);
assert.strictEqual(flattened.length, 1);
assert.strictEqual(flattened[0].id, "AccountServiceTest.createsContact");
assert.strictEqual(flattened[0].message, "expected Pond Supply, actual Pond Supply Primary");
assert.strictEqual(flattened[0].file, "/repo/force-app/classes/AccountServiceTest.cls");
assert.strictEqual(flattened[0].line, 8);
