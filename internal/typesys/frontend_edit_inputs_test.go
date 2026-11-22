package typesys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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
			old, _, oldErr := UpdateFrontendSnapshotRecords(p, s, records.Bytes(), before.Identity())
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
