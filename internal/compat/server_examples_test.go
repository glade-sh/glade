package compat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/soql"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/vm"
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

func TestServerExampleHarnessFiltersVisibleProbes(t *testing.T) {
	project := ServerExampleProjectReport{Probes: []ServerExampleProbeResult{
		{Name: "versions", Path: "/services/data", Outcome: "pass"},
		{Name: "apexrest-1", Path: "/services/apexrest/widgets/1", Outcome: "unsupported"},
		{Name: "apexrest-2", Path: "/services/apexrest/orders/1", Outcome: "fail"},
	}}
	applyServerExampleReportFilters(&project, ServerExampleHarnessOptions{
		RouteFilter:   "widgets",
		ProbeFilter:   "apexrest",
		OutcomeFilter: "unsupported",
		BlockersOnly:  true,
	})
	if len(project.Probes) != 1 || project.Probes[0].Path != "/services/apexrest/widgets/1" {
		t.Fatalf("filtered probes = %#v", project.Probes)
	}
	if serverExampleProjectMatches("example-projects/src-nmb-nu-develop", "sf-cred") {
		t.Fatalf("project filter matched the wrong project")
	}
	if !serverExampleProjectMatches("example-projects/sf-cred-pkg-develop", "cred") {
		t.Fatalf("project filter missed project")
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

func TestServerExampleSchemaAddsEmailTemplateStandardObject(t *testing.T) {
	org := storage.NewOrgState()
	applyServerExampleSchema(&org, schema.Schema{})

	template := org.Objects["EmailTemplate"]
	if template.Definition.KeyPrefix != "00X" {
		t.Fatalf("EmailTemplate key prefix = %q", template.Definition.KeyPrefix)
	}
	for _, fieldName := range []string{"DeveloperName", "IsActive", "Name", "NamespacePrefix", "Subject"} {
		if _, ok := template.Definition.Fields[fieldName]; !ok {
			t.Fatalf("missing EmailTemplate field %s in %#v", fieldName, template.Definition.Fields)
		}
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

func TestServerExampleApexRESTCartBuildBodyUsesSeededOrder(t *testing.T) {
	body := serverExampleApexRESTBody("/selfservice/cart/build/", "POST")
	var payload struct {
		OrderID string `json:"OrderId"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.OrderID != "a0o000000000001AAA" {
		t.Fatalf("OrderId = %q", payload.OrderID)
	}
}

func TestServerExampleApexRESTSObjectsPatchBodyIncludesID(t *testing.T) {
	body := serverExampleApexRESTBody("/selfservice/sobjects/", "PATCH")
	var payload []map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 1 || payload[0]["Id"] != "001000000000001AAA" {
		t.Fatalf("PATCH body = %s", body)
	}
}

func TestServerExampleProbeOverlayKeepsCartOrderScoped(t *testing.T) {
	base := storage.NewOrgState()
	applyServerExampleSchema(&base, schema.Schema{Objects: []schema.Object{{Name: "Order__c"}}})

	cartOrg := base.Clone()
	applyServerExampleProbeOverlay(&cartOrg, serverExampleProbe{Path: "/services/apexrest/selfservice/cart/build/"})
	if records := cartOrg.Objects["Order__c"].Records; len(records) != 1 {
		t.Fatalf("cart order records = %#v", records)
	}

	deleteOrg := base.Clone()
	applyServerExampleProbeOverlay(&deleteOrg, serverExampleProbe{Path: "/services/apexrest/selfservice/sobjects/"})
	if records := deleteOrg.Objects["Order__c"].Records; len(records) != 0 {
		t.Fatalf("delete probe order records = %#v", records)
	}
}

func TestServerExampleProbeOverlayKeepsEmailEncryptionSettingsScoped(t *testing.T) {
	base := storage.NewOrgState()
	applyServerExampleSchema(&base, schema.Schema{Objects: []schema.Object{{Name: "NimbleAMSSettings__c"}}})

	emailOrg := base.Clone()
	applyServerExampleProbeOverlay(&emailOrg, serverExampleProbe{Path: "/services/apexrest/selfservice/email/SocialVerify"})
	settings := emailOrg.Objects["NimbleAMSSettings__c"].Records
	if len(settings) != 1 {
		t.Fatalf("email settings records = %#v", settings)
	}
	for _, record := range settings {
		if record.Fields["AESEncryptionKey__c"].String == "" || record.Fields["AESEncryptionIV__c"].String == "" {
			t.Fatalf("email encryption fields = %#v", record.Fields)
		}
	}
	templates := emailOrg.Objects["EmailTemplate"].Records
	template := templates[storage.ID("00X000000000001AAA")]
	if template.Fields["DeveloperName"].String != "NimbleAMSSocialVerify" {
		t.Fatalf("email template records = %#v", templates)
	}
	if template.Fields["NamespacePrefix"].Kind != storage.ValueNull || !template.Fields["IsActive"].Boolean {
		t.Fatalf("email template probe fields = %#v", template.Fields)
	}
	result, err := soql.ParseAndExecute(emailOrg, "SELECT Id FROM EmailTemplate WHERE DeveloperName = 'NimbleAMSSocialVerify' AND IsActive = true AND NamespacePrefix = null")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 || result.Records[0].Fields["Id"].ID != "00X000000000001AAA" {
		t.Fatalf("email template query records = %#v", result.Records)
	}

	settingsOrg := base.Clone()
	applyServerExampleProbeOverlay(&settingsOrg, serverExampleProbe{Path: "/services/apexrest/selfservice/settings/LoginType"})
	if records := settingsOrg.Objects["NimbleAMSSettings__c"].Records; len(records) != 0 {
		t.Fatalf("settings probe encryption records = %#v", records)
	}
}

func TestServerExampleProbeOverlayKeepsWebhookProviderScoped(t *testing.T) {
	base := storage.NewOrgState()
	applyServerExampleSchema(&base, schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{Name: "NimbleAMSSettings__c"},
		{Name: "Setup_Data__c"},
		{Name: "VerifiableEnvironment__mdt"},
		{
			Name:               "Setup_Settings__c",
			CustomSettingsType: "Hierarchy",
			Fields: []schema.Field{
				{Name: "SetupOwnerId", Type: "Text"},
				{Name: "Environment__c", Type: "Text"},
				{Name: "IsInternalOrg__c", Type: "Checkbox"},
			},
		},
	}})
	base.Namespace = "verifiable"

	webhookOrg := base.Clone()
	applyServerExampleProbeOverlay(&webhookOrg, serverExampleProbe{Path: "/services/apexrest/webhookEvents"})
	accounts := webhookOrg.Objects["Account"].Records
	provider := accounts[storage.ID("001000000000002AAA")]
	if provider.Fields["Name"].String != "local-provider" {
		t.Fatalf("webhook provider account = %#v", accounts)
	}
	setup := webhookOrg.Objects["Setup_Data__c"].Records[storage.ID("a0v000000000001AAA")]
	if mappings := setup.Fields["Data_Mappings__c"].String; !strings.Contains(mappings, `"tpField":"providerId","sfField":"Name"`) {
		t.Fatalf("webhook mappings = %s", mappings)
	}
	env := webhookOrg.Objects["VerifiableEnvironment__mdt"].Records[storage.ID("m0e000000000001AAA")]
	if endpoint := env.Fields["Endpoint__c"].String; endpoint == "" {
		t.Fatalf("webhook environment = %#v", webhookOrg.Objects["VerifiableEnvironment__mdt"].Records)
	}
	assertServerExampleSetupSettingsOrgDefault(t, webhookOrg)

	createOrg := base.Clone()
	applyServerExampleProbeOverlay(&createOrg, serverExampleProbe{Path: "/services/apexrest/webhookevent/create"})
	if _, ok := createOrg.Objects["Account"].Records[storage.ID("001000000000002AAA")]; ok {
		t.Fatalf("webhook create provider leaked = %#v", createOrg.Objects["Account"].Records)
	}
	setup = createOrg.Objects["Setup_Data__c"].Records[storage.ID("a0v000000000001AAA")]
	if mappings := setup.Fields["Data_Mappings__c"].String; !strings.Contains(mappings, `"tpField":"providerId","sfField":"Id"`) {
		t.Fatalf("webhook create mappings = %s", mappings)
	}
	assertServerExampleSetupSettingsOrgDefault(t, createOrg)
}

func assertServerExampleSetupSettingsOrgDefault(t *testing.T, org storage.OrgState) {
	t.Helper()
	program, err := vm.CompileAnonymous(`
System.assertEquals('Production', Setup_Settings__c.getOrgDefaults().Environment__c);
System.assertEquals(false, Setup_Settings__c.getOrgDefaults().IsInternalOrg__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
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
