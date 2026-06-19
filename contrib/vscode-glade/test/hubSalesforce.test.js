const assert = require("assert");
const salesforce = require("../out/hub/salesforce");

assert.deepStrictEqual(
  salesforce.salesforceTargetStatusArgs(),
  ["org", "display", "--json"],
);

assert.deepStrictEqual(
  salesforce.salesforceTargetStateFromRun({
    code: 0,
    stdout: JSON.stringify({
      result: {
        alias: "core-scratch",
        username: "dev@example.com",
        connectedStatus: "Connected",
      },
    }),
    stderr: "",
  }),
  { label: "core-scratch", state: "ready", detail: "dev@example.com" },
);

assert.deepStrictEqual(
  salesforce.salesforceTargetStateFromRun({
    code: 0,
    stdout: JSON.stringify({
      result: {
        username: "dev@example.com",
        connectedStatus: "RefreshTokenExpired",
      },
    }),
    stderr: "",
  }),
  { label: "dev@example.com", state: "stale", detail: "RefreshTokenExpired" },
);

assert.deepStrictEqual(
  salesforce.salesforceTargetStateFromRun({
    code: 1,
    stdout: "",
    stderr: "No default org found",
  }),
  { label: "no default target", state: "missing", detail: "No default org found" },
);
