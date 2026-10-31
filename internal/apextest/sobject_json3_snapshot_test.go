package apextest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSObjectJSON3TranslationSnapshot(t *testing.T) {
	root := t.TempDir()
	fixture := filepath.Join("testdata", "conformance", "sobject_json3", "project")
	if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	sources := newSourceCache()
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "standardValueSetTranslations", "Industry-fr.standardValueSetTranslation-meta.xml")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(original)+"\n")
	captured, err := sources.readCapturedProjectMetadata(path)
	if err != nil || !bytes.Equal(captured, original) {
		t.Fatalf("captured translation changed after a disk edit: %v", err)
	}
	if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
		t.Fatal("translation metadata edit did not invalidate the captured runtime generation")
	}
}
