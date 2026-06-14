package visualforce

import (
	"strconv"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderEnvironmentConcurrentAccess(t *testing.T) {
	machine := vm.New(nil)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				SetVMRenderEnvironment(machine, project.Project{Root: "/tmp/project-" + strconv.Itoa(i)})
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = renderEnvironmentFromVM(machine)
			}
		}()
	}
	wg.Wait()
}
