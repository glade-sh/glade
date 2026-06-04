package surfaceledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexdocs"
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

func TestBuildDocsSnapshotClassifiesDataReferenceDocs(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "object-reference/sforce_api_objects_asyncapexjob.md", "# AsyncApexJob\n\n### Status\n")

	rows, err := BuildDocsSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	byID := rowsByID(rows)
	if row, ok := byID[DataObjectID("AsyncApexJob")]; !ok || row.Product != ProductDataRef || row.Area != AreaData {
		t.Fatalf("AsyncApexJob object row = %#v, ok=%v", row, ok)
	}
	if row, ok := byID[DataFieldID("AsyncApexJob", "Status")]; !ok || row.Kind != KindField {
		t.Fatalf("AsyncApexJob.Status field row = %#v, ok=%v", row, ok)
	}
}

func TestBuildDocsSnapshotUsesApexSignatureParameterTypes(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "apex/apex_class_System_FeatureManagement.md", "# FeatureManagement Class\n\n### checkPackageBooleanValue(apiName)\n\n#### Signature\n\n`public static Boolean checkPackageBooleanValue(String\napiName)`\n")
	writeDoc(t, root, "apex/apex_methods_system_database.md", "# Database Class\n\n### executeBatch(batchClassObject)\n\n#### Signature\n\n`public static ID executeBatch(Object\nbatchClassObject)`\n\n### executeBatch(batchClassObject, scope)\n\n#### Signature\n\n`public static ID executeBatch(Object\nbatchClassObject, Integer scope)`\n")

	rows, err := BuildDocsSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	byID := rowsByID(rows)
	if _, ok := byID[ApexMemberID("System", "FeatureManagement", "checkPackageBooleanValue", []string{"String"})]; !ok {
		t.Fatalf("FeatureManagement docs row did not use typed parameter list: %#v", rows)
	}
	if _, ok := byID[ApexMemberID("System", "Database", "executeBatch", []string{"Object", "Integer"})]; !ok {
		t.Fatalf("Database.executeBatch docs row did not use typed parameter list: %#v", rows)
	}
	if _, ok := byID[ApexMemberID("System", "Database", "executeBatch", []string{"Object"})]; !ok {
		t.Fatalf("Database.executeBatch single-argument docs row did not use typed parameter list: %#v", rows)
	}
}

func TestRowsFromDocsInventoryCanonicalizesApexNamespace(t *testing.T) {
	rows := RowsFromDocsInventory(apexdocs.Inventory{
		Documents: []apexdocs.Document{{
			SourcePath: "apex/apex_interface_database_batchable.md",
			Kind:       "interface",
			Namespace:  "database",
			Name:       "Batchable",
			Members: []apexdocs.Member{{
				Kind:      "method",
				Name:      "execute",
				Signature: "execute(Database.BatchableContext, List<Object>)",
			}},
		}},
	})

	byID := rowsByID(rows)
	if _, ok := byID[ApexTypeID("Database", "Batchable")]; !ok {
		t.Fatalf("docs type row did not canonicalize Database namespace: %#v", rows)
	}
	if _, ok := byID[ApexMemberID("Database", "Batchable", "execute", []string{"Database.BatchableContext", "List<Object>"})]; !ok {
		t.Fatalf("docs member row did not canonicalize Database namespace: %#v", rows)
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
