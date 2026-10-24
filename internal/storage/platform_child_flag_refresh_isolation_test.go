package storage

import "testing"

// Native API 62/67 FinanceBalanceSnapshots rows in
// standard_platform_child_fields/org.tsv:2,27 require createable=false.
func TestPlatformChildUnsetFlagRefreshSharedDefinitionRoute(t *testing.T) {
	definition := enrichedStandardOverlayDefinition("FinanceBalanceSnapshot", nil)
	field := definition.Fields["AccountId"]
	field.Createable = nil
	definition.Fields["AccountId"] = field
	shared := definition
	if !RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("unset native flag did not refresh")
	}
	if flag := definition.Fields["AccountId"].Createable; flag == nil || *flag {
		t.Fatal("refreshed definition does not have explicit native false")
	}
	if shared.Fields["AccountId"].Createable != nil {
		t.Fatal("refresh mutated a shared definition")
	}
	if RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("explicit native false refreshed again")
	}
}
