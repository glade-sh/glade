package apextest

import (
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"path/filepath"
	"testing"
)

// Executes the admitted indexed helper without TestContext/fixed-result bypass.
// Local storage is immediately searchable; Salesforce readiness was separate.
func TestExecuteIndexedBooleanSearchContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeBooleanSearch53.cls"), "public class GladeBooleanSearch53 {\n public static final String MARKER='glade-boolean-a1-20260906-74@example.invalid';\n public static final String A='glaboolseventyfouralpha',B='glaboolseventyfourbeta',C='glaboolseventyfourgamma';\n public static List<Id> setup(){\n  System.assertEquals(0,[SELECT count() FROM Contact WHERE Email=:MARKER],'Owned marker collision');\n  List<Contact> records=new List<Contact>{new Contact(LastName=A,Title='A',Email=MARKER),new Contact(LastName=B,Title='B',Email=MARKER),new Contact(LastName=A+' '+B,Title='AB',Email=MARKER),new Contact(LastName=C,Title='C',Email=MARKER)};insert records;\n  List<Id> ids=new List<Id>();for(Contact row:records)ids.add(row.Id);return ids;\n }\n public static String query(String expression){return 'FIND \\''+String.escapeSingleQuotes(expression)+'\\' IN NAME FIELDS RETURNING Contact(Id,Title WHERE Email = \\''+MARKER+'\\')';}\n private static Map<String,Id> validateOwned(List<Id> ids){\n  System.assertEquals(4,ids.size());System.assertEquals(4,new Set<Id>(ids).size());\n  List<Contact> records=[SELECT Id,Title,Email FROM Contact WHERE Id IN :ids];System.assertEquals(4,records.size());Map<String,Id> byLabel=new Map<String,Id>();\n  for(Contact row:records){System.assertEquals(MARKER,row.Email);System.assertEquals(false,byLabel.containsKey(row.Title));byLabel.put(row.Title,row.Id);}\n  System.assertEquals(new Set<String>{'A','B','AB','C'},byLabel.keySet());return byLabel;\n }\n private static Set<Id> matches(String expression){List<List<SObject>> groups=Search.query(query(expression));System.assertEquals(1,groups.size());Set<Id> ids=new Set<Id>();for(SObject row:groups[0])ids.add((Id)row.get('Id'));System.assertEquals(groups[0].size(),ids.size());return ids;}\n public static Boolean ready(List<Id> ids){Map<String,Id> m=validateOwned(ids);return matches(A)==new Set<Id>{m.get('A'),m.get('AB')} && matches(B)==new Set<Id>{m.get('B'),m.get('AB')} && matches(C)==new Set<Id>{m.get('C')};}\n public static void verifyIndexed(List<Id> ids){\n  Map<String,Id> m=validateOwned(ids);System.assert(ready(ids),'INDEX_NOT_READY: no Boolean matching verdict');\n  System.assertEquals(new Set<Id>{m.get('AB')},matches(A+' AND '+B),'AND');\n  System.assertEquals(new Set<Id>{m.get('A'),m.get('B'),m.get('AB')},matches(A+' OR '+B),'OR');\n  System.assertEquals(new Set<Id>{m.get('AB'),m.get('C')},matches('('+A+' AND '+B+') OR '+C),'Original grouped shape');\n  System.assertEquals(new Set<Id>{m.get('AB')},matches(A+' AND ('+B+' OR '+C+')'),'Right grouped');\n  System.assertEquals(new Set<Id>{m.get('A'),m.get('AB')},matches(A+' OR '+B+' AND '+C),'AND precedence');\n  System.assertEquals(new Set<Id>{m.get('AB'),m.get('C')},matches('('+A+' AND '+B+') OR glaboolseventyfourgam*'),'Grouped prefix');\n }\n public static void cleanup(List<Id> ids){validateOwned(ids);delete [SELECT Id FROM Contact WHERE Id IN :ids AND Email=:MARKER];System.assertEquals(0,[SELECT count() FROM Contact WHERE Id IN :ids]);}\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeBooleanSearch53.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}], \"sourceApiVersion\": \"53.0\"}")
	machine := vm.New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if err := RegisterCompiledProjectRuntimeForRequest(machine, CompileProjectRuntimeForRequest(loadTestIndex(t, root))); err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymousWithOptions("List<Id> ids=GladeBooleanSearch53.setup();GladeBooleanSearch53.verifyIndexed(ids);List<List<SObject>> spaceRows=Search.query(GladeBooleanSearch53.query(GladeBooleanSearch53.A+' '+GladeBooleanSearch53.B));System.assertEquals(1,spaceRows.size());System.assertEquals(1,spaceRows[0].size());System.assertEquals(ids[2],spaceRows[0][0].get('Id'));List<List<SObject>> groupRows=Search.query(GladeBooleanSearch53.query('('+GladeBooleanSearch53.A+' '+GladeBooleanSearch53.B+' AND '+GladeBooleanSearch53.C+')'));System.assertEquals(1,groupRows.size());System.assertEquals(0,groupRows[0].size());", vm.CompileOptions{APIVersion: "53.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
