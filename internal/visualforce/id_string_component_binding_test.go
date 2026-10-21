package visualforce

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

// The declaration is retained with its native API 59/67 deployment observations.
// The assignment subtest checks local integration, without claiming DOM parity.
func TestIdAttributeStringNativeComponentBinding(t *testing.T) {
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api+"/declaration", func(t *testing.T) {
			root := t.TempDir()
			fixture := filepath.Join("..", "apextest", "testdata", "conformance", "page_attribute_identity", "project")
			err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LoadProject(p); err != nil {
				t.Fatalf("native deployment accepted the component: %v", err)
			}
		})
		t.Run(api+"/assignment", func(t *testing.T) {
			machine := vm.New(nil)
			if err := machine.RegisterClass(vm.Class{Name: "MeadowAttributeController", APIVersion: api, Fields: map[string]vm.Field{
				"chosen": {Name: "chosen", Type: "String", Property: true, HasGetter: true, HasSetter: true, Access: "public"},
			}}); err != nil {
				t.Fatal(err)
			}
			controller := vm.Object("MeadowAttributeController")
			const identity = "001000000000001AAA"
			attrs := []Attribute{{Name: "recordKey", Type: "Id", AssignTo: "{!chosen}"}}
			if err := applyComponentAssignTo(machine, &controller, attrs, map[string]vm.Value{"recordKey": vm.String(identity)}); err != nil {
				t.Fatal(err)
			}
			if got := controller.Fields["chosen"]; got.Kind != vm.ValueString || got.Text != identity {
				t.Fatalf("assigned property=%+v", got)
			}
		})
	}
}

func TestIdAttributeStringBindingRetainsNativeTypeMismatch(t *testing.T) {
	body, err := os.ReadFile("testdata/v06_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Cases []struct {
			ID     string `json:"id"`
			Inputs map[string]struct {
				Files map[string]string `json:"files"`
			} `json:"inputs"`
			Diagnostics map[string][]struct {
				Message string `json:"message"`
			} `json:"diagnostics"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(body, &capture); err != nil {
		t.Fatal(err)
	}
	for _, row := range capture.Cases {
		if row.ID != "diagnostic_assign_type_mismatch" {
			continue
		}
		for _, api := range []string{"59.0", "67.0"} {
			t.Run(api, func(t *testing.T) {
				root := t.TempDir()
				for path, text := range row.Inputs[api].Files {
					writeFile(t, filepath.Join(root, path), text)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				_, err = LoadProject(p)
				want := row.Diagnostics[api]
				if len(want) == 0 || err == nil || !strings.Contains(err.Error(), want[0].Message) {
					t.Fatalf("native=%v; Glade=%v", want, err)
				}
			})
		}
		return
	}
	t.Fatal("missing captured incompatible-type control")
}
