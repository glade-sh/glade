const assert = require("assert");
const actions = require("../out/environmentActions");

const existing = [
  { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  { name: "bug-123", dbPath: "/repo/.glade/envs/bug-123.sqlite", fixturePath: "/repo/fixtures/bug-123.json" },
];

assert.deepStrictEqual(
  actions.addEnvironment(existing, { name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite" }),
  [...existing, { name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite" }],
);

assert.throws(
  () => actions.addEnvironment(existing, { name: "dev", dbPath: "/repo/.glade/envs/other.sqlite" }),
  /environment "dev" already exists/,
);

assert.deepStrictEqual(
  actions.removeEnvironment(existing, "bug-123"),
  [{ name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" }],
);

assert.throws(() => actions.removeEnvironment(existing, "dev"), /cannot delete the dev environment/);
assert.strictEqual(actions.cloneName("bug-123"), "bug-123-copy");
assert.strictEqual(actions.cloneName("bug-123-copy"), "bug-123-copy-2");
assert.strictEqual(
  actions.nextCloneName("bug-123", [
    ...existing,
    { name: "bug-123-copy", dbPath: "/repo/.glade/envs/bug-123-copy.sqlite" },
  ]),
  "bug-123-copy-2",
);
assert.deepStrictEqual(actions.clonedEnvironment(existing[0], "/repo"), {
  name: "dev-copy",
  dbPath: "/repo/.glade/envs/dev-copy.sqlite",
  fixturePath: undefined,
});
assert.deepStrictEqual(
  actions.clonedEnvironment(existing[1], "/repo", [
    ...existing,
    { name: "bug-123-copy", dbPath: "/repo/.glade/envs/bug-123-copy.sqlite" },
  ]),
  {
    name: "bug-123-copy-2",
    dbPath: "/repo/.glade/envs/bug-123-copy-2.sqlite",
    fixturePath: "/repo/fixtures/bug-123.json",
  },
);
assert.deepStrictEqual(actions.settingsValue(existing, "/repo"), [
  { name: "dev", dbPath: ".glade/envs/dev.sqlite", fixturePath: undefined },
  { name: "bug-123", dbPath: ".glade/envs/bug-123.sqlite", fixturePath: "fixtures/bug-123.json" },
]);
