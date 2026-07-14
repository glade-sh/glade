package vm

import (
	"io"

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
	}
}

// Valid reports whether the template was created by
// NewRestoredRuntimeTemplate with a machine.
func (t RestoredRuntimeTemplate) Valid() bool {
	return t.valid && t.machine != nil
}

// CloneOrg returns a fresh isolated runtime org. Invalid templates return the
// zero OrgState.
func (t RestoredRuntimeTemplate) CloneOrg() storage.OrgState {
	if !t.Valid() {
		return storage.OrgState{}
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
