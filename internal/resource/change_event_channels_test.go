package resource

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// CDC03 uses this deployed channel member at both source APIs. Preserve its
// selection through discovery, captured metadata, package hashing and loading.
func TestChangeDataCaptureSelectedChannelProjectAndSnapshotAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"source", "metadata", "dependency", "shim"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				name := "Selected.platformEventChannelMember-meta.xml"
				if route == "metadata" {
					name = "Selected.platformEventChannelMember"
				}
				path := filepath.Join(root, "force-app/main/default/platformEventChannelMembers", name)
				body, err := os.ReadFile("../apextest/testdata/conformance/cdc_journal_delivery/extended/project/force-app/main/default/platformEventChannelMembers/ChangeEvents_ContactChangeEvent.platformEventChannelMember-meta.xml")
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, string(body))
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Contains(SetupRecordMetadataFiles(p), path) {
					t.Fatal("channel selection missing from captured project metadata")
				}
				before, err := packageartifact.SourceHash(p)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, string(body)+"\n")
				after, err := packageartifact.SourceHash(p)
				if err != nil || before == after {
					t.Fatalf("channel metadata missing from package hash: %v", err)
				}
				child := p
				if route == "dependency" {
					p = project.Project{ManagedPackageDependencies: []project.ManagedPackageDependency{{Status: "loaded", Project: &child}}}
				}
				if route == "shim" {
					p = project.Project{PackageShims: []project.PackageShim{{Status: "loaded", Project: &child}}}
				}
				for _, loader := range []string{"strict", "tolerant", "captured"} {
					t.Run(loader, func(t *testing.T) {
						org := storage.NewOrgState()
						var err error
						switch loader {
						case "strict":
							err = ApplyProject(&org, p)
						case "tolerant":
							_, err = ApplyProjectTolerant(&org, p)
						case "captured":
							writeFile(t, path, "<invalid")
							err = ApplyProject(&org, p, func(got string) ([]byte, error) {
								if got == path {
									return body, nil
								}
								return os.ReadFile(got)
							})
						}
						if err != nil {
							t.Fatal(err)
						}
						encoded, err := json.Marshal(org.Metadata)
						if err != nil {
							t.Fatal(err)
						}
						var got struct {
							Entities []string `json:"changeEventEntities"`
						}
						if err := json.Unmarshal(encoded, &got); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got.Entities, []string{"ContactChangeEvent"}) {
							t.Fatalf("selected channel entities=%v", got.Entities)
						}
					})
				}
			})
		}
	}
}
