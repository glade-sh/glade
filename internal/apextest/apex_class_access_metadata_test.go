package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// This is documented metadata projection coverage, not a native capture.
func TestApexClassAccessMetadataConformance(t *testing.T) {
	var data struct {
		APIs            []string
		Files           map[string]string
		TestClass       string
		ExpectedMethods int
	}
	raw, err := os.ReadFile("testdata/conformance/apex_class_access_metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIs, ",") != "62.0,64.0,67.0" || data.ExpectedMethods != 4 {
		t.Fatal("incomplete documented cases")
	}
	for _, api := range data.APIs {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			for name, source := range data.Files {
				writeFile(t, filepath.Join(root, filepath.FromSlash(name)), strings.ReplaceAll(source, "{{API}}", api))
			}
			report := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{data.TestClass}, NoDiskCache: true, Parallelism: 1})
			if got := report.Summary(); got.Total != data.ExpectedMethods || got.Passed != data.ExpectedMethods {
				raw, _ := json.Marshal(report)
				t.Fatal(string(raw))
			}
		})
	}
}

func TestApexClassAccessImportControls(t *testing.T) {
	for _, shape := range []string{"enabled", "existing", "collision", "disabled-unresolved"} {
		t.Run(shape, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "DeclaredAccess.permissionset-meta.xml")
			enabled := "true"
			if shape == "disabled-unresolved" {
				enabled = "false"
			}
			writeFile(t, file, `<PermissionSet><label>Declared access</label><classAccesses><apexClass>KnownEndpoint</apexClass><enabled>`+enabled+`</enabled></classAccesses><classAccesses><apexClass>MissingEndpoint</apexClass><enabled>true</enabled></classAccesses></PermissionSet>`)
			p := project.Project{PermissionSetFiles: []string{file}}
			org := storage.NewOrgState()
			parent := storage.ID("0PS000000000021")
			target := storage.ID("01p000000000011")
			org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{parent: {ID: parent, Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("DeclaredAccess"), "Label": storage.StringValue("Keep existing label")}}}}
			org.Objects["ApexClass"] = storage.ObjectState{Records: map[storage.ID]storage.Record{target: {ID: target, Object: "ApexClass", Fields: map[string]storage.Value{"Name": storage.StringValue("KnownEndpoint"), "NamespacePrefix": storage.NullValue()}}}}
			old := storage.Record{ID: "0J0000000000001", Object: "SetupEntityAccess", Fields: map[string]storage.Value{"ParentId": storage.StringValue(string(parent) + "GAA"), "SetupEntityId": storage.StringValue(string(target) + "AAA"), "SetupEntityType": storage.StringValue("ApexClass")}}
			if shape == "collision" {
				old.Fields["SetupEntityId"] = storage.IDValue("01p000000000099")
			}
			if shape == "existing" || shape == "collision" {
				org.Objects["SetupEntityAccess"] = storage.ObjectState{Records: map[storage.ID]storage.Record{old.ID: old}}
			}
			ApplyProjectPermissionSets(&org, p)
			ApplyProjectPermissionSets(&org, p)
			want := 1
			if shape == "collision" {
				want = 2
			}
			if shape == "disabled-unresolved" {
				want = 0
			}
			if got := len(org.Objects["SetupEntityAccess"].Records); got != want {
				t.Fatalf("junctions=%d,want%d", got, want)
			}
			if shape == "existing" || shape == "collision" {
				if got := org.Objects["SetupEntityAccess"].Records[old.ID]; !reflect.DeepEqual(got.Fields["SetupEntityId"], old.Fields["SetupEntityId"]) {
					t.Fatal("existing junction overwritten")
				}
			}
			if org.Objects["PermissionSet"].Records[parent].Fields["Label"].String != "Keep existing label" {
				t.Fatal("permission set overwritten")
			}
			if len(org.Objects["ApexClass"].Records) != 1 || len(org.Objects["PermissionSetAssignment"].Records) != 0 {
				t.Fatal("unresolved provider or assignment created")
			}
		})
	}
}
