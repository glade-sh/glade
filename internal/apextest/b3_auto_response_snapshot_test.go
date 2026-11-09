package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestB3AutoResponseMetadataSnapshotRetainsRuleAndTemplate(t *testing.T) {
	const fixture = "testdata/conformance/b3_auto_response/project"
	for _, relative := range []string{"autoResponseRules/Lead.autoResponseRules-meta.xml", "autoResponseRules/Case.autoResponseRules-meta.xml", "email/unfiled$public/B3Response.email-meta.xml", "email/unfiled$public/B3Response.email"} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(fixture, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, e := filepath.Rel(fixture, path)
				if e != nil {
					return e
				}
				data, e := os.ReadFile(path)
				if e != nil {
					return e
				}
				writeFile(t, filepath.Join(root, rel), string(data))
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
				t.Fatalf("captured metadata changed: %v", err)
			}
			if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
				t.Fatal("metadata edit did not invalidate runtime snapshot")
			}
		})
	}
}
