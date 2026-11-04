package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// The host route repairs the local field and matches the complete local
// definition; native REST values are checked independently in storage tests.
func TestStandardPolicyPicklistVisualforceRepairRoute(t *testing.T) {
	for _, version := range []string{"59.0", "67.0"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			writeDescribeHostFile(t, filepath.Join(base, "classes/PolicyPicklistHost.cls"), `public class PolicyPicklistHost { public String getObserved() { return String.valueOf(Schema.SObjectType.Lead.fields.GenderIdentity.getDescribe().getPicklistValues().size()); } }`)
			writeDescribeHostFile(t, filepath.Join(base, "classes/PolicyPicklistHost.cls-meta.xml"), `<ApexClass><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
			writeDescribeHostFile(t, filepath.Join(base, "pages/PolicyPicklistHost.page"), `<apex:page controller="PolicyPicklistHost">FIELD251POLICY:{!observed}</apex:page>`)
			writeDescribeHostFile(t, filepath.Join(base, "pages/PolicyPicklistHost.page-meta.xml"), fmt.Sprintf(`<ApexPage><apiVersion>%s</apiVersion></ApexPage>`, version))
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
			field := state.Definition.Fields["GenderIdentity"]
			count := len(field.PicklistValues)
			if count == 0 {
				t.Fatal("missing complete local list")
			}
			field.PicklistValues = nil
			field.PicklistValuesConfigured = false
			state.Definition.Fields["GenderIdentity"] = field
			org.Objects["Lead"] = state
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
				t.Fatal(err)
			}
			result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: "PolicyPicklistHost"})
			if err != nil || result.Error != nil || !strings.Contains(result.HTML, fmt.Sprintf("FIELD251POLICY:%d", count)) {
				t.Fatalf("VF route vs complete local count: %v / %+v", err, result)
			}
		})
	}
}

func TestStandardPolicyPicklistVisualforceNativeValues(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/standard_policy_picklist_routes"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	for _, version := range []string{"59.0", "67.0"} {
		t.Run(version, func(t *testing.T) {
			expected := map[string]string{}
			for _, row := range rows[1:] {
				if row[0] == version && row[1] == "visualforce" {
					var value string
					if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
						t.Fatal(err)
					}
					expected[row[2]] = value
				}
			}
			if len(expected) != 8 {
				t.Fatalf("native VF rows=%d", len(expected))
			}
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			for _, name := range []string{"Case251PicklistProbe.cls", "Case251PicklistProbe.cls-meta.xml"} {
				body, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				writeDescribeHostFile(t, filepath.Join(base, "classes", name), string(body))
			}
			page := "Case251Picklist" + strings.TrimSuffix(version, ".0")
			for _, ext := range []string{".page", ".page-meta.xml"} {
				body, err := os.ReadFile(filepath.Join(fixture, page+ext))
				if err != nil {
					t.Fatal(err)
				}
				writeDescribeHostFile(t, filepath.Join(base, "pages", page+ext), string(body))
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			vf, err := visualforce.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			org := storage.NewOrgState()
			for key := range expected {
				parts := strings.SplitN(key, ".", 2)
				storage.EnsureStandardObject(&org, parts[0])
			}
			for key := range expected {
				parts := strings.SplitN(key, ".", 2)
				state := org.Objects[parts[0]]
				field := state.Definition.Fields[parts[1]]
				field.PicklistValues = nil
				field.PicklistValuesConfigured = false
				state.Definition.Fields[parts[1]] = field
				org.Objects[parts[0]] = state
			}
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
				t.Fatal(err)
			}
			result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: page})
			if err != nil || result.Error != nil {
				t.Fatalf("render: %v / %+v", err, result)
			}
			_, after, ok := strings.Cut(result.HTML, "CASE251START")
			if !ok {
				t.Fatalf("missing marker %s", result.HTML)
			}
			payload, _, ok := strings.Cut(after, "CASE251END")
			if !ok {
				t.Fatal("missing end marker")
			}
			var got map[string]string
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatalf("payload %q: %v", payload, err)
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("VF=%v native=%v", got, expected)
			}
		})
	}
}
