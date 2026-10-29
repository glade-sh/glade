package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestMethodWorkerPoolRejectsChangedRuntimeGeneration(t *testing.T) {
	for _, boundary := range []string{"unchanged", "schema", "runtime"} {
		t.Run(boundary, func(t *testing.T) {
			seed := storage.NewOrgState()
			storage.EnsureStandardObject(&seed, "Account")
			pool := &testWorkerOrgPool{seed: seed, capacity: 1}
			worker := pool.take("OwnedTest", nil)
			switch boundary {
			case "schema":
				worker.org.ClearRuntimeSchemaStamp()
			case "runtime":
				worker.org.InvalidateRuntimeGeneration()
			}
			pool.put(worker)
			want := 0
			if boundary == "unchanged" {
				want = 1
			}
			if len(pool.idle) != want {
				t.Fatalf("retained %d workers; want %d", len(pool.idle), want)
			}
		})
	}
}
