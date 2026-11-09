package apextest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const staticInitMdtRecord = `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>%s</label><protected>false</protected><values><field>Bucket__c</field><value xsi:type="xsd:string">%s</value></values><values><field>Status__c</field><value xsi:type="xsd:string">%s</value></values><values><field>Handler__c</field><value xsi:type="xsd:string">%s</value></values></CustomMetadata>` + "\n"

func staticInitMdtFiles(api string) map[string]string {
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>\n"
	field := func(name string) string {
		return `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>` + name + `</fullName><label>` + name + `</label><type>Text</type><length>80</length></CustomField>` + "\n"
	}
	cls := func(name, body string) map[string]string {
		return map[string]string{
			"force-app/main/default/classes/" + name + ".cls":          body,
			"force-app/main/default/classes/" + name + ".cls-meta.xml": meta,
		}
	}
	files := map[string]string{
		"sfdx-project.json": `{"sourceApiVersion": "` + api + `", "packageDirectories": [{"path": "force-app", "default": true}]}`,
		"force-app/main/default/objects/Rule__mdt/Rule__mdt.object-meta.xml":        `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Rule</label><pluralLabel>Rules</pluralLabel><visibility>Public</visibility></CustomObject>` + "\n",
		"force-app/main/default/objects/Rule__mdt/fields/Bucket__c.field-meta.xml":  field("Bucket__c"),
		"force-app/main/default/objects/Rule__mdt/fields/Status__c.field-meta.xml":  field("Status__c"),
		"force-app/main/default/objects/Rule__mdt/fields/Handler__c.field-meta.xml": field("Handler__c"),
		"force-app/main/default/customMetadata/Rule.R1.md-meta.xml":                 strings.Replace(staticInitMdtRecord, "%s", "R1", 1),
	}
	rec := func(n, bucket, status, handler string) {
		files["force-app/main/default/customMetadata/Rule."+n+".md-meta.xml"] = sprintfMdt(n, bucket, status, handler)
	}
	rec("R1", "A", "Open", "")
	rec("R2", "A", "Closed", "")
	rec("R3", "B", "Open", "")
	rec("H1", "H", "Open", "GoodHandler")
	for k, v := range cls("Loader", `public class Loader {
 public Map<String, List<String>> get(List<String> buckets) {
  Map<String, List<String>> out = new Map<String, List<String>>();
  for (Rule__mdt r : Rule__mdt.getAll().values()) {
   if (!buckets.contains(r.Bucket__c)) continue;
   if (!out.containsKey(r.Bucket__c)) out.put(r.Bucket__c, new List<String>());
   out.get(r.Bucket__c).add(r.Status__c);
  }
  return out;
 }
}`) {
		files[k] = v
	}
	for k, v := range cls("Svc", `public class Svc {
 private static Map<String, List<String>> buckets = new Loader().get(new List<String>{'A','B'});
 private static List<String> a = buckets.get('A');
 private String status;
 public Svc(String s) { status = s; }
 public Boolean isA() { return a.contains(status); }
}`) {
		files[k] = v
	}
	for k, v := range cls("Handler", `public interface Handler { void run(List<SObject> rows); }`) {
		files[k] = v
	}
	for k, v := range cls("GoodHandler", `public class GoodHandler implements Handler { public static Integer calls = 0; public void run(List<SObject> rows) { calls++; } }`) {
		files[k] = v
	}
	for k, v := range cls("Dispatcher", `public class Dispatcher {
 public static void dispatch(List<SObject> rows) {
  for (Rule__mdt r : [SELECT Handler__c FROM Rule__mdt WHERE Handler__c != null]) {
   Object o = Type.forName(r.Handler__c).newInstance();
   if (!(o instanceof Handler)) throw new IllegalArgumentException('Invalid handler implementation: ' + r.Handler__c);
   ((Handler) o).run(rows);
  }
 }
}`) {
		files[k] = v
	}
	files["force-app/main/default/triggers/AccTrg.trigger"] = `trigger AccTrg on Account (before insert) { Dispatcher.dispatch(Trigger.new); if (!new Svc('Open').isA()) { throw new TypeException('static'); } }`
	files["force-app/main/default/triggers/AccTrg.trigger-meta.xml"] = "<ApexTrigger xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexTrigger>\n"
	for k, v := range cls("SvcTest", `@IsTest
private class SvcTest {
 @IsTest static void staticInitIsPopulated() {
  System.assert(new Svc('Open').isA());
  System.assert(new Svc('Closed').isA());
  System.assert(!new Svc('Nope').isA());
 }
 @IsTest static void triggerDispatch() {
  insert new Account(Name = 'x');
  System.assertEquals(1, GoodHandler.calls);
 }
}`) {
		files[k] = v
	}
	return files
}

func sprintfMdt(label, bucket, status, handler string) string {
	r := strings.NewReplacer("%s", "\x00")
	_ = r
	out := staticInitMdtRecord
	for _, v := range []string{label, bucket, status, handler} {
		out = strings.Replace(out, "%s", v, 1)
	}
	return out
}

// A static initializer that builds a map through a service reading custom
// metadata must see the project's records, from tests and from triggers.
func TestStaticInitializerBuildsMapFromCustomMetadataService(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			for path, content := range staticInitMdtFiles(api) {
				writeFile(t, filepath.Join(root, path), content)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{NoDiskCache: true})
			data, _ := json.Marshal(run)
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
				t.Fatalf("static init / trigger dispatch over custom metadata failed: %s", data)
			}
		})
	}
}

// In a namespaced project, a custom metadata record names a local object and
// field without the prefix, while describe returns the prefixed name. The
// service that groups buckets compares the two, so the static map must hold
// the bucket and the list must not be null.
func TestStaticInitializerBucketsNamespacedEntityDefinitionRelationships(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>\n"
			rec := func(object, field, status string) string {
				return `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>b</label><protected>false</protected><values><field>Object__c</field><value xsi:type="xsd:string">` + object + `</value></values><values><field>Field__c</field><value xsi:type="xsd:string">` + field + `</value></values><values><field>Status__c</field><value xsi:type="xsd:string">` + status + `</value></values></CustomMetadata>` + "\n"
			}
			text := func(n string) string {
				return `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>` + n + `</fullName><label>` + n + `</label><type>Text</type><length>80</length></CustomField>` + "\n"
			}
			files := map[string]string{
				"sfdx-project.json": `{"namespace":"pkg","sourceApiVersion":"` + api + `","packageDirectories":[{"path":"force-app","default":true}]}`,
				"force-app/main/default/objects/Delivery__c/Delivery__c.object-meta.xml":      `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Delivery</label><pluralLabel>Deliveries</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>` + "\n",
				"force-app/main/default/objects/Delivery__c/fields/Status__c.field-meta.xml":  text("Status__c"),
				"force-app/main/default/objects/Grouped__mdt/Grouped__mdt.object-meta.xml":    `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Grouped</label><pluralLabel>Grouped</pluralLabel><visibility>Public</visibility></CustomObject>` + "\n",
				"force-app/main/default/objects/Grouped__mdt/fields/Object__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Object__c</fullName><label>Object</label><type>MetadataRelationship</type><referenceTo>EntityDefinition</referenceTo><relationshipLabel>Objects</relationshipLabel><relationshipName>Object</relationshipName></CustomField>` + "\n",
				"force-app/main/default/objects/Grouped__mdt/fields/Field__c.field-meta.xml":  `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Field__c</fullName><label>Field</label><type>MetadataRelationship</type><referenceTo>FieldDefinition</referenceTo><relationshipLabel>Fields</relationshipLabel><relationshipName>Field</relationshipName><metadataRelationshipControllingField>Grouped__mdt.Object__c</metadataRelationshipControllingField></CustomField>` + "\n",
				"force-app/main/default/objects/Grouped__mdt/fields/Status__c.field-meta.xml": text("Status__c"),
				"force-app/main/default/customMetadata/Grouped.Open.md-meta.xml":              rec("Delivery__c", "Status__c", "Open"),
				"force-app/main/default/classes/Loader.cls": `public class Loader {
 public Map<String, List<String>> get(String objectName, String fieldName) {
  Map<String, List<String>> out = new Map<String, List<String>>();
  for (Grouped__mdt b : [SELECT Status__c, Object__r.QualifiedApiName, Field__r.QualifiedApiName FROM Grouped__mdt]) {
   if (b.Object__r.QualifiedApiName != objectName || b.Field__r.QualifiedApiName != fieldName) continue;
   if (!out.containsKey('A')) out.put('A', new List<String>());
   out.get('A').add(b.Status__c);
  }
  return out;
 }
}`,
				"force-app/main/default/classes/Svc.cls": `public class Svc {
 private static Map<String, List<String>> buckets = new Loader().get(Delivery__c.SObjectType.getDescribe().getName(), Delivery__c.Status__c.getDescribe().getName());
 private static List<String> a = buckets.get('A');
 public static Boolean isOpen(String s) { return a.contains(s); }
}`,
				"force-app/main/default/classes/SvcNsTest.cls": `@IsTest
private class SvcNsTest {
 @IsTest static void bucketIsFound() { System.assert(Svc.isOpen('Open')); }
}`,
			}
			for _, n := range []string{"Loader", "Svc", "SvcNsTest"} {
				files["force-app/main/default/classes/"+n+".cls-meta.xml"] = meta
			}
			root := t.TempDir()
			for path, content := range files {
				writeFile(t, filepath.Join(root, path), content)
			}
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
			data, _ := json.Marshal(run)
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
				t.Fatalf("namespaced entity relationship bucket missing: %s", data)
			}
		})
	}
}
