package visualforce

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func testRunner(t *testing.T) *vm.VM {
	t.Helper()
	org := storage.NewOrgState()
	runner := vm.New(nil)
	runner.Org = &org
	return runner
}

func vmObject(name string) vm.Value {
	return vm.Object(name)
}

func vmList(values ...string) vm.Value {
	items := make([]vm.Value, 0, len(values))
	for _, value := range values {
		items = append(items, vm.String(value))
	}
	return vm.List(items...)
}
