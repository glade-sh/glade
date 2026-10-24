package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestGenerationContainmentNamedAndRequestEntrypointRoutes(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		for _, input := range []string{"metadata", "class-marker", "custom-metadata-marker"} {
			t.Run(version+"/"+input, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "project")
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true},{"path":"../shared"}]}`, version))
				writeFile(t, filepath.Join(root, "force-app", "classes", "OwnedEntrypointTest.cls"), `@IsTest private class OwnedEntrypointTest { @IsTest static void check() { System.assertEquals(1,1); } }`)
				writeFile(t, filepath.Join(root, "..", "shared", ".keep"), "")
				index := loadTestIndex(t, root)
				outside := filepath.Join(root, "..", "outside", "Owned.cls")
				switch input {
				case "class-marker":
					writeFile(t, outside, "public class Owned {}")
					extra := typesys.Build(project.Project{Root: root, ApexFiles: []string{outside, outside}}, schema.Schema{})
					index.Types = append(index.Types, extra.Types...)
				case "metadata", "custom-metadata-marker":
					if input == "metadata" {
						outside = filepath.Join(filepath.Dir(outside), "Owned.Seed.md-meta.xml")
						writeFile(t, outside, workspaceMetadataXML)
					}
					// The indexed record escapes every loaded root; declaring its
					// directory as a package would make this a valid input.
					index.CustomMetadataRecords = []schema.CustomMetadataRecord{
						{ObjectName: "Owned__mdt", DeveloperName: "Seed", File: outside},
						{ObjectName: "Owned__mdt", DeveloperName: "Seed", File: outside},
					}
				}
				writeFile(t, filepath.Join(filepath.Dir(outside), "sfdx-project.json"), `{}`)
				for _, route := range []struct {
					name string
					run  func() error
				}{
					{"named-test", func() error {
						run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
						return fmt.Errorf("%s", firstRunProblem(run))
					}},
					{"anonymous-registration", func() error { return RegisterProjectRuntime(vm.New(nil), index) }},
					{"request-registration", func() error { return RegisterProjectRuntimeForRequest(vm.New(nil), index) }},
					{"new-request-runtime", func() error { _, err := NewProjectRuntimeForRequest(index); return err }},
					{"uncached-request-registration", func() error { return RegisterUncachedProjectRuntimeForRequest(vm.New(nil), index) }},
					{"request-compilation", func() error { _, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil); return err }},
				} {
					t.Run(route.name, func(t *testing.T) {
						if err := route.run(); err == nil || !strings.Contains(err.Error(), "outside source root") {
							t.Fatalf("escaped %s reached %s: %v", input, route.name, err)
						}
					})
				}
			})
		}
	}
}
