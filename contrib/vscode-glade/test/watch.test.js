const assert = require('assert');
const watch = require('../out/tests/watchModel');

assert.deepStrictEqual(
  watch.watchArgs({ projectRoot: '/repo', packageDirectories: [], salesforceExtensions: {} }),
  ['test', '--project', '/repo', '--daemon', '--watch'],
);
