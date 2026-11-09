package apextest

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
	"path/filepath"
	"testing"
)

func TestNamedCredentialGenerationCapturesDependencyMetadata(t *testing.T) {
	root := t.TempDir()
	depRoot := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(depRoot, "force-app/main/default/namedCredentials/ProbeEndpoint.namedCredential-meta.xml")
	const data = `<NamedCredential><label>Probe endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType></NamedCredential>`
	writeFile(t, path, data)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	dep, err := project.Load(depRoot)
	if err != nil {
		t.Fatal(err)
	}
	p.ManagedPackageDependencies = []project.ManagedPackageDependency{{Status: "loaded", Project: &dep}}
	sources := newSourceCache()
	sources.seedProjectSnapshot(&p)
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := sources.captureProjectRuntimeMetadata(typesys.Index{Project: typesys.ProjectInfo{Root: root}}, &generation); err != nil {
		t.Fatal(err)
	}
	body, err := sources.readCapturedProjectMetadata(path)
	if err != nil || string(body) != data || !generation.inputs[path].Present {
		t.Fatalf("dependency credential capture: %q %v", body, err)
	}
}
