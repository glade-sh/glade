package compat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/soql"
	"github.com/open-aer/oaer/internal/storage"
)

func TestServerExampleHarnessReportsSeedsRoutesAndBlockers(t *testing.T) {
	root := localTestDir(t, ".oaer-test-server-examples")
	testProjects := []string{
		"example-projects/alpha-pkg-develop",
		"example-projects/beta-pkg-develop",
		"example-projects/gamma-pkg-develop",
		"example-projects/delta-pkg-develop",
	}
	for _, rel := range testProjects {
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
		if err := os.WriteFile(filepath.Join(projectPath, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	restClass := `@RestResource(urlMapping='/widgets')
global with sharing class WidgetEndpoint {
  @HttpPost global static void handle() {}
}`
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(testProjects[0]), "force-app", "main", "default", "classes", "WidgetEndpoint.cls"), []byte(restClass), 0o644); err != nil {
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
	if len(report.Projects) != len(testProjects) {
		t.Fatalf("projects = %d", len(report.Projects))
	}
	first := report.Projects[0]
	if first.DataFiles != 1 || first.SeededObjects != 1 || first.SeededRecords != 1 {
		t.Fatalf("seed summary = files %d objects %d records %d", first.DataFiles, first.SeededObjects, first.SeededRecords)
	}
	if len(first.RestResources) != 1 || first.RestResources[0].Method != "POST" || first.RestResources[0].Path != "/widgets" {
		t.Fatalf("rest routes = %#v", first.RestResources)
	}
	if !hasOwnerLane(report, "lane-2-apex-rest") {
		t.Fatalf("missing apex rest owner lane: %#v", report.OwnerLanes)
	}
}

func TestServerExampleHarnessReportsNoProjects(t *testing.T) {
	root := localTestDir(t, ".oaer-test-server-examples-empty")
	report, err := RunServerExampleHarness(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("report unexpectedly not ok: %#v", report)
	}
	if len(report.Projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(report.Projects))
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
	if serverExampleProjectMatches("example-projects/beta-pkg-develop", "alpha") {
		t.Fatalf("project filter matched the wrong project")
	}
	if !serverExampleProjectMatches("example-projects/alpha-pkg-develop", "alpha") {
		t.Fatalf("project filter missed project")
	}
}

func TestParseServerExampleRestRoutesMarksWildcardMappings(t *testing.T) {
	source := `@RestResource(urlMapping='/api/*')
global class WildcardResource {
  @HttpGet global static void getIt() {}
}
@RestResource(urlMapping='/exact')
global class ExactResource {
  @HttpPost global static void postIt() {}
}`

	routes := parseServerExampleRestRoutes(source)
	if len(routes) != 2 {
		t.Fatalf("routes = %#v", routes)
	}
	if routes[0].Path != "/api/" || !routes[0].Wildcard {
		t.Fatalf("wildcard route = %#v", routes[0])
	}
	if routes[1].Path != "/exact" || routes[1].Wildcard {
		t.Fatalf("exact route = %#v", routes[1])
	}
}

func TestServerExampleApexRESTWildcardProbeAddsSyntheticChildPath(t *testing.T) {
	probes := serverExampleProbes([]ServerExampleRestRoute{{
		Method:   http.MethodGet,
		Path:     "/api/",
		Wildcard: true,
	}}, false)

	got := probes[len(probes)-1].Path
	if got != "/services/apexrest/api/oaer-probe" {
		t.Fatalf("wildcard probe path = %q", got)
	}
}

func TestServerExampleApexRESTWildcardProbeUsesDiscoveredChildPath(t *testing.T) {
	probes := serverExampleProbes([]ServerExampleRestRoute{{
		Method:    http.MethodGet,
		Path:      "/api/",
		Wildcard:  true,
		ProbePath: "widgets",
	}}, false)

	got := probes[len(probes)-1].Path
	if got != "/services/apexrest/api/widgets" {
		t.Fatalf("wildcard probe path = %q", got)
	}
}

func TestServerExampleApexRESTExactProbeKeepsRoutePath(t *testing.T) {
	probes := serverExampleProbes([]ServerExampleRestRoute{{
		Method: http.MethodGet,
		Path:   "/api",
	}}, false)

	got := probes[len(probes)-1].Path
	if got != "/services/apexrest/api" {
		t.Fatalf("exact probe path = %q", got)
	}
}

func TestClassifyServerExampleApexRESTServerErrorAsPass(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Code = http.StatusInternalServerError
	rec.Body.WriteString(`{"error":"synthetic application exception"}`)

	result := classifyServerExampleProbe(serverExampleProbe{
		Name:   "apexrest-1",
		Family: "apex-rest",
		Method: http.MethodPost,
		Path:   "/services/apexrest/widgets",
	}, rec)

	if result.Outcome != "pass" {
		t.Fatalf("outcome = %q, result = %#v", result.Outcome, result)
	}
}

func TestClassifyServerExampleApexRESTDispatchedExceptionAsPass(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Code = http.StatusNotImplemented
	rec.Body.WriteString(`[{"errorCode":"UNSUPPORTED_FEATURE","message":"Apex REST execution failed in SyntheticEndpoint.handle: unsupported call \"SyntheticApi.v1.run\""}]`)

	result := classifyServerExampleProbe(serverExampleProbe{
		Name:   "apexrest-1",
		Family: "apex-rest",
		Method: http.MethodPost,
		Path:   "/services/apexrest/widgets",
	}, rec)

	if result.Outcome != "pass" {
		t.Fatalf("outcome = %q, result = %#v", result.Outcome, result)
	}
}

func TestClassifyServerExampleNonApexRESTServerErrorAsFailure(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Code = http.StatusInternalServerError
	rec.Body.WriteString(`{"error":"synthetic server exception"}`)

	result := classifyServerExampleProbe(serverExampleProbe{
		Name:   "query",
		Family: "query",
		Method: http.MethodGet,
		Path:   "/services/data/v61.0/query",
	}, rec)

	if result.Outcome != "fail" {
		t.Fatalf("outcome = %q, result = %#v", result.Outcome, result)
	}
}

func TestServerExampleApexRESTBodyInfersListDeserializeTargets(t *testing.T) {
	source := `@RestResource(urlMapping='/synthetic')
global class SyntheticEndpoint {
  @HttpPost global static void postStrings() {
    List<String> values = (List<String>) JSON.deserialize(RestContext.request.requestBody.toString(), List<String>.class);
  }
  @HttpPut global static void putRecords() {
    List<SObject> records = (List<SObject>) JSON.deserialize(RestContext.request.requestBody.toString(), List<SObject>.class);
  }
  @HttpPatch global static void patchItems() {
    List<SyntheticItem> items = (List<SyntheticItem>) JSON.deserialize(RestContext.request.requestBody.toString(), List<SyntheticItem>.class);
  }
  global class SyntheticItem {
    public String name;
  }
}`
	routes := parseServerExampleRestRoutes(source)
	if len(routes) != 3 {
		t.Fatalf("routes = %#v", routes)
	}
	for _, route := range routes {
		if body := serverExampleApexRESTBody(route); body != `[]` {
			t.Fatalf("%s body = %s", route.Method, body)
		}
	}
}

func TestServerExampleApexRESTBodyInfersSimpleDTOFields(t *testing.T) {
	source := `@RestResource(urlMapping='/synthetic')
global class SyntheticEndpoint {
  global class RequestDTO {
    public String name;
    public Integer count;
    public Boolean active;
    public Date targetDate;
    public List<String> tags;
    public Map<String, Object> metadata;
  }

  @HttpPost global static void handle() {
    RequestDTO dto = (RequestDTO) System.JSON.deserialize(RestContext.request.requestBody.toString(), RequestDTO.class);
  }
}`
	routes := parseServerExampleRestRoutes(source)
	if len(routes) != 1 {
		t.Fatalf("routes = %#v", routes)
	}
	body := serverExampleApexRESTBody(routes[0])
	var object map[string]any
	if err := json.Unmarshal([]byte(body), &object); err != nil {
		t.Fatalf("body %s is not JSON object: %v", body, err)
	}
	if object["name"] != "sample" || object["count"] != float64(0) || object["active"] != false || object["targetDate"] != "2024-01-01" {
		t.Fatalf("primitive DTO fields = %#v", object)
	}
	if tags, ok := object["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("tags = %#v", object["tags"])
	}
	if metadata, ok := object["metadata"].(map[string]any); !ok || len(metadata) != 0 {
		t.Fatalf("metadata = %#v", object["metadata"])
	}
}

func TestServerExampleApexRESTBodyUsesValidSyntheticIDs(t *testing.T) {
	source := `@RestResource(urlMapping='/synthetic')
global class SyntheticEndpoint {
  global class RequestDTO {
    public String accountId;
    public Id ownerId;
    public String version;
  }

  @HttpPost global static void handle() {
    RequestDTO dto = (RequestDTO) JSON.deserialize(RestContext.request.requestBody.toString(), RequestDTO.class);
  }
}`
	routes := parseServerExampleRestRoutes(source)
	body := serverExampleApexRESTBody(routes[0])
	var object map[string]any
	if err := json.Unmarshal([]byte(body), &object); err != nil {
		t.Fatalf("body %s is not JSON object: %v", body, err)
	}
	if object["accountId"] != "001000000000001AAA" || object["ownerId"] != "001000000000001AAA" || object["version"] != "1" {
		t.Fatalf("DTO fields = %#v", object)
	}
}

func TestServerExampleSchemaMarksHierarchyCustomSettings(t *testing.T) {
	org := storage.NewOrgState()
	applyServerExampleSchema(&org, schema.Schema{Objects: []schema.Object{{
		Name:               "SyntheticSettings__c",
		CustomSettingsType: "Hierarchy",
	}}})
	definition := org.Objects["SyntheticSettings__c"].Definition
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

func TestServerExampleSyntheticSeedsStandardObjectsFromSchema(t *testing.T) {
	org := storage.NewOrgState()
	loaded := schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{
			Name: "Contact",
			Fields: []schema.Field{{
				Name:             "AccountId",
				Type:             "Lookup",
				ReferenceTo:      []string{"Account"},
				RelationshipName: "Account",
			}},
		},
	}}
	applyServerExampleSchema(&org, loaded)

	applyServerExampleSyntheticSeeds(&org, loaded)

	accounts := org.Objects["Account"].Records
	account := accounts[storage.ID("001000000009001AAA")]
	if len(accounts) != 1 || account.Fields["Name"].String != "Synthetic Account" {
		t.Fatalf("Account records = %#v", accounts)
	}
	contacts := org.Objects["Contact"].Records
	contact := contacts[storage.ID("003000000009001AAA")]
	if len(contacts) != 1 || contact.Fields["LastName"].String != "Contact" {
		t.Fatalf("Contact records = %#v", contacts)
	}
	if contact.Fields["AccountId"].ID != account.ID {
		t.Fatalf("Contact AccountId = %#v, want %s", contact.Fields["AccountId"], account.ID)
	}
}

func TestServerExampleSyntheticSeedsSkipObjectsAbsentFromSchema(t *testing.T) {
	org := serverExampleBaseOrg(storage.NewFixture())
	loaded := schema.Schema{}
	applyServerExampleSchema(&org, loaded)

	applyServerExampleSyntheticSeeds(&org, loaded)

	if records := org.Objects["Account"].Records; len(records) != 0 {
		t.Fatalf("Account records = %#v", records)
	}
	if records := org.Objects["Contact"].Records; len(records) != 0 {
		t.Fatalf("Contact records = %#v", records)
	}
}

func TestServerExampleCustomMetadataSeedsModernFiles(t *testing.T) {
	root := localTestDir(t, ".oaer-test-server-example-cmdt")
	projectPath := filepath.Join(root, "example-projects", "synthetic-package")
	writeServerExampleTestFile(t, filepath.Join(projectPath, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeServerExampleTestFile(t, filepath.Join(projectPath, "force-app", "main", "default", "objects", "RouteConfig__mdt", "RouteConfig__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Route Config</label></CustomObject>`)
	writeServerExampleTestFile(t, filepath.Join(projectPath, "force-app", "main", "default", "objects", "RouteConfig__mdt", "fields", "Route__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Route__c</fullName><type>Text</type></CustomField>`)
	writeServerExampleTestFile(t, filepath.Join(projectPath, "force-app", "main", "default", "objects", "RouteConfig__mdt", "fields", "IsActive__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>IsActive__c</fullName><type>Checkbox</type></CustomField>`)
	writeServerExampleTestFile(t, filepath.Join(projectPath, "force-app", "main", "default", "objects", "RouteConfig__mdt", "fields", "Class__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Class__c</fullName><type>Text</type></CustomField>`)
	writeServerExampleTestFile(t, filepath.Join(projectPath, "force-app", "main", "default", "customMetadata", "RouteConfig.WidgetApi.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
  <label>Widget API</label>
  <values><field>Class__c</field><value xsi:type="xsd:string">pkg.WidgetEndpoint</value></values>
  <values><field>IsActive__c</field><value xsi:type="xsd:boolean">true</value></values>
  <values><field>Route__c</field><value xsi:type="xsd:string">widgets</value></values>
</CustomMetadata>`)

	p, err := project.Load(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	loadedSchema, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = p.Namespace
	applyServerExampleSchema(&org, loadedSchema)
	if err := applyServerExampleCustomMetadata(&org, p.CustomMetadataFiles); err != nil {
		t.Fatal(err)
	}
	result, err := soql.ParseAndExecute(org, "SELECT Id FROM RouteConfig__mdt WHERE pkg__Route__c = 'widgets' AND pkg__IsActive__c = true")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 {
		t.Fatalf("RouteConfig__mdt rows = %d records = %#v org=%#v", result.Rows, result.Records, org.Objects["RouteConfig__mdt"].Records)
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

func writeServerExampleTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverServerExampleRestRoutesSkipsClaudeWorktrees(t *testing.T) {
	root := localTestDir(t, ".oaer-test-server-examples-worktree")
	projectPath := filepath.Join(root, "example-projects", "worktree-pkg-develop")
	classesDir := filepath.Join(projectPath, "force-app", "main", "default", "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	restClass := `@RestResource(urlMapping='/widgets')
global with sharing class WidgetEndpoint {
  @HttpPost global static void handle() {}
}`
	if err := os.WriteFile(filepath.Join(classesDir, "WidgetEndpoint.cls"), []byte(restClass), 0o644); err != nil {
		t.Fatal(err)
	}

	worktreeClassesDir := filepath.Join(projectPath, ".claude", "worktrees", "some-branch", "force-app", "main", "default", "classes")
	if err := os.MkdirAll(worktreeClassesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeClassesDir, "WidgetEndpoint.cls"), []byte(restClass), 0o644); err != nil {
		t.Fatal(err)
	}

	routes, err := discoverServerExampleRestRoutes(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d: %#v", len(routes), routes)
	}
	if routes[0].Path != "/widgets" {
		t.Fatalf("unexpected route path: %q", routes[0].Path)
	}
}

func hasOwnerLane(report ServerExampleHarnessReport, lane string) bool {
	for _, entry := range report.OwnerLanes {
		if entry.OwnerLane == lane {
			return true
		}
	}
	return false
}
