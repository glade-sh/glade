package surfaceledger

import "testing"

func TestCanonicalSurfaceIDs(t *testing.T) {
	tests := map[string]string{
		"apex type":        ApexTypeID("System", "Label"),
		"apex member":      ApexMemberID("System", "Label", "get", []string{"String", "String"}),
		"tooling object":   ToolingObjectID("ApexClass"),
		"tooling field":    ToolingFieldID("ApexClass", "Body"),
		"data object":      DataObjectID("AsyncApexJob"),
		"data field":       DataFieldID("AsyncApexJob", "Status"),
		"rest resource":    RestResourceID("composite", "post"),
		"visualforce attr": VisualforceAttrID("apex", "page", "showHeader"),
		"lwc module":       LWCModuleID("@salesforce/apex"),
	}
	want := map[string]string{
		"apex type":        "apex:System.Label",
		"apex member":      "apex:System.Label.get(String,String)",
		"tooling object":   "tooling:ApexClass",
		"tooling field":    "tooling:ApexClass.Body",
		"data object":      "data-reference:AsyncApexJob",
		"data field":       "data-reference:AsyncApexJob.Status",
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

func TestCanonicalSurfaceIDsCleanApexNames(t *testing.T) {
	got := ApexMemberID("System", "Describe\u200bFieldResult", "getSOAPType", nil)
	want := "apex:Schema.DescribeFieldResult.getSOAPType"
	if got != want {
		t.Fatalf("cleaned apex member id = %q, want %q", got, want)
	}

	got = ApexMemberID("System", "DescribeFieldResult", "getSObjectField", []string{})
	want = "apex:Schema.DescribeFieldResult.getSObjectField()"
	if got != want {
		t.Fatalf("cleaned sobject acronym id = %q, want %q", got, want)
	}

	got = ApexMemberID("cache", "Org", "put", []string{"String", "Object", "cache.Visibility"})
	want = "apex:Cache.Org.put(String,Object,Cache.Visibility)"
	if got != want {
		t.Fatalf("cleaned cache id = %q, want %q", got, want)
	}

	got = ApexMemberID("cache", "Partition", "get", []string{"System.Type", "String"})
	want = ApexMemberID("Cache", "Partition", "get", []string{"Type", "String"})
	if got != want {
		t.Fatalf("cleaned System.Type id = %q, want %q", got, want)
	}

	got = ApexMemberID("System", "System", "scheduleBatch", []string{"Database.Batchable", "String", "Integer"})
	want = ApexMemberID("System", "System", "scheduleBatch", []string{"Object", "String", "Integer"})
	if got != want {
		t.Fatalf("cleaned Database.Batchable id = %q, want %q", got, want)
	}

	got = ApexTypeID("ApexPages", "ApexPages")
	want = ApexTypeID("System", "ApexPages")
	if got != want {
		t.Fatalf("cleaned ApexPages namespace id = %q, want %q", got, want)
	}

	got = ApexMemberID("ApexPages", "StandardSetController", "setSelected", []string{"sObject[]"})
	want = ApexMemberID("ApexPages", "StandardSetController", "setSelected", []string{"List<Object>"})
	if got != want {
		t.Fatalf("cleaned sObject array id = %q, want %q", got, want)
	}
}

func TestGladeSnapshotMarksDatabaseStatefulSupported(t *testing.T) {
	rows := BuildGladeSnapshot()
	id := ApexTypeID("Database", "Stateful")
	for _, row := range rows {
		if row.SurfaceID != id {
			continue
		}
		if row.GladeBehavior != BehaviorSupported {
			t.Fatalf("Database.Stateful behavior = %q, want %q", row.GladeBehavior, BehaviorSupported)
		}
		return
	}
	t.Fatalf("missing %s row", id)
}

func TestRowsCarrySurfaceFamilyAndImplementationTarget(t *testing.T) {
	apex := RowFromDocs(SurfaceLedgerRow{
		SurfaceID: ApexMemberID("System", "Schema", "getGlobalDescribe", []string{}),
		Product:   ProductApex,
		Area:      AreaRuntime,
		Kind:      KindMethod,
	})
	if apex.SalesforceSurfaceFamily != "apex" || apex.GladeImplementationTarget != "runtime" || apex.ShapeSource != "reference" {
		t.Fatalf("apex row sources/target = family:%q target:%q shape:%q", apex.SalesforceSurfaceFamily, apex.GladeImplementationTarget, apex.ShapeSource)
	}

	rest := RowFromDocs(SurfaceLedgerRow{
		SurfaceID: RestResourceID("sobjects", "get"),
		Product:   ProductREST,
		Area:      AreaServer,
		Kind:      KindResource,
	})
	if rest.SalesforceSurfaceFamily != "rest-api" || rest.GladeImplementationTarget != "server-or-explicit-unsupported" {
		t.Fatalf("rest row family/target = %q/%q", rest.SalesforceSurfaceFamily, rest.GladeImplementationTarget)
	}
}

func TestOwnerForRuntimeCaseCheckDoesNotAllocate(t *testing.T) {
	row := SurfaceLedgerRow{
		SurfaceID: ApexMemberID("Database", "Batchable", "execute", []string{"Database.BatchableContext", "List<Object>"}),
		Product:   ProductApex,
		Area:      AreaRuntime,
	}
	allocs := testing.AllocsPerRun(100, func() {
		if OwnerFor(row) != "data-runtime" {
			t.Fatalf("owner = %q, want data-runtime", OwnerFor(row))
		}
	})
	if allocs != 0 {
		t.Fatalf("OwnerFor allocations = %.0f, want 0", allocs)
	}
}
