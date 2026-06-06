package surfaceledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildEvidenceSnapshotReadsSurfaceID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "label_get",
  "evidence": [{"symbol": "System.Label.get", "surfaceId": "apex:System.Label.get(String,String)"}],
  "source": [{"path": "force-app/main/default/classes/T.cls", "content": "class T {}"}],
  "command": {"kind": "exec"},
  "expected": {"stdout": ""}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	id := ApexMemberID("System", "Label", "get", []string{"String", "String"})
	if rowsByID(rows)[id].Evidence != EvidenceFixture {
		t.Fatalf("evidence row missing surface id %s: %#v", id, rows)
	}
}

func TestBuildEvidenceSnapshotMarksRuntimeGuideEvidenceSupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "query-runtime-soqlsosl-orderby",
  "evidence": [
    {"symbol": "SOQL ORDER BY", "surfaceId": "unknown:sforce_api_calls_soql_select_orderby", "kind": "test"},
    {"symbol": "Dynamic SOQL", "surfaceId": "unknown:apex_dynamic_soql", "kind": "test"}
  ],
  "command": {"kind": "test"},
  "expected": {"result": {"ok": true}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	row := rowsByID(rows)["unknown:sforce_api_calls_soql_select_orderby"]
	if row.Evidence != EvidenceFixture || row.GladeBehavior != BehaviorSupported {
		t.Fatalf("guide row evidence/behavior = %s/%s, want fixture/supported: %#v", row.Evidence, row.GladeBehavior, rows)
	}
	row = rowsByID(rows)["unknown:apex_dynamic_soql"]
	if row.Evidence != EvidenceFixture || row.GladeBehavior != BehaviorSupported {
		t.Fatalf("dynamic row evidence/behavior = %s/%s, want fixture/supported: %#v", row.Evidence, row.GladeBehavior, rows)
	}
}

func TestBuildEvidenceSnapshotDoesNotPromoteNonQueryUnknownEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "other_runtime_unknown",
  "evidence": [{"symbol": "Other runtime guide", "surfaceId": "unknown:some_other_runtime_guide", "kind": "test"}],
  "command": {"kind": "test"},
  "expected": {"result": {"ok": true}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	row := rowsByID(rows)["unknown:some_other_runtime_guide"]
	if row.Evidence != EvidenceFixture || row.GladeBehavior != BehaviorNone {
		t.Fatalf("non-query unknown evidence/behavior = %s/%s, want fixture/none: %#v", row.Evidence, row.GladeBehavior, rows)
	}
}

func TestBuildEvidenceSnapshotDoesNotMarkUnsupportedRuntimeGuideSupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "unsupported_soql_guide",
  "evidence": [{"symbol": "SOQL guide", "surfaceId": "unknown:unsupported_soql_statements", "kind": "test"}],
  "command": {"kind": "test"},
  "expected": {"error": {"type": "UnsupportedFeature"}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	row := rowsByID(rows)["unknown:unsupported_soql_statements"]
	if row.Evidence != EvidenceFixture || row.GladeBehavior != BehaviorNone {
		t.Fatalf("unsupported guide row evidence/behavior = %s/%s, want fixture/none: %#v", row.Evidence, row.GladeBehavior, rows)
	}
}

func TestBuildEvidenceSnapshotMarksUnsupportedEvidenceUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "service_bound_unsupported",
  "evidence": [{"symbol": "WebStoreContext.getCommerceContext", "surfaceId": "apex:System.WebStoreContext.getCommerceContext()", "kind": "unsupported"}],
  "command": {"kind": "exec"},
  "expected": {"error": {"type": "UnsupportedFeature"}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	id := ApexMemberID("System", "WebStoreContext", "getCommerceContext", []string{})
	row := rowsByID(rows)[id]
	if row.Evidence != EvidenceFixture || row.GladeBehavior != BehaviorUnsupported {
		t.Fatalf("unsupported evidence/behavior = %s/%s, want fixture/unsupported: %#v", row.Evidence, row.GladeBehavior, rows)
	}
}

func TestBuildEvidenceSnapshotClassifiesBareMethodEvidenceIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "bare_method_ids",
  "evidence": [
    {"symbol": "RestRequest.getHeader", "surfaceId": "apex:System.RestRequest.getHeader", "kind": "exec"},
    {"symbol": "Limits.getAsyncCalls", "surfaceId": "apex:System.Limits.getAsyncCalls", "kind": "exec"},
    {"symbol": "Test.isRunningTest", "surfaceId": "apex:System.Test.isRunningTest", "kind": "test"},
    {"symbol": "Test.createStubQueryRows", "surfaceId": "apex:System.Test.createStubQueryRows", "kind": "test"},
    {"symbol": "Type.getName", "surfaceId": "apex:System.Type.getName", "kind": "exec"},
    {"symbol": "RestResponse.statusCode", "surfaceId": "apex:System.RestResponse.statusCode", "kind": "exec"}
  ],
  "command": {"kind": "exec"},
  "expected": {"stdout": ""}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	byID := rowsByID(rows)
	for _, id := range []string{"apex:System.RestRequest.getHeader", "apex:System.Limits.getAsyncCalls", "apex:System.Test.isRunningTest", "apex:System.Test.createStubQueryRows", "apex:System.Type.getName"} {
		if row := byID[id]; row.Kind != KindMethod {
			t.Fatalf("%s kind = %s, want method: %#v", id, row.Kind, rows)
		}
	}
	if row := byID["apex:System.RestResponse.statusCode"]; row.Kind != KindProperty {
		t.Fatalf("RestResponse.statusCode kind = %s, want property: %#v", row.Kind, rows)
	}
}

func TestBuildEvidenceSnapshotSkipsCorpusManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local-tests-corpus.json")
	if err := os.WriteFile(path, []byte(`{"target":"local Apex test execution corpus","project":"example","projects":[{"project":"example","summary":{"total":1}}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(rows))
	}
}

func TestBuildEvidenceSnapshotInfersKnownNamespaceType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "schema_record_type_info",
  "evidence": [{"symbol": "Schema.RecordTypeInfo"}],
  "command": {"kind": "test"},
  "expected": {"result": {"ok": true}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if rowsByID(rows)[ApexTypeID("Schema", "RecordTypeInfo")].Evidence != EvidenceFixture {
		t.Fatalf("Schema.RecordTypeInfo did not infer known namespace type: %#v", rows)
	}
}

func TestBuildEvidenceSnapshotSkipsDatabaseFamilyLabelsWithoutSurfaceID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "database_family",
  "evidence": [{"symbol": "Database.query"}],
  "command": {"kind": "test"},
  "expected": {"result": {"ok": true}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("Database.query family label inferred fake surface rows: %#v", rows)
	}
}

func TestBuildEvidenceSnapshotSkipsKnownNamespaceLowercaseMemberLabelsWithoutSurfaceID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "system_runtime_label",
  "evidence": [{"symbol": "System.isFuture"}],
  "command": {"kind": "exec"},
  "expected": {"stdout": ""}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("System.isFuture label inferred fake type rows: %#v", rows)
	}
}

func TestBuildEvidenceSnapshotSkipsHumanBehaviorLabelsWithoutSurfaceID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "soql_behavior_packet",
  "evidence": [{"symbol": "SOQL aggregate GROUP BY/HAVING"}],
  "command": {"kind": "test"},
  "expected": {"result": {"ok": true}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("human behavior label inferred fake surface rows: %#v", rows)
	}
}

func TestBuildEvidenceSnapshotInfersKnownZeroArgMethods(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "schema_describe_methods",
  "evidence": [{"symbol": "Schema.DescribeFieldResult.getLabel"}],
  "command": {"kind": "test"},
  "expected": {"result": {"ok": true}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	id := ApexMemberID("Schema", "DescribeFieldResult", "getLabel", []string{})
	if rowsByID(rows)[id].Evidence != EvidenceFixture {
		t.Fatalf("Schema.DescribeFieldResult.getLabel did not infer zero-arg method: %#v", rows)
	}
}

func TestBuildEvidenceSnapshotTreatsNoParenDescribeMemberAsProperty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	data := `{
  "name": "schema_describe_tab_properties",
  "evidence": [{"symbol": "Schema.DescribeTabSetResult.name", "surfaceId": "apex:Schema.DescribeTabSetResult.name"}],
  "command": {"kind": "test"},
  "expected": {"result": {"ok": true}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := BuildEvidenceSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	id := ApexMemberID("Schema", "DescribeTabSetResult", "name", nil)
	row := rowsByID(rows)[id]
	if row.Kind != KindProperty || row.Namespace != "Schema" || row.TypeName != "DescribeTabSetResult" || row.MemberName != "name" {
		t.Fatalf("property row = kind:%s namespace:%s type:%s member:%s rows:%#v", row.Kind, row.Namespace, row.TypeName, row.MemberName, rows)
	}
}
