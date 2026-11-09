package lwcbrowser

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestPlatformWorkspaceAPIMatchesNativeLightningRows executes the generated
// lightning/platformWorkspaceApi module in node against the real workspace
// service. Expected values come from a native capture in a scratch org
// (Standard Lightning app, desktop Chromium, component API 67.0 and 59.0;
// see ORG-RUN notes in the B10 report). They are native rows, not inference.
func TestPlatformWorkspaceAPIMatchesNativeLightningRows(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	service, err := filepath.Abs("../../lwcruntime/src/shell/workspace-service.mjs")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	shim := strings.ReplaceAll(PlatformWorkspaceAPIModuleJS(), "/lightning/runtime/shell/workspace-service.js", "file://"+service)
	if err := os.WriteFile(filepath.Join(dir, "shim.mjs"), []byte(shim), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--input-type=module")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(platformWorkspaceNativeRowsJS)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("workspace native rows: %v\n%s", err, output)
	}
}

const platformWorkspaceNativeRowsJS = `import assert from "node:assert/strict";
globalThis.document = { getElementById: () => null };
globalThis.window = { location: { href: "http://localhost/lwc/preview" } };
const ws = await import("./shim.mjs");
const unsupported = (name) => ({ name: "Error", message: "Error: API ` + "`" + `" + name + "` + "`" + ` is not currently supported in this application." });
const missing = (message) => ({ name: "Error", message });
// Outside a console every tab method rejects after its argument check.
const rows = [
  ["closeTab", ["b10-missing-tab"], unsupported("closeTab")],
  ["closeTab", [null], missing("error: unable to close a tab - missing tabId")],
  ["refreshTab", ["b10-missing-tab"], unsupported("refreshTab")],
  ["refreshTab", ["b10-missing-tab", { includeAllSubtabs: true }], unsupported("refreshTab")],
  ["refreshTab", [null], missing("error: unable to refresh a tab - missing tabId")],
  ["setTabIcon", ["b10-missing-tab", "utility:add"], unsupported("setTabIcon")],
  ["setTabIcon", [null, null], missing("error: unable to set icon to a tab - missing tabId or icon")],
  ["setTabHighlighted", ["b10-missing-tab", true], unsupported("setTabHighlighted")],
  ["setTabHighlighted", [null, null], missing("error: unable to highlight a tab - missing tabId or highlighted flag")],
  ["disableTabClose", ["b10-missing-tab", true], unsupported("disableTabClose")],
  ["disableTabClose", [null, null], missing("error: unable to disable tab close - missing tabId or disabled flag")],
  ["getFocusedTabInfo", [], unsupported("getFocusedTabInfo")],
  ["getAllTabInfo", [], unsupported("getAllTabInfo")],
  ["getTabInfo", ["b10-missing-tab"], unsupported("getTabInfo")],
  ["getTabInfo", [null], missing("error: unable to get info for a tab - missing tabId")],
  ["focusTab", ["b10-missing-tab"], unsupported("focusTab")],
  ["setTabLabel", ["b10-missing-tab", "B10"], unsupported("setTabLabel")],
  ["setTabLabel", [null, null], missing("error: unable to set label to a tab - missing tabId or label")],
];
for (const [name, args, want] of rows) {
  await assert.rejects(() => ws[name](...args), (error) => {
    assert.deepEqual({ name: error.name, message: error.message }, want, name + JSON.stringify(args));
    return true;
  }, name + JSON.stringify(args));
}
assert.equal(await ws.openTab({ pageReference: { type: "standard__navItemPage", attributes: { apiName: "B10Probe" } }, focus: false }), null);
assert.equal(await ws.openSubtab("b10-missing-tab", { pageReference: { type: "standard__navItemPage", attributes: { apiName: "B10Probe" } }, focus: false }), null);
`

// TestPlatformWorkspaceAPIConsoleMatchesNativeRows covers the console half of
// the same capture: a Lightning console app, tab opened as a workspace tab.
func TestPlatformWorkspaceAPIConsoleMatchesNativeRows(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	service, err := filepath.Abs("../../lwcruntime/src/shell/workspace-service.mjs")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	shim := strings.ReplaceAll(PlatformWorkspaceAPIModuleJS(), "/lightning/runtime/shell/workspace-service.js", "file://"+service)
	if err := os.WriteFile(filepath.Join(dir, "shim.mjs"), []byte(shim), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--input-type=module")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(platformWorkspaceConsoleRowsJS)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("workspace console native rows: %v\n%s", err, output)
	}
}

const platformWorkspaceConsoleRowsJS = `import assert from "node:assert/strict";
globalThis.document = { getElementById: () => null };
globalThis.window = { location: { href: "http://localhost/lwc/preview" } };
const ws = await import("./shim.mjs");
ws.configureWorkspace({ console: true, focusedTabId: "t1", tabs: [{ tabId: "t1", label: "Home", url: "/lwc/preview/home", workspaceTab: true }] });
const MISS = "b10-missing-tab";
const reject = async (promise, message, label) => {
  await assert.rejects(() => promise, (error) => { assert.equal(error.name, "Error"); assert.equal(error.message, message, label); return true; }, label);
};
const NATIVE_KEYS = ["closeable", "customIcon", "customIconAlt", "customTitle", "focused", "highlighted", "icon", "iconAlt", "isSubtab", "pageReference", "parentTabId", "pinned", "recordId", "tabId", "title", "url"];
// Console rows for a tab id that does not exist.
await reject(ws.getTabInfo(MISS), "Error: getTabInfo() - Failed to get tabInfo for tabId ` + "`" + `" + MISS + "` + "`" + `", "getTabInfo");
await reject(ws.focusTab(MISS), "Error: focusTab() - Could not focus tab.", "focusTab");
await reject(ws.closeTab(MISS), "Error: closeTab() - Could not close tab.", "closeTab");
await reject(ws.refreshTab(MISS), "Error: Unable to refresh tab ` + "`" + `" + MISS + "` + "`" + `", "refreshTab");
await reject(ws.setTabLabel(MISS, "B10"), "Error: setTabLabel() - Failed to update tab label.", "setTabLabel");
await reject(ws.setTabIcon(MISS, "utility:add"), "Error: setTabIcon() - Failed to update tab icon.", "setTabIcon");
await reject(ws.setTabHighlighted(MISS, true), "Error: setTabHighlighted() - Failed to update tab highlight.", "setTabHighlighted");
await reject(ws.disableTabClose(MISS, true), "Error: disableTabClose() - Failed to update tab disable close.", "disableTabClose");
await reject(ws.openSubtab(MISS, { pageReference: { type: "standard__navItemPage", attributes: { apiName: "B10Probe" } } }), "Error: openSubtab() - Could not open subtab.", "openSubtab");
await reject(ws.closeTab(null), "error: unable to close a tab - missing tabId", "closeTab null");
// A subtab opened with focus: true.
const page = { type: "standard__objectPage", attributes: { objectApiName: "Account", actionName: "home" } };
const sid = await ws.openSubtab("t1", { pageReference: page, focus: true });
assert.equal(typeof sid, "string");
const focused = await ws.getFocusedTabInfo();
for (const key of NATIVE_KEYS) assert.ok(key in focused, key);
assert.equal(focused.tabId, sid);
assert.equal(focused.isSubtab, true);
assert.equal(focused.parentTabId, "t1");
assert.equal(focused.closeable, true);
assert.equal(focused.highlighted, false);
assert.equal(focused.pinned, false);
assert.equal(focused.recordId, null);
assert.equal(focused.customTitle, undefined);
assert.equal(focused.focused, true);
const all = await ws.getAllTabInfo();
assert.equal(all.length, 1);
assert.equal(all[0].isSubtab, false);
assert.equal(all[0].parentTabId, null);
assert.equal(all[0].focused, true);
assert.equal(all[0].subtabs.length, 1);
assert.equal(all[0].subtabs[0].tabId, sid);
assert.deepEqual({ ...await ws.getTabInfo(sid) }, { ...focused });
// Label, icon, highlight.
assert.equal((await ws.setTabLabel(sid, "B10 Label")).customTitle, "B10 Label");
await reject(ws.setTabLabel(sid, 42), "Error: Invalid or missing label ` + "`" + `42` + "`" + `", "label 42");
assert.equal((await ws.setTabLabel(sid, "")).customTitle, "");
const iconed = await ws.setTabIcon(sid, "utility:add", { iconAlt: "B10" });
assert.equal(iconed.customIcon, "utility:add");
assert.equal(iconed.customIconAlt, "B10");
assert.equal((await ws.setTabIcon(sid, "utility:add")).customIconAlt, undefined);
assert.equal((await ws.setTabHighlighted(sid, true, { pulse: true, state: "error" })).highlighted, true);
assert.equal((await ws.setTabHighlighted(sid, true, { state: "success" })).highlighted, true);
assert.equal((await ws.setTabHighlighted(sid, true, { state: "warning" })).highlighted, true);
assert.equal((await ws.setTabHighlighted(sid, true, { pulse: "x" })).highlighted, true);
assert.equal((await ws.setTabHighlighted(sid, false)).highlighted, false);
for (const bad of ["info", "bogus", "", true]) {
  await reject(ws.setTabHighlighted(sid, true, { state: bad }), "Error: Invalid value ` + "`" + `" + bad + "` + "`" + ` for state option property", "state " + bad);
}
// Closing is blocked while disableTabClose is on.
assert.equal((await ws.disableTabClose(sid, true)).closeable, false);
await reject(ws.closeTab(sid), "Error: closeTab() - Could not close tab.", "close disabled");
assert.equal((await ws.disableTabClose(sid, false)).closeable, true);
assert.equal(await ws.refreshTab(sid, { includeAllSubtabs: false }), true);
assert.equal(await ws.focusTab("t1"), true);
assert.equal((await ws.getTabInfo(sid)).focused, false);
// A subtab opened without focus does not take focus.
const quiet = await ws.openSubtab("t1", { pageReference: page });
assert.equal((await ws.getTabInfo(quiet)).focused, false);
assert.equal(await ws.closeTab(quiet), true);
assert.equal(await ws.closeTab(sid), true);
assert.equal((await ws.getAllTabInfo())[0].subtabs.length, 0);
`

// The floor (59.0) and ceiling (67.0) captures agree, so the shim carries no
// API-version gate. c.closeTab.workspace depends on tab load timing and is the
// one row that differed between runs.
func TestPlatformWorkspaceNativeCaptureIsAPIIndependent(t *testing.T) {
	read := func(name string) map[string]any {
		data, err := os.ReadFile(filepath.Join("testdata", "lwc_console_workspace", name))
		if err != nil {
			t.Fatal(err)
		}
		var rows map[string]any
		if err := json.Unmarshal(data, &rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if !reflect.DeepEqual(read("std59.json"), read("std67.json")) {
		t.Fatal("standard-app capture differs between API 59.0 and 67.0")
	}
	floor, ceiling := read("con59.json"), read("con67.json")
	delete(floor, "c.closeTab.workspace")
	delete(ceiling, "c.closeTab.workspace")
	if !reflect.DeepEqual(floor, ceiling) {
		t.Fatal("console capture differs between API 59.0 and 67.0")
	}
}
