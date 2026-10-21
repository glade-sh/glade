package apextest

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSetupMetadataSnapshotRetainsCapturedBodies(t *testing.T) {
	for _, relative := range []string{"settings/BusinessHours.settings-meta.xml", "duplicateRules/Contact.Grove_Email_Check.duplicateRule-meta.xml"} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join("testdata", "conformance", "setup_metadata", "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			sources := newSourceCache()
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "force-app", "main", "default", relative)
			original, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, target, string(original)+"\n")
			captured, err := sources.readCapturedProjectMetadata(target)
			if err != nil || !bytes.Equal(captured, original) {
				t.Fatalf("captured metadata changed after a disk edit: %v", err)
			}
			if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
				t.Fatal("metadata edit did not invalidate the captured runtime generation")
			}
		})
	}
}

func TestProjectRuntimeMetadataUsesGenerationSnapshot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	snapshot, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	sources := newSourceCache()
	sources.capturedProject = &snapshot
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\n")
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
		t.Fatalf("capture with immutable project snapshot: %v", err)
	}
	captured, err := sources.capturedProjectSnapshot(index.Project.Root)
	if err != nil || captured.Root != snapshot.Root {
		t.Fatalf("captured project = %q, %v; want %q", captured.Root, err, snapshot.Root)
	}
}
