package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Native self and different-lookup results: flow_relationship_id_target_boundary/org.tsv.
// This test exercises the different-lookup target.
func TestFlowRelationshipIDOtherLookupNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/flow_relationship_id_target_boundary/sources/other"
	body := `Account parent = new Account(Name='Owned parent'); insert parent;
 Contact child = new Contact(LastName='Owned child', AccountId=parent.Id); insert child;
 Contact saved = [SELECT Other__c FROM Contact WHERE Id=:child.Id WITH SYSTEM_MODE];
 System.assertEquals(parent.Id, saved.Other__c);`
	for _, api := range []struct{ version, dir string }{{"62.0", "api62"}, {"67.0", "api67"}} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api.version+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api.version+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				for _, item := range []struct{ source, target string }{
					{filepath.Join(fixture, api.dir, "OwnedRel.flow-meta.xml"), "flows/OwnedRel.flow-meta.xml"},
					{filepath.Join(fixture, "Other__c.field-meta.xml"), "objects/Contact/fields/Other__c.field-meta.xml"},
				} {
					data, err := os.ReadFile(item.source)
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default", item.target), string(data))
				}
				if route == "istest" {
					path := filepath.Join(root, "force-app/main/default/classes/OwnedTargetTest.cls")
					writeFile(t, path, `@IsTest private class OwnedTargetTest { @IsTest static void sample() { `+body+` } }`)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api.version+"</apiVersion></ApexClass>")
					run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
					if got := run.Summary(); got.Total != 1 || got.Failed != 0 {
						t.Fatalf("summary=%+v problem=%s", got, firstRunProblem(run))
					}
					return
				}
				index := loadTestIndex(t, root)
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api.version})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err = runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Native API 62/67 anonymous and @IsTest self-lookup rows 2-9 include
// both populated and empty triggering relationships.
func TestFlowRelationshipIDSelfLookupNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/flow_relationship_id_target_boundary/sources/self"
	body := `Account parent = new Account(Name='Owned parent'); insert parent;
 Contact child = new Contact(LastName='Owned child', AccountId=parent.Id); insert child;
 Contact saved = [SELECT AccountId FROM Contact WHERE Id=:child.Id];
 System.assertEquals(parent.Id, saved.AccountId);
 Contact empty = new Contact(LastName='Empty child'); insert empty;
 Contact savedEmpty = [SELECT AccountId FROM Contact WHERE Id=:empty.Id];
 System.assertEquals(null, savedEmpty.AccountId);`
	for _, api := range []struct{ version, dir string }{{"62.0", "api62"}, {"67.0", "api67"}} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api.version+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api.version+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				data, err := os.ReadFile(filepath.Join(fixture, api.dir, "OwnedRel.flow-meta.xml"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/flows/OwnedRel.flow-meta.xml"), string(data))
				if route == "istest" {
					path := filepath.Join(root, "force-app/main/default/classes/OwnedSelfTargetTest.cls")
					writeFile(t, path, `@IsTest private class OwnedSelfTargetTest { @IsTest static void sample() { `+body+` } }`)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api.version+"</apiVersion></ApexClass>")
					run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
					if got := run.Summary(); got.Total != 1 || got.Failed != 0 {
						t.Fatalf("summary=%+v problem=%s", got, firstRunProblem(run))
					}
					return
				}
				index := loadTestIndex(t, root)
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api.version})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err = runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
