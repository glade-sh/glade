package vm_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPersonAccountReviewHostNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/person_account_followups"
	f, err := os.Open(filepath.Join(fixture, "review", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{}
	for _, row := range rows[1:] {
		if row[1] == "anon" {
			want[row[0]] = append(want[row[0]], row[2]+"|"+row[3])
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				if !strings.HasPrefix(rel, "force-app/") && !strings.HasPrefix(rel, "config/") {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				src := string(data)
				if strings.HasSuffix(rel, ".cls-meta.xml") {
					src = strings.ReplaceAll(strings.ReplaceAll(src, "62.0", api), "67.0", api)
				}
				dest := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
					return err
				}
				return os.WriteFile(dest, []byte(src), 0600)
			})
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"sfdx-project.json":                                      fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api),
				"force-app/main/default/classes/PersonHost.cls":          `global class PersonHost { @AuraEnabled global static String lwc() { return String.join(PersonFollowupReview.observe(),'~'); } global String vf() { return String.join(PersonFollowupReview.observe(),'~'); } }`,
				"force-app/main/default/classes/PersonHost.cls-meta.xml": fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api),
			}
			for name, content := range files {
				dest := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dest, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			for _, route := range []string{"lwc", "visualforce"} {
				t.Run(route, func(t *testing.T) {
					machine := vm.New(nil)
					org := apextest.OrgFromIndex(index)
					org.APIVersion = api
					machine.SetOrg(&org)
					if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
						t.Fatal(err)
					}
					var result vm.UIInvocationResult
					var err error
					if route == "lwc" {
						result, err = machine.InvokeLWCMethod("PersonHost", "lwc", nil)
					} else {
						result, err = machine.InvokeVisualforceAction("PersonHost", "vf", "/apex/PersonHost", nil)
					}
					if err != nil || !result.Success {
						t.Fatalf("host: %v %#v", err, result.Error)
					}
					text, ok := result.ReturnValue.(string)
					if !ok {
						t.Fatalf("return %#v", result.ReturnValue)
					}
					got := strings.Split(text, "~")
					expected := want[api]
					if len(got) != 17 || len(expected) != 17 {
						t.Fatalf("rows got %d want %d", len(got), len(expected))
					}
					for i := range got {
						if got[i] != expected[i] {
							t.Errorf("got %q want %q", got[i], expected[i])
						}
					}
				})
			}
		})
	}
}
