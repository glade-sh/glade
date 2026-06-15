const assert = require("assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const store = require("../out/workbench/store");

const root = fs.mkdtempSync(path.join(os.tmpdir(), "glade-workbench-store-"));
const queries = [
  {
    id: "soql-1",
    kind: "soql",
    name: "Accounts",
    body: "SELECT Id, Name FROM Account",
    createdAt: "2026-06-14T18:00:00.000Z",
    updatedAt: "2026-06-14T18:00:00.000Z",
    lastRunAt: "2026-06-14T19:00:00.000Z",
  },
];

assert.deepStrictEqual(store.readWorkbenchEntries(root, "soql"), []);

store.writeWorkbenchEntries(root, "soql", queries);

assert.deepStrictEqual(store.readWorkbenchEntries(root, "soql"), queries);

assert.deepStrictEqual(
  JSON.parse(fs.readFileSync(path.join(root, ".glade", "workbench", "queries.json"), "utf8")),
  { version: 1, entries: queries },
);

assert.throws(() => store.workbenchStorePath(root, "bad"), /unsupported workbench entry kind/);

fs.writeFileSync(path.join(root, ".glade", "workbench", "queries.json"), JSON.stringify({ version: 2, entries: [] }));
assert.throws(() => store.readWorkbenchEntries(root, "soql"), /unsupported workbench store version/);
