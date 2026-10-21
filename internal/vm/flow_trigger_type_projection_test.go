package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// flow_trigger_type_projection observes named API62/65 copy controls and
// API67 view controls. No general metadata-text normalization is asserted.
func TestAutolaunchedFlowTriggerTypeCopyPreservesNull(t *testing.T) {
	org := testDataOrg()
	org.APIVersion = "65.0"
	org.Metadata.Flows = []storage.FlowRule{
		{Name: "QuietCatalogIdleFlow", Active: true, SourceStatus: "Active", ProcessType: "AutoLaunchedFlow"},
		{Name: "QuietCatalogRecordFlow", Active: true, SourceStatus: "Active", ProcessType: "AutoLaunchedFlow", TriggerType: "RecordBeforeSave"},
	}
	org.Objects["CatalogNote__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "CatalogNote__c", Fields: map[string]storage.Field{
			"Kind__c": {APIName: "Kind__c", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "RecordBeforeSave", Label: "RecordBeforeSave", Active: true}}},
		},
	}}
	storage.EnsureUniqueKeyPrefixes(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymousWithOptions(`
FlowDefinitionView idle=[SELECT TriggerType FROM FlowDefinitionView WHERE ApiName='QuietCatalogIdleFlow' AND IsActive=true];
FlowDefinitionView triggered=[SELECT TriggerType FROM FlowDefinitionView WHERE ApiName='QuietCatalogRecordFlow' AND IsActive=true];
System.assertEquals(null,idle.TriggerType,'omitted trigger is null');
System.assertEquals('RecordBeforeSave',triggered.TriggerType,'explicit trigger control');
CatalogNote__c copied=new CatalogNote__c(Kind__c=idle.TriggerType);
CatalogNote__c control=new CatalogNote__c(Kind__c=triggered.TriggerType);
CatalogNote__c emptyValue=new CatalogNote__c(Kind__c='');
CatalogNote__c nullValue=new CatalogNote__c(Kind__c=null);
insert new List<CatalogNote__c>{copied,control,emptyValue,nullValue};
System.assertEquals(null,copied.Kind__c,'caller copy');
Set<Id> keys=new Set<Id>{copied.Id,control.Id,emptyValue.Id,nullValue.Id};
Map<Id,CatalogNote__c> saved=new Map<Id,CatalogNote__c>([SELECT Kind__c FROM CatalogNote__c WHERE Id IN :keys]);
System.assertEquals(idle.TriggerType,saved.get(copied.Id).Kind__c,'copied metadata query value');
System.assertEquals('RecordBeforeSave',saved.get(control.Id).Kind__c,'explicit trigger persistence control');
System.assertEquals(null,saved.get(emptyValue.Id).Kind__c,'empty persistence control');
System.assertEquals(null,saved.get(nullValue.Id).Kind__c,'null persistence control');
`, CompileOptions{APIVersion: "65.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
