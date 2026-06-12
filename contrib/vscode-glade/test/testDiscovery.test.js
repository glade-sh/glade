const assert = require("assert");
const discovery = require("../out/tests/discovery");

const source = `
@IsTest
private class AccountServiceTest {
  @IsTest
  static void createsContact() {}

  testMethod static void legacyMethod() {}
}
`;

const tests = discovery.discoverApexTests("force-app/classes/AccountServiceTest.cls", source);
assert.deepStrictEqual(tests, {
  className: "AccountServiceTest",
  methods: ["createsContact", "legacyMethod"],
});

assert.deepStrictEqual(
  discovery.currentApexTestAtOffset("force-app/classes/AccountServiceTest.cls", source, source.indexOf("createsContact") + 3),
  { className: "AccountServiceTest", methodName: "createsContact" },
);

assert.deepStrictEqual(
  discovery.currentApexTestAtOffset("force-app/classes/AccountServiceTest.cls", source, source.indexOf("legacyMethod") + 3),
  { className: "AccountServiceTest", methodName: "legacyMethod" },
);

assert.strictEqual(
  discovery.currentApexTestAtOffset("force-app/classes/AccountServiceTest.cls", source, source.indexOf("private class")),
  undefined,
);
