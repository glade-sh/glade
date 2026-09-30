package apextest

import (
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"path/filepath"
	"testing"
)

// Executes the admitted indexed helper without TestContext/fixed-result bypass.
// Local storage is immediately searchable; Salesforce readiness was separate.
func TestExecuteIndexedPhraseSearchContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladePhraseIndexed53.cls"), "public class GladePhraseIndexed53 {\n public static final String MARKER='glade-phrase-owned-20260906@example.invalid';\n public static final String A='glaphrasealpha',B='glaphrasebeta';\n public static List<Id> setup(){\n  System.assertEquals(0,[SELECT count() FROM Contact WHERE Email=:MARKER],'owned marker collision');\n  List<Contact> rows=new List<Contact>{new Contact(LastName=A+' '+B,Title='adjacent',Email=MARKER),new Contact(LastName=A+' bridge '+B,Title='separated',Email=MARKER),new Contact(LastName=B+' '+A,Title='reversed',Email=MARKER),new Contact(LastName=A,Title='single',Email=MARKER)};insert rows;\n  List<Id> ids=new List<Id>();for(Contact row:rows)ids.add(row.Id);return ids;\n }\n private static Map<String,Id> owned(List<Id> ids){\n  System.assertEquals(4,ids.size(),'owned IDs');System.assertEquals(4,new Set<Id>(ids).size(),'distinct IDs');\n  Map<String,Id> labels=new Map<String,Id>();for(Contact row:[SELECT Id,Title,Email FROM Contact WHERE Id IN :ids]){System.assertEquals(MARKER,row.Email,'owned marker');labels.put(row.Title,row.Id);}\n  System.assertEquals(new Set<String>{'adjacent','separated','reversed','single'},labels.keySet(),'owned labels');return labels;\n }\n private static Set<Id> matches(String expression){\n  String q='FIND \\''+String.escapeSingleQuotes(expression)+'\\' IN NAME FIELDS RETURNING Contact(Id WHERE Email = \\''+MARKER+'\\')';\n  List<List<SObject>> groups=Search.query(q);System.assertEquals(1,groups.size(),'returning group');return new Map<Id,SObject>(groups[0]).keySet();\n }\n public static Boolean ready(List<Id> ids){Map<String,Id> labels=owned(ids);return matches(A)==new Set<Id>(ids) && matches(B)==new Set<Id>{labels.get('adjacent'),labels.get('separated'),labels.get('reversed')};}\n public static String observe(List<Id> ids){\n  Map<String,Id> labels=owned(ids);System.assert(ready(ids),'INDEX_NOT_READY: no phrase observation credit');\n  List<Object> rows=new List<Object>();\n  for(String expression:new List<String>{'\"'+A+' '+B+'\"','\"'+B+' '+A+'\"',A+' '+B}){\n   Map<String,Object> row=new Map<String,Object>{'expression'=>expression};\n   try{Set<Id> found=matches(expression);List<String> names=new List<String>();for(String label:labels.keySet())if(found.contains(labels.get(label)))names.add(label);names.sort();System.assertEquals(found.size(),names.size(),'no foreign result IDs');row.put('matches',names);}catch(Exception error){row.put('type',error.getTypeName());row.put('message',error.getMessage());}\n   rows.add(row);\n  }\n  System.assertEquals(3,rows.size(),'complete expressions');return JSON.serialize(new Map<String,Object>{'done'=>true,'rows'=>rows});\n }\n public static void cleanup(List<Id> ids){owned(ids);delete [SELECT Id FROM Contact WHERE Id IN :ids AND Email=:MARKER];System.assertEquals(0,[SELECT count() FROM Contact WHERE Id IN :ids],'owned records removed');}\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladePhraseIndexed53.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <apiVersion>53.0</apiVersion>\n    <status>Active</status>\n</ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}], \"sourceApiVersion\": \"53.0\"}")
	machine := vm.New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if err := RegisterCompiledProjectRuntimeForRequest(machine, CompileProjectRuntimeForRequest(loadTestIndex(t, root))); err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymousWithOptions("List<Id> ownedIds=GladePhraseIndexed53.setup();\nMap<String,Object> observed=(Map<String,Object>)JSON.deserializeUntyped(GladePhraseIndexed53.observe(ownedIds));\nSystem.assertEquals(true,observed.get('done'),'all observations complete');\nList<Object> rows=(List<Object>)observed.get('rows');System.assertEquals(3,rows.size(),'three queries');\nString a=GladePhraseIndexed53.A,b=GladePhraseIndexed53.B;\nList<String> queries=new List<String>{'\"'+a+' '+b+'\"','\"'+b+' '+a+'\"',a+' '+b};\nList<List<String>> expected=new List<List<String>>{new List<String>{'adjacent'},new List<String>{'reversed'},new List<String>{'adjacent','reversed','separated'}};\nfor(Integer i=0;i<3;i++){\n Map<String,Object> row=(Map<String,Object>)rows[i];\n System.assertEquals(new Set<String>{'expression','matches'},row.keySet(),'query '+i+' fields');\n System.assertEquals(queries[i],row.get('expression'),'query '+i+' input');\n System.assertEquals(JSON.serialize(expected[i]),JSON.serialize(row.get('matches')),'query '+i+' matches');\n}\nSystem.debug(LoggingLevel.ERROR,'PHRASE_ASSERTIONS_PASSED=3');\n", vm.CompileOptions{APIVersion: "53.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
