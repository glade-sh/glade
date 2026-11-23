// Golden results are captured from the pinned unmodified compiler.
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const require = createRequire(import.meta.url);
const cjs = require("@lwc/style-compiler/dist/index.cjs");
const esm = await import("@lwc/style-compiler");
const hash = (s) => createHash("sha256").update(s).digest("hex");

function walk(dir, suffix, found = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
    const file = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(file, suffix, found);
    else if (entry.isFile() && entry.name.endsWith(suffix)) found.push(file);
  }
  return found;
}
function sources() {
  const items = [];
  for (const dir of ["internal/lwc", "testdata/local-tests"]) {
    for (const file of walk(path.join(root, dir), ".css")) {
      items.push([path.relative(root, file), fs.readFileSync(file, "utf8")]);
    }
  }
  for (const file of walk(path.join(root, "internal/lwc/compile/testdata"), ".json")) {
    const json = JSON.parse(fs.readFileSync(file, "utf8"));
    function visit(value, id) {
      if (!value || typeof value !== "object") return;
      if (typeof value.css === "string") items.push([`${path.relative(root, file)}:${id}.css`, value.css]);
      for (const [key, child] of Object.entries(value)) if (child && typeof child === "object") visit(child, `${id}.${key}`);
    }
    visit(json, "$");
  }
  for (const file of walk(path.join(root, "internal/lwc/compile"), "_test.go")) {
    const content = fs.readFileSync(file, "utf8");
    const expr = /writeCompileFixtureFile\(t, filepath\.Join\([^\n]*?\.css"\),\s*`([^`]*)`\)/g;
    let match, i = 0;
    while ((match = expr.exec(content))) items.push([`${path.relative(root, file)}:css${++i}`, match[1]]);
  }
  for (const [i, css] of [
    ".plain:dir(ltr)::before, .other:hover { color: red }",
    ":host(.one, .two) > .child, .plain { color: blue }",
    ":host { color: red } .plain { color: blue } :host(.x) { color: green }",
    "@keyframes pulse { from { opacity: 0 } to { opacity: 1 } } .x { animation: pulse 1s }",
    ".broken:dir(invalid) { color: red } #bad { color: blue }",
    "/* malformed */ .x { color: red",
    ":is(.x, :host) { color: red }",
  ].entries()) items.push([`control:${i}`, css]);
  return items;
}
function result(transform, source, config) {
  try { return { code: transform(source, "fixture.css", config).code }; }
  catch (error) {
    function clean(e) {
      return { name: e.name, message: String(e.message).split(process.cwd()).join("<cwd>"), reason: e.reason, line: e.line, column: e.column, errors: e.errors?.map(clean) };
    }
    return { error: clean(error) };
  }
}
export function capture() {
  const output = {};
  for (const [name, source] of sources()) {
    const cases = [];
    for (const apiVersion of [59, 67, 68]) for (const scoped of [false, true]) for (const disableSyntheticShadowSupport of [false, true]) for (const experimentalErrorRecoveryMode of [false, true]) {
      const options = { apiVersion, scoped, disableSyntheticShadowSupport, experimentalErrorRecoveryMode };
      const a = result(cjs.transform, source, options);
      const b = result(esm.transform, source, options);
      if (JSON.stringify(a) !== JSON.stringify(b)) throw new Error(`ESM and CJS differ: ${name} ${JSON.stringify(options)}`);
      cases.push(a);
    }
    if (Object.hasOwn(output, name)) throw new Error(`duplicate fixture ${name}`);
    output[name] = hash(JSON.stringify(cases));
  }
  return output;
}
if (process.argv[2] === "--capture") process.stdout.write(`${JSON.stringify(capture(), null, 2)}\n`);
