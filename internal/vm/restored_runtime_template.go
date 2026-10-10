package vm

import (
	"io"
	"sync"

	"github.com/glade-sh/glade/internal/storage"
)

// RestoredRuntimeTemplate owns the mutable org and machine roots used to clone
// an isolated Apex test runtime. Its zero value is invalid. The owned roots are
// intentionally opaque so cache callers cannot retain mutable aliases.
//
// NewRestoredRuntimeTemplate takes ownership of org and machine. Callers must
// not mutate either input after construction.
type RestoredRuntimeTemplate struct {
	org     storage.RuntimeTemplate
	machine *VM
	valid   bool
	clones  *restoredTemplateCloneBarrier
}

// restoredTemplateCloneBarrier runs registered work before the first CloneOrg
// returns. Until then no clone of the template org exists, so nothing outside
// the template can write into the definitions it shares with its clones.
type restoredTemplateCloneBarrier struct {
	mu          sync.Mutex
	cloned      bool
	beforeFirst []func()
}

// NewRestoredRuntimeTemplate creates an immutable clone boundary around a
// builder-owned org and machine.
func NewRestoredRuntimeTemplate(org storage.OrgState, machine *VM) RestoredRuntimeTemplate {
	if machine == nil {
		return RestoredRuntimeTemplate{}
	}
	template := storage.NewRuntimeTemplate(org)
	PrimeRuntimeTemplateSchema(&template)
	return RestoredRuntimeTemplate{
		org:     template,
		machine: machine,
		valid:   true,
		clones:  &restoredTemplateCloneBarrier{},
	}
}

// Valid reports whether the template was created by
// NewRestoredRuntimeTemplate with a machine.
func (t RestoredRuntimeTemplate) Valid() bool {
	return t.valid && t.machine != nil
}

// BeforeFirstCloneOrg registers fn to run once, before the first CloneOrg
// call returns. Concurrent CloneOrg calls wait for it. It reports false, and
// does not register fn, when the template is invalid or was already cloned.
func (t RestoredRuntimeTemplate) BeforeFirstCloneOrg(fn func()) bool {
	if !t.Valid() || t.clones == nil {
		return false
	}
	t.clones.mu.Lock()
	defer t.clones.mu.Unlock()
	if t.clones.cloned {
		return false
	}
	t.clones.beforeFirst = append(t.clones.beforeFirst, fn)
	return true
}

// CloneOrg returns a fresh isolated runtime org. Invalid templates return the
// zero OrgState.
func (t RestoredRuntimeTemplate) CloneOrg() storage.OrgState {
	if !t.Valid() {
		return storage.OrgState{}
	}
	if t.clones != nil {
		t.clones.mu.Lock()
		if !t.clones.cloned {
			t.clones.cloned = true
			for _, fn := range t.clones.beforeFirst {
				fn()
			}
			t.clones.beforeFirst = nil
		}
		t.clones.mu.Unlock()
	}
	return t.org.CloneRuntimeOrg()
}

// CloneMachine returns a fresh isolated VM. Invalid templates return nil.
func (t RestoredRuntimeTemplate) CloneMachine(stdout io.Writer) *VM {
	if !t.Valid() {
		return nil
	}
	return t.machine.CloneRuntime(stdout)
}
