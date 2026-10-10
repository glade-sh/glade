package vm

import (
	"sort"
	"strconv"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// uncachedSchemaStampForOrg recomputes the stamp with every object hashed from
// scratch, bypassing schemaStampObjectMemos.
func uncachedSchemaStampForOrg(org *storage.OrgState) string {
	h := schemaStampFNVOffset
	h = schemaStampHashLower(h, org.Namespace)
	h = schemaStampHashByte(h, '|')
	names := make([]string, 0, len(org.Objects))
	for name, object := range org.Objects {
		if object.Definition.APIName != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		objectHash := schemaStampHashObject(name, org.Objects[name].Definition)
		for shift := 0; shift < 64; shift += 8 {
			h = schemaStampHashByte(h, byte(objectHash>>shift))
		}
	}
	return strconv.FormatUint(h, 16)
}

func schemaStampMemoTestOrg() storage.OrgState {
	org := storage.NewOrgState()
	for _, name := range []string{"Account", "Contact", "Opportunity", "Case"} {
		storage.EnsureStandardObject(&org, name)
	}
	org.Objects["Thing__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Thing__c", Label: "Thing", PluralLabel: "Things", KeyPrefix: "a01",
		Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
	}}
	return org
}

func metadataDeployItem(typeName string, fields map[string]string) Value {
	item := Value{Kind: ValueObject, Type: typeName, Fields: map[string]Value{}}
	for name, text := range fields {
		item.Fields[name] = Value{Kind: ValueString, Text: text}
	}
	return item
}

// Every definition-change path must change the stamp of an org whose standard
// objects were memoized, and the memoized stamp must equal a full rehash.
// Template cases mutate a runtime clone whose definitions are shared with the
// primed template, so memo entries recorded for the template are live.
func TestSchemaStampMemoSeesEveryDefinitionChangePath(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, *storage.OrgState)
	}{
		{"EnsureMutableObjectDefinition label", func(t *testing.T, org *storage.OrgState) {
			definition, ok := storage.EnsureMutableObjectDefinition(org, "Account")
			if !ok {
				t.Fatal("Account not found")
			}
			definition.Label = "Runtime Account"
			object := org.Objects["Account"]
			object.Definition = *definition
			org.Objects["Account"] = object
		}},
		{"field add", func(t *testing.T, org *storage.OrgState) {
			definition, _ := storage.EnsureMutableObjectDefinition(org, "Account")
			definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldDecimal}
			object := org.Objects["Account"]
			object.Definition = *definition
			org.Objects["Account"] = object
		}},
		{"field remove", func(t *testing.T, org *storage.OrgState) {
			definition, _ := storage.EnsureMutableObjectDefinition(org, "Contact")
			for name := range definition.Fields {
				if name != "Id" {
					delete(definition.Fields, name)
					break
				}
			}
			object := org.Objects["Contact"]
			object.Definition = *definition
			org.Objects["Contact"] = object
		}},
		{"field add in place on shared map", func(t *testing.T, org *storage.OrgState) {
			org.Objects["Account"].Definition.Fields["InPlace__c"] = storage.Field{APIName: "InPlace__c", Type: storage.FieldString}
		}},
		{"field type edit in place on shared map", func(t *testing.T, org *storage.OrgState) {
			fields := org.Objects["Account"].Definition.Fields
			field := fields["Name"]
			field.Type = storage.FieldBoolean
			fields["Name"] = field
		}},
		{"reference target edit in place on shared slice", func(t *testing.T, org *storage.OrgState) {
			for _, field := range org.Objects["Contact"].Definition.Fields {
				if len(field.ReferenceTo) > 0 {
					field.ReferenceTo[0] = "Renamed__c"
					return
				}
			}
			t.Fatal("Contact has no reference field")
		}},
		{"relationship edit in place on shared slice", func(t *testing.T, org *storage.OrgState) {
			relations := org.Objects["Contact"].Definition.Relations
			if len(relations) == 0 {
				t.Fatal("Contact has no relations")
			}
			relations[0].ChildRelationship = "Renamed__r"
		}},
		{"inherited relationship provenance edit in place on shared slice", func(t *testing.T, org *storage.OrgState) {
			relations := org.Objects["Contact"].Definition.Relations
			if len(relations) == 0 {
				t.Fatal("Contact has no relations")
			}
			relations[0].InheritedFrom = "Activity"
		}},
		{"metadata deploy custom field on standard object", func(t *testing.T, org *storage.OrgState) {
			machine := New(nil)
			machine.Org = org
			if err := machine.applyCustomFieldDeployment(metadataDeployItem("Metadata.CustomField", map[string]string{"fullName": "Account.Deployed__c", "label": "Deployed"})); err != nil {
				t.Fatal(err)
			}
		}},
		{"metadata install custom object", func(t *testing.T, org *storage.OrgState) {
			machine := New(nil)
			machine.Org = org
			if err := machine.applyCustomObjectDeployment(metadataDeployItem("Metadata.CustomObject", map[string]string{"fullName": "Installed__c", "label": "Installed"})); err != nil {
				t.Fatal(err)
			}
		}},
		{"standard object install", func(t *testing.T, org *storage.OrgState) {
			storage.EnsureStandardObject(org, "Lead")
		}},
		{"record type added", func(t *testing.T, org *storage.OrgState) {
			definition, _ := storage.EnsureMutableObjectDefinition(org, "Case")
			definition.RecordTypes = append(definition.RecordTypes, storage.RecordTypeInfo{Name: "Support", DeveloperName: "Support"})
			object := org.Objects["Case"]
			object.Definition = *definition
			org.Objects["Case"] = object
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			template := storage.NewRuntimeTemplate(schemaStampMemoTestOrg())
			PrimeRuntimeTemplateSchema(&template)
			before := template.RuntimeSchemaStamp
			if before != uncachedSchemaStampForOrg(&template.Org) {
				t.Fatal("primed stamp differs from a full rehash")
			}
			clone := template.CloneRuntimeOrg()
			base := New(nil)
			base.PrimeMetadataSchema(&clone)

			tc.mutate(t, &clone)
			// Paths that do not clear the trusted hint model callers that
			// edit shared maps in place; the hint is dropped as SetOrg's
			// callers do after ClearRuntimeSchemaStamp.
			clone.ClearRuntimeSchemaStamp()
			want := uncachedSchemaStampForOrg(&clone)
			if want == before {
				t.Fatal("mutation does not change the full-rehash stamp")
			}
			if got := schemaCacheStampForOrg(&clone); got != want {
				t.Fatalf("memoized stamp = %q, full rehash %q", got, want)
			}
			machine := base.CloneRuntime(nil)
			machine.SetRuntimeTemplateOrg(&clone)
			if got := machine.metadataCacheStamp; got != want || got == before {
				t.Fatalf("installed stamp = %q, want %q (template %q)", got, want, before)
			}
		})
	}
}

// ClearRuntimeSchemaStamp is the boundary that drops the trusted template
// hint: after it, installing the org recomputes the stamp through the memo.
func TestSchemaStampMemoAfterClearRuntimeSchemaStamp(t *testing.T) {
	template := storage.NewRuntimeTemplate(schemaStampMemoTestOrg())
	PrimeRuntimeTemplateSchema(&template)
	clone := template.CloneRuntimeOrg()
	if clone.RuntimeSchemaStamp != template.RuntimeSchemaStamp {
		t.Fatal("clone did not inherit the template stamp")
	}
	// Without a definition change the recomputed stamp is the template's.
	clone.ClearRuntimeSchemaStamp()
	machine := New(nil)
	machine.SetOrg(&clone)
	if got := machine.metadataCacheStamp; got != template.RuntimeSchemaStamp {
		t.Fatalf("recomputed unchanged stamp = %q, want %q", got, template.RuntimeSchemaStamp)
	}
	definition, _ := storage.EnsureMutableObjectDefinition(&clone, "Opportunity")
	definition.PluralLabel = "Deals"
	object := clone.Objects["Opportunity"]
	object.Definition = *definition
	clone.Objects["Opportunity"] = object
	if clone.RuntimeSchemaStamp != "" {
		t.Fatal("EnsureMutableObjectDefinition kept the template stamp")
	}
	machine.SetOrg(&clone)
	if got, want := machine.metadataCacheStamp, uncachedSchemaStampForOrg(&clone); got != want || got == template.RuntimeSchemaStamp {
		t.Fatalf("stamp after change = %q, want %q (template %q)", got, want, template.RuntimeSchemaStamp)
	}
}

// Fails if the memo is removed: a deep clone of an org, as apextest cold
// builds clone the cached standard org, rehashes only its custom object. The
// clone owns new maps and slices, so every standard object is matched by
// content. (Two orgs built separately can differ in relation order, which the
// stamp hashes, so they are not used here.)
func TestSchemaStampMemoSkipsUnchangedStandardObjects(t *testing.T) {
	first := schemaStampMemoTestOrg()
	firstStamp := schemaCacheStampForOrg(&first)
	second := first.Clone()
	before := schemaStampObjectsHashed.Load()
	secondStamp := schemaCacheStampForOrg(&second)
	if hashed := schemaStampObjectsHashed.Load() - before; hashed != 1 {
		t.Fatalf("second stamp hashed %d objects, want only the custom object", hashed)
	}
	if secondStamp != firstStamp || secondStamp != uncachedSchemaStampForOrg(&second) {
		t.Fatalf("memoized stamp %q, first %q, full rehash %q", secondStamp, firstStamp, uncachedSchemaStampForOrg(&second))
	}
}
