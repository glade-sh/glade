package visualforce

import (
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderEnvironmentConcurrentAccess(t *testing.T) {
	machine := vm.New(nil)
	defer ClearVMRenderEnvironment(machine)
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

	t.Run("lazy-resolution", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
		lazyMachine := vm.New(nil)
		SetVMLazyRenderEnvironment(lazyMachine, root)
		var lazyWG sync.WaitGroup
		for i := 0; i < 4; i++ {
			lazyWG.Add(1)
			go func() {
				defer lazyWG.Done()
				for j := 0; j < 5; j++ {
					if _, err := resolveVMRenderEnvironment(lazyMachine); err != nil {
						t.Errorf("resolve lazy environment: %v", err)
						return
					}
				}
			}()
		}
		lazyWG.Wait()
		ClearVMRenderEnvironment(lazyMachine)
		if got := renderEnvironmentFromVM(lazyMachine).Project.Root; got != "" {
			t.Errorf("cleared context was restored: %q", got)
		}
	})
}
