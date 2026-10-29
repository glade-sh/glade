package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// A textual stamp is an acceleration hint, never authority to borrow Values
// or derived indexes from an unrelated org. Only a clone of the primed runtime
// generation may use SetRuntimeTemplateOrg's shared preparation fast path.
func TestRuntimeTemplateOrgRequiresGenerationProvenance(t *testing.T) {
	base, template := describeSharePrimedBase(t)
	warm := base.CloneRuntime(nil)
	warmOrg := template.CloneRuntimeOrg()
	warm.SetRuntimeTemplateOrg(&warmOrg)
	if warm.Org.RuntimeSchemaStamp == "" || warm.describeShare == nil {
		t.Fatal("control did not enter frozen preparation route")
	}
	_ = warm.globalDescribeMapValue()
	other := describeShareOrg()
	object := other.Objects["Widget__c"]
	delete(other.Objects, "Widget__c")
	object.Definition.APIName = "Cedar__c"
	other.Objects["Cedar__c"] = object
	otherTemplate := storage.NewRuntimeTemplate(other)
	PrimeRuntimeTemplateSchema(&otherTemplate)
	// Even equal compact stamp text does not make distinct generations related.
	otherTemplate.Org.RuntimeSchemaStamp = template.Org.RuntimeSchemaStamp
	otherTemplate.RuntimeSchemaStamp = template.RuntimeSchemaStamp
	otherOrg := otherTemplate.CloneRuntimeOrg()
	machine := base.CloneRuntime(nil)
	machine.SetRuntimeTemplateOrg(&otherOrg)
	describeShareRunApex(t, machine, `System.assert(Schema.getGlobalDescribe().containsKey('cedar__c')); System.assert(!Schema.getGlobalDescribe().containsKey('widget__c'));`)
	sibling := base.CloneRuntime(nil)
	siblingOrg := template.CloneRuntimeOrg()
	sibling.SetRuntimeTemplateOrg(&siblingOrg)
	describeShareRunApex(t, sibling, `System.assert(Schema.getGlobalDescribe().containsKey('widget__c')); System.assert(!Schema.getGlobalDescribe().containsKey('cedar__c'));`)
}
