package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestRunDevLWCHelpUsesLWCHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(t.Context(), []string{"dev", "lwc", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	got := stdout.String()
	wantRoutes := strings.Join([]string{
		"Preview routes:",
		"  /lwc/preview/component/c/contextProbe",
		"  /lwc/preview/record/Account/<recordId>?page=Account_Record_Page",
		"  /lwc/preview/app/App_Page",
		"  /lwc/preview/home/Home_Page",
		"  /lwc/preview/tab/Lwc_Probe",
		"  /lwc/preview/utility/Support_Utility",
		"  /lwc/preview/flow/Membership_Flow",
		"  /lwc/preview/cmp/c/actionProbe?c__mode=demo",
		"  /lwc/preview/action/Account/<recordId>/Update_Status",
		"  /lwc/preview/action/global/Global_Status",
		"  /lwc/preview/community/Partner_Portal/Account",
		"  /lwc/preview/community/Partner_Portal/cmp/c/communityProbe",
	}, "\n")
	if !strings.Contains(got, wantRoutes) {
		t.Fatalf("missing clean preview route block:\n%s\nin help:\n%s", wantRoutes, got)
	}
	for _, want := range []string{
		"Start a local LWC preview development shell",
		"Preview feature:",
		"glade dev lwc [--project <root>] [--db <path>] [--context <name>] [--open]",
		"--db .glade/envs/lwc-preview.sqlite",
		"--target component|record-page|app-page|home-page|tab|url-addressable|record-action|global-action|utility-bar|flow-screen|flow-action",
		"--action Update_Status",
		"--flow Membership_Flow",
		"--state key=value",
		"Preview routes:",
		"/lwc/preview/component/c/contextProbe",
		"/lwc/preview/tab/Lwc_Probe",
		"/lwc/preview/utility/Support_Utility",
		"/lwc/preview/flow/Membership_Flow",
		"/lwc/preview/cmp/c/actionProbe?c__mode=demo",
		"/lwc/preview/action/Account/<recordId>/Update_Status",
		"/lwc/preview/action/global/Global_Status",
		"/lwc/preview/community/Partner_Portal/Account",
		"/lwc/preview/community/Partner_Portal/cmp/c/communityProbe",
		"Community routes open from named contexts in glade.lwc.json.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "community-page") {
		t.Fatalf("help advertises community as a direct target flag: %s", got)
	}
	if strings.Contains(got, "Visualforce_Tab") {
		t.Fatalf("help contains stale Visualforce_Tab route: %s", got)
	}
}

func TestDevLWCRoutesListComponentsPagesTabsAndVisualforce(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets>
    <target>lightning__RecordPage</target>
    <target>lightning__UrlAddressable</target>
  </targets>
</LightningComponentBundle>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/actionProbe/actionProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordAction</target></targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordAction"><actionType>ScreenAction</actionType></targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml"), `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Account</sobjectType>
  <type>RecordPage</type>
</FlexiPage>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flexipages/Support_Utility.flexipage-meta.xml"), `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <type>UtilityBar</type>
</FlexiPage>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml"), `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>LWC Probe</label>
  <lwcComponent>c:contextProbe</lwcComponent>
</CustomTab>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/tabs/VF_Probe.tab-meta.xml"), `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>VF Probe</label>
  <page>WidgetHost</page>
</CustomTab>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/quickActions/Account.Update_Status.quickAction-meta.xml"), `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Update Status</label>
  <targetObject>Account</targetObject>
  <lightningComponent>c:actionProbe</lightningComponent>
</QuickAction>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/quickActions/Global_Status.quickAction-meta.xml"), `<QuickAction xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Global Status</label>
  <lightningComponent>c:actionProbe</lightningComponent>
</QuickAction>`)
	writeTestFile(t, filepath.Join(root, "glade.lwc.json"), `{"contexts":{"communityAccount":{"target":"communityPage","component":"c:contextProbe","page":"Account","community":{"site":"Partner_Portal","basePath":"/partners"}},"membershipFlow":{"target":"flowScreen","component":"c:contextProbe","flow":{"apiName":"Membership_Flow"}}}}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	routes := devLWCPreviewRoutes(p)
	got := strings.Join(routes, "\n")
	for _, want := range []string{
		"/lwc/preview/component/c/contextProbe",
		"/lwc/preview/cmp/c/contextProbe?c__name=value",
		"/lwc/preview/record/Account/<recordId>?page=Account_Record_Page",
		"/lwc/preview/action/Account/<recordId>/Update_Status",
		"/lwc/preview/action/global/Global_Status",
		"/lwc/preview/community/Partner_Portal/Account",
		"/lwc/preview/flow/Membership_Flow",
		"/lwc/preview/utility/Support_Utility",
		"/lwc/preview/tab/Lwc_Probe",
		"/lwc/preview/tab/VF_Probe -> /apex/WidgetHost",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in routes:\n%s", want, got)
		}
	}
}

func TestWriteDevLWCReadyFileWritesShellRoutes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
</LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	readyPath := filepath.Join(t.TempDir(), "ready.json")
	selection := devLWCSelection{
		Name: "accountRecord",
		Context: lwcshell.PageContext{
			Kind:          lwcshell.RenderTargetRecordPage,
			ObjectAPIName: "Account",
			RecordID:      "001000000000001AAA",
			PageName:      "Account_Record_Page",
		},
	}
	if err := writeDevLWCReadyFile(readyPath, "127.0.0.1:39410", p, selection); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(readyPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		URL             string   `json:"url"`
		Addr            string   `json:"addr"`
		Routes          []string `json:"routes"`
		SelectedURL     string   `json:"selectedUrl"`
		SelectedContext string   `json:"selectedContext"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("readiness file is not JSON: %v\n%s", err, data)
	}
	if got.URL != "http://127.0.0.1:39410" || got.Addr != "127.0.0.1:39410" {
		t.Fatalf("ready = %#v", got)
	}
	if len(got.Routes) != 1 || got.Routes[0] != "/lwc/preview/component/c/contextProbe" {
		t.Fatalf("routes = %#v", got.Routes)
	}
	if got.SelectedContext != "accountRecord" {
		t.Fatalf("selectedContext = %q", got.SelectedContext)
	}
	wantURL := "http://127.0.0.1:39410/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page"
	if got.SelectedURL != wantURL {
		t.Fatalf("selectedUrl = %q, want %q", got.SelectedURL, wantURL)
	}
}

func TestDevLWCSelectedURLUsesContext(t *testing.T) {
	ctx := lwcshell.PageContext{
		Kind:          lwcshell.RenderTargetRecordPage,
		ObjectAPIName: "Account",
		RecordID:      "001000000000001AAA",
		PageName:      "Account_Record_Page",
		FormFactor:    "Large",
		State:         map[string]string{"c__mode": "demo"},
	}

	got := devLWCSelectedURL("http://127.0.0.1:39410", ctx)

	want := "http://127.0.0.1:39410/lwc/preview/record/Account/001000000000001AAA?formFactor=Large&page=Account_Record_Page&state.c__mode=demo"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestDevLWCSelectedURLUsesURLAddressableAndActionContexts(t *testing.T) {
	cases := []struct {
		name string
		ctx  lwcshell.PageContext
		want string
	}{
		{
			name: "url addressable",
			ctx: lwcshell.PageContext{
				Kind:          lwcshell.RenderTargetURLAddressable,
				ComponentName: "c:actionProbe",
				AppName:       "Sales",
				State:         map[string]string{"c__mode": "demo"},
			},
			want: "http://127.0.0.1:39410/lwc/preview/cmp/c/actionProbe?app=Sales&state.c__mode=demo",
		},
		{
			name: "component with record context",
			ctx: lwcshell.PageContext{
				Kind:          lwcshell.RenderTargetComponent,
				ComponentName: "c:recordProbe",
				ObjectAPIName: "Account",
				RecordID:      "001000000000001AAA",
				AppName:       "Sales",
				FormFactor:    "Large",
			},
			want: "http://127.0.0.1:39410/lwc/preview/component/c/recordProbe?app=Sales&formFactor=Large&objectApiName=Account&recordId=001000000000001AAA",
		},
		{
			name: "record action",
			ctx: lwcshell.PageContext{
				Kind:          lwcshell.RenderTargetQuickAction,
				ObjectAPIName: "Account",
				RecordID:      "001000000000001AAA",
				ActionName:    "Account.Update_Status",
			},
			want: "http://127.0.0.1:39410/lwc/preview/action/Account/001000000000001AAA/Update_Status",
		},
		{
			name: "global action",
			ctx: lwcshell.PageContext{
				Kind:       lwcshell.RenderTargetQuickAction,
				ActionName: "Global_Status",
			},
			want: "http://127.0.0.1:39410/lwc/preview/action/global/Global_Status",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := devLWCSelectedURL("http://127.0.0.1:39410", tc.ctx); got != tc.want {
				t.Fatalf("url = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDevLWCOptionsSupportUtilityAndFlowTargets(t *testing.T) {
	opts, err := parseDevLWCOptions([]string{
		"--target", "flow-screen",
		"--component", "c:flowProbe",
		"--flow", "Membership_Flow",
		"--flow-input", "recordId=001000000000001AAA",
		"--form-factor", "Small",
	})
	if err != nil {
		t.Fatal(err)
	}
	selection, err := opts.selection()
	if err != nil {
		t.Fatal(err)
	}
	if selection.Context.Kind != lwcshell.RenderTargetFlowScreen {
		t.Fatalf("kind = %q", selection.Context.Kind)
	}
	if selection.Context.ComponentName != "c:flowProbe" || selection.Context.Flow.APIName != "Membership_Flow" {
		t.Fatalf("flow context = %#v", selection.Context)
	}
	if got := selection.Context.Flow.InputVariables["recordId"]; got != "001000000000001AAA" {
		t.Fatalf("flow inputs = %#v", selection.Context.Flow.InputVariables)
	}
	if selection.Context.FormFactor != "Small" {
		t.Fatalf("formFactor = %q", selection.Context.FormFactor)
	}
	if got := devLWCSelectedURL("http://127.0.0.1:39410", selection.Context); got != "http://127.0.0.1:39410/lwc/preview/flow/Membership_Flow?formFactor=Small" {
		t.Fatalf("selected URL = %q", got)
	}

	opts, err = parseDevLWCOptions([]string{"--target", "utility-bar", "--page", "Support_Utility"})
	if err != nil {
		t.Fatal(err)
	}
	selection, err = opts.selection()
	if err != nil {
		t.Fatal(err)
	}
	if selection.Context.Kind != lwcshell.RenderTargetUtilityBar || selection.Context.PageName != "Support_Utility" {
		t.Fatalf("utility context = %#v", selection.Context)
	}
}

func TestDevLWCOptionsApplyContextPresetAndExplicitOverrides(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "glade.lwc.json"), `{
  "contexts": {
    "accountRecord": {
      "target": "recordPage",
      "objectApiName": "Account",
      "recordId": "001000000000001AAA",
      "page": "Account_Record_Page",
      "formFactor": "Large",
      "state": {"c__mode": "preset"}
    }
  }
}`)
	opts, err := parseDevLWCOptions([]string{
		"--project", root,
		"--context", "accountRecord",
		"--record", "001000000000002AAA",
		"--action", "Update_Status",
		"--form-factor", "Small",
		"--state", "c__mode=override",
	})
	if err != nil {
		t.Fatal(err)
	}

	selection, err := opts.selection()
	if err != nil {
		t.Fatal(err)
	}

	if selection.Name != "accountRecord" {
		t.Fatalf("selection name = %q", selection.Name)
	}
	if selection.Context.RecordID != "001000000000002AAA" {
		t.Fatalf("recordId = %q", selection.Context.RecordID)
	}
	if selection.Context.ActionName != "Update_Status" {
		t.Fatalf("actionName = %q", selection.Context.ActionName)
	}
	if selection.Context.FormFactor != "Small" {
		t.Fatalf("formFactor = %q", selection.Context.FormFactor)
	}
	if selection.Context.State["c__mode"] != "override" {
		t.Fatalf("state = %#v", selection.Context.State)
	}
}

func TestRunDevLWCOpenUsesStubbedOpener(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
</LightningComponentBundle>`)
	writeTestFile(t, filepath.Join(root, "glade.lwc.json"), `{
  "contexts": {
    "componentProbe": {
      "target": "component",
      "component": "c:contextProbe"
    }
  }
}`)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var opened string
	oldOpen := devLWCOpenURL
	devLWCOpenURL = func(url string) error {
		opened = url
		cancel()
		return nil
	}
	defer func() { devLWCOpenURL = oldOpen }()

	err := runDevLWC(ctx, []string{"--project", root, "--addr", "127.0.0.1:0", "--context", "componentProbe", "--open"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(opened, "/lwc/preview/component/c/contextProbe") {
		t.Fatalf("opened = %q", opened)
	}
}

func TestRunDevLWCOpenDefaultsToWorkbench(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
</LightningComponentBundle>`)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var opened string
	oldOpen := devLWCOpenURL
	devLWCOpenURL = func(url string) error {
		opened = url
		cancel()
		return nil
	}
	defer func() { devLWCOpenURL = oldOpen }()

	err := runDevLWC(ctx, []string{"--project", root, "--addr", "127.0.0.1:0", "--open"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(opened, "/lwc") {
		t.Fatalf("opened = %q", opened)
	}
}

func TestRunDevLWCOpenDefaultsToWorkbenchWhenProjectHasDefaultContext(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
</LightningComponentBundle>`)
	writeTestFile(t, filepath.Join(root, "glade.lwc.json"), `{
  "defaultContext": "accountRecord",
  "contexts": {
    "accountRecord": {
      "target": "recordPage",
      "objectApiName": "Account",
      "recordId": "001000000000001AAA",
      "page": "Account_Record_Page"
    }
  }
}`)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var opened string
	var stdout bytes.Buffer
	oldOpen := devLWCOpenURL
	devLWCOpenURL = func(url string) error {
		opened = url
		cancel()
		return nil
	}
	defer func() { devLWCOpenURL = oldOpen }()

	err := runDevLWC(ctx, []string{"--project", root, "--addr", "127.0.0.1:0", "--open"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(opened, "/lwc") {
		t.Fatalf("opened = %q", opened)
	}
	if !strings.Contains(stdout.String(), "Default context accountRecord: ") {
		t.Fatalf("stdout missing default context label:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Selected context accountRecord: ") {
		t.Fatalf("stdout should not call implicit default context selected:\n%s", stdout.String())
	}
}

func TestRunDevLWCUsesDBForLocalBuilderSearch(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
</LightningComponentBundle>`)
	dbPath := filepath.Join(root, ".glade", "envs", "lwc-preview.sqlite")
	store, org, err := openDBStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	storage.EnsureStandardObject(&org, "Account")
	account := org.Objects["Account"]
	if account.Records == nil {
		account.Records = make(map[storage.ID]storage.Record)
	}
	account.Records["001DBPREVIEW001"] = storage.Record{
		ID:     "001DBPREVIEW001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Database Preview Account"),
		},
	}
	org.Objects["Account"] = account
	if err := store.Save(org); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	readyPath := filepath.Join(t.TempDir(), "ready.json")
	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- runDevLWC(ctx, []string{"--project", root, "--addr", "127.0.0.1:0", "--db", dbPath, "--ready-file", readyPath, "--no-progress"}, &stdout, &stderr)
	}()

	ready := waitForDevLWCReadyFileOrDone(t, readyPath, done, &stdout, &stderr)
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ready.URL + "/lightning/local/records.json?object=Account&q=database")
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var payload struct {
		Records []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("runDevLWC error = %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	if len(payload.Records) != 1 || payload.Records[0].ID != "001DBPREVIEW001" || payload.Records[0].Title != "Database Preview Account" {
		t.Fatalf("records = %#v", payload.Records)
	}
}

const devLWCReadyFileTimeout = 30 * time.Second

func waitForDevLWCReadyFile(t *testing.T, path string) devLWCReadyFile {
	t.Helper()
	deadline := time.Now().Add(devLWCReadyFileTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			var ready devLWCReadyFile
			if err := json.Unmarshal(data, &ready); err != nil {
				t.Fatalf("ready file JSON: %v\n%s", err, data)
			}
			if ready.URL != "" {
				return ready
			}
		} else {
			lastErr = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for ready file %s: %v", path, lastErr)
	return devLWCReadyFile{}
}

func waitForDevLWCReadyFileOrDone(t *testing.T, path string, done <-chan error, stdout, stderr *bytes.Buffer) devLWCReadyFile {
	t.Helper()
	deadline := time.Now().Add(devLWCReadyFileTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("runDevLWC error before ready file = %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
			t.Fatalf("runDevLWC exited before ready file stdout=%q stderr=%q", stdout.String(), stderr.String())
		default:
		}
		data, err := os.ReadFile(path)
		if err == nil {
			var ready devLWCReadyFile
			if err := json.Unmarshal(data, &ready); err != nil {
				t.Fatalf("ready file JSON: %v\n%s", err, data)
			}
			if ready.URL != "" {
				return ready
			}
		} else {
			lastErr = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for ready file %s: %v stdout=%q stderr=%q", path, lastErr, stdout.String(), stderr.String())
	return devLWCReadyFile{}
}
