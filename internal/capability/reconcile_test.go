package capability

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestTypeFromDocsSource(t *testing.T) {
	cases := map[string]string{
		"apex_class_System_Json.md":                    "Json",
		"apex_methods_system_string.md":                "string",
		"apex_interface_System_Callable.md":            "Callable",
		"apex_enum_System_AccessType.md":               "AccessType",
		"apex_System_PageReference_getContentAsPDF.md": "PageReference",
		"apex_namespace_System.md":                     "",
		"docs/apex_class_Database_SaveResult.md":       "SaveResult",
		"":                                             "",
	}
	for source, want := range cases {
		if got := typeFromDocsSource(source); got != want {
			t.Errorf("typeFromDocsSource(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestBuildReconciliationDerivesRuntimeStatus(t *testing.T) {
	cat := Catalog{
		SourceDocuments: 4,
		SourceMembers:   4,
		Entries: []CatalogEntry{
			// Executable verdict carried from the catalog.
			{Symbol: "Assert.areEqual", Area: "Tests, async, and limits", Namespace: "System",
				TypeName: "Assert", MemberName: "areEqual", Kind: "method",
				Target: TargetExecutableParity, Status: StatusSupported, DocsSource: "apex_class_System_Assert.md"},
			// Type-known but no verdict -> typed.
			{Symbol: "PageReference.getContentAsPDF", Area: "Integration, security, and UI", Namespace: "System",
				TypeName: "getContentAsPDF", MemberName: "", Kind: "method",
				Target: TargetLocalModel, Status: StatusUnknown, DocsSource: "apex_System_PageReference_getContentAsPDF.md"},
			// Type not in the symbol table -> unknown.
			{Symbol: "Imaginary.doThing", Area: "Core stdlib", Namespace: "System",
				TypeName: "doThing", MemberName: "", Kind: "method",
				Target: TargetExecutableParity, Status: StatusUnknown, DocsSource: "apex_System_Imaginary_doThing.md"},
			// Guide doc -> doc, never a runtime target.
			{Symbol: "Variables", Area: "Language and guide docs",
				TypeName: "Variables", Kind: "guide",
				Target: TargetUnsupported, Status: StatusUnsupported, DocsSource: "apex_language_variables.md"},
		},
	}

	platform := []typesys.TypeSymbol{
		{Name: "Assert", Kind: apexast.DeclarationClass},
		{Name: "PageReference", Kind: apexast.DeclarationClass},
	}

	rec := BuildReconciliation(cat, platform)

	statusBySymbol := map[string]DerivedStatus{}
	for _, item := range rec.Worklist {
		statusBySymbol[item.Symbol] = item.Status
	}

	if rec.RuntimeTargets.Total != 3 {
		t.Fatalf("runtime targets total = %d, want 3", rec.RuntimeTargets.Total)
	}
	if rec.RuntimeTargets.Supported != 1 {
		t.Errorf("supported = %d, want 1", rec.RuntimeTargets.Supported)
	}
	if rec.RuntimeTargets.Typed != 1 {
		t.Errorf("typed = %d, want 1", rec.RuntimeTargets.Typed)
	}
	if rec.RuntimeTargets.Unknown != 1 {
		t.Errorf("unknown = %d, want 1", rec.RuntimeTargets.Unknown)
	}
	if got := rec.RuntimeTargetUnknownCount(); got != 1 {
		t.Errorf("RuntimeTargetUnknownCount = %d, want 1", got)
	}

	// The PageReference page reports the method name as TypeName; the owning
	// type must be recovered from the doc source path.
	if statusBySymbol["PageReference.getContentAsPDF"] != DerivedTyped {
		t.Errorf("getContentAsPDF derived = %q, want typed", statusBySymbol["PageReference.getContentAsPDF"])
	}
	if statusBySymbol["Imaginary.doThing"] != DerivedUnknown {
		t.Errorf("Imaginary.doThing derived = %q, want unknown", statusBySymbol["Imaginary.doThing"])
	}
	// Verdict-bearing surfaces are not in the worklist.
	if _, ok := statusBySymbol["Assert.areEqual"]; ok {
		t.Errorf("supported surface should not appear in worklist")
	}
}

func TestBuildReconciliationWorklistOrdersByImpact(t *testing.T) {
	cat := Catalog{
		Entries: []CatalogEntry{
			{Symbol: "Prod.typed", Area: "Product namespaces", TypeName: "Prod", Kind: "method",
				Target: TargetTypedStub, Status: StatusUnknown, DocsSource: "apex_prod_Prod_typed.md"},
			{Symbol: "Core.typed", Area: "Core stdlib", TypeName: "Core", Kind: "method",
				Target: TargetExecutableParity, Status: StatusUnknown, DocsSource: "apex_System_Core_typed.md"},
			{Symbol: "Core.gone", Area: "Core stdlib", TypeName: "Missing", Kind: "method",
				Target: TargetExecutableParity, Status: StatusUnknown, DocsSource: "apex_System_Missing_gone.md"},
		},
	}
	platform := []typesys.TypeSymbol{{Name: "Core", Kind: apexast.DeclarationClass}}

	rec := BuildReconciliation(cat, platform)
	if len(rec.Worklist) != 3 {
		t.Fatalf("worklist length = %d, want 3", len(rec.Worklist))
	}
	// executable-parity unknown ranks first, then executable-parity typed,
	// then typed-stub product namespace.
	if rec.Worklist[0].Symbol != "Core.gone" || rec.Worklist[0].Status != DerivedUnknown {
		t.Errorf("worklist[0] = %+v, want Core.gone unknown", rec.Worklist[0])
	}
	if rec.Worklist[1].Symbol != "Core.typed" || rec.Worklist[1].Status != DerivedTyped {
		t.Errorf("worklist[1] = %+v, want Core.typed typed", rec.Worklist[1])
	}
	if rec.Worklist[2].Target != TargetTypedStub {
		t.Errorf("worklist[2] target = %q, want typed-stub", rec.Worklist[2].Target)
	}
}

func TestWriteReconciliationMarkdownIsDeterministic(t *testing.T) {
	cat := Catalog{
		SourceDocuments: 1,
		Entries: []CatalogEntry{
			{Symbol: "Core.typed", Area: "Core stdlib", TypeName: "Core", Kind: "method",
				Target: TargetExecutableParity, Status: StatusUnknown, DocsSource: "apex_System_Core_typed.md"},
		},
	}
	platform := []typesys.TypeSymbol{{Name: "Core", Kind: apexast.DeclarationClass}}
	rec := BuildReconciliation(cat, platform)

	var a, b bytes.Buffer
	if err := WriteReconciliationMarkdown(&a, rec); err != nil {
		t.Fatal(err)
	}
	if err := WriteReconciliationMarkdown(&b, rec); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("markdown output is not deterministic")
	}
	if !strings.Contains(a.String(), "Runtime target coverage") {
		t.Errorf("markdown missing coverage section")
	}
}
