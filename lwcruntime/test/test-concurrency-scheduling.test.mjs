import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const packageJSON = JSON.parse(
  await readFile(new URL("../package.json", import.meta.url), "utf8"),
);

test("runtime test files use bounded configurable concurrency", () => {
  assert.match(
    packageJSON.scripts.test,
    /--test-concurrency=\$\{GLADE_LWCRUNTIME_TEST_CONCURRENCY:-2\}/,
  );
  assert.match(packageJSON.scripts.test, /GLADE_LWCRUNTIME_TEST_BINARY=bin\/glade/);
  assert.match(packageJSON.scripts.test, /go build -o bin\/glade \.\/cmd\/glade/);
});
