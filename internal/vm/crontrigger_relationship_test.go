package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestScheduledCronTriggerReferenceAndNameLifecycle(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Map<String, Schema.SObjectField> fields = CronTrigger.SObjectType.getDescribe().fields.getMap();
System.assertEquals(false, fields.containsKey('CronJobDetail'));
Schema.DescribeFieldResult reference = fields.get('CronJobDetailId').getDescribe();
System.assertEquals(Schema.SOAPType.ID, reference.getSOAPType());
System.assertEquals('CronJobDetail', reference.getRelationshipName());
System.assertEquals(CronJobDetail.SObjectType, reference.getReferenceTo()[0]);
Id first = System.schedule('owned nightly', '0 0 0 1 1 ? 2099', new ScheduledWorker());
System.assertEquals(false, CronTrigger.SObjectType.getDescribe().fields.getMap().containsKey('CronJobDetail'));
CronTrigger row = [SELECT Id, CronJobDetailId, CronJobDetail.Name, CronJobDetail.JobType FROM CronTrigger WHERE Id = :first AND CronJobDetail.Name = 'owned nightly'];
System.assertEquals('owned nightly', row.CronJobDetail.Name);
System.assertEquals('7', row.CronJobDetail.JobType);
Boolean rejected = false;
try {
 System.schedule('owned nightly', '0 0 0 1 1 ? 2099', new ScheduledWorker());
} catch (System.AsyncException e) {
 rejected = true;
}
System.assertEquals(true, rejected);
System.abortJob(first);
System.assertEquals(0, [SELECT COUNT() FROM CronTrigger WHERE Id = :first]);
Id second = System.schedule('owned nightly', '0 0 0 1 1 ? 2099', new ScheduledWorker());
System.assertNotEquals(first, second);
System.abortJob(second);
`, CompileOptions{APIVersion: "66.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "ScheduledWorker", Interfaces: []string{"Schedulable"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
