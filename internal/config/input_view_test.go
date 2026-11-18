package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
)

func TestLoadNearestWithInputsPinsMissingConfigs(t *testing.T) {
	root := t.TempDir()
	v := inputview.New()
	_, _, err := LoadNearestWithInputs(root, v)
	if !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte("project:\n  root: .\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var changed *inputview.ChangedError
	if err := v.Recheck(); !errors.As(err, &changed) {
		t.Fatalf("recheck: %v", err)
	}
}
