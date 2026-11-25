const assert = require('assert');
const fs = require('fs');
const Module = require('module');
const os = require('os');
const path = require('path');
const originalLoad = Module._load;
class Collection {
  constructor() { this.items = new Map(); }
  add(item) { this.items.set(item.id, item); }
  get(id) { return this.items.get(id); }
  forEach(fn) { this.items.forEach(fn); }
  replace(items) { this.items.clear(); items.forEach(item => this.add(item)); }
}
class Item {
  constructor(id) { this.id = id; this.label = id; this.children = new Collection(); }
}
const events = [];
const controller = { items: new Collection(), createRunProfile() {}, createTestRun() { return { started() {}, passed: item => events.push(['passed', item.id]), failed: (item, msg) => events.push(['failed', item.id, msg.message]), errored: (item, msg) => events.push(['errored', item.id, msg.message]), end() {} }; } };
const vscode = { CancellationTokenSource: class { constructor() { this.token = { isCancellationRequested: false }; } dispose() {} }, TestRunRequest: class { constructor(include) { this.include = include; } }, tests: { createTestController: () => controller }, TestRunProfileKind: { Run: 1, Debug: 2 }, TestMessage: class { constructor(message) { this.message = message; } }, workspace: {}, window: {} };
vscode.RelativePattern = class {};
Module._load = function(request, parent, isMain) { return request === 'vscode' ? vscode : originalLoad.call(this, request, parent, isMain); };
const runner = require('../out/tests/runner');
const { GladeTestController } = require('../out/tests/controller');
const failedCounts = [];
const instance = new GladeTestController({ subscriptions: [] }, undefined, undefined, count => failedCounts.push(count));
instance.project = { projectRoot: '/repo' };
instance.logCommand = () => {};
const cls = new Item('Example');
const a = new Item('Example.a'); const b = new Item('Example.b');
a.parent = cls; b.parent = cls; cls.children.add(a); cls.children.add(b); controller.items.add(cls);
instance.itemData.set(cls, { className: 'Example' });
instance.itemData.set(a, { className: 'Example', methodName: 'a' });
instance.itemData.set(b, { className: 'Example', methodName: 'b' });
const calls = [];
let report;
runner.runApexTest = async (_project, className, methodName) => { calls.push([className, methodName]); return report; };
const makeReport = cases => ({ summary: { durationMs: 1 }, suites: cases });
const row = (methodName, status, message) => ({ className: 'Example', methodName, status, problem: message ? { message } : undefined });
const suite = cases => [{ name: 'Example', cases }];
async function run(include, exclude) { calls.length = 0; events.length = 0; await instance.run({ include, exclude }, { isCancellationRequested: false }); }
(async () => {
  report = makeReport([{ name: 'test selector', cases: [{ name: 'selector', status: 'runtime_error', problem: { type: 'Selector', message: 'no test method matched', detail: 'check the selector' } }] }]);
  await run([a]); assert(events.some(e => e[0] !== 'passed' && e[1] === a.id && e[2].includes('Selector') && e[2].includes('no test method matched') && e[2].includes('check the selector')));
  report = makeReport(suite([row('b', 'pass')]));
  await run([a]); assert(!events.some(e => e[0] === 'passed' && e[1] === a.id));
  report = makeReport(suite([row('a', 'fail', 'assertion failed')]));
  await run([a]); assert(events.some(e => e[0] === 'failed' && e[2].includes('assertion failed')));
  report = makeReport(suite([row('a', 'pass')]));
  await run([a]); assert(events.some(e => e[0] === 'passed' && e[1] === a.id));
  report = makeReport(suite([row('a', 'pass'), row('b', 'fail', 'unrelated')]));
  await run([a]); assert(events.some(e => e[0] === 'passed' && e[1] === a.id));
  report = makeReport([...suite([row('a', 'pass')]), { name: 'Other', cases: [{ className: 'Other', methodName: 'bad', status: 'fail', problem: { message: 'unrelated' } }] }]);
  await run([cls]); assert(events.some(e => e[0] === 'passed' && e[1] === cls.id));
  report = makeReport(suite([row('b', 'fail', 'unrelated')]));
  await run([a]); assert(events.some(e => e[0] === 'errored' && e[1] === a.id));
  report = makeReport(suite([row('a', 'pass'), row('b', 'pass')]));
  await run([cls], [b]); assert.deepStrictEqual(calls, [['Example', 'a']]);
  await run([cls, a], []); assert.deepStrictEqual(calls, [['Example', undefined]]);
  await run([a], [cls]); assert.deepStrictEqual(calls, []);
  await run([cls], [cls]); assert.deepStrictEqual(calls, []);
  await run([], []); assert.deepStrictEqual(calls, []);
  await run(undefined, []); assert.deepStrictEqual(calls, [['Example', undefined]]);
  report = makeReport(suite([row('a', 'fail', 'failure')]));
  await run([a]);
  const refreshedClass = new Item('Example');
  const refreshedA = new Item('Example.a');
  refreshedClass.children.add(refreshedA);
  controller.items.replace([refreshedClass]);
  instance.itemData.set(refreshedClass, { className: 'Example' });
  instance.itemData.set(refreshedA, { className: 'Example', methodName: 'a' });
  calls.length = 0; events.length = 0;
  await instance.runFailed();
  assert.deepStrictEqual(calls, [['Example', 'a']]);
  assert(events.some(e => e[0] === 'failed' && e[1] === refreshedA.id));

  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'glade-discovery-'));
  try {
    const source = path.join(root, 'Example.cls');
    fs.writeFileSync(source, '@IsTest class Example { @IsTest static void b() {} }');
    vscode.workspace.findFiles = async () => [{ fsPath: source }];
    controller.createTestItem = id => new Item(id);
    const messages = [];
    vscode.window.showInformationMessage = message => messages.push(message);
    instance.project = { projectRoot: root };
    await instance.discover();
    assert.strictEqual(controller.items.get('Example').children.get('Example.a'), undefined);
    assert.strictEqual(failedCounts.at(-1), 0);
    calls.length = 0;
    await instance.runFailed();
    assert.deepStrictEqual(calls, []);
    assert.deepStrictEqual(messages, ['Glade has no failed local Apex tests to rerun.']);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
