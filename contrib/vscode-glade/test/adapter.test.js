const assert = require("assert");
const Module = require("module");

const originalLoad = Module._load;
Module._load = function patchedLoad(request, parent, isMain) {
  if (request === "vscode") {
    return {
      DebugAdapterExecutable: class DebugAdapterExecutable {
        constructor(command, args) {
          this.command = command;
          this.args = args;
        }
      },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const adapter = require("../out/adapter");

assert.deepStrictEqual(
  adapter.debugAdapterArgs({ project: "/repo", dbPath: "/repo/.glade/envs/dev.sqlite", dryRun: true }),
  ["dap", "--project", "/repo", "--db", "/repo/.glade/envs/dev.sqlite", "--dry-run"],
);

assert.deepStrictEqual(
  adapter.debugAdapterArgs({ project: "/repo" }),
  ["dap", "--project", "/repo"],
);

Module._load = originalLoad;
