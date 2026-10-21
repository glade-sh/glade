package apextest

import (
	"path/filepath"
	"testing"
)

func TestRunJournalRestoresTimestampAllocationBetweenTests(t *testing.T) {
	for _, test := range []struct {
		name            string
		setup           bool
		separateClasses bool
		parallelMethods bool
	}{
		{name: "class_methods"},
		{name: "setup_class_methods", setup: true},
		{name: "setup_worker_methods", setup: true, parallelMethods: true},
		{name: "shared_classes", separateClasses: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "ClockGateContract.cls"), `public interface ClockGateContract {}`)
			writeFile(t, filepath.Join(classes, "ClockGate.cls"), `
public class ClockGate {
  public class Active implements ClockGateContract {}
  private static Datetime minute(Datetime value) {
    return Datetime.newInstanceGmt(value.yearGmt(), value.monthGmt(), value.dayGmt(),
      value.hourGmt(), value.minuteGmt(), 0);
  }
  public static String outcome(Datetime created, Boolean active, Boolean known) {
    if (minute(created) > minute(Datetime.now())) return null;
    if (active) return 'Skipped';
    if (!known) return 'Error';
    return 'Started';
  }
}
`)
			methods := []struct{ name, body string }{
				{"aEarlierWrites", `
    for (Integer i = 0; i < 65; i++) insert new Account(Name = 'Earlier');
    System.assertEquals(65, [SELECT COUNT() FROM Account]);`},
				{"bOverlap", `
    System.assertEquals(0, [SELECT COUNT() FROM Account]);
    System.assertEquals(1, [SELECT COUNT() FROM ApexTypeImplementor
      WHERE ClassName = 'ClockGate.Active' AND InterfaceName = 'ClockGateContract'
      AND IsConcrete = true]);
    Account row = new Account(Name = 'Current');
    insert row;
    row = [SELECT Id, CreatedDate FROM Account WHERE Id = :row.Id];
    Test.startTest();
    System.assertEquals('Skipped', ClockGate.outcome(row.CreatedDate, true, true));
    System.assertEquals('Started', ClockGate.outcome(row.CreatedDate, false, true));
    Test.stopTest();`},
				{"cUnknown", `
    System.assertEquals(0, [SELECT COUNT() FROM Account]);
    Account row = new Account(Name = 'Current');
    insert row;
    row = [SELECT Id, CreatedDate FROM Account WHERE Id = :row.Id];
    System.assertEquals('Error', ClockGate.outcome(row.CreatedDate, false, false));`},
				{"dFutureControl", `
    System.assertEquals(null, ClockGate.outcome(Datetime.now().addDays(1), true, true));
    System.assertEquals(null, ClockGate.outcome(Datetime.now().addDays(1), false, false));`},
			}
			setup := ""
			if test.setup {
				setup = "@TestSetup static void setup() { insert new Contact(LastName = 'Seed'); }\n"
			}
			if test.separateClasses {
				for _, method := range methods {
					className := "ClockIsolation" + method.name + "Test"
					writeFile(t, filepath.Join(classes, className+".cls"), "@IsTest private class "+className+" { @IsTest static void check() {"+method.body+"} }")
				}
			} else {
				source := "@IsTest private class ClockIsolationTest {\n" + setup
				for _, method := range methods {
					source += "@IsTest static void " + method.name + "() {" + method.body + "}\n"
				}
				writeFile(t, filepath.Join(classes, "ClockIsolationTest.cls"), source+"}")
			}
			index := loadTestIndex(t, root)
			controlClass, controlMethod := "ClockIsolationTest", "bOverlap"
			if test.separateClasses {
				controlClass, controlMethod = "ClockIsolationbOverlapTest", "check"
			}
			control := Run(index, Options{Parallelism: 1, SelectedClasses: []string{controlClass}, SelectedMethod: controlMethod})
			if got := control.Summary(); got.Total != 1 || got.Passed != 1 {
				t.Fatalf("isolated overlap control = %#v", got)
			}
			run := Run(index, Options{Parallelism: 1, ParallelMethods: test.parallelMethods})
			if got := run.Summary(); got.Total != len(methods) || got.Passed != len(methods) {
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						if result.Problem != nil {
							t.Logf("%s.%s: %s", result.ClassName, result.MethodName, result.Problem.Message)
						}
					}
				}
				t.Fatalf("summary = %#v", got)
			}
		})
	}
}
