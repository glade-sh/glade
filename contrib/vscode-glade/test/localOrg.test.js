const assert = require("assert");
const localOrg = require("../out/localOrgModel");

const rows = localOrg.objectRowsFromInspect({
  path: "/repo/.glade/envs/dev.sqlite",
  schemaVersion: 1,
  objects: 2,
  records: 46,
  byObject: { Contact: 34, Account: 12 },
  users: 1,
  profiles: 1,
  permissions: 0,
});

assert.deepStrictEqual(rows, [
  { name: "Account", rows: 12 },
  { name: "Contact", rows: 34 },
]);

assert.deepStrictEqual(localOrg.summaryFromInspect({ objects: 2, records: 46, byObject: {} }), {
  objects: 2,
  records: 46,
  users: 0,
  profiles: 0,
  permissions: 0,
});
