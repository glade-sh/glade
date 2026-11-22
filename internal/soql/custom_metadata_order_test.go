package soql

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows (docs/native-evidence/share_mdt_native_fixtures): ORDER BY
// MasterLabel and Label on a custom metadata type keep the default order.
// DeveloperName, custom fields and every other object order as before.
func TestCustomMetadataOrderDropsOnlyLabelSpecs(t *testing.T) {
	mdt := storage.ObjectDefinition{APIName: "Synth__mdt"}
	other := storage.ObjectDefinition{APIName: "Synth__c"}
	order := []OrderSpec{{Field: "MasterLabel", Desc: true}, {Field: "label"}, {Field: "DeveloperName", Desc: true}, {Field: "Val__c"}}
	if got, want := customMetadataOrder(mdt, order), []OrderSpec{{Field: "DeveloperName", Desc: true}, {Field: "Val__c"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("custom metadata order = %+v, want %+v", got, want)
	}
	if got := customMetadataOrder(other, order); !reflect.DeepEqual(got, order) {
		t.Fatalf("ordinary object order changed: %+v", got)
	}
	only := []OrderSpec{{Field: "DeveloperName"}}
	if got := customMetadataOrder(mdt, only); !reflect.DeepEqual(got, only) {
		t.Fatalf("DeveloperName order changed: %+v", got)
	}
	if got := customMetadataOrder(mdt, nil); got != nil {
		t.Fatalf("empty order changed: %+v", got)
	}
}
