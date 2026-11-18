package inputview

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func observedFixture(t *testing.T) (dir, file, missing string, observations []Observation) {
	t.Helper()
	dir = t.TempDir()
	file = filepath.Join(dir, "input.xml")
	missing = filepath.Join(dir, "optional.json")
	if err := os.WriteFile(file, []byte("A"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := New()
	if _, err := v.ReadFile(file); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadFile(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing read: %v", err)
	}
	if _, err := v.ReadDir(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Stat(dir); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err != nil {
		t.Fatal(err)
	}
	// Persisted observations survive a JSON round trip unchanged.
	encoded, err := json.Marshal(v.Observations())
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &observations); err != nil {
		t.Fatal(err)
	}
	if len(observations) != 4 {
		t.Fatalf("observations=%d, want 4: %+v", len(observations), observations)
	}
	return dir, file, missing, observations
}

func TestVerifyAcceptsUnchangedObservations(t *testing.T) {
	_, file, _, observations := observedFixture(t)
	if err := Verify(observations, nil); err != nil {
		t.Fatal(err)
	}
	// A touch changes no content, so it does not invalidate.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	if err := Verify(observations, nil); err != nil {
		t.Fatalf("touch invalidated: %v", err)
	}
}

func TestVerifyDetectsEachInputClass(t *testing.T) {
	cases := map[string]struct {
		change func(t *testing.T, dir, file, missing string)
		what   string
	}{
		"content": {func(t *testing.T, dir, file, missing string) {
			if err := os.WriteFile(file, []byte("B"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "file content"},
		"absent file added": {func(t *testing.T, dir, file, missing string) {
			if err := os.WriteFile(missing, []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "file added"},
		"file removed": {func(t *testing.T, dir, file, missing string) {
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
		}, "file removed"},
		"listing member added": {func(t *testing.T, dir, file, missing string) {
			if err := os.WriteFile(filepath.Join(dir, "other.xml"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "directory listing"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir, file, missing, observations := observedFixture(t)
			tc.change(t, dir, file, missing)
			err := Verify(observations, nil)
			var changed *ChangedError
			if !errors.As(err, &changed) {
				t.Fatalf("got %v, want ChangedError", err)
			}
			// The directory sorts before its members, so a membership change
			// may surface as its listing first.
			if changed.What != tc.what && changed.What != "directory listing" {
				t.Fatalf("what=%q, want %q", changed.What, tc.what)
			}
		})
	}
}

func TestVerifySkipsObservationsTheCallerVerified(t *testing.T) {
	_, file, _, observations := observedFixture(t)
	if err := os.WriteFile(file, []byte("B"), 0o600); err != nil {
		t.Fatal(err)
	}
	skip := func(o Observation) bool { return o.Kind == "file" && o.Path == file }
	if err := Verify(observations, skip); err != nil {
		t.Fatalf("skipped observation was verified: %v", err)
	}
}

func TestVerifyRejectsUnknownKinds(t *testing.T) {
	if err := Verify([]Observation{{Path: "x", Kind: "future"}}, nil); err == nil {
		t.Fatal("unknown observation kind accepted")
	}
}

// The startup cache lives in .glade under the project root, and editors drop
// hidden swap files beside sources; neither is a listing change.
func TestDirectoryListingIgnoresHiddenEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "one"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := New()
	if _, err := v.ReadDirNonHidden(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".glade"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".one.swp"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err != nil {
		t.Fatalf("hidden entries changed the listing: %v", err)
	}
}
