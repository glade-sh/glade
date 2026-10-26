package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestSiteNetworkTestContextNativeProjectRoutes(t *testing.T) {
	const fixture = "testdata/conformance/site_network_test_context"
	raw, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	cases, err := os.ReadFile(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(cases)), "\n")[1:] {
		id, _, _ := strings.Cut(line, "\t")
		ids[id] = true
	}
	// Each metadata route executes the same deployed synthetic observation class.
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"disabled", "disabled_scratch_definition", "disabled_alternate_definition", "disabled_cumulusci", "disabled_cumulusci_template", "explicit_features", "scratch_setting", "site_metadata"} {
			t.Run(api+"/istest/"+shape, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				configuration := "networks_disabled"
				switch shape {
				case "disabled_scratch_definition", "disabled_alternate_definition", "disabled_cumulusci", "disabled_cumulusci_template":
					source, err := os.ReadFile(filepath.Join(fixture, "project/config/project-scratch-def.json"))
					if err != nil {
						t.Fatal(err)
					}
					path := "config/project-scratch-def.json"
					if shape == "disabled_alternate_definition" {
						path = "config/hc-project-scratch-def.json"
					}
					if shape == "disabled_cumulusci" || shape == "disabled_cumulusci_template" {
						path = "orgs/synthetic.json"
						config := "cumulusci.yml"
						if shape == "disabled_cumulusci_template" {
							config = "cumulusci.template.yml"
						}
						writeFile(t, filepath.Join(root, config), "orgs:\n  scratch:\n    dev:\n      config_file: orgs/synthetic.json\n")
					}
					writeFile(t, filepath.Join(root, path), string(source))
				case "explicit_features":
					configuration = "community_created"
					writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Sites, Communities]\n")
				case "scratch_setting":
					configuration = "networks_enabled"
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`)
				case "site_metadata":
					configuration = "community_created"
					writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Sites, Communities]\n")
					writeFile(t, filepath.Join(root, "force-app/main/default/sites/Synthetic.site-meta.xml"), `<CustomSite xmlns="http://soap.sforce.com/2006/04/metadata"><active>false</active><label>Synthetic</label><siteType>ChatterNetwork</siteType></CustomSite>`)
				}
				for _, name := range []string{"AuthContextProbe", "AuthContextProbeTest"} {
					source, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/classes", name+".cls"))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
					writeFile(t, path, string(source))
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native declarations compile: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				org := orgFromIndex(index)
				if len(org.Objects["Network"].Records) != 0 {
					t.Fatal("native enabled-without-Network-metadata shape has zero Network rows")
				}
				run := Run(index, Options{SelectedClasses: []string{"AuthContextProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 {
					t.Fatalf("expected one observation: %+v", run.Summary())
				}
				problem := run.Suites[0].Cases[0].Problem
				if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
					t.Fatalf("missing observation transport: %+v", problem)
				}
				_, payload, ok := strings.Cut(problem.Message, "OBS|")
				if !ok {
					t.Fatalf("missing observation: %s", problem.Message)
				}
				var got map[string]string
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatal(err)
				}
				checked := 0
				for _, row := range rows[1:] {
					if row[0] != configuration || row[1] != api || row[2] != "istest" || !ids[row[3]] {
						continue
					}
					checked++
					if got[row[3]] != row[4] {
						t.Errorf("%s: native=%q Glade=%q", row[3], row[4], got[row[3]])
					}
				}
				if checked != len(ids) {
					t.Fatalf("checked %d native cells, want %d", checked, len(ids))
				}
			})
		}
	}
}
