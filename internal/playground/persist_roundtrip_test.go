package playground

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

// A successful persist must be identical after reopening, independently of a
// later compile failure or the result cache.
func TestRunnerPersistedOrgRoundTrips(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, body := range []string{"insert new Account(Name = 'one');", "insert new Account(Name = 'two'); insert new Account(Name = 'three');"} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: root, ID: "default"})
			if err != nil {
				t.Fatal(err)
			}
			opts := RunnerOptions{Version: "test", DBPath: filepath.Join(root, "org.sqlite")}
			runner := NewRunner(ws, opts)
			result, err := runner.Run(t.Context(), RunRequest{AnonymousBody: body, Mode: RunModePersist, LimitMode: "permissive"})
			if err != nil || result.Status != RunStatusPass {
				t.Fatalf("run: %#v, %v", result, err)
			}
			before := runner.Org()
			after := NewRunner(ws, opts).Org()
			if !samePlaygroundOrg(t, before, after) {
				for name, obj := range before.Objects {
					a, _ := json.Marshal(obj)
					b, _ := json.Marshal(after.Objects[name])
					if string(a) != string(b) {
						t.Errorf("object %s differs after reopen\nbefore: %s\nafter: %s", name, a, b)
					}
				}
				t.Fatal("persisted org differs after reopen")
			}
		})
	}
}
