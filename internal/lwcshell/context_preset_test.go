package lwcshell

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadContextPresetsReadsProjectFile(t *testing.T) {
	root := t.TempDir()
	writeContextPresetFile(t, root, `{
  "defaultContext": "accountRecord",
  "contexts": {
    "accountRecord": {
      "target": "recordPage",
      "objectApiName": "Account",
      "recordId": "001000000000001AAA",
      "page": "Account_Record_Page",
      "app": "Sales",
      "tab": "Accounts",
      "formFactor": "Large",
      "state": {"c__mode": "demo"}
    }
  }
}`)

	file, err := LoadContextPresets(root)
	if err != nil {
		t.Fatal(err)
	}
	if file.DefaultContext != "accountRecord" {
		t.Fatalf("defaultContext = %q", file.DefaultContext)
	}
	preset, err := file.Preset("accountRecord")
	if err != nil {
		t.Fatal(err)
	}
	if preset.ObjectAPIName != "Account" || preset.RecordID != "001000000000001AAA" {
		t.Fatalf("preset = %#v", preset)
	}
	if got := preset.State["c__mode"]; got != "demo" {
		t.Fatalf("state c__mode = %q", got)
	}
}

func TestLoadContextPresetsIgnoresMissingProjectFile(t *testing.T) {
	file, err := LoadContextPresets(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if file.DefaultContext != "" || len(file.Contexts) != 0 {
		t.Fatalf("file = %#v", file)
	}
}

func TestLoadContextPresetsInvalidFileUsesDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeContextPresetFile(t, root, `{"contexts":`)

	_, err := LoadContextPresets(root)
	requireContextDiagnostic(t, err, "GLADELWC020")
}

func TestContextPresetFileMissingPresetUsesDiagnostic(t *testing.T) {
	file := ContextPresetFile{
		Contexts: map[string]ContextPreset{
			"home": {Target: "homePage", Page: "Custom_Home"},
		},
	}

	_, err := file.Preset("missing")
	requireContextDiagnostic(t, err, "GLADELWC021")
}

func TestContextPresetToPageContextMapsTargets(t *testing.T) {
	tests := []struct {
		name   string
		preset ContextPreset
		want   PageContext
	}{
		{
			name: "record page",
			preset: ContextPreset{
				Target:        "recordPage",
				ObjectAPIName: "Account",
				RecordID:      "001000000000001AAA",
				Page:          "Account_Record_Page",
				App:           "Sales",
				Tab:           "Accounts",
				FormFactor:    "Large",
				State:         map[string]string{"c__mode": "demo"},
			},
			want: PageContext{
				Kind:          RenderTargetRecordPage,
				ObjectAPIName: "Account",
				RecordID:      "001000000000001AAA",
				PageName:      "Account_Record_Page",
				AppName:       "Sales",
				TabName:       "Accounts",
				FormFactor:    "Large",
				State:         map[string]string{"c__mode": "demo"},
			},
		},
		{
			name:   "app page",
			preset: ContextPreset{Target: "appPage", Page: "Sales_Dashboard"},
			want:   PageContext{Kind: RenderTargetAppPage, PageName: "Sales_Dashboard"},
		},
		{
			name:   "home page",
			preset: ContextPreset{Target: "homePage", Page: "Custom_Home"},
			want:   PageContext{Kind: RenderTargetHomePage, PageName: "Custom_Home"},
		},
		{
			name:   "component",
			preset: ContextPreset{Target: "component", Component: "c:contextProbe"},
			want:   PageContext{Kind: RenderTargetComponent, ComponentName: "c:contextProbe"},
		},
		{
			name:   "url addressable",
			preset: ContextPreset{Target: "url-addressable", Component: "c:actionProbe", State: map[string]string{"c__mode": "demo"}},
			want:   PageContext{Kind: RenderTargetURLAddressable, ComponentName: "c:actionProbe", State: map[string]string{"c__mode": "demo"}},
		},
		{
			name:   "record action",
			preset: ContextPreset{Target: "record-action", ObjectAPIName: "Account", RecordID: "001000000000001AAA", Action: "Update_Status"},
			want:   PageContext{Kind: RenderTargetQuickAction, ObjectAPIName: "Account", RecordID: "001000000000001AAA", ActionName: "Update_Status"},
		},
		{
			name:   "global action",
			preset: ContextPreset{Target: "global-action", Action: "Global_Status"},
			want:   PageContext{Kind: RenderTargetQuickAction, ActionName: "Global_Status"},
		},
		{
			name: "flow screen",
			preset: ContextPreset{
				Target:    "flowScreen",
				Component: "c:communityFlow2",
				Flow: FlowContext{
					APIName:        "Membership_Flow",
					InputVariables: map[string]any{"recordId": "001000000000001AAA"},
				},
			},
			want: PageContext{
				Kind:          RenderTargetFlowScreen,
				ComponentName: "c:communityFlow2",
				Flow: FlowContext{
					APIName:        "Membership_Flow",
					InputVariables: map[string]any{"recordId": "001000000000001AAA"},
				},
			},
		},
		{
			name:   "tab",
			preset: ContextPreset{Target: "tab", Tab: "Lwc_Probe"},
			want:   PageContext{Kind: RenderTargetTab, TabName: "Lwc_Probe"},
		},
		{
			name: "community page",
			preset: ContextPreset{
				Target:    "community-page",
				Component: "c:communityProbe",
				Page:      "Account",
				Community: CommunityContext{
					Site:      "Partner_Portal",
					BasePath:  "/partners",
					SiteID:    "0DM000000000001",
					NetworkID: "0DB000000000001",
					Guest:     true,
					Language:  "en-US",
					RouteParams: map[string]string{
						"recordId": "001000000000001AAA",
					},
					Menus: map[string][]CommunityMenuItem{
						"main": {{Label: "Accounts", Target: "Account", Type: "comm__namedPage"}},
					},
					ManagedContent: map[string]CommunityContentItem{
						"welcome": {ContentKey: "welcome", Title: "Welcome", Body: "Local content"},
					},
				},
				PageReference: map[string]any{
					"type":       "comm__namedPage",
					"attributes": map[string]any{"name": "Account"},
				},
			},
			want: PageContext{
				Kind:          RenderTargetCommunityPage,
				ComponentName: "c:communityProbe",
				PageName:      "Account",
				Community: CommunityContext{
					Site:      "Partner_Portal",
					BasePath:  "/partners",
					SiteID:    "0DM000000000001",
					NetworkID: "0DB000000000001",
					Guest:     true,
					Language:  "en-US",
					RouteParams: map[string]string{
						"recordId": "001000000000001AAA",
					},
					Menus: map[string][]CommunityMenuItem{
						"main": {{Label: "Accounts", Target: "Account", Type: "comm__namedPage"}},
					},
					ManagedContent: map[string]CommunityContentItem{
						"welcome": {ContentKey: "welcome", Title: "Welcome", Body: "Local content"},
					},
				},
				PageReference: map[string]any{
					"type":       "comm__namedPage",
					"attributes": map[string]any{"name": "Account"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.preset.ToPageContext()
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != tt.want.Kind ||
				got.ComponentName != tt.want.ComponentName ||
				got.PageName != tt.want.PageName ||
				got.RecordID != tt.want.RecordID ||
				got.ObjectAPIName != tt.want.ObjectAPIName ||
				got.AppName != tt.want.AppName ||
				got.TabName != tt.want.TabName ||
				got.FormFactor != tt.want.FormFactor ||
				got.State["c__mode"] != tt.want.State["c__mode"] ||
				got.Community.Site != tt.want.Community.Site ||
				got.Community.BasePath != tt.want.Community.BasePath ||
				got.Community.SiteID != tt.want.Community.SiteID ||
				got.Community.NetworkID != tt.want.Community.NetworkID ||
				got.Community.Guest != tt.want.Community.Guest ||
				got.Community.Language != tt.want.Community.Language ||
				got.Community.RouteParams["recordId"] != tt.want.Community.RouteParams["recordId"] ||
				(tt.want.Community.Menus != nil && len(got.Community.Menus["main"]) != 1) ||
				(tt.want.Community.ManagedContent != nil && got.Community.ManagedContent["welcome"].Title != "Welcome") ||
				got.Flow.APIName != tt.want.Flow.APIName ||
				(tt.want.Flow.InputVariables != nil && got.Flow.InputVariables["recordId"] != tt.want.Flow.InputVariables["recordId"]) ||
				(tt.want.PageReference != nil && got.PageReference["type"] != tt.want.PageReference["type"]) {
				t.Fatalf("context = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestContextPresetToPageContextValidatesDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		preset ContextPreset
		code   string
	}{
		{name: "unsupported target", preset: ContextPreset{Target: "consolePage"}, code: "GLADELWC022"},
		{name: "record id required", preset: ContextPreset{Target: "recordPage", ObjectAPIName: "Account", Page: "Account_Record_Page"}, code: "GLADELWC023"},
		{name: "record page required", preset: ContextPreset{Target: "recordPage", ObjectAPIName: "Account", RecordID: "001000000000001AAA"}, code: "GLADELWC024"},
		{name: "app page required", preset: ContextPreset{Target: "appPage"}, code: "GLADELWC024"},
		{name: "home page required", preset: ContextPreset{Target: "homePage"}, code: "GLADELWC024"},
		{name: "community context required", preset: ContextPreset{Target: "communityPage", Component: "c:communityProbe", Page: "Account"}, code: "GLADELWC100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.preset.ToPageContext()
			requireContextDiagnostic(t, err, tt.code)
		})
	}
}

func writeContextPresetFile(t *testing.T, root string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "glade.lwc.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireContextDiagnostic(t *testing.T, err error, code string) {
	t.Helper()
	var presetErr *ContextPresetError
	if !errors.As(err, &presetErr) {
		t.Fatalf("err = %v, want ContextPresetError", err)
	}
	if presetErr.Diagnostic.Code != code {
		t.Fatalf("diagnostic code = %q, want %q", presetErr.Diagnostic.Code, code)
	}
}
