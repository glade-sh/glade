package startupcache

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// gob drops a pointer to an empty string; nil, empty and non-empty custom label
// translation sources must all survive the split test cache.
func TestSplitCachePreservesLabelTranslationSourceRaw(t *testing.T) {
	root := t.TempDir()
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[],"sourceApiVersion":"62.0"}`)
	proof, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	empty, padded := "", "   "
	org := storage.NewOrgState()
	org.Metadata.Labels = []storage.LabelMetadata{
		{Name: "Old", Language: "fr"},
		{Name: "Empty", Language: "fr", TranslationSourceRaw: &empty},
		{Name: "Padded", Language: "fr", TranslationSourceRaw: &padded},
		{Name: "Empty2", Language: "de", TranslationSourceRaw: new(string)},
	}
	want := append([]storage.LabelMetadata(nil), org.Metadata.Labels...)
	entry, err := NewEntryWithValidatedInput(proof, org, CompiledRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	entry.RuntimeABI = "label-source-test"
	entry.RuntimeKey = "label-source-key"
	if _, err := WriteWithStats(&entry, SubdirTest); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFreshRuntimeWithValidatedInput(root, SubdirTest, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
	if err != nil || got == nil {
		t.Fatalf("fresh cache=%v err=%v", got, err)
	}
	if !reflect.DeepEqual(got.Org.Metadata.Labels, want) {
		t.Fatalf("labels restored as %#v, want %#v", got.Org.Metadata.Labels, want)
	}
	if got.Org.Metadata.Labels[1].TranslationSourceRaw == got.Org.Metadata.Labels[3].TranslationSourceRaw {
		t.Fatal("restored empty sources share storage")
	}
}
