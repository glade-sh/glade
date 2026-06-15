const assert = require("assert");
const model = require("../out/workbench/model");

const now = "2026-06-14T18:00:00.000Z";

assert.deepStrictEqual(
  model.entryFromInput("anonymousApex", "  Seed account  ", "  Account a = new Account(Name = 'Acme');  ", now, "apex-1"),
  {
    id: "apex-1",
    kind: "anonymousApex",
    name: "Seed account",
    body: "Account a = new Account(Name = 'Acme');",
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
    { id: "run", kind: "anonymousApex", name: "Alpha", body: "System.debug('run');", createdAt: "2026-06-01T00:00:00.000Z", updatedAt: "2026-06-03T00:00:00.000Z", lastRunAt: "2026-06-05T00:00:00.000Z" },
    { id: "new", kind: "soql", name: "Beta", body: "SELECT Name FROM Account", createdAt: "2026-06-01T00:00:00.000Z", updatedAt: "2026-06-04T00:00:00.000Z" },
    { id: "tie-a", kind: "soql", name: "Alpha", body: "SELECT Id FROM Account", createdAt: "2026-06-01T00:00:00.000Z", updatedAt: "2026-06-02T00:00:00.000Z" },
  ]).map((entry) => entry.id),
  ["run", "new", "tie-a", "old"],
);

const rows = model.workbenchTreeRows([
  { id: "query-1", kind: "soql", name: "Accounts", body: "SELECT Id FROM Account", createdAt: now, updatedAt: now },
  { id: "apex-1", kind: "anonymousApex", name: "Seed Account", body: "System.debug('seed');", createdAt: now, updatedAt: now, lastRunAt: "2026-06-14T19:00:00.000Z" },
], "dev");

assert.deepStrictEqual(rows, [
  { id: "environment", type: "environment", label: "Environment", description: "dev" },
  { id: "anonymousApex", type: "group", kind: "anonymousApex", label: "Anonymous Apex", count: 1 },
  { id: "anonymousApex:apex-1", type: "entry", kind: "anonymousApex", entryId: "apex-1", label: "Seed Account", description: "Last run 2026-06-14T19:00:00.000Z" },
  { id: "soql", type: "group", kind: "soql", label: "SOQL", count: 1 },
  { id: "soql:query-1", type: "entry", kind: "soql", entryId: "query-1", label: "Accounts", description: "Updated 2026-06-14T18:00:00.000Z" },
]);

assert.deepStrictEqual(
  model.parseExecJsonSummary(JSON.stringify({
    command: "exec",
    status: "passed",
    summary: { debugEvents: 2, soqlQueries: 1, dml: 3, cpuTimeMs: 4, log: "/repo/.glade/logs/exec.log" },
  })),
  { status: "passed", debugEvents: 2, soqlQueries: 1, dml: 3, cpuTimeMs: 4, log: "/repo/.glade/logs/exec.log" },
);

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

assert.throws(() => model.parseExecJsonSummary("{"), /invalid exec JSON/);
assert.throws(() => model.parseSoqlJsonResult('{"records":{}}'), /SOQL JSON records must be an array/);
