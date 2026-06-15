const assert = require("assert");
const model = require("../out/workbench/model");

const now = "2026-06-14T18:00:00.000Z";

assert.deepStrictEqual(
  model.entryFromInput("soql", "  Accounts  ", "  SELECT Id FROM Account  ", now, "query-1"),
  {
    id: "query-1",
    kind: "soql",
    name: "Accounts",
    body: "SELECT Id FROM Account",
    createdAt: now,
    updatedAt: now,
  },
);

assert.throws(() => model.entryFromInput("soql", "  ", "SELECT Id FROM Account", now), /name is required/);
assert.throws(() => model.entryFromInput("soql", "Accounts", "  ", now), /body is required/);
assert.throws(() => model.entryFromInput("bad", "Accounts", "SELECT Id FROM Account", now), /unsupported workbench entry kind/);

assert.deepStrictEqual(
  model.sortEntries([
    { id: "old", kind: "soql", name: "Zeta", body: "SELECT Id FROM Contact", createdAt: "2026-06-01T00:00:00.000Z", updatedAt: "2026-06-02T00:00:00.000Z" },
    { id: "run", kind: "soql", name: "Run", body: "SELECT Id FROM Account", createdAt: "2026-06-01T00:00:00.000Z", updatedAt: "2026-06-03T00:00:00.000Z", lastRunAt: "2026-06-05T00:00:00.000Z" },
    { id: "new", kind: "soql", name: "Beta", body: "SELECT Name FROM Account", createdAt: "2026-06-01T00:00:00.000Z", updatedAt: "2026-06-04T00:00:00.000Z" },
    { id: "tie-a", kind: "soql", name: "Alpha", body: "SELECT Id FROM Account", createdAt: "2026-06-01T00:00:00.000Z", updatedAt: "2026-06-02T00:00:00.000Z" },
  ]).map((entry) => entry.id),
  ["run", "new", "tie-a", "old"],
);

const rows = model.workbenchTreeRows([
  { id: "query-1", kind: "soql", name: "Accounts", body: "SELECT Id FROM Account", createdAt: now, updatedAt: now },
], "dev");

assert.deepStrictEqual(rows, [
  { id: "environment", type: "environment", label: "Environment", description: "dev" },
  { id: "soql", type: "group", kind: "soql", label: "SOQL", count: 1 },
  { id: "soql:query-1", type: "entry", kind: "soql", entryId: "query-1", label: "Accounts", description: "Updated 2026-06-14T18:00:00.000Z" },
]);

assert.deepStrictEqual(
  model.parseSoqlJsonResult(JSON.stringify({
    totalSize: 1,
    done: true,
    records: [{ attributes: { type: "Account" }, Id: "001000000000001", Name: "Acme" }],
  })),
  {
    totalSize: 1,
    done: true,
    records: [{ attributes: { type: "Account" }, Id: "001000000000001", Name: "Acme" }],
    columns: ["Id", "Name"],
  },
);

assert.throws(() => model.parseSoqlJsonResult('{"records":{}}'), /SOQL JSON records must be an array/);
