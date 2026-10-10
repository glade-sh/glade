package apextest

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"
)

// Mirrors the SF234 API62 assertion: restoring one master cannot restore a
// two-master-detail child while the other master remains in the recycle bin.
func TestRunMasterDetailTwoParentUndeleteAPI62Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTwoParentUndeleteAssertions231.cls"), "@IsTest private class GladeTwoParentUndeleteAssertions231 {\n @IsTest static void assertChildRemainsDeletedWhenOtherCascadeMasterIsDeleted() {\n  GladeTwoParentA231__c first=new GladeTwoParentA231__c(Name='first'); insert first;\n  GladeTwoParentB231__c second=new GladeTwoParentB231__c(Name='second'); insert second;\n  GladeTwoParentChild231__c child=new GladeTwoParentChild231__c(Name='junction',First__c=first.Id,Second__c=second.Id); insert child; Id childId=child.Id;\n  delete first; delete second; undelete first;\n  GladeTwoParentA231__c firstState=[SELECT Id,IsDeleted FROM GladeTwoParentA231__c WHERE Id=:first.Id ALL ROWS];\n  GladeTwoParentB231__c secondState=[SELECT Id,IsDeleted FROM GladeTwoParentB231__c WHERE Id=:second.Id ALL ROWS];\n  GladeTwoParentChild231__c childState=[SELECT Id,First__c,Second__c,IsDeleted FROM GladeTwoParentChild231__c WHERE Id=:childId ALL ROWS];\n  System.assertEquals(false,firstState.IsDeleted);\n  System.assertEquals(true,secondState.IsDeleted);\n  System.assertEquals(true,childState.IsDeleted);\n  System.assertEquals(first.Id,childState.First__c);\n  System.assertEquals(second.Id,childState.Second__c);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTwoParentUndeleteAssertions231.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n")
	for _, object := range []struct{ name, label, plural, sharing string }{
		{"GladeTwoParentA231__c", "Glade Two Parent A 231", "Glade Two Parents A 231", "ReadWrite"},
		{"GladeTwoParentB231__c", "Glade Two Parent B 231", "Glade Two Parents B 231", "ReadWrite"},
		{"GladeTwoParentChild231__c", "Glade Two Parent Child 231", "Glade Two Parent Children 231", "ControlledByParent"},
	} {
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", object.name, object.name+".object-meta.xml"), "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>"+object.label+"</label><nameField><label>"+object.label+" Name</label><type>Text</type></nameField><pluralLabel>"+object.plural+"</pluralLabel><sharingModel>"+object.sharing+"</sharingModel></CustomObject>\n")
	}
	for _, field := range []struct {
		name, target, relation, label string
		order                         int
	}{
		{"First__c", "GladeTwoParentA231__c", "GladeTwoParentFirstChildren231", "First", 0},
		{"Second__c", "GladeTwoParentB231__c", "GladeTwoParentSecondChildren231", "Second", 1},
	} {
		writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeTwoParentChild231__c/fields", field.name+".field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>"+field.name+"</fullName><externalId>false</externalId><label>"+field.label+"</label><referenceTo>"+field.target+"</referenceTo><relationshipLabel>Glade Two Parent "+field.label+" Children 231</relationshipLabel><relationshipName>"+field.relation+"</relationshipName><relationshipOrder>"+strconv.Itoa(field.order)+"</relationshipOrder><reparentableMasterDetail>false</reparentableMasterDetail><trackFeedHistory>false</trackFeedHistory><trackTrending>false</trackTrending><type>MasterDetail</type><writeRequiresMasterRead>true</writeRequiresMasterRead></CustomField>\n")
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"volunteers-masterdetail-two-parent-undelete-assertions-api62-project40\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"40.0\"}")

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 || got.Failed != 0 {
		body, _ := json.Marshal(run)
		t.Fatalf("admitted two-parent master-detail undelete contract: %s", body)
	}
}
