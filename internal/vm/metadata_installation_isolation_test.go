package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Installation is a new owner boundary even when immutable schema preparation
// has the same generation. These are host isolation controls; native read rows
// remain testdata/describe_read_workload/org.tsv:7-10,16-19.
func TestMetadataInstallationDiscardsOwnedDescribeValues(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"template", "prime", "generic"} {
				t.Run(fmt.Sprintf("namespace=%v/api=%s/%s", namespaced, api, route), func(t *testing.T) {
					base, template := describeSharePrimedBaseFor(t, namespaced)
					machine := describeShareClone(base, template)
					raw := machine.schemaGlobalDescribe()
					key := mapKey(String("Account"))
					token := raw.Map[key]
					if token.Fields == nil {
						t.Fatal("missing control token")
					}
					token.Fields["object"] = String("Contact")
					object := machine.Org.Objects["Account"].Definition
					desc := machine.describeSObjectValue("Account", object)
					desc.Fields["label"] = String("private label")
					fields, ok := machine.privateDescribeFieldMap(desc.Fields["fields"])
					if !ok {
						t.Fatal("missing field map")
					}
					field := fields.Map[mapKey(String("name"))]
					field.Fields["field"] = String("Id")
					org := template.CloneRuntimeOrg()
					switch route {
					case "template":
						machine.SetRuntimeTemplateOrg(&org)
					case "prime":
						machine.PrimeMetadataSchema(&org)
						machine.SetRuntimeTemplateOrg(&org)
					case "generic":
						machine.SetOrg(&org)
					}
					program, err := CompileAnonymousWithOptions(`System.assertEquals('Account', String.valueOf(Schema.getGlobalDescribe().get('account')));
System.assertNotEquals('private label', Account.SObjectType.getDescribe().getLabel());
System.assertEquals('Name', Account.SObjectType.getDescribe().fields.getMap().get('name').getDescribe().getName());`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					// Already returned values remain owned by their old holders.
					if token.Fields["object"].Text != "Contact" || desc.Fields["label"].Text != "private label" || field.Fields["field"].Text != "Id" {
						t.Fatal("installation rewrote a retained holder")
					}
					if _, err := describeShareClone(base, template).Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestMetadataPrimingReplacesUnstampedPreparation(t *testing.T) {
	machine := New(nil)
	first := describeShareOrg()
	machine.PrimeMetadataSchema(&first)
	old := machine.describeShare
	second := first.Clone()
	object := second.Objects["Account"]
	field := object.Definition.Fields["Name"]
	field.Label = "new label"
	object.Definition.Fields["Name"] = field
	second.Objects["Account"] = object
	// Field label is deliberately outside the compact schema stamp. Equal text may
	// not authorize retaining preparation from the previous mutable org.
	if schemaCacheStampForOrg(&first) != schemaCacheStampForOrg(&second) {
		t.Fatal("control stamps differ")
	}
	machine.PrimeMetadataSchema(&second)
	if machine.describeShare == old {
		t.Fatal("mutable priming retained another org's preparation")
	}
	// Frozen siblings, in contrast, are authorized to share by generation.
	template := storage.NewRuntimeTemplate(second)
	PrimeRuntimeTemplateSchema(&template)
	machine.PrimeMetadataSchema(&template.Org)
	sibling := New(nil)
	sibling.PrimeMetadataSchema(&template.Org)
	if machine.describeShare != sibling.describeShare {
		t.Fatal("frozen generation lost shared preparation")
	}
}
