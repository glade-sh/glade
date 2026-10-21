package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStandardTimeDescribeHasNoSObjectReference(t *testing.T) {
	// The owned BusinessHours catalog describes these fourteen scalar Time
	// fields. No new Salesforce API/context parity is claimed by this replay.
	program, err := CompileAnonymous(`
List<String> days = new List<String>{'Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'};
for (Integer repeat = 0; repeat < 2; repeat++) {
 Map<String, Schema.SObjectField> fields = BusinessHours.SObjectType.getDescribe().fields.getMap();
 for (String day : days) {
  for (String suffix : new List<String>{'StartTime', 'EndTime'}) {
   Schema.DescribeFieldResult scalar = fields.get(day + suffix).getDescribe();
   System.assertEquals(Schema.DisplayType.Time, scalar.getType(), day + suffix);
   System.assertEquals(0, scalar.getReferenceTo().size(), day + suffix);
   System.assertEquals(null, scalar.getRelationshipName(), day + suffix);
  }
 }
 for (Schema.SObjectField token : fields.values()) {
  for (Schema.SObjectType target : token.getDescribe().getReferenceTo()) {
   System.assertNotEquals(null, target.getDescribe().getName());
  }
 }
 Schema.DescribeFieldResult creator = fields.get('CreatedById').getDescribe();
 System.assertEquals(1, creator.getReferenceTo().size());
 System.assertEquals('User', creator.getReferenceTo()[0].getDescribe().getName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
