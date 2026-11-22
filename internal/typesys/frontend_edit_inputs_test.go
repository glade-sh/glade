package typesys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestFrontendEditCapturedInputsGeneration(t *testing.T) {
	for _, change := range []string{"stable", "changed-again", "restore-original", "other-source", "sidecar", "sidecar-added", "sidecar-removed", "same-stat", "missing", "physical", "configuration"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "force-app/main/default/classes")
			a, b := filepath.Join(dir, "First.cls"), filepath.Join(dir, "Second.cls")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			writeFile(t, a, "public class First { Integer before; }")
			writeFile(t, b, "public class Second { Integer stable; }")
			writeFile(t, b+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index, artifacts := BuildWithArtifacts(p, s)
			before, err := FrontendInputsForBuild(p, s, index, &artifacts)
			if err != nil {
				t.Fatal(err)
			}
			var records bytes.Buffer
			if err := WriteFrontendSnapshotRecords(&records, index, &artifacts); err != nil {
				t.Fatal(err)
			}
			writeFile(t, a, "public class First { Integer after; }")
			current, err := CaptureFrontendInputs(p, s)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "changed-again":
				writeFile(t, a, "public class First { Integer newer; }")
			case "restore-original":
				writeFile(t, a, "public class First { Integer before; }")
			case "other-source":
				writeFile(t, b, "public class Second { Integer changed; }")
			case "sidecar":
				writeFile(t, b+"-meta.xml", "<ApexClass><apiVersion>62.0</apiVersion></ApexClass>")
			case "sidecar-added":
				writeFile(t, a+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
			case "sidecar-removed":
				if err := os.Remove(b + "-meta.xml"); err != nil {
					t.Fatal(err)
				}
			case "same-stat":
				info, err := os.Stat(b)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, b, "public class Second { Integer change; }")
				if err := os.Chtimes(b, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(b); err != nil {
					t.Fatal(err)
				}
			case "physical":
				target := filepath.Join(dir, "target.txt")
				writeFile(t, target, "public class Second { Integer stable; }")
				if err := os.Remove(b); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, b); err != nil {
					t.Fatal(err)
				}
			case "configuration":
				p.SourceAPIVersion = "62.0"
			}
			old, oldArtifacts, oldErr := UpdateFrontendSnapshotRecords(p, s, records.Bytes(), before.Identity())
			got, gotArtifacts, gotErr := UpdateFrontendSnapshotRecordsWithInputs(p, s, records.Bytes(), before.Identity(), current)
			if (oldErr == nil) != (gotErr == nil) {
				t.Fatalf("old error %v, captured-input error %v", oldErr, gotErr)
			}
			if oldErr == nil {
				a, _ := json.Marshal(old)
				b, _ := json.Marshal(got)
				if !bytes.Equal(a, b) {
					t.Fatal("captured-input update differs from existing route")
				}
				var oldRecords, gotRecords bytes.Buffer
				if err := WriteFrontendSnapshotRecords(&oldRecords, old, &oldArtifacts); err != nil {
					t.Fatal(err)
				}
				if err := WriteFrontendSnapshotRecords(&gotRecords, got, &gotArtifacts); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(oldRecords.Bytes(), gotRecords.Bytes()) {
					t.Fatal("captured-input serialized records differ from existing route")
				}
				if change == "stable" {
					fresh, freshArtifacts := BuildWithArtifacts(p, s)
					var freshRecords bytes.Buffer
					if err := WriteFrontendSnapshotRecords(&freshRecords, fresh, &freshArtifacts); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(freshRecords.Bytes(), gotRecords.Bytes()) {
						t.Fatal("captured-input serialized records differ from fresh build")
					}
				}
				if err := ValidateBuildGeneration(got, &gotArtifacts); err != nil {
					t.Fatal(err)
				}
			} else if change == "stable" || change == "changed-again" {
				t.Fatalf("unexpected rejection: %v", gotErr)
			}
		})
	}
}

func TestFrontendPriorIdentityReusesCapturedReads(t *testing.T) {
	root := t.TempDir()
	p := project.Project{Root: root}
	for i := 0; i < 1500; i++ {
		name := fmt.Sprintf("Item%04d", i)
		path := filepath.Join(root, name+".cls")
		writeFile(t, path, "public class "+name+" {}")
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
		p.ApexFiles = append(p.ApexFiles, path)
	}
	_, artifacts := BuildWithArtifacts(p, schema.Schema{})
	current, err := CaptureFrontendInputs(p, schema.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	reader := func(path string) ([]byte, error) { reads++; return os.ReadFile(path) }
	old, err := captureFrontendInputsWithReader(p, schema.Schema{}, nil, artifacts.SourceDigests, nil, reader)
	if err != nil {
		t.Fatal(err)
	}
	if reads != 3000 {
		t.Fatalf("baseline compatibility source and sidecar reads = %d, want 3000", reads)
	}
	reads = 0
	reused, err := captureFrontendInputsWithReader(p, schema.Schema{}, nil, artifacts.SourceDigests, current, reader)
	if err != nil {
		t.Fatal(err)
	}
	if reads != 0 {
		t.Fatalf("captured compatibility reads = %d, want 0", reads)
	}
	if old.Identity() != reused.Identity() {
		t.Fatal("captured compatibility identity differs")
	}
}

// Reuse is valid only for the captured non-source generation, even when a
// local source edit would otherwise qualify for the incremental path.
func TestFrontendEditRejectsChangedNonSourceGeneration(t *testing.T) {
	for _, change := range []string{
		"flow-bytes", "flow-membership", "artifact-bytes", "artifact-loaded",
		"schema", "custom-metadata", "source-added", "source-removed",
		"dependency-source", "dependency-sidecar", "dependency-membership", "dependency-flow", "dependency-schema", "dependency-api", "dependency-namespace", "dependency-unloaded", "dependency-loaded",
		"shim-source", "shim-sidecar", "shim-membership", "shim-flow", "shim-api", "shim-namespace", "shim-unloaded", "shim-loaded",
	} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			class := filepath.Join(root, "force-app/classes/First.cls")
			second := filepath.Join(root, "force-app/classes/Second.cls")
			flow := filepath.Join(root, "force-app/flows/Example.flow-meta.xml")
			artifact := filepath.Join(root, "dependency.json")
			depRoot, shimRoot := filepath.Join(root, "dependency"), filepath.Join(root, "shim")
			depClass, shimClass := filepath.Join(depRoot, "Dependency.cls"), filepath.Join(shimRoot, "Stub.cls")
			depFlow := filepath.Join(depRoot, "Dependency.flow-meta.xml")
			shimFlow := filepath.Join(shimRoot, "Shim.flow-meta.xml")
			depObject := filepath.Join(depRoot, "objects/Thing__c/Thing__c.object-meta.xml")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			writeFile(t, class, "public class First { Integer before; }")
			writeFile(t, second, "public class Second {}")
			writeFile(t, flow, "<Flow><label>Before</label></Flow>")
			writeFile(t, artifact, `{"version":"before"}`)
			writeFile(t, depClass, "global class Dependency { Integer before; }")
			writeFile(t, shimClass, "global class Stub { Integer before; }")
			writeFile(t, depFlow, "<Flow><label>Before</label></Flow>")
			writeFile(t, shimFlow, "<Flow><label>Before</label></Flow>")
			writeFile(t, depObject, `<CustomObject><label>Before</label><pluralLabel>Things</pluralLabel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			dep := &project.Project{Root: depRoot, ApexFiles: []string{depClass}, FlowFiles: []string{depFlow}, ObjectFiles: []string{depObject}, SourceAPIVersion: "67.0"}
			shim := &project.Project{Root: shimRoot, ApexFiles: []string{shimClass}, FlowFiles: []string{shimFlow}, SourceAPIVersion: "67.0"}
			if change == "artifact-loaded" {
				captured, err := packageartifact.BuildCaptured(packageartifact.BuildCapturedOptions{
					Namespace: "artifact", SourceAPIVersion: "67.0",
					ApexTypes: []packageartifact.ApexType{{Kind: apexast.DeclarationClass, Name: "LibraryType", Namespace: "artifact", Modifiers: []string{"global"}}},
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := packageartifact.WriteJSON(artifact, captured); err != nil {
					t.Fatal(err)
				}
			}
			p.ManagedPackageDependencies = []project.ManagedPackageDependency{{Namespace: "dep", SourceRoot: depRoot, Status: "loaded", Project: dep}, {Namespace: "artifact", ArtifactPath: artifact, Status: "missing"}}
			p.PackageShims = []project.PackageShim{{Namespace: "shim", SourceRoot: shimRoot, Status: "loaded", Project: shim}}
			if change == "dependency-loaded" {
				p.ManagedPackageDependencies[0].Status = "missing"
				p.ManagedPackageDependencies[0].Project = nil
			}
			if change == "shim-loaded" {
				p.PackageShims[0].Status = "missing"
				p.PackageShims[0].Project = nil
			}
			s := schema.Schema{}
			index, artifacts := BuildWithArtifacts(p, s)
			before, err := FrontendInputsForBuild(p, s, index, &artifacts)
			if err != nil {
				t.Fatal(err)
			}
			var records bytes.Buffer
			if err := WriteFrontendSnapshotRecords(&records, index, &artifacts); err != nil {
				t.Fatal(err)
			}
			writeFile(t, class, "public class First { Integer after; }")
			control, err := CaptureFrontendInputs(p, s)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := UpdateFrontendSnapshotRecords(p, s, records.Bytes(), before.Identity()); err != nil {
				t.Fatalf("existing edit route rejected unchanged non-source inputs: %v", err)
			}
			if _, _, err := UpdateFrontendSnapshotRecordsWithInputs(p, s, records.Bytes(), before.Identity(), control); err != nil {
				t.Fatalf("captured edit route rejected unchanged non-source inputs: %v", err)
			}
			switch change {
			case "flow-bytes":
				writeFile(t, flow, "<Flow><label>After</label></Flow>")
			case "flow-membership":
				p.FlowFiles = nil
			case "artifact-bytes":
				writeFile(t, artifact, `{"version":"after"}`)
			case "artifact-loaded":
				p.ManagedPackageDependencies[1].Status = "loaded"
				loaded, loadedArtifacts := BuildWithArtifacts(p, s)
				found := false
				for _, typ := range loaded.Types {
					if typ.Name == "LibraryType" && typ.Artifact {
						found = true
					}
				}
				if !found {
					t.Fatal("loaded artifact type missing from build")
				}
				if err := WriteFrontendSnapshotRecords(&bytes.Buffer{}, loaded, &loadedArtifacts); err == nil {
					t.Fatal("serialized loaded artifact without byte provenance")
				}
			case "schema":
				s.Objects = []schema.Object{{Name: "New__c"}}
			case "custom-metadata":
				s.CustomMetadataRecords = []schema.CustomMetadataRecord{{FullName: "Settings.Row", ObjectName: "Settings__mdt"}}
			case "source-added":
				extra := filepath.Join(root, "force-app/classes/Extra.cls")
				writeFile(t, extra, "public class Extra {}")
				p.ApexFiles = append(p.ApexFiles, extra)
			case "source-removed":
				p.ApexFiles = []string{class}
			case "dependency-source":
				writeFile(t, depClass, "global class Dependency { Integer after; }")
			case "dependency-sidecar":
				writeFile(t, depClass+"-meta.xml", "<ApexClass><apiVersion>62.0</apiVersion></ApexClass>")
			case "dependency-membership":
				dep.ApexFiles = nil
			case "dependency-flow":
				writeFile(t, depFlow, "<Flow><label>After</label></Flow>")
			case "dependency-schema":
				writeFile(t, depObject, `<CustomObject><label>After</label><pluralLabel>Things</pluralLabel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
			case "dependency-api":
				dep.SourceAPIVersion = "62.0"
			case "dependency-namespace":
				p.ManagedPackageDependencies[0].Namespace = "other"
			case "dependency-unloaded":
				p.ManagedPackageDependencies[0].Status = "missing"
				p.ManagedPackageDependencies[0].Project = nil
			case "dependency-loaded":
				p.ManagedPackageDependencies[0].Status = "loaded"
				p.ManagedPackageDependencies[0].Project = dep
			case "shim-source":
				writeFile(t, shimClass, "global class Stub { Integer after; }")
			case "shim-sidecar":
				writeFile(t, shimClass+"-meta.xml", "<ApexClass><apiVersion>62.0</apiVersion></ApexClass>")
			case "shim-membership":
				shim.ApexFiles = nil
			case "shim-flow":
				writeFile(t, shimFlow, "<Flow><label>After</label></Flow>")
			case "shim-api":
				shim.SourceAPIVersion = "62.0"
			case "shim-namespace":
				p.PackageShims[0].Namespace = "other"
			case "shim-unloaded":
				p.PackageShims[0].Status = "missing"
				p.PackageShims[0].Project = nil
			case "shim-loaded":
				p.PackageShims[0].Status = "loaded"
				p.PackageShims[0].Project = shim
			}
			current, err := CaptureFrontendInputs(p, s)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := UpdateFrontendSnapshotRecords(p, s, records.Bytes(), before.Identity()); err == nil {
				t.Fatal("existing edit route accepted changed non-source inputs")
			}
			if _, _, err := UpdateFrontendSnapshotRecordsWithInputs(p, s, records.Bytes(), before.Identity(), current); err == nil {
				t.Fatal("captured edit route accepted changed non-source inputs")
			}
		})
	}
}
