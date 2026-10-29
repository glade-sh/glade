package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Schema.<StandardObject> is a valid declared, constructed, parameter, return,
// generic-argument and inherited-field type, for standard and setup objects. Native: testdata/schema_relationship_gate/org.tsv:301,
// 307,319,325 (apex_explicit_schema, API 62.0 and 67.0, loaded deploy and
// @IsTest pass; cases.json RelationExtra62R2/RelationExtra67R2).
func TestQualifiedReceiverSchemaStandardObjectTypeRoutes(t *testing.T) {
	const base = `public virtual class GardenSchemaBase {
    public Schema.Contact item = new Schema.Contact();
    protected Schema.Account parent;
    protected Schema.Group baseGroup = new Schema.Group();
    protected List<Schema.User> baseUsers = new List<Schema.User>();
}`
	const child = `public class GardenSchemaChild extends GardenSchemaBase {
    public static Schema.Contact shared = new Schema.Contact();
    public Schema.Group queue;
    public List<Schema.Group> queues = new List<Schema.Group>();
    public Map<Id, Schema.Group> queueById = new Map<Id, Schema.Group>();
    public Boolean observe() {
        Schema.Contact local = new Schema.Contact();
        Schema.Account account = new Schema.Account(Name = 'Garden');
        Schema.Group g = new Schema.Group(Name = 'Garden', Type = 'Queue');
        Schema.User u = new Schema.User(LastName = 'Garden');
        List<Schema.Group> gs = new List<Schema.Group>{ g };
        Map<Id, Schema.User> users = new Map<Id, Schema.User>();
        parent = account;
        queue = g;
        return item.Account == null && local.Account == null && check(account) && make().Account == null &&
            pick(g).Name == 'Garden' && gs.size() == 1 && users.isEmpty() && baseGroup.Name == null && baseUsers.isEmpty() && u.LastName == 'Garden';
    }
    public static Boolean check(Schema.Account value) { return value.Name == 'Garden'; }
    public static Schema.Contact make() { return shared; }
    public static Schema.Group pick(Schema.Group value) { return value; }
}`
	const explicit = `public class GardenSchemaExplicit { public static Boolean value(){Schema.Contact c=new Schema.Contact(); return c.Account == null;} }`
	const anonymous = `Schema.Contact c = new Schema.Contact();
Schema.Account a = new Schema.Account(Name = 'Garden');
System.assert(c.Account == null);
System.assert(a.Name == 'Garden');
Schema.Group g = new Schema.Group(Name = 'Garden', Type = 'Queue');
Schema.User u = new Schema.User(LastName = 'Garden');
List<Schema.Group> gs = new List<Schema.Group>{ g };
Map<Id, Schema.Group> byId = new Map<Id, Schema.Group>();
Map<Id, Schema.User> users = new Map<Id, Schema.User>();
System.assert(gs.size() == 1 && byId.isEmpty() && users.isEmpty() && u.LastName == 'Garden');`
	for _, tc := range []struct {
		api, metadata string
		objects       []schema.Object
	}{
		{"62.0", "declared", []schema.Object{{Name: "Account"}, {Name: "Contact"}}},
		{"67.0", "declared", []schema.Object{{Name: "Account"}, {Name: "Contact"}}},
		{"62.0", "standard-only", nil},
		{"67.0", "standard-only", nil},
	} {
		api := tc.api
		t.Run(api+"/"+tc.metadata, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			for name, source := range map[string]string{"GardenSchemaBase.cls": base, "GardenSchemaChild.cls": child, "GardenSchemaExplicit.cls": explicit} {
				path := filepath.Join(root, name)
				writeSemaFile(t, path, source)
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{Objects: tc.objects})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			t.Run("named", func(t *testing.T) {
				if result := Analyze(index); result.HasErrors() {
					t.Fatalf("diagnostics=%+v", result.Diagnostics)
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				if result := AnalyzeAnonymous(index, anonymous, api); result.HasErrors() {
					t.Fatalf("diagnostics=%+v", result.Diagnostics)
				}
			})
			t.Run("type-reference", func(t *testing.T) {
				a := NewAnalyzer()
				a.Analyze(index)
				model := buildSemaTypeMemberView(index)
				for _, reference := range []string{"Schema.Contact", "Schema.Account", "schema.contact", "Schema.Group", "Schema.User"} {
					if !semaTypeReferenceKnownAtVersion(a, model, "", reference, api) {
						t.Fatalf("type %q rejected", reference)
					}
				}
			})
		})
	}
}

// The unqualified SObjectType.<object>.fields describe forms stay valid.
// Native: ../apextest/testdata/conformance/qualified_receivers/cases.tsv:7,8,11
// with org.tsv (SObjectType.Account.fields.Name, SObjectType.Account.fieldSets,
// Schema.SObjectType.Account.fields.getMap() compile at API 62.0 and 67.0).
// Custom objects: qualified_receivers/cases.tsv:20,21,24 with org.tsv.
// Native rejects only <object>.SObjectType.fields.getMap()/get(String):
// testdata/conformance/receiver_member_routes/org.tsv:30-37,70-77.
// SObjectType.Account.fields.Name.getDescribe() has no native row; it keeps
// the previously accepted behaviour.
func TestQualifiedReceiverShortSObjectTypeFieldsRoutes(t *testing.T) {
	const body = `Schema.DescribeFieldResult a = SObjectType.Account.fields.Name;
Schema.DescribeFieldResult b = SObjectType.Account.fields.Name.getSObjectField().getDescribe();
Map<String, Schema.SObjectField> c = SObjectType.Account.fields.getMap();
Schema.DescribeFieldResult d = Schema.SObjectType.Account.fields.Name;
Schema.DescribeSObjectResult e = SObjectType.Account.getDescribe();
List<Schema.SObjectField> f = new List<Schema.SObjectField>{ Account.Name, Account.Industry };
Map<Schema.SObjectField, String> g = new Map<Schema.SObjectField, String>{ Account.Name => 'n' };
Map<String, Schema.FieldSet> h = SObjectType.Account.fieldSets.getMap();
Schema.SObjectField i = SObjectType.Account.fields.getMap().get('Name');
String j = SObjectType.Account.fields.Name.getName();
Schema.SObjectField k = Schema.SObjectType.Account.fields.getMap().get('Name');
Object l = SObjectType.Account.fields.Name.getDescribe();
Map<String, Schema.SObjectField> m = SObjectType.GardenBed__c.fields.getMap();
Schema.DescribeFieldResult n = SObjectType.GardenBed__c.fields.Depth__c;
String o = SObjectType.GardenBed__c.fields.Depth__c.getName();
Schema.SObjectField p = SObjectType.GardenBed__c.fields.getMap().get('Depth__c');
Map<String, Schema.FieldSet> q = SObjectType.GardenBed__c.fieldSets.getMap();`
	for _, api := range []string{"62.0", "67.0"} {
		for _, metadata := range []string{"declared", "custom-only"} {
			t.Run(api+"/"+metadata, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "GardenShortFields.cls")
				writeSemaFile(t, path, "public class GardenShortFields {\n    public static Schema.SObjectField nameField = SObjectType.Account.fields.Name.getSObjectField();\n    public static Boolean observe() {\n"+body+"\n        return a != null;\n    }\n}")
				objects := []schema.Object{{Name: "GardenBed__c", Fields: []schema.Field{{Name: "Depth__c", Type: "Number"}}}}
				if metadata == "declared" {
					objects = append(objects, schema.Object{Name: "Account"})
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{Objects: objects})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				if result := Analyze(index); result.HasErrors() {
					t.Fatalf("named diagnostics=%+v", result.Diagnostics)
				}
				if result := AnalyzeAnonymous(index, body, api); result.HasErrors() {
					t.Fatalf("anonymous diagnostics=%+v", result.Diagnostics)
				}
				a := NewAnalyzer()
				a.Analyze(index)
				model, scope := buildSemaTypeMemberView(index), newIRSemaScope(nil)
				for _, receiver := range []string{"SObjectType.Account.fields", "Schema.SObjectType.Account.fields", "SObjectType.Account.fieldSets", "SObjectType.GardenBed__c.fields"} {
					if !semaQualifiedCallReceiverKnown(a, "", receiver, scope, model) {
						t.Fatalf("receiver %q rejected", receiver)
					}
				}
			})
		}
	}
}
