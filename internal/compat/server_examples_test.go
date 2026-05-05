package compat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/storage"
)

func TestServerExampleHarnessReportsSeedsRoutesAndBlockers(t *testing.T) {
	root := localTestDir(t, ".oaer-test-server-examples")
	for _, rel := range serverExampleProjects {
		projectPath := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Join(projectPath, "force-app", "main", "default", "classes"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(projectPath, "data"), 0o755); err != nil {
			t.Fatal(err)
		}
		data := `{"records":[{"attributes":{"type":"Account","referenceId":"Acme"},"Name":"Acme"}]}`
		if err := os.WriteFile(filepath.Join(projectPath, "data", "Accounts.json"), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	restClass := `@RestResource(urlMapping='/webhookEvents')
global with sharing class WebHook {
  @HttpPost global static void handle() {}
}`
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(serverExampleProjects[0]), "force-app", "main", "default", "classes", "WebHook.cls"), []byte(restClass), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunServerExampleHarness(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Missing != 0 {
		t.Fatalf("missing = %d", report.Counts.Missing)
	}
	if report.Counts.Pass == 0 {
		t.Fatalf("expected passing probes: %#v", report.Counts)
	}
	if report.Counts.Fail != 0 || report.Counts.Missing != 0 {
		t.Fatalf("unexpected hard blockers: %#v", report.Counts)
	}
	if len(report.Projects) != len(serverExampleProjects) {
		t.Fatalf("projects = %d", len(report.Projects))
	}
	first := report.Projects[0]
	if first.DataFiles != 1 || first.SeededObjects != 1 || first.SeededRecords != 1 {
		t.Fatalf("seed summary = files %d objects %d records %d", first.DataFiles, first.SeededObjects, first.SeededRecords)
	}
	if len(first.RestResources) != 1 || first.RestResources[0].Method != "POST" || first.RestResources[0].Path != "/webhookEvents" {
		t.Fatalf("rest routes = %#v", first.RestResources)
	}
	if !hasOwnerLane(report, "lane-2-apex-rest") {
		t.Fatalf("missing apex rest owner lane: %#v", report.OwnerLanes)
	}
}

func TestServerExampleHarnessReportsMissingProjects(t *testing.T) {
	root := localTestDir(t, ".oaer-test-server-examples-missing")
	report, err := RunServerExampleHarness(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("report unexpectedly ok: %#v", report)
	}
	if report.Counts.Missing != len(serverExampleProjects) {
		t.Fatalf("missing = %d", report.Counts.Missing)
	}
}

func TestServerExampleSchemaMarksHierarchyCustomSettings(t *testing.T) {
	org := storage.NewOrgState()
	applyServerExampleSchema(&org, schema.Schema{Objects: []schema.Object{{
		Name:               "NimbleAMSSettings__c",
		CustomSettingsType: "Hierarchy",
	}}})
	definition := org.Objects["NimbleAMSSettings__c"].Definition
	if definition.Metadata["kind"] != "customSetting" || definition.Metadata["customSettingsType"] != "Hierarchy" {
		t.Fatalf("metadata = %#v", definition.Metadata)
	}
}

func TestServerExampleSchemaAddsPublicAccountStandardFields(t *testing.T) {
	org := storage.NewOrgState()
	applyServerExampleSchema(&org, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	account := org.Objects["Account"]
	field, ok := account.Definition.Fields["Website"]
	if !ok || field.Type != storage.FieldString {
		t.Fatalf("Website field = %#v, %v", field, ok)
	}
	if resolved, ok := storage.ResolveFieldName(account.Definition, org.Namespace, "website"); !ok || resolved != "Website" {
		t.Fatalf("resolve website = %q, %v", resolved, ok)
	}
}

func TestServerExampleApexRESTProbeDataForWebhookEvents(t *testing.T) {
	if body := serverExampleApexRESTBody("/webhookEvents", "POST"); !strings.Contains(body, `"providerId":"local-provider"`) {
		t.Fatalf("webhook body = %s", body)
	}
	headers := serverExampleApexRESTHeaders("/webhookEvents")
	if headers["X-WebhookType"] == "" || headers["X-WebhookId"] == "" {
		t.Fatalf("webhook headers = %#v", headers)
	}
}

func TestServerExampleApexRESTOrderBodyIncludesBillTo(t *testing.T) {
	body := serverExampleApexRESTBody("/selfservice/order/", "POST")
	var payload struct {
		Order map[string]any `json:"Order"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Order["BillTo__c"] != "001000000000001AAA" {
		t.Fatalf("BillTo__c = %#v", payload.Order["BillTo__c"])
	}
}

func localTestDir(t *testing.T, prefix string) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", prefix+"-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove %s: %v", dir, err)
		}
	})
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func hasOwnerLane(report ServerExampleHarnessReport, lane string) bool {
	for _, entry := range report.OwnerLanes {
		if entry.OwnerLane == lane {
			return true
		}
	}
	return false
}
