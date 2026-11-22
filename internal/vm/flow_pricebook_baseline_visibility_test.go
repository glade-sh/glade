package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowPricebookBaselineVisibilityContext(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Pricebook2")
	state := org.Objects["Pricebook2"]
	ids := storage.NewIDGenerator(map[string]string{"Pricebook2": state.Definition.KeyPrefix})
	standard, err := ids.Next("Pricebook2")
	if err != nil {
		t.Fatal(err)
	}
	custom, err := ids.Next("Pricebook2")
	if err != nil {
		t.Fatal(err)
	}
	state.SetRecords(map[storage.ID]storage.Record{
		standard: {ID: standard, Object: "Pricebook2", Fields: map[string]storage.Value{"IsStandard": storage.BooleanValue(true)}},
		custom:   {ID: custom, Object: "Pricebook2", Fields: map[string]storage.Value{"IsStandard": storage.BooleanValue(false)}},
	})
	org.Objects["Pricebook2"] = state
	machine := New(nil)
	machine.Org = &org
	machine.SetTestStandardPricebookBaseline(&org)
	if machine.newDMLEngine(nil).FlowLookupRecordVisible != nil {
		t.Fatal("anonymous route has a test visibility filter")
	}
	machine.EnableTestContext()
	engine := machine.newDMLEngine(nil)
	if engine.FlowLookupRecordVisible == nil || engine.FlowLookupRecordVisible(standard) || !engine.FlowLookupRecordVisible(custom) {
		t.Fatal("isolated test baseline visibility is wrong")
	}
	machine.SetTestSeeAllData(true)
	if machine.newDMLEngine(nil).FlowLookupRecordVisible != nil {
		t.Fatal("see-all-data route has a test visibility filter")
	}
	machine.SetTestSeeAllData(false)
	state = org.Objects["Pricebook2"]
	state.SetRecords(map[storage.ID]storage.Record{custom: {ID: custom, Object: "Pricebook2", Fields: map[string]storage.Value{"IsStandard": storage.BooleanValue(true)}}})
	org.Objects["Pricebook2"] = state
	machine.SetTestStandardPricebookBaseline(&org)
	updated := machine.newDMLEngine(nil)
	if updated.FlowLookupRecordVisible == nil || !updated.FlowLookupRecordVisible(standard) || updated.FlowLookupRecordVisible(custom) {
		t.Fatal("new baseline was not reflected in the next engine")
	}
	if engine.FlowLookupRecordVisible(standard) || !engine.FlowLookupRecordVisible(custom) {
		t.Fatal("existing engine baseline changed")
	}
}
