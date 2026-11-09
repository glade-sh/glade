package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// The schema_alias native rows reject these calls at both API versions;
// excluding the helper must not silently remove the trigger that reaches it.
func TestKnownSchemaMemberTriggerRestoration(t *testing.T) {
	object, err := os.ReadFile("testdata/conformance/unloaded_dependency/schema_alias/sources/CedarAlias__c.object-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, objectName := range []string{"Account", "CedarAlias__c"} {
			t.Run(api+"/"+objectName, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-optional"}]}]}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/CedarAlias__c/CedarAlias__c.object-meta.xml"), string(object))
				classFile := filepath.Join(root, "force-app/main/default/classes/CedarSchemaHelper.cls")
				writeFile(t, classFile, "public class CedarSchemaHelper { public static void apply() { Schema."+objectName+".absentMember(); } }")
				writeFile(t, classFile+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				triggerFile := filepath.Join(root, "force-app/main/default/triggers/CedarSchemaGate.trigger")
				writeFile(t, triggerFile, "trigger CedarSchemaGate on Account (before insert) { CedarSchemaHelper.apply(); }")
				writeFile(t, triggerFile+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion></ApexTrigger>")
				index := loadTestIndex(t, root)
				sources := newSourceCache()
				sources.expectedDigestLookup = index.SourceDigest
				gate := &compileGate{dependencyIndex: &index, dependencySources: sources, fileEntry: map[string]int{}, classEntry: map[string]int{}}
				gate.diagnostics = []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA008", File: classFile, Message: `method "apply" calls unknown method "Schema.` + objectName + `.absentMember"`}}
				gate.add(testreport.CompileExclusion{ClassName: "CedarSchemaHelper", File: classFile, Root: "CedarSchemaHelper", Message: "Method does not exist or incorrect signature: void absentMember() from the type " + objectName})
				gate.add(testreport.CompileExclusion{ClassName: "CedarSchemaGate", File: triggerFile, Root: "CedarSchemaHelper", DependsOn: "CedarSchemaHelper"})
				restored, err := gate.restoreDependencyTriggers(vm.New(nil))
				if err != nil || len(restored) != 1 {
					t.Fatalf("known Schema member error must retain its trigger: triggers=%v error=%v", restored, err)
				}
			})
		}
	}
}

func TestKnownSchemaMemberTriggerWarmProjectRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-optional"}]}]}`)
			classDir := filepath.Join(root, "force-app/main/default/classes")
			for name, source := range map[string]string{
				"CedarSchemaHelper":  "public class CedarSchemaHelper { public static void apply() { Schema.Account.absentMember(); } }",
				"CedarSchemaObserve": "@IsTest private class CedarSchemaObserve { @IsTest static void observe() { insert new Account(Name='cedar'); } }",
			} {
				path := filepath.Join(classDir, name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			trigger := filepath.Join(root, "force-app/main/default/triggers/CedarSchemaGate.trigger")
			writeFile(t, trigger, "trigger CedarSchemaGate on Account (before insert) { CedarSchemaHelper.apply(); }")
			writeFile(t, trigger+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion></ApexTrigger>")
			index := loadTestIndex(t, root)
			InvalidateRuntimeCaches()
			opts := Options{Parallelism: 1, PerfCounters: true}
			cold := Run(index, opts)
			ResetPerfCounters()
			warm := Run(index, opts)
			for _, run := range []testreport.Run{cold, warm} {
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("missing observer: %+v", run.Summary())
				}
				observed := run.Suites[0].Cases[0]
				if observed.Status == testreport.StatusPass || observed.Problem == nil || !strings.Contains(observed.Problem.Message, "absentMember") {
					t.Fatalf("known member error lost: %+v", observed)
				}
			}
			if SnapshotPerfCounters().Phases.MemoryCacheHits == 0 {
				t.Fatal("second project run did not hit the runtime memory cache")
			}
		})
	}
}
