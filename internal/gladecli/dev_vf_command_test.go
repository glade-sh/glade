package gladecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPrintDevVFStartupSummaryListsPagesAndWatchedFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/pages/Core.page"), `<apex:page/>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/pages/CardHost.page"), `<apex:page/>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	printDevVFStartupSummary(&out, "127.0.0.1:8080", p)

	for _, want := range []string{
		"Visualforce dev server: http://127.0.0.1:8080",
		"Pages:",
		"  /apex/CardHost",
		"  /apex/Core",
		"Watching " + p.Root + " for .page, .component, .cls, aura, lwc, and static resource changes.",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}

func TestPrintDevVFStartupSummaryBoundsPages(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	for i := 0; i < 12; i++ {
		name := "Page" + string(rune('A'+i))
		writeTestFile(t, filepath.Join(root, "force-app/main/default/pages", name+".page"), `<apex:page/>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	printDevVFStartupSummary(&out, "127.0.0.1:8080", p)

	if !strings.Contains(out.String(), "Pages: 12 available") {
		t.Fatalf("summary did not show page count:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "... 4 pages omitted.") {
		t.Fatalf("summary did not cap pages:\n%s", out.String())
	}
	if strings.Contains(out.String(), "/apex/PageI") {
		t.Fatalf("summary included pages beyond compact startup cap:\n%s", out.String())
	}
}

func TestRunDevVFHelpUsesVisualforceHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(t.Context(), []string{"dev", "vf", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"Start a local Visualforce preview development server",
		"Preview feature:",
		"glade dev vf [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]",
		"--ready-file /tmp/glade-vf-ready.json",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}

func TestWriteDevVFReadyFileWritesURLAddressAndPages(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/pages/Core.page"), `<apex:page/>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/pages/CardHost.page"), `<apex:page/>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	readyPath := filepath.Join(t.TempDir(), "ready.json")
	if err := writeDevVFReadyFile(readyPath, "127.0.0.1:48321", p); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(readyPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		URL   string   `json:"url"`
		Addr  string   `json:"addr"`
		Pages []string `json:"pages"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("readiness file is not JSON: %v\n%s", err, data)
	}
	if got.URL != "http://127.0.0.1:48321" {
		t.Fatalf("URL = %q", got.URL)
	}
	if got.Addr != "127.0.0.1:48321" {
		t.Fatalf("addr = %q", got.Addr)
	}
	wantPages := []string{"/apex/CardHost", "/apex/Core"}
	if strings.Join(got.Pages, ",") != strings.Join(wantPages, ",") {
		t.Fatalf("pages = %#v, want %#v", got.Pages, wantPages)
	}
}

func TestApplyDevVFProjectDataFixturesSeedsStorageFixtures(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "data", "accounts.json"), `{
  "version": "glade.storage.v1",
  "objects": [{
    "name": "Account",
    "records": [{
      "id": "001000000000001AAA",
      "fields": {"Name": {"kind": "string", "string": "Acme"}}
    }]
  }]
}`)
	writeTestFile(t, filepath.Join(root, "data", "notes.json"), `{"notes":["not a storage fixture"]}`)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}

	if err := applyDevVFProjectDataFixtures(root, &org); err != nil {
		t.Fatal(err)
	}

	account := org.Objects["Account"]
	record := account.Records["001000000000001AAA"]
	if record.Fields["Name"].String != "Acme" {
		t.Fatalf("seeded Account = %#v", account.Records)
	}
}

func TestApplyDevVFProjectDataFixturesSeedsSFDXTreeData(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "data", "accounts.json"), `[
  {
    "attributes": {"type": "Account", "referenceId": "LocalShellAccount"},
    "Name": "Local Shell Account",
    "Industry": "Technology",
    "Active__c": true
  }
]`)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":      {APIName: "Name", Type: storage.FieldString},
			"Industry":  {APIName: "Industry", Type: storage.FieldString},
			"Active__c": {APIName: "Active__c", Type: storage.FieldBoolean},
		}},
		Records: map[storage.ID]storage.Record{},
	}

	if err := applyDevVFProjectDataFixtures(root, &org); err != nil {
		t.Fatal(err)
	}

	account := org.Objects["Account"]
	if len(account.Records) != 1 {
		t.Fatalf("records = %#v", account.Records)
	}
	for _, record := range account.Records {
		if record.Fields["Name"].String != "Local Shell Account" || !record.Fields["Active__c"].Boolean {
			t.Fatalf("record = %#v", record)
		}
	}
}

func TestApplyDevVFProjectDataFixturesIgnoresSFDXDataPlanArrays(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "data", "sample-data-plan.json"), `[
  {
    "sobject": "Account",
    "saveRefs": true,
    "resolveRefs": false,
    "files": ["Accounts.json"]
  }
]`)
	writeTestFile(t, filepath.Join(root, "data", "accounts.json"), `[
  {
    "attributes": {"type": "Account", "referenceId": "LocalShellAccount"},
    "Name": "Local Shell Account"
  }
]`)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{},
	}

	if err := applyDevVFProjectDataFixtures(root, &org); err != nil {
		t.Fatal(err)
	}

	account := org.Objects["Account"]
	if len(account.Records) != 1 {
		t.Fatalf("records = %#v", account.Records)
	}
}

func TestApplyDevVFProjectDataFixturesIgnoresUnknownJSONWithBOM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "data", "export.json"), "\xef\xbb\xbf"+`{
  "records": {},
  "cleaners": [],
  "extId": "External_Id__c",
  "queries": []
}`)
	org := storage.NewOrgState()

	if err := applyDevVFProjectDataFixtures(root, &org); err != nil {
		t.Fatal(err)
	}
}

func TestApplyDevVFProjectDataFixturesSeedsSFDXTreeDataWithBOM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "data", "accounts.json"), "\xef\xbb\xbf\n"+`[
  {
    "attributes": {"type": "Account", "referenceId": "LocalShellAccount"},
    "Name": "Local Shell Account"
  }
]`)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{},
	}

	if err := applyDevVFProjectDataFixtures(root, &org); err != nil {
		t.Fatal(err)
	}

	account := org.Objects["Account"]
	if len(account.Records) != 1 {
		t.Fatalf("records = %#v", account.Records)
	}
}

func TestNormalizeDevVFServeErrorIgnoresNormalServerClose(t *testing.T) {
	if err := normalizeDevVFServeError(http.ErrServerClosed); err != nil {
		t.Fatalf("ErrServerClosed normalized to %v", err)
	}
	want := errors.New("listen failed")
	if err := normalizeDevVFServeError(want); !errors.Is(err, want) {
		t.Fatalf("unexpected normalized error: %v", err)
	}
}
