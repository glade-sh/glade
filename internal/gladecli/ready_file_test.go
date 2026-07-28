package gladecli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadyFileAtomicallyPublishesCompleteReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ready.json")
	old := []byte("{\"state\":\"old\"}\n")
	new := []byte("{\"state\":\"new\",\"routes\":[\"/lwc\"]}\n")
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeReadyFileAtomically(path, new, 0o644, func(tempPath, destination string) error {
		staged, err := os.ReadFile(tempPath)
		if err != nil {
			return err
		}
		if string(staged) != string(new) {
			t.Fatalf("staged readiness file = %q, want %q", staged, new)
		}
		if !json.Valid(staged) {
			t.Fatalf("staged readiness file is not JSON: %q", staged)
		}
		published, err := os.ReadFile(destination)
		if err != nil {
			return err
		}
		if string(published) != string(old) {
			t.Fatalf("destination changed before rename: got %q, want %q", published, old)
		}
		if !json.Valid(published) {
			t.Fatalf("published readiness file is not JSON: %q", published)
		}
		return os.Rename(tempPath, destination)
	}); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(new) {
		t.Fatalf("published readiness file = %q, want %q", got, new)
	}
	if !json.Valid(got) {
		t.Fatalf("published readiness file is not JSON: %q", got)
	}
}
