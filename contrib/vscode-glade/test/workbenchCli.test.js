const assert = require("assert");
const cli = require("../out/workbench/cli");

assert.deepStrictEqual(
  cli.queryArgs("/repo", "/repo/.glade/envs/dev.sqlite", "SELECT Id FROM Account", 50),
  ["db", "query", "--json", "--project", "/repo", "--db", "/repo/.glade/envs/dev.sqlite", "--limit", "50", "SELECT Id FROM Account"],
);

assert.deepStrictEqual(
  cli.queryArgs("/repo", "/repo/.glade/envs/dev.sqlite", "SELECT Id FROM Account"),
  ["db", "query", "--json", "--project", "/repo", "--db", "/repo/.glade/envs/dev.sqlite", "SELECT Id FROM Account"],
);

assert.deepStrictEqual(
  cli.describeArgs("/repo", "/repo/.glade/envs/dev.sqlite", "Account"),
  ["db", "describe", "--json", "--project", "/repo", "--db", "/repo/.glade/envs/dev.sqlite", "Account"],
);

assert.deepStrictEqual(
  cli.describeArgs("/repo", "/repo/.glade/envs/dev.sqlite"),
  ["db", "describe", "--json", "--project", "/repo", "--db", "/repo/.glade/envs/dev.sqlite"],
);

assert.throws(() => cli.queryArgs("/repo", "/repo/.glade/envs/dev.sqlite", "SELECT Id FROM Account", 0), /limit must be a positive integer/);

assert.deepStrictEqual(
  cli.parseQueryOutput(JSON.stringify({ records: [{ Id: "001", Name: "Acme" }], done: true, totalSize: 1 })),
  { records: [{ Id: "001", Name: "Acme" }], done: true, totalSize: 1, columns: ["Id", "Name"] },
);
