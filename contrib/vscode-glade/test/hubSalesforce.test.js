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

assert.deepStrictEqual(
  salesforce.salesforceTargetStateFromRun({
    code: 1,
    stdout: JSON.stringify({
      name: "NoDefaultEnvError",
      message: "No default environment found. Use -o or --target-org to specify an environment.",
      stack: "NoDefaultEnvError: No default environment found.\n    at OrgDisplayCommand.catch",
      status: 1,
      commandName: "OrgDisplayCommand",
    }),
    stderr: "",
  }),
  {
    label: "no default org",
    state: "missing",
    detail: "Set a default Salesforce org, then check again.",
  },
);
