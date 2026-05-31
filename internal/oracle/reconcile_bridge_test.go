package oracle

import (
	"testing"

	"github.com/glade-sh/glade/internal/capability"
)

func TestBuildInventoryFromReconciliationMapsWorklist(t *testing.T) {
	cat := capability.Catalog{
		Entries: []capability.CatalogEntry{
			{
				Symbol:     "PageReference.getContentAsPDF()",
				TypeName:   "PageReference",
				MemberName: "getContentAsPDF",
				Kind:       "method",
				Signature:  "public Blob getContentAsPDF()",
				Area:       "Core stdlib",
				DocsSource: "apex_System_PageReference_getContentAsPDF.md",
			},
			{
				Symbol:     "Database.query(query)",
				TypeName:   "Database",
				MemberName: "query",
				Kind:       "method",
				Signature:  "public static List<SObject> query(String query)",
				Area:       "Data platform",
				DocsSource: "apex_class_System_Database.md",
			},
		},
	}
	rec := capability.Reconciliation{
		Worklist: []capability.ReconcileWorkItem{
			{
				Symbol:     "Database.query(query)",
				Area:       "Data platform",
				Target:     capability.TargetExecutableParity,
				Kind:       "method",
				Status:     capability.DerivedUnknown,
				OwnerType:  "Database",
				DocsSource: "apex_class_System_Database.md",
			},
			{
				Symbol:     "PageReference.getContentAsPDF()",
				Area:       "Core stdlib",
				Target:     capability.TargetLocalModel,
				Kind:       "method",
				Status:     capability.DerivedTyped,
				OwnerType:  "PageReference",
				DocsSource: "apex_System_PageReference_getContentAsPDF.md",
			},
		},
	}

	inv := BuildInventoryFromReconciliation(rec, cat)
	if len(inv.Surfaces) != 2 {
		t.Fatalf("surfaces = %d", len(inv.Surfaces))
	}
	// Worklist order is preserved (priority already applied upstream).
	first := inv.Surfaces[0]
	if first.Type != "Database" || first.Member != "query" {
		t.Fatalf("first surface = %#v", first)
	}
	if len(first.Parameters) != 1 || first.Parameters[0] != "String" {
		t.Fatalf("params = %#v", first.Parameters)
	}
	if first.ReturnType != "List<SObject>" {
		t.Fatalf("return type = %q", first.ReturnType)
	}
	if first.Status != SurfaceUnknown {
		t.Fatalf("status = %q", first.Status)
	}
	if first.SurfaceID != "Database.query(String)" {
		t.Fatalf("surfaceID = %q", first.SurfaceID)
	}

	second := inv.Surfaces[1]
	if second.Status != SurfaceCompileShapeKnown {
		t.Fatalf("typed status = %q", second.Status)
	}
	if second.ReturnType != "Blob" {
		t.Fatalf("return type = %q", second.ReturnType)
	}
	if len(second.Sources) == 0 || second.Sources[0] != "reconcile" {
		t.Fatalf("sources = %#v", second.Sources)
	}
}

func TestParseSignatureShape(t *testing.T) {
	cases := []struct {
		sig    string
		ret    string
		params []string
	}{
		{"public static String escapeSingleQuotes(String s)", "String", []string{"String"}},
		{"public Blob getContentAsPDF()", "Blob", nil},
		{"global static List<SObject> query(String soql, Object bind)", "List<SObject>", []string{"String", "Object"}},
		{"", "", nil},
		{"getContentAsPDF()", "", nil},
	}
	for _, c := range cases {
		ret, params := parseSignatureShape(c.sig)
		if ret != c.ret {
			t.Fatalf("sig %q ret = %q want %q", c.sig, ret, c.ret)
		}
		if len(params) != len(c.params) {
			t.Fatalf("sig %q params = %#v want %#v", c.sig, params, c.params)
		}
		for i := range params {
			if params[i] != c.params[i] {
				t.Fatalf("sig %q param[%d] = %q want %q", c.sig, i, params[i], c.params[i])
			}
		}
	}
}
