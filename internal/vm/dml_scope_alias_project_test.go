package vm_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestDMLScopeAliasActualTestProject(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	t.Cleanup(apextest.ResetPerfCounters)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"sfdx-project.json": fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api),
				"force-app/main/default/classes/PublicationProbe.cls": `@IsTest private class PublicationProbe {
 public static List<Account> rows;
 public static Integer seen = 0;
 public static Integer beforeCalls = 0;
 public static Integer afterCalls = 0;
 public static Boolean inspect = true;
 public static void observe(Boolean beforeEvent, Boolean inserting, List<Account> chunk) {
  if (!inspect) return;
  System.assert(chunk.size() <= 200);
  for (Integer i = 0; i < seen; i++) System.assertNotEquals(null, rows[i].Id);
  if (beforeEvent) {
   beforeCalls++;
   for (Account row : chunk) {
    if (inserting) System.assertEquals(null, row.Id);
    else System.assertNotEquals(null, row.Id);
   }
  } else {
   afterCalls++;
   for (Integer i = seen; i < seen + chunk.size(); i++) System.assertNotEquals(null, rows[i].Id);
   seen += chunk.size();
  }
 }
 @IsTest static void bulkPublication() {
  for (Integer count : new List<Integer>{1,63,64,65,199,200,201,400,1000}) {
   rows = new List<Account>(); seen = 0; beforeCalls = 0; afterCalls = 0;
   for (Integer i = 0; i < count; i++) rows.add(new Account(Name='Owned '+i));
   List<Account> alias = rows;
   insert rows;
   System.assertEquals(count, seen);
   System.assertEquals((count+199)/200, afterCalls);
   System.assertEquals(afterCalls, beforeCalls);
   for (Account row : alias) { System.assertNotEquals(null, row.Id); row.Description='updated'; }
   seen = 0; beforeCalls = 0; afterCalls = 0;
   update rows;
   System.assertEquals(count, seen);
   System.assertEquals((count+199)/200, afterCalls);
  }
 }
 @IsTest static void eligibleInsertPublication() {
  inspect = false;
  List<Account> localRows = new List<Account>();
  for (Integer i = 0; i < 1000; i++) localRows.add(new Account(Name='Owned '+i));
  insert localRows;
  for (Account row : localRows) System.assertNotEquals(null, row.Id);
 }
 @IsTest static void eligibleUpdatePublication() {
  inspect = true;
  rows = new List<Account>();
  for (Integer i = 0; i < 1000; i++) rows.add(new Account(Name='Owned '+i));
  seen=0; beforeCalls=0; afterCalls=0;
  // Direct caller aliases make every seed chunk ineligible. Release them
  // before the independently measured update, which must batch all five chunks.
  Account seed0=rows[0], seed200=rows[200], seed400=rows[400], seed600=rows[600], seed800=rows[800];
  insert rows;
  System.assertNotEquals(null,seed0.Id); System.assertNotEquals(null,seed200.Id);
  System.assertNotEquals(null,seed400.Id); System.assertNotEquals(null,seed600.Id); System.assertNotEquals(null,seed800.Id);
  seed0=null;seed200=null;seed400=null;seed600=null;seed800=null;
  rows = null;
  inspect = false;
  Test.startTest();
  List<Account> localRows = [SELECT Id, Name FROM Account];
  update localRows;
  for (Account row : localRows) System.assertNotEquals(null, row.Id);
  Test.stopTest();
 }
}`,
				"force-app/main/default/triggers/PublicationTrigger.trigger": `trigger PublicationTrigger on Account (before insert, after insert, before update, after update) {
 PublicationProbe.observe(Trigger.isBefore, Trigger.isInsert, Trigger.new);
}`,
			}
			for name, content := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				if filepath.Ext(path) == ".cls" || filepath.Ext(path) == ".trigger" {
					if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			for _, warm := range []bool{false, true} {
				for _, method := range []string{"bulkPublication", "eligibleInsertPublication", "eligibleUpdatePublication"} {
					apextest.ResetPerfCounters()
					run := apextest.Run(index, apextest.Options{SelectedMethod: method, NoDiskCache: true, Parallelism: 1, PerfCounters: true})
					if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
						t.Fatalf("warm=%v method=%s: %+v; %+v", warm, method, got, run)
					}
					if method != "bulkPublication" {
						dmlAssertBatchCounts(t, apextest.SnapshotPerfCounters().VMPerf, 5, 1000)
					}
				}
			}
		})
	}
}

// Read the optional counter fields as JSON so this guard also compiles on the
// explicit performance base. Zero/missing counters then fail at runtime.
func dmlAssertBatchCounts(t *testing.T, counters vm.PerfCounters, calls, rows uint64) {
	t.Helper()
	encoded, err := json.Marshal(counters)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		DML struct {
			Calls uint64 `json:"scopeAliasBatchCalls"`
			Rows  uint64 `json:"scopeAliasBatchRows"`
		} `json:"dml"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.DML.Calls != calls || decoded.DML.Rows != rows {
		t.Fatalf("optimized chunk counts = %d/%d; want %d/%d (calls/rows)", decoded.DML.Calls, decoded.DML.Rows, calls, rows)
	}
}

func TestDMLScopeAliasBulkHostRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			// Hosted source has its own floor; the Apex helper and trigger stay at 62
			// there. At 62/67 all source components share the selected API.
			apexAPI := api
			if api == "59.0" {
				apexAPI = "62.0"
			}
			files := map[string]string{
				"sfdx-project.json": fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, apexAPI),
				"force-app/main/default/classes/BulkHost.cls": `public class BulkHost {
 @AuraEnabled public static String seed() {
  BulkObserver.reset();
  List<Account> rows=new List<Account>();
  for(Integer i=0;i<400;i++) rows.add(new Account(Name='Owned '+i));
  List<Account> alias=rows;
  insert rows;
  for(Account row:alias) System.assertNotEquals(null,row.Id);
  return BulkObserver.summary();
 }
 @AuraEnabled public static String revise() {
  BulkObserver.reset();
  List<Account> rows=[SELECT Id,Name FROM Account];
  System.assertEquals(400,rows.size());
  List<Account> alias=rows;
  for(Account row:rows) row.Name='Revised';
  update rows;
  for(Account row:alias) {System.assertNotEquals(null,row.Id); System.assertEquals('Revised',row.Name);}
  System.assertEquals(400,[SELECT COUNT() FROM Account WHERE Name='Revised']);
  return BulkObserver.summary();
 }
 public String insertAction() {return seed();}
 public String updateAction() {return revise();}
}`,
				"force-app/main/default/classes/BulkObserver.cls": `public class BulkObserver {
 public static Integer beforeCalls=0;
 public static Integer afterCalls=0;
 public static Integer seen=0;
 public static List<Id> priorIds=new List<Id>();
 public static void reset() {beforeCalls=0;afterCalls=0;seen=0;priorIds=new List<Id>();}
 public static String summary() {System.assertEquals(400,seen);System.assertEquals(2,beforeCalls);System.assertEquals(2,afterCalls);return seen+':'+beforeCalls+':'+afterCalls;}
 public static void observe() {
  System.assertEquals(200,Trigger.new.size());
  System.assertEquals(afterCalls*200,seen);
  for(Id prior:priorIds) System.assertNotEquals(null,prior);
  if(Trigger.isBefore) {
   beforeCalls++; System.assertEquals(afterCalls+1,beforeCalls);
   for(Account row:Trigger.new) {if(Trigger.isInsert) System.assertEquals(null,row.Id);else System.assertNotEquals(null,row.Id);}
  } else {
   afterCalls++;System.assertEquals(beforeCalls,afterCalls);
   for(Account row:Trigger.new) {System.assertNotEquals(null,row.Id);priorIds.add(row.Id);}
   seen+=Trigger.new.size();
  }
 }
}`,
				"force-app/main/default/triggers/BulkObserverTrigger.trigger": `trigger BulkObserverTrigger on Account (before insert,after insert,before update,after update) {BulkObserver.observe();}`,
			}
			for name, content := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(name, ".cls") || strings.HasSuffix(name, ".trigger") {
					version := apexAPI
					tag := "ApexClass"
					if strings.HasSuffix(name, ".trigger") {
						tag = "ApexTrigger"
					}
					if strings.HasSuffix(name, "BulkHost.cls") {
						version = api
					}
					meta := fmt.Sprintf("<%s><apiVersion>%s</apiVersion></%s>", tag, version, tag)
					if err := os.WriteFile(path+"-meta.xml", []byte(meta), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			for _, route := range []string{"anonymous", "lwc", "visualforce"} {
				for _, operation := range []string{"insert", "update"} {
					t.Run(route+"/"+operation, func(t *testing.T) {
						for _, warm := range []bool{false, true} {
							machine := vm.New(nil)
							org := apextest.OrgFromIndex(index)
							org.APIVersion = apexAPI
							machine.SetOrg(&org)
							if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
								t.Fatal(err)
							}
							invoke := func(op string) {
								method := "seed"
								action := "insertAction"
								if op == "update" {
									method = "revise"
									action = "updateAction"
								}
								if route == "anonymous" {
									program, err := vm.CompileAnonymous("System.assertEquals('400:2:2', BulkHost." + method + "());")
									if err != nil {
										t.Fatal(err)
									}
									if _, err := machine.Execute(program); err != nil {
										t.Fatal(err)
									}
									return
								}
								var result vm.UIInvocationResult
								var err error
								if route == "lwc" {
									result, err = machine.InvokeLWCMethod("BulkHost", method, nil)
								} else {
									result, err = machine.InvokeVisualforceAction("BulkHost", action, "/apex/BulkHost", nil)
								}
								if err != nil || !result.Success || result.ReturnValue != "400:2:2" {
									t.Fatalf("warm=%v: err=%v result=%+v", warm, err, result)
								}
							}
							if operation == "update" {
								invoke("insert")
							}
							recorder := vm.NewPerfRecorder()
							machine.SetPerfRecorder(recorder)
							invoke(operation)
							dmlAssertBatchCounts(t, recorder.Snapshot(), 2, 400)
						}
					})
				}
			}
		})
	}
}
