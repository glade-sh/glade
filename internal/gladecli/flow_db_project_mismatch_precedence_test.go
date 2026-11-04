package gladecli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowDBProjectMismatchPrecedesIdentityGuard(t *testing.T) {
	for _, missingID := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved-identity", true: "missing-identity"}[missingID], func(t *testing.T) {
			first := filepath.Join(t.TempDir(), "first")
			second := filepath.Join(t.TempDir(), "second")
			writeTestProject(t, first)
			writeTestProject(t, second)
			db := filepath.Join(t.TempDir(), "org.sqlite")
			store, org, err := openDBStore(db, first)
			if err != nil {
				t.Fatal(err)
			}
			if missingID {
				org.Metadata.ProjectFlowRootID = ""
				if err = store.Save(org); err != nil {
					t.Fatal(err)
				}
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			store, _, err = openDBStore(db, second)
			if err == nil {
				store.Close()
				t.Fatal("different project opened")
			}
			if !strings.Contains(err.Error(), "database belongs to a different Glade project schema") {
				t.Fatalf("expected project mismatch: %v", err)
			}
			if missingID {
				store, _, err = openDBStore(db, first)
				if err == nil {
					store.Close()
					t.Fatal("identityless original project opened")
				}
				if !strings.Contains(err.Error(), "rebuild the database") {
					t.Fatalf("expected rebuild guidance: %v", err)
				}
			} else {
				store, _, err = openDBStore(db, first)
				if err != nil {
					t.Fatalf("original project no longer opens: %v", err)
				}
				if err = store.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
