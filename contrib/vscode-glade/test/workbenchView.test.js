const assert = require("assert");
const Module = require("module");

const originalLoad = Module._load;
Module._load = function patchedLoad(request, parent, isMain) {
  if (request === "vscode") {
    class TreeItem {
      constructor(label, collapsibleState) {
        this.label = label;
        this.collapsibleState = collapsibleState;
      }
    }
    class ThemeIcon {
      constructor(id) {
        this.id = id;
      }
    }
    class EventEmitter {
      constructor() {
        this.event = () => undefined;
      }
      fire() {}
    }
    return {
      TreeItem,
      ThemeIcon,
      EventEmitter,
      TreeItemCollapsibleState: { None: 0, Expanded: 2 },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const { WorkbenchView, toWorkbenchTreeItem } = require("../out/views/workbenchView");

const rows = [
  { id: "environment", type: "environment", label: "Environment", description: "dev" },
  { id: "soql", type: "group", kind: "soql", label: "SOQL", count: 1 },
  { id: "soql:soql-1", type: "entry", kind: "soql", entryId: "soql-1", label: "Accounts", description: "Updated now" },
];

const view = new WorkbenchView();
assert.strictEqual(view.getChildren().length, 2);
assert.strictEqual(view.getChildren()[0].label, "Anonymous Apex");
assert.strictEqual(view.getChildren()[0].command.command, "glade.workbench.newAnonymousApex");
assert.strictEqual(view.getChildren()[1].label, "SOQL");
assert.strictEqual(view.getChildren()[1].command.command, "glade.workbench.newSoql");

view.setRows(rows);
assert.strictEqual(view.getChildren().length, rows.length + 2);

const soqlItem = toWorkbenchTreeItem(rows[2]);
assert.deepStrictEqual(soqlItem.command, {
  command: "glade.workbench.runEntry",
  title: "Accounts",
  arguments: ["soql-1"],
});

const groupItem = toWorkbenchTreeItem(rows[1]);
assert.strictEqual(groupItem.description, "1");
assert.strictEqual(groupItem.command, undefined);
