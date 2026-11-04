package storage

import "testing"

func TestStandardPicklistPolicyCheckAllocationBudget(t *testing.T) {
	for _, name := range []string{"Case", "Opportunity"} {
		t.Run(name, func(t *testing.T) {
			definition := enrichedStandardOverlayDefinition(name, nil)
			if standardObjectFieldsNeedWrite(definition, "") {
				t.Fatal("enriched definition requires repair")
			}
			if got := testing.AllocsPerRun(20, func() { standardObjectFieldsNeedWrite(definition, "") }); got != 0 {
				t.Fatalf("picklist policy check allocates %.0f per call, want 0", got)
			}
		})
	}
}
