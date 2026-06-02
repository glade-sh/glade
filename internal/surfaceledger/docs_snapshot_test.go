package surfaceledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildDocsSnapshotKeepsProductPathInIdentity(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "apex/system_object.md", "# Object Class\n\n## length()\n")
	writeDoc(t, root, "lightning-aura/ref_attr_types_object.md", "# Object\n\nAura guide text.\n")

	rows, err := BuildDocsSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	byID := rowsByID(rows)
	if _, ok := byID[ApexTypeID("System", "Object")]; !ok {
		t.Fatalf("missing Apex Object row in %#v", rows)
	}
	if _, ok := byID["aura:ref_attr_types_object"]; !ok {
		t.Fatalf("missing Aura Object row in %#v", rows)
	}
	if byID[ApexTypeID("System", "Object")].DocsSource != "apex/system_object.md" {
		t.Fatalf("apex docs source = %q", byID[ApexTypeID("System", "Object")].DocsSource)
	}
}

func TestBuildDocsSnapshotExtractsAPIVersionText(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "apex/system_label.md", "# Label Class\n\nAvailable in API version 60.0 and later.\n")

	rows, err := BuildDocsSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	row := rowsByID(rows)[ApexTypeID("System", "Label")]
	if row.APIVersion != "60.0" {
		t.Fatalf("api version = %q, want 60.0", row.APIVersion)
	}
}

func writeDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rowsByID(rows []SurfaceLedgerRow) map[string]SurfaceLedgerRow {
	out := map[string]SurfaceLedgerRow{}
	for _, row := range rows {
		out[row.SurfaceID] = row
	}
	return out
}
