package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestNetworkMetadataCaptureRetainsProjectBytesRoutes(t *testing.T) {
	for _, route := range []string{"package", "extra", "external_extra"} {
		t.Run(route, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "project")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			dir := "force-app"
			if route == "extra" {
				dir = "supplemental"
			}
			if route == "external_extra" {
				dir = "../shared"
			}
			if route != "package" {
				writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  extraMetadataDirs: ["+dir+"]\n")
			}
			path := filepath.Join(root, dir, "networks/Synthetic Portal.network-meta.xml")
			native, err := os.ReadFile("testdata/conformance/network_configuration/Synthetic Portal.network-meta.xml")
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(native))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sources := newSourceCache()
			sources.seedProjectSnapshot(&p)
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			if err := sources.captureProjectRuntimeMetadata(typesys.Index{Project: typesys.ProjectInfo{Root: root}}, &generation); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, "changed after capture")
			data, err := sources.readCapturedProjectMetadata(path)
			if err != nil || string(data) != string(native) || !generation.inputs[path].Present {
				t.Fatalf("Network metadata generation not retained: body=%q err=%v input=%+v", data, err, generation.inputs[path])
			}
		})
	}
}
