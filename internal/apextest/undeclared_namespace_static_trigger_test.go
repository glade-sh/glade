package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// A project with no declared dependency and no packaging namespace calls a
// static method of an unindexed namespace from a trigger and from a test
// setup. The namespace's custom objects are in source; its classes are not.
// Offline Glade cannot run that method. Release behaviour (v0.3.1) keeps the
// calling units behind the compile gate so tests that only DML the triggered
// object still run. Deferring the call instead turned every such test into an
// unsupported-call failure.
func TestUndeclaredNamespaceStaticCallKeepsCompileExclusion(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","namespace":null,"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(base, "objects/zzpkg__Note__c/zzpkg__Note__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Note</label><pluralLabel>Notes</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeFile(t, filepath.Join(base, "triggers/AccountDispatch.trigger"), "trigger AccountDispatch on Account (before insert, after insert, before update, after update) {\n"+
				"zzpkg.Dispatcher.run(Trigger.isBefore, Trigger.isAfter, Trigger.isInsert, Trigger.isUpdate, Trigger.new, Trigger.old, Schema.SObjectType.Account);\n}")
			writeFile(t, filepath.Join(base, "triggers/AccountDispatch.trigger-meta.xml"), "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
			write := func(name, source string) {
				path := filepath.Join(base, "classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			write("AccountInsertTest", `@IsTest private class AccountInsertTest {
    @TestSetup static void setup() { insert new Account(Name = 'Parent'); }
    @IsTest static void insertsWithoutDispatcher() {
        insert new Account(Name = 'Child');
        System.assertEquals(2, [SELECT COUNT() FROM Account]);
    }
}`)
			write("DispatcherConfigTest", `@IsTest private class DispatcherConfigTest {
    @TestSetup static void setup() {
        List<zzpkg.Dispatcher.Token> tokens = zzpkg.Dispatcher.getConfig();
        zzpkg.Dispatcher.setConfig(tokens);
    }
    @IsTest static void readsConfig() { System.assert(true); }
}`)
			index := loadTestIndex(t, root)
			for _, warm := range []bool{false, true} {
				run := Run(index, Options{Parallelism: 1})
				summary := run.Summary()
				if summary.Total != 2 || summary.Passed != 1 || summary.CompileErrors != 1 || summary.RuntimeErrors != 0 || summary.Unsupported != 0 {
					t.Fatalf("warm=%v summary=%+v excluded=%+v problem=%s", warm, summary, run.ExcludedClasses, firstRunProblem(run))
				}
				excluded := map[string]bool{}
				for _, entry := range run.ExcludedClasses {
					excluded[strings.ToLower(entry.ClassName)] = true
				}
				if !excluded["accountdispatch"] || !excluded["dispatcherconfigtest"] || excluded["accountinserttest"] {
					t.Fatalf("warm=%v excluded=%+v", warm, run.ExcludedClasses)
				}
			}
		})
	}
}
