package apextest

import (
	"path/filepath"
	"testing"
)

// The stopTest drain must settle when an inner-class Schedulable has a cron
// with no year, so its job is due by the clock but skipped by the drain.
// Inner-class jobs keep their CronTrigger and do not run (scheduled_nesting_stop
// rows); a top-level recurring job still runs once, as before.
func TestRunScheduledNestingDrainSettlesForRecurringCrons(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RecurTop.cls"), `
public class RecurTop implements Schedulable {
  public void execute(SchedulableContext context) { insert new Account(Name = 'recur-run'); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RecurPlanner.cls"), `
public class RecurPlanner {
  public class Tick implements Schedulable {
    public void execute(SchedulableContext context) { insert new Account(Name = 'recur-run'); }
  }
  public static List<Id> everyQuarterHour(String name, Schedulable job) {
    List<Id> ids = new List<Id>();
    for (Integer minute = 0; minute < 60; minute += 15) {
      ids.add(System.schedule(name + ' ' + minute, '0 ' + minute + ' * * * ?', job));
    }
    return ids;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RecurDrainTest.cls"), `
@IsTest
private class RecurDrainTest {
  public class Local implements Schedulable {
    public void execute(SchedulableContext context) { insert new Account(Name = 'recur-run'); }
  }
  static Integer ran() { return [SELECT COUNT() FROM Account WHERE Name = 'recur-run']; }
  static Integer crons() { return [SELECT COUNT() FROM CronTrigger WHERE CronJobDetail.Name LIKE 'recur%']; }
  @IsTest static void testClassInnerHourlyInWindow() {
    Test.startTest();
    System.schedule('recur hourly', '0 5 * * * ?', new Local());
    Test.stopTest();
    System.assertEquals(0, ran());
    System.assertEquals(1, crons());
  }
  @IsTest static void testClassInnerYearlessBeforeStart() {
    System.schedule('recur yearly', '20 30 8 10 2 ?', new Local());
    Test.startTest();
    Test.stopTest();
    System.assertEquals(0, ran());
    System.assertEquals(1, crons());
  }
  @IsTest static void helperInnerQuarterHours() {
    Test.startTest();
    List<Id> ids = RecurPlanner.everyQuarterHour('recur quarter', new RecurPlanner.Tick());
    Test.stopTest();
    System.assertEquals(4, ids.size());
    System.assertEquals(0, ran());
    System.assertEquals(4, crons());
  }
  @IsTest static void topLevelHourlyRunsOnce() {
    Test.startTest();
    System.schedule('recur top', '0 30 * * * ?', new RecurTop());
    Test.stopTest();
    System.assertEquals(1, ran());
    System.assertEquals(1, crons());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 4 || got.Passed != 4 {
		for _, suite := range run.Suites {
			for _, item := range suite.Cases {
				if item.Problem != nil {
					t.Logf("%s: %#v", item.MethodName, *item.Problem)
				}
			}
		}
		t.Fatalf("summary = %#v", got)
	}
}
