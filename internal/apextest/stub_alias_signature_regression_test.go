package apextest

import (
	"path/filepath"
	"testing"
)

// Alias equality is captured in schema_type_alias_boundaries/org.tsv:2-5.
func TestStubAliasSignatureProject(t *testing.T) {
	for _, api := range []string{"49.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"sample","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := map[string]string{
				"SampleService": `public virtual class SampleService {
 public virtual List<SObject> records(sObjectType kind, Set<Id> ids, List<String> names) { return null; }
 public virtual List<SObject> records(sObjectType kind, Set<Id> ids, List<String> names, String filter) { return null; }
 public virtual List<SObject> fields(SObjectField field) { return null; }
 public virtual List<SObject> description(DescribeFieldResult field) { return null; }
}`,
				"SampleProvider": `@IsTest public class SampleProvider implements System.StubProvider {
 private Map<String,List<Type>> signatures = new Map<String,List<Type>>();
 public void add(String name, Type a, Type b, Type c) { signatures.put(name, new List<Type>{a, b, c}); }
 public void addOne(String name, Type a) { signatures.put(name, new List<Type>{a}); }
 public Object handleMethodCall(Object receiver, String name, Type resultType, List<Type> types, List<String> names, List<Object> values) {
  if (signatures.containsKey(name) && signatures.get(name).equals(types)) return new List<SObject>{new Account(Name='matched')};
  return null;
 }
}`,
				"SampleProbe": `@IsTest private class SampleProbe {
 @IsTest static void matches() {
  SampleProvider provider = new SampleProvider();
  provider.add('records', SObjectType.class, Set<Id>.class, List<String>.class);
  provider.addOne('fields', SObjectField.class);
  provider.addOne('description', DescribeFieldResult.class);
  SampleService service = (SampleService) Test.createStub(SampleService.class, provider);
  for (SObject row : service.records(Account.SObjectType, new Set<Id>(), new List<String>())) System.assertEquals('matched', row.get('Name'));
  System.assertEquals('matched', service.fields(null)[0].get('Name'));
  System.assertEquals('matched', service.description(null)[0].get('Name'));
 }
}`,
			}
			for name, source := range classes {
				dir := filepath.Join(root, "force-app/main/default/classes")
				writeFile(t, filepath.Join(dir, name+".cls"), source)
				writeFile(t, filepath.Join(dir, name+".cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"SampleProbe"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected test selection: %+v", run.Summary())
			}
			if result := run.Suites[0].Cases[0]; result.Problem != nil {
				t.Fatalf("project test failed: %+v", result.Problem)
			}
		})
	}
}
