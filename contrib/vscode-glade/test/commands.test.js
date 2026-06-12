const assert = require('assert');
const commands = require('../out/commandModel');

assert.strictEqual(
  commands.apexSourceFromDocument({ text: 'System.debug(1);\nSystem.debug(2);', selection: { start: 0, end: 16 } }),
  'System.debug(1);'
);

assert.deepStrictEqual(
  commands.execAnonymousArgs('System.debug(1);'),
  ['exec', '--debug-log', '-', 'System.debug(1);']
);

assert.deepStrictEqual(
  commands.execAnonymousArgs("System.debug('hi');", "/repo", "/repo/.glade/envs/dev.sqlite"),
  ["exec", "--debug-log", "-", "--project", "/repo", "--db", "/repo/.glade/envs/dev.sqlite", "System.debug('hi');"],
);

assert.deepStrictEqual(
  commands.debugAnonymousConfig('/tmp/project', 'System.debug(1);', '/tmp/project/.glade/envs/dev.sqlite'),
  {
    type: 'glade',
    request: 'launch',
    name: 'Glade: Debug Anonymous Apex',
    project: '/tmp/project',
    dbPath: '/tmp/project/.glade/envs/dev.sqlite',
    source: 'System.debug(1);',
  }
);

assert.deepStrictEqual(
  commands.debugTestConfig('/tmp/project', 'InvoiceServiceTest', 'updatesTotals', '/tmp/project/.glade/envs/dev.sqlite'),
  {
    type: 'glade',
    request: 'launch',
    name: 'Glade: Debug InvoiceServiceTest.updatesTotals',
    project: '/tmp/project',
    dbPath: '/tmp/project/.glade/envs/dev.sqlite',
    source: 'updatesTotals();',
    className: 'InvoiceServiceTest',
    methodName: 'updatesTotals',
  }
);

assert.strictEqual(
  commands.editorAnonymousSource({ text: '', selection: { start: 0, end: 0 } }),
  undefined
);

assert.strictEqual(
  commands.editorAnonymousSource({ text: '  \n\t', selection: { start: 0, end: 0 } }),
  undefined
);

assert.strictEqual(
  commands.editorAnonymousSource({ text: 'System.debug(1);', selection: { start: 0, end: 0 } }),
  'System.debug(1);'
);
