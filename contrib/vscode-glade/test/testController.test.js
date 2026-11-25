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

    // A class run with one failing and one passing method records the method
    // and the class. Deleting the failing method must clear both.
    const both = '@IsTest class Example { @IsTest static void a() {} @IsTest static void b() {} }';
    const onlyB = '@IsTest class Example { @IsTest static void b() {} }';
    const runClass = async () => { calls.length = 0; events.length = 0; await instance.run({ include: [controller.items.get('Example')] }, { isCancellationRequested: false }); };
    fs.writeFileSync(source, both);
    await instance.discover();
    report = makeReport(suite([row('a', 'fail', 'assertion failed'), row('b', 'pass')]));
    await runClass();
    assert(events.some(e => e[0] === 'failed' && e[1] === 'Example') && events.some(e => e[0] === 'failed' && e[1] === 'Example.a'));
    assert.strictEqual(failedCounts.at(-1), 2);
    calls.length = 0;
    await instance.runFailed();
    assert.deepStrictEqual(calls, [['Example', undefined]]);
    assert.strictEqual(failedCounts.at(-1), 2);
    fs.writeFileSync(source, onlyB);
    await instance.discover();
    assert.strictEqual(failedCounts.at(-1), 0);
    calls.length = 0; messages.length = 0;
    await instance.runFailed();
    assert.deepStrictEqual(calls, []);
    assert.deepStrictEqual(messages, ['Glade has no failed local Apex tests to rerun.']);

    // The class also stops being failed when its only failing method passes.
    fs.writeFileSync(source, both);
    await instance.discover();
    report = makeReport(suite([row('a', 'fail', 'assertion failed'), row('b', 'pass')]));
    await runClass();
    assert.strictEqual(failedCounts.at(-1), 2);
    report = makeReport(suite([row('a', 'pass')]));
    calls.length = 0;
    await instance.run({ include: [controller.items.get('Example').children.get('Example.a')] }, { isCancellationRequested: false });
    assert.strictEqual(failedCounts.at(-1), 0);

    // With two failing methods the class stays failed until both are gone.
    report = makeReport(suite([row('a', 'fail', 'first'), row('b', 'fail', 'second')]));
    await runClass();
    assert.strictEqual(failedCounts.at(-1), 3);
    fs.writeFileSync(source, onlyB);
    await instance.discover();
    assert.strictEqual(failedCounts.at(-1), 2);
    calls.length = 0;
    await instance.runFailed();
    assert.deepStrictEqual(calls, [['Example', undefined]]);

    // A class that failed on its own, with no failed method behind it, is
    // kept when discovery drops one of its methods.
    fs.writeFileSync(source, both);
    await instance.discover();
    report = makeReport(suite([row('a', 'pass'), row('b', 'pass')]));
    await runClass();
    assert.strictEqual(failedCounts.at(-1), 0);
    // Refresh discovery after the class run starts and before its report arrives.
    const runAcrossRefresh = async results => {
      fs.writeFileSync(source, both);
      await instance.discover();
      const normalRunner = runner.runApexTest;
      let finish;
      runner.runApexTest = () => new Promise(resolve => { finish = resolve; });
      try {
        const pending = runClass();
        assert.strictEqual(typeof finish, 'function');
        fs.writeFileSync(source, onlyB);
        await instance.discover();
        finish(makeReport(suite(results)));
        await pending;
      } finally {
        runner.runApexTest = normalRunner;
      }
    };
    await runAcrossRefresh([row('a', 'fail', 'removed during run'), row('b', 'pass')]);
    calls.length = 0; messages.length = 0;
    await instance.runFailed();
    assert.deepStrictEqual(calls, []);
    assert.deepStrictEqual(messages, ['Glade has no failed local Apex tests to rerun.']);
    assert.strictEqual(failedCounts.at(-1), 0);
    assert(events.some(e => e[0] === 'failed' && e[1] === 'Example.a'));

    // A failure from a method still in discovery keeps the class eligible.
    await runAcrossRefresh([row('a', 'fail', 'removed during run'), row('b', 'fail', 'still present')]);
    assert.strictEqual(failedCounts.at(-1), 2);
    calls.length = 0;
    report = makeReport(suite([row('b', 'pass')]));
    await instance.runFailed();
    assert.deepStrictEqual(calls, [['Example', undefined]]);
    assert.strictEqual(failedCounts.at(-1), 0);

    report = makeReport([{ name: 'test selector', cases: [{ name: 'selector', status: 'runtime_error', problem: { type: 'Selector', message: 'no test method matched' } }] }]);
    await runClass();
    assert.strictEqual(failedCounts.at(-1), 1);
    fs.writeFileSync(source, onlyB);
    await instance.discover();
    assert.strictEqual(failedCounts.at(-1), 1);
    calls.length = 0;
    await instance.runFailed();
    assert.deepStrictEqual(calls, [['Example', undefined]]);

    vscode.workspace.findFiles = async () => [];
    await instance.discover();
    assert.strictEqual(failedCounts.at(-1), 0);

    fs.writeFileSync(source, both);
    vscode.workspace.findFiles = async () => [{ fsPath: source }];
    await instance.discover();
    report = makeReport(suite([row('a', 'fail', 'before project switch'), row('b', 'pass')]));
    await runClass();
    assert.strictEqual(failedCounts.at(-1), 2);
    instance.setProject({ projectRoot: path.join(root, 'other-project') });
    await instance.discover();
    assert.strictEqual(failedCounts.at(-1), 0);
    calls.length = 0;
    await instance.runFailed();
    assert.deepStrictEqual(calls, []);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
