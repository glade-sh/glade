package vm

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRestoredRuntimeTemplateRunsWorkBeforeFirstCloneOrg(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	template := NewRestoredRuntimeTemplate(org, New(nil))
	var runs atomic.Int32
	if !template.BeforeFirstCloneOrg(func() { runs.Add(1) }) {
		t.Fatal("fresh template refused work before its first clone")
	}
	if runs.Load() != 0 {
		t.Fatal("registered work ran before any clone")
	}
	// Copies of the template value share one barrier.
	copied := template
	const workers = 8
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = copied.CloneOrg()
			if runs.Load() != 1 {
				t.Error("CloneOrg returned before the registered work finished")
			}
		}()
	}
	wg.Wait()
	_ = template.CloneOrg()
	if got := runs.Load(); got != 1 {
		t.Fatalf("registered work ran %d times, want 1", got)
	}
	if template.BeforeFirstCloneOrg(func() { runs.Add(1) }) {
		t.Fatal("cloned template accepted work for before its first clone")
	}
	if (RestoredRuntimeTemplate{}).BeforeFirstCloneOrg(func() {}) {
		t.Fatal("invalid template accepted work")
	}
}
