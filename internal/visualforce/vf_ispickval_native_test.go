package visualforce_test

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

const ispickvalFamily = "testdata/vf_ispickval"

var ispickvalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type ispickvalRow struct{ api, name, outcome, value string }

func ispickvalRows(t *testing.T, api string) []ispickvalRow {
	t.Helper()
	f, err := os.Open(filepath.Join(ispickvalFamily, "api"+api[:2], "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]ispickvalRow, 0, len(records)-1)
	for _, r := range records[1:] {
		if r[0] != api {
			t.Fatalf("native API=%q, want %q", r[0], api)
		}
		rows = append(rows, ispickvalRow{r[0], r[1], r[2], r[3]})
	}
	return rows
}

func ispickvalPageName(name string) string {
	return "PickProbe_" + strings.ReplaceAll(name, "-", "_")
}

// ispickvalProject copies the owned project at the given API version. Only the
// named pages are kept; keep == nil keeps none, so a caller picks its cases.
func ispickvalProject(t *testing.T, api string, keep map[string]bool) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(ispickvalFamily, "project")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.ToSlash(rel), "force-app/main/default/pages/") {
			base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(rel), ".page-meta.xml"), ".page")
			if !keep[base] {
				return nil
			}
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "67.0", api)), 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// ispickvalOrg is the scratch org's running user: UserType Standard, en_US,
// America/Los_Angeles, no geocode accuracy.
func ispickvalOrg() *storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"00e000000000001AAA": {ID: "00e000000000001AAA", Object: "Profile", Fields: map[string]storage.Value{
			"Name": storage.StringValue("System Administrator"), "UserType": storage.StringValue("Standard"),
		}},
	}}
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"005000000000001AAA": {ID: "005000000000001AAA", Object: "User", Fields: map[string]storage.Value{
			"Username": storage.StringValue("probe@example.test"), "ProfileId": storage.IDValue("00e000000000001AAA"),
			"UserType": storage.StringValue("Standard"), "LocaleSidKey": storage.StringValue("en_US"),
			"TimeZoneSidKey": storage.StringValue("America/Los_Angeles"), "LanguageLocaleKey": storage.StringValue("en_US"),
		}},
	}}
	return &org
}

func ispickvalMachine(t *testing.T, root string) (*vm.VM, project.Project, visualforce.Index) {
	t.Helper()
	return ispickvalMachineWith(t, root, nil)
}

// ispickvalMachineWith is ispickvalMachine with a hook that adjusts the org
// (for example to load a standard object definition) before the VM starts.
func ispickvalMachineWith(t *testing.T, root string, adjust func(*storage.OrgState)) (*vm.VM, project.Project, visualforce.Index) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, err := visualforce.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(typesys.Build(p, sch), nil)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	org := ispickvalOrg()
	if adjust != nil {
		adjust(org)
	}
	machine.SetOrg(org)
	machine.SetCurrentUser(org.Objects["User"].Records["005000000000001AAA"])
	if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
		t.Fatal(err)
	}
	visualforce.SetVMRenderEnvironment(machine, p)
	return machine, p, index
}

// ispickvalNormalize applies the capture's own normalization: a message channel
// token is a fresh UUID for each render.
func ispickvalNormalize(t *testing.T, name, got string) string {
	t.Helper()
	if !strings.HasPrefix(name, "msg-channel") {
		return got
	}
	re := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	tokens := re.FindAllString(got, -1)
	for _, token := range tokens {
		if !ispickvalUUID.MatchString(token) || token != tokens[0] {
			t.Fatalf("%s: tokens %v are not one UUID", name, tokens)
		}
	}
	return re.ReplaceAllString(got, "{uuid}")
}

func TestVisualforceISPICKVALNativeRows(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			rows := ispickvalRows(t, api)
			if len(rows) != 115 {
				t.Fatalf("native rows=%d, want 115", len(rows))
			}
			render := map[string]bool{}
			for _, row := range rows {
				if row.outcome == "render" {
					render[ispickvalPageName(row.name)] = true
				}
			}
			machine, _, index := ispickvalMachine(t, ispickvalProject(t, api, render))
			for _, row := range rows {
				if row.outcome != "render" {
					continue
				}
				page := ispickvalPageName(row.name)
				t.Run("render/"+row.name, func(t *testing.T) {
					for _, route := range []string{"RenderPage", "AnonymousGetContent"} {
						var rendered string
						if route == "RenderPage" {
							result, err := visualforce.RenderPage(visualforce.PageRenderRequest{VFIndex: index, Machine: machine, PageName: page})
							if err != nil {
								t.Fatalf("%s: %v", route, err)
							}
							if result.Error != nil {
								t.Fatalf("%s: %v", route, result.Error.Message)
							}
							rendered = result.HTML
						} else {
							program, err := vm.CompileAnonymous(fmt.Sprintf("System.debug(new PageReference('/apex/%s').getContent().toString());", page))
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.Execute(program)
							if err != nil {
								t.Fatalf("%s: %v", route, err)
							}
							if len(result.Debug) != 1 {
								t.Fatalf("%s: debug rows=%d", route, len(result.Debug))
							}
							rendered = result.Debug[0]
						}
						got := outputTextCaseHTML(t, rendered)
						value, present := got[row.name]
						if !present || len(got) != 1 {
							t.Fatalf("%s: HTML=%q", route, got)
						}
						want := outputTextCaseHTML(t, `<div data-case="expected">`+row.value+`</div>`)["expected"]
						if actual := ispickvalNormalize(t, row.name, value); actual != want {
							t.Fatalf("%s: HTML=%q, native=%q", route, actual, want)
						}
					}
				})
			}
			for _, row := range rows {
				if row.outcome != "error" {
					continue
				}
				t.Run("error/"+row.name, func(t *testing.T) {
					root := ispickvalProject(t, api, map[string]bool{ispickvalPageName(row.name): true})
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					_, err = visualforce.LoadProject(p)
					if err == nil {
						t.Fatalf("LoadProject succeeded, native error=%q", row.value)
					}
					if !strings.Contains(err.Error(), row.value) {
						t.Fatalf("error=%q, native=%q", err.Error(), row.value)
					}
				})
			}
		})
	}
}

// These shapes were accepted before the ISPICKVAL change. They have no native
// row, so validation must not introduce a new rejection for them.
func TestVisualforceISPICKVALNativeRowsPreserveBase(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		for _, tc := range []struct{ name, expression string }{
			{"organization-other", "$Organization.TimeZoneSidKey"},
			{"uncaptured-operator", "$User.UserType >= 'Standard'"},
			{"case-function", "CASE(CONTAINS('abc', 'a'), true, 'T', 'F')"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				page := ispickvalPageName("case-true")
				root := ispickvalProject(t, api, map[string]bool{page: true})
				path := filepath.Join(root, "force-app/main/default/pages", page+".page")
				body := `<apex:page controller="PickProbeController">{!` + tc.expression + `}</apex:page>`
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := visualforce.LoadProject(p); err != nil {
					t.Fatalf("base-compatible expression rejected: %v", err)
				}
			})
		}
	}
}
