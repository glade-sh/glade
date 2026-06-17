package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
)

func TestRunDevLWCHelpUsesLWCHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(t.Context(), []string{"dev", "lwc", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"Start a local LWC preview development shell",
		"Preview feature:",
		"glade dev lwc [--project <root>] [--context <name>] [--open]",
		"--target component|record-page|app-page|home-page|tab|url-addressable|record-action|global-action",
		"--action Update_Status",
		"--state key=value",
		"Preview routes:",
		"/lwc/preview/component/c/contextProbe",
		"/lwc/preview/tab/Lwc_Probe",
		"/lwc/preview/cmp/c/actionProbe?c__mode=demo",
		"/lwc/preview/action/Account/<recordId>/Update_Status",
		"/lwc/preview/action/global/Global_Status",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
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
	writeTestFile(t, filepath.Join(root, "glade.lwc.json"), `{"contexts":{"communityAccount":{"target":"communityPage","component":"c:contextProbe","page":"Account","community":{"site":"Partner_Portal","basePath":"/partners"}}}}`)
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

	err := runDevLWC(ctx, []string{"--project", root, "--addr", "127.0.0.1:0", "--context", "componentProbe", "--open"}, &bytes.Buffer{})
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

	err := runDevLWC(ctx, []string{"--project", root, "--addr", "127.0.0.1:0", "--open"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(opened, "/lwc") {
		t.Fatalf("opened = %q", opened)
	}
}
