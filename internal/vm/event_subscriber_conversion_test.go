package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEventSubscriberRecordConverter(t *testing.T) {
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	row := storage.Record{ID: "0BK000000000001", Object: "EventBusSubscriber", MetadataSourceUnit: storage.ProjectEventSubscriberSource + "01q000000000001"}
	empty := machine.recordConverterForOrg(&org)(row)
	if _, ok := empty.Fields[sobjectEventSubscriberStorageIDField]; ok {
		t.Fatal("empty org used subscriber conversion")
	}
	org.Objects["EventBusSubscriber"] = storage.ObjectState{Records: map[storage.ID]storage.Record{row.ID: row}}
	visible := machine.recordConverterForOrg(&org)(row)
	if got := visible.Fields[sobjectEventSubscriberStorageIDField]; got.Fields["value"].Text != string(row.ID) {
		t.Fatalf("internal ID = %q", got.Fields["value"].Text)
	}
	if got := visible.Fields["Id"]; got.Fields["value"].Text != string(storage.EventSubscriberVisibleID) {
		t.Fatalf("visible ID = %q", got.Fields["value"].Text)
	}
	delete(org.Objects, "EventBusSubscriber")
	cleared := machine.recordConverterForOrg(&org)(row)
	if _, ok := cleared.Fields[sobjectEventSubscriberStorageIDField]; ok {
		t.Fatal("removed subscriber still uses subscriber conversion")
	}
}
