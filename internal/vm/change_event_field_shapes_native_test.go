package vm

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestChangeEventFieldShapesNativeOrgInstallationAPIRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/change_event_describe/followups/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		want := map[string]any{}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			cells := strings.SplitN(line, "\t", 5)
			if len(cells) != 5 {
				t.Fatal("invalid native observation")
			}
			if cells[0] == api && cells[1] == "features-on" && cells[2] == "administrator.anonymous" && (strings.HasPrefix(cells[3], "shape.") || cells[3] == "features.AccountChangeEvent") {
				var value any
				if err := json.Unmarshal([]byte(cells[4]), &value); err != nil {
					t.Fatal(err)
				}
				want[cells[3]] = value
			}
		}
		if len(want) != 4 {
			t.Fatalf("missing native shapes for API %s", api)
		}
		for _, context := range []string{"anonymous", "istest"} {
			for _, install := range []string{"register_after_org", "set_org_after_register", "template_org_after_register"} {
				t.Run(api+"/"+context+"/"+install, func(t *testing.T) {
					org := testDataOrg()
					org.APIVersion = api
					storage.EnsureStandardObject(&org, "Account")
					machine := New(nil)
					if install == "register_after_org" {
						machine.SetOrg(&org)
					}
					if err := machine.RegisterTrigger(Trigger{Name: "AccountDescribeObserver", Object: "AccountChangeEvent", Timing: "after", Operation: "insert", APIVersion: api}); err != nil {
						t.Fatal(err)
					}
					if install == "set_org_after_register" {
						machine.SetOrg(&org)
					} else if install == "template_org_after_register" {
						template := storage.NewRuntimeTemplate(org)
						PrimeRuntimeTemplateSchema(&template)
						machine.PrimeMetadataSchema(&template.Org)
						org = template.CloneRuntimeOrg()
						machine.SetRuntimeTemplateOrg(&org)
					}
					if context == "istest" {
						machine.EnableTestContext()
					}
					payload := changeEventObservation(t, machine, api, `
Map<String, Object> rows = new Map<String, Object>();
Map<String, Schema.SObjectField> fields = Schema.getGlobalDescribe().get('AccountChangeEvent').getDescribe().fields.getMap();
for (String name : new List<String>{'FirstName','LastName','Salutation'}) {
    Schema.DescribeFieldResult f = fields.get(name).getDescribe();
    rows.put('shape.' + name, new Map<String,Object>{'type'=>String.valueOf(f.getType()),'length'=>f.getLength(),'nillable'=>f.isNillable(),'label'=>f.getLabel()});
}
Map<String,Object> presence = new Map<String,Object>();
for (String name : new List<String>{'Division','IsCustomerPortal','RecordTypeId'}) presence.put(name, fields.containsKey(name));
rows.put('features.AccountChangeEvent', presence);
System.assert(false, 'OBSERVED|' + JSON.serialize(rows));`)
					var got map[string]any
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatal(err)
					}
					for id, expected := range want {
						if !reflect.DeepEqual(got[id], expected) {
							t.Errorf("%s: Glade=%v; native=%v", id, got[id], expected)
						}
					}
				})
			}
		}
	}
}
