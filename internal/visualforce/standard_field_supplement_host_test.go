package visualforce_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardFieldSupplementVisualforceNative(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_field_supplement_routes/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows[1:] {
		if row[1] != "visualforce" {
			continue
		}
		count++
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			name := "Case251Page" + strings.TrimSuffix(row[0], ".0")
			source, err := os.ReadFile("../apextest/testdata/conformance/standard_field_supplement_routes/" + name + ".page")
			if err != nil {
				t.Fatal(err)
			}
			writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			page := filepath.Join(root, "force-app/main/default/pages", name)
			writeDescribeHostFile(t, page+".page", string(source))
			writeDescribeHostFile(t, page+".page-meta.xml", fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>%s</label></ApexPage>`, row[0], name))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			vf, err := visualforce.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = row[0]
			storage.EnsureStandardObject(&org, "Lead")
			machine := vm.New(nil)
			machine.SetOrg(&org)
			for _, state := range []string{"cold", "warm"} {
				t.Run(state, func(t *testing.T) {
					result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: name})
					if err != nil || result.Error != nil {
						t.Fatalf("render: %v / %v", err, result.Error)
					}
					want := strings.Split(row[2], "|")
					if len(want) != 4 {
						t.Fatalf("invalid native marker: %q", row[2])
					}
					_, after, found := strings.Cut(result.HTML, "CASE251|")
					actual, _, ended := strings.Cut(after, "|END251")
					parts := strings.Split(actual, "|")
					if !found || !ended || len(parts) != 2 || parts[0] != want[1] || parts[1] != want[2] {
						t.Fatalf("native marker %q, local marker %q", row[2], actual)
					}
				})
			}
		})
	}
	if count != 2 {
		t.Fatalf("expected two native VF rows, got %d", count)
	}
}

// Compare local VF repair to the complete local definition; the native Apex
// list is verified separately on its captured route.
func TestStandardFieldSupplementVisualforcePicklistRepair(t *testing.T) {
	for _, host := range []string{"59.0", "67.0"} {
		t.Run(host, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			writeDescribeHostFile(t, filepath.Join(base, "classes/FieldPicklistHost.cls"), `public class FieldPicklistHost { public String getObserved() { return String.valueOf(Schema.SObjectType.Lead.fields.CleanStatus.getDescribe().getPicklistValues().size()); } }`)
			writeDescribeHostFile(t, filepath.Join(base, "classes/FieldPicklistHost.cls-meta.xml"), `<ApexClass><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
			writeDescribeHostFile(t, filepath.Join(base, "pages/FieldPicklistHost.page"), `<apex:page controller="FieldPicklistHost">FIELD251PICK:{!observed}</apex:page>`)
			writeDescribeHostFile(t, filepath.Join(base, "pages/FieldPicklistHost.page-meta.xml"), fmt.Sprintf(`<ApexPage><apiVersion>%s</apiVersion></ApexPage>`, host))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			vf, err := visualforce.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, schema)
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Lead")
			state := org.Objects["Lead"]
			field := state.Definition.Fields["CleanStatus"]
			count := len(field.PicklistValues)
			if count == 0 {
				t.Fatal("missing complete local list")
			}
			field.Type = storage.FieldAny
			field.DisplayType = "ANY"
			field.PicklistValues = nil
			state.Definition.Fields["CleanStatus"] = field
			org.Objects["Lead"] = state
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
				t.Fatal(err)
			}
			result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: "FieldPicklistHost"})
			if err != nil || result.Error != nil || !strings.Contains(result.HTML, fmt.Sprintf("FIELD251PICK:%d", count)) {
				t.Fatalf("local VF/complete definition count: %v / %+v", err, result)
			}
		})
	}
}
