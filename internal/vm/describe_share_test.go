package vm

import (
	"encoding/csv"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDescribeShareNativeConformance(t *testing.T) {
	source, err := os.ReadFile("testdata/describe_sharing/probe.apex")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/describe_sharing/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 7 || strings.Join(rows[0], ",") != "source_api,row,observation" {
		t.Fatal("incomplete native matrix")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var want []string
			for _, row := range rows[1:] {
				if row[0] == api {
					want = append(want, "ROW\t"+row[1]+"\t"+row[2])
				}
			}
			if len(want) != 3 {
				t.Fatal("missing native observations")
			}
			base, template := describeSharePrimedBase(t)
			for range 2 {
				clone := describeShareClone(base, template)
				got, errText := describeShareRun(t, clone, describeShareRow{name: api, source: string(source)})
				if errText != "" || got != strings.Join(want, " | ") {
					t.Fatalf("native=%q; Glade=%q; error=%s", want, got, errText)
				}
			}
		})
	}
}

func TestDescribeShareSchemaMutationDoesNotLeak(t *testing.T) {
	for _, objectName := range []string{"Widget__c", "Setting__mdt"} {
		t.Run(objectName, func(t *testing.T) {
			org := describeShareOrg()
			org.Objects["Setting__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Setting__mdt", Fields: map[string]storage.Field{"Value__c": {APIName: "Value__c", Type: storage.FieldString}}}}
			template := storage.NewRuntimeTemplate(org)
			PrimeRuntimeTemplateSchema(&template)
			base := New(nil)
			base.PrimeMetadataSchema(&template.Org)
			warm := describeShareClone(base, &template)
			original := describeShareFieldMapOf(t, warm, objectName)
			writer := describeShareClone(base, &template)
			definition, ok := storage.EnsureMutableObjectDefinition(writer.Org, objectName)
			if !ok {
				t.Fatal("missing definition")
			}
			definition.Fields["Private__c"] = storage.Field{APIName: "Private__c", Type: storage.FieldString}
			writer.clearMetadataCaches()
			changed := describeShareFieldMapOf(t, writer, objectName)
			if _, ok := changed.Map[mapKey(String("private__c"))]; !ok {
				t.Fatal("writer lost schema overlay")
			}
			reader := describeShareClone(base, &template)
			pristine := describeShareFieldMapOf(t, reader, objectName)
			if _, ok := pristine.Map[mapKey(String("private__c"))]; ok {
				t.Fatal("schema overlay leaked into sibling")
			}
			if !reflect.DeepEqual(original.MapOrder, pristine.MapOrder) {
				t.Fatal("sibling field order changed")
			}
		})
	}
}

func describeShareFieldMapOf(t *testing.T, machine *VM, objectName string) Value {
	t.Helper()
	name, definition, ok := machine.describeObjectDefinition(objectName)
	if !ok {
		t.Fatalf("%s is not describable", objectName)
	}
	describe := machine.describeSObjectValue(name, definition)
	fields := describe.Fields["fields"]
	if fields.Kind != ValueObject {
		t.Fatalf("%s describe has no fields wrapper", objectName)
	}
	return fields.Fields["map"]
}

func mapIdentity(value Value) uintptr {
	return reflect.ValueOf(value.Map).Pointer()
}

func describeShareRunApex(t *testing.T, machine *VM, source string) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Clones of one primed base build each describe's schema parts once: the
// second clone reuses the first clone's prepared definition and field map,
// while each clone keeps its own describe root and fields wrapper.
func TestDescribeShareClonesShareSchemaParts(t *testing.T) {
	base, template := describeSharePrimedBase(t)
	first := describeShareClone(base, template)
	second := describeShareClone(base, template)
	firstMap := describeShareFieldMapOf(t, first, "Account")
	secondMap := describeShareFieldMapOf(t, second, "Account")
	if mapIdentity(firstMap) != mapIdentity(secondMap) {
		t.Fatal("second clone rebuilt the Account field map")
	}
	if !reflect.DeepEqual(firstMap.MapOrder, secondMap.MapOrder) || len(firstMap.Map) != len(secondMap.Map) {
		t.Fatal("shared field map differs between clones")
	}
	firstDefinition := first.describeDefCache["account"]
	secondDefinition := second.describeDefCache["account"]
	if reflect.ValueOf(firstDefinition.Fields).Pointer() != reflect.ValueOf(secondDefinition.Fields).Pointer() {
		t.Fatal("second clone prepared its own Account definition")
	}
	firstRoot := first.describeCache[first.describeSObjectCacheKey("Account")]
	secondRoot := second.describeCache[second.describeSObjectCacheKey("Account")]
	if reflect.ValueOf(firstRoot.Fields).Pointer() == reflect.ValueOf(secondRoot.Fields).Pointer() ||
		reflect.ValueOf(firstRoot.Fields["fields"].Fields).Pointer() == reflect.ValueOf(secondRoot.Fields["fields"].Fields).Pointer() {
		t.Fatal("clones share a describe root or fields wrapper")
	}
}

// An unprimed base (empty schema stamp) keeps today's per-VM describes.
func TestDescribeShareEmptyStampBuildsPerVM(t *testing.T) {
	unprimed := New(nil)
	left := unprimed.CloneRuntime(nil)
	right := unprimed.CloneRuntime(nil)
	leftOrg, rightOrg := describeShareOrg(), describeShareOrg()
	left.SetOrg(&leftOrg)
	right.SetOrg(&rightOrg)
	if mapIdentity(describeShareFieldMapOf(t, left, "Account")) == mapIdentity(describeShareFieldMapOf(t, right, "Account")) {
		t.Fatal("unprimed VMs share a field map")
	}
}

// Every way a clone can stop running the template schema detaches it from the
// shared describes: SetOrg, a mismatched or cleared stamp at install, a
// definition replaced after install, and runtime metadata changes.
func TestDescribeShareFailsClosed(t *testing.T) {
	base, template := describeSharePrimedBase(t)
	builder := describeShareClone(base, template)
	shared := mapIdentity(describeShareFieldMapOf(t, builder, "Account"))

	t.Run("SetOrg", func(t *testing.T) {
		clone := describeShareClone(base, template)
		org := template.CloneRuntimeOrg()
		clone.SetOrg(&org)
		if mapIdentity(describeShareFieldMapOf(t, clone, "Account")) == shared {
			t.Fatal("clone after SetOrg read the shared field map")
		}
	})
	t.Run("mismatched stamp at install", func(t *testing.T) {
		clone := base.CloneRuntime(nil)
		org := template.CloneRuntimeOrg()
		org.RuntimeSchemaStamp = "different-runtime-schema"
		clone.SetRuntimeTemplateOrg(&org)
	})
	t.Run("definition replaced after install", func(t *testing.T) {
		clone := describeShareClone(base, template)
		definition, ok := storage.EnsureMutableObjectDefinition(clone.Org, "Account")
		if !ok {
			t.Fatal("Account definition missing")
		}
		definition.Fields["Extra__c"] = storage.Field{APIName: "Extra__c", Label: "Extra", Type: storage.FieldString}
		fields := describeShareFieldMapOf(t, clone, "Account")
		if mapIdentity(fields) == shared {
			t.Fatal("clone with a replaced definition read the shared field map")
		}
		if _, ok := fields.Map[mapKey(String("extra__c"))]; !ok {
			t.Fatal("clone with a replaced definition lost its own field")
		}
		sibling := describeShareClone(base, template)
		siblingFields := describeShareFieldMapOf(t, sibling, "Account")
		if mapIdentity(siblingFields) != shared {
			t.Fatal("sibling stopped sharing after another clone changed its schema")
		}
		if _, ok := siblingFields.Map[mapKey(String("extra__c"))]; ok {
			t.Fatal("another clone's field reached the shared field map")
		}
	})
	t.Run("definition replaced without a stamp change", func(t *testing.T) {
		clone := describeShareClone(base, template)
		object := clone.Org.Objects["Account"]
		object.Definition = object.Definition.Clone()
		object.Definition.Fields["Quiet__c"] = storage.Field{APIName: "Quiet__c", Label: "Quiet", Type: storage.FieldString}
		clone.Org.Objects["Account"] = object
		fields := describeShareFieldMapOf(t, clone, "Account")
		if mapIdentity(fields) == shared {
			t.Fatal("clone with a replaced definition read the shared field map")
		}
		if _, ok := fields.Map[mapKey(String("quiet__c"))]; !ok {
			t.Fatal("clone with a replaced definition lost its own field")
		}
	})
	t.Run("metadata caches cleared", func(t *testing.T) {
		clone := describeShareClone(base, template)
		_ = describeShareFieldMapOf(t, clone, "Account")
		clone.clearMetadataCaches()
	})
}

// Apex code reaches a field map or token only through a private copy: the first
// exit swaps a copy into this VM's wrapper, every later exit reuses it, map
// aliases of one field keep one token, and the shared map is left unchanged.
func TestDescribeShareFirstExitPrivatizesFieldMap(t *testing.T) {
	base, template := describeSharePrimedBase(t)
	clone := describeShareClone(base, template)
	sharedMap := describeShareFieldMapOf(t, clone, "Account")
	sharedTokens := len(sharedMap.Map)
	sharedName := sharedMap.Map[mapKey(String("name"))]

	describeShareRunApex(t, clone, `Schema.SObjectField t = Schema.SObjectType.Account.fields.getMap().get('name');
t.label = 'zz';
System.assertEquals('zz', Account.SObjectType.getDescribe().fields.getMap().get('Name').label);
System.assertEquals('zz', Schema.SObjectType.Account.fields.Name.label);`)

	private := describeShareFieldMapOf(t, clone, "Account")
	if mapIdentity(private) == mapIdentity(sharedMap) {
		t.Fatal("getMap exposed the shared field map")
	}
	if private.Ref == sharedMap.Ref {
		t.Fatal("private field map kept the shared map's identity")
	}
	lower, upper := private.Map[mapKey(String("name"))], private.Map[mapKey(String("Name"))]
	if reflect.ValueOf(lower.Fields).Pointer() != reflect.ValueOf(upper.Fields).Pointer() {
		t.Fatal("aliases of one field no longer share one token")
	}
	if lower.Fields["label"].Text != "zz" {
		t.Fatalf("private token label = %q, want zz", lower.Fields["label"].Text)
	}
	if sharedName.Fields["label"].Text != "Account Name" || len(sharedMap.Map) != sharedTokens {
		t.Fatal("token write reached the shared field map")
	}

	sibling := describeShareClone(base, template)
	if got := describeShareFieldMapOf(t, sibling, "Account"); mapIdentity(got) != mapIdentity(sharedMap) {
		t.Fatal("sibling did not reuse the shared field map")
	}
	describeShareRunApex(t, sibling, `System.assertEquals('Account Name', Schema.SObjectType.Account.fields.getMap().get('name').label);`)
}
