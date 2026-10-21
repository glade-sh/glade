package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// The public Apex grammar admits method declarations as trigger members,
// separately from statements. These declarations must not execute on entry.
// https://github.com/apex-dev-tools/apex-parser/blob/main/antlr/BaseApexParser.g4
// Invocation of trigger-local methods remains an explicit runtime boundary.
func TestRunTriggerLocalDeclarationsDoNotBlockStatements(t *testing.T) {
	for _, api := range []string{"39.0", "67.0"} {
		for _, helper := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/helper=%t", api, helper), func(t *testing.T) {
				root := t.TempDir()
				trigger := `trigger OwnedDeclarationTrigger on Account (before insert) {
for (Account row : Trigger.new) { row.Description = 'visited'; }
`
				if helper {
					trigger += `private void observe(String text) { throw new IllegalArgumentException(text); }` + "\n"
				}
				trigger += `for (Account row : Trigger.new) { row.Description += '-once'; }
}`
				writeFile(t, filepath.Join(root, "OwnedDeclarationTrigger.trigger"), trigger)
				writeFile(t, filepath.Join(root, "OwnedDeclarationTrigger.trigger-meta.xml"), `<ApexTrigger><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
				writeFile(t, filepath.Join(root, "OwnedDeclarationTest.cls"), `@IsTest private class OwnedDeclarationTest {
@IsTest static void unrelatedObject() { Contact row = new Contact(LastName='Sprout'); insert row; System.assertNotEquals(null,row.Id); }
@IsTest static void surroundingStatements() { Account row = new Account(Name='Sprout'); insert row; Account stored=[SELECT Description FROM Account WHERE Id=:row.Id]; System.assertEquals('visited-once',stored.Description); }
}`)
				writeFile(t, filepath.Join(root, "OwnedDeclarationTest.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
				if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 || got.Failed != 0 {
					t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
				}
			})
		}
	}
}

func TestRunTriggerLocalInvocationRemainsUnsupported(t *testing.T) {
	for _, spelling := range []string{"observe", "ObSeRvE"} {
		t.Run(spelling, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "OwnedLocalCall.trigger"), `trigger OwnedLocalCall on Account (before insert) {
`+spelling+`('local invocation');
private void observe(String text) { throw new IllegalArgumentException(text); }
}`)
			writeFile(t, filepath.Join(root, "OwnedLocalCallTest.cls"), `@IsTest private class OwnedLocalCallTest {
@IsTest static void invocation() { insert new Account(Name='Sprout'); }
}`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
			if got := run.Summary(); got.Total != 1 || got.Unsupported != 1 || got.Passed != 0 {
				t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
			}
			if got := firstRunProblem(run); !strings.Contains(strings.ToLower(got), "trigger-local method observe") {
				t.Fatalf("local invocation boundary=%q", got)
			}
		})
	}
}

// Ordinary class dispatch must not inherit the trigger-local invocation guard.
func TestRunTriggerLocalBoundaryKeepsClassMethodScope(t *testing.T) {
	for _, helper := range []bool{false, true} {
		t.Run(fmt.Sprintf("helper=%t", helper), func(t *testing.T) {
			root := t.TempDir()
			source := `trigger OwnedScopeTrigger on Account (before insert) {
for (Account row : Trigger.new) { row.Description=OwnedScopeControl.render(); }
`
			if helper {
				source += `private void observe(String text) { throw new IllegalArgumentException(text); }`
			}
			source += `}`
			writeFile(t, filepath.Join(root, "OwnedScopeTrigger.trigger"), source)
			writeFile(t, filepath.Join(root, "OwnedScopeControl.cls"), `public class OwnedScopeControl {
public static String render() { return observe('class'); }
public static String observe(String text) { return text; }
}`)
			writeFile(t, filepath.Join(root, "OwnedScopeTest.cls"), `@IsTest private class OwnedScopeTest {
@IsTest static void classCalls() {
Account row=new Account(Name='Sprout'); insert row;
Account stored=[SELECT Description FROM Account WHERE Id=:row.Id];
System.assertEquals('class',stored.Description);
System.assertEquals('after',OwnedScopeControl.observe('after'));
}
}`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 || got.Failed != 0 {
				t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
			}
		})
	}
}
