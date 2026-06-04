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
