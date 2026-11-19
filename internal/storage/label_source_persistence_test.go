package storage

import (
	"path/filepath"
	"testing"
)

func TestLabelTranslationSourcePersistence(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	empty, padded := "", " \u00a0 "
	org := NewOrgState()
	org.Metadata.Labels = []LabelMetadata{
		{Name: "Old", Language: "fr", Value: ""},
		{Name: "Empty", Language: "fr", Value: "", TranslationSourceRaw: &empty},
		{Name: "Padded", Language: "fr", Value: "", TranslationSourceRaw: &padded},
	}
	if err := store.Save(org); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Metadata.Labels) != 3 {
		t.Fatalf("labels=%d", len(loaded.Metadata.Labels))
	}
	for i, want := range []*string{nil, &empty, &padded} {
		got := loaded.Metadata.Labels[i].TranslationSourceRaw
		if (got == nil) != (want == nil) || (want != nil && *got != *want) {
			t.Fatalf("label %d raw=%v, want %v", i, got, want)
		}
	}
}
