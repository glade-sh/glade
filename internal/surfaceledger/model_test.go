package surfaceledger

import "testing"

func TestCanonicalSurfaceIDs(t *testing.T) {
	tests := map[string]string{
		"apex type":        ApexTypeID("System", "Label"),
		"apex member":      ApexMemberID("System", "Label", "get", []string{"String", "String"}),
		"tooling object":   ToolingObjectID("ApexClass"),
		"tooling field":    ToolingFieldID("ApexClass", "Body"),
		"rest resource":    RestResourceID("composite", "post"),
		"visualforce attr": VisualforceAttrID("apex", "page", "showHeader"),
		"lwc module":       LWCModuleID("@salesforce/apex"),
	}
	want := map[string]string{
		"apex type":        "apex:System.Label",
		"apex member":      "apex:System.Label.get(String,String)",
		"tooling object":   "tooling:ApexClass",
		"tooling field":    "tooling:ApexClass.Body",
		"rest resource":    "rest:composite.post",
		"visualforce attr": "visualforce:apex:page.showHeader",
		"lwc module":       "lwc:@salesforce/apex",
	}
	for name, got := range tests {
		if got != want[name] {
			t.Fatalf("%s id = %q, want %q", name, got, want[name])
		}
	}
}

func TestAuraObjectDoesNotJoinApexObject(t *testing.T) {
	aura := RowFromDocs(SurfaceLedgerRow{
		SurfaceID: "aura:ref_attr_types_object",
		Product:   ProductAura,
		Area:      "ui",
		TypeName:  "Object",
		Kind:      KindGuide,
	})
	apex := RowFromDocs(SurfaceLedgerRow{
		SurfaceID: ApexTypeID("System", "Object"),
		Product:   ProductApex,
		Area:      "runtime",
		Namespace: "System",
		TypeName:  "Object",
		Kind:      KindType,
	})

	ledger := Merge([]SurfaceLedgerRow{aura}, nil, []SurfaceLedgerRow{apex}, nil)

	if len(ledger.Rows) != 2 {
		t.Fatalf("merged rows = %d, want 2", len(ledger.Rows))
	}
	if ledger.Rows[0].SurfaceID == ledger.Rows[1].SurfaceID {
		t.Fatalf("product collision joined %q", ledger.Rows[0].SurfaceID)
	}
}
