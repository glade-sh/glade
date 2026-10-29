package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestDMLPolicyMetadataSnapshotKeepsMatchingAndAssignmentSources(t *testing.T) {
	const fixture = "testdata/conformance/dml_policies/project"
	for _, relative := range []string{"matchingRules/Contact.matchingRule-meta.xml", "assignmentRules/Case.assignmentRules-meta.xml", "assignmentRules/Lead.assignmentRules-meta.xml"} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(fixture, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(fixture, path)
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), string(source))
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
			path := filepath.Join(root, "force-app/main/default", relative)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(original)+"\n")
			captured, err := sources.readCapturedProjectMetadata(path)
			if err != nil || string(captured) != string(original) {
				t.Fatalf("captured policy changed after disk edit: %v", err)
			}
			if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
				t.Fatal("policy edit did not invalidate runtime metadata")
			}
		})
	}
}
