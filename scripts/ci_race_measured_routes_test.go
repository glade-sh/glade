package scripts

import (
	"os/exec"
	"testing"
)

func TestCIRacePushMeasuredSlowRoutes(t *testing.T) {
	program := `
import importlib.util,sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
c=m.scope()
assert c['isolatedTests']['./internal/vm']['TestAssignmentSaveVMAnonymousAndIsTestAPIRoutes']>=682
assert sum(c['isolatedTests']['./internal/sema'].values())>=390
pkg='./internal/gladecli';name='TestHomeProjectRoutesTwoProcessesPendingInstall'
routes=['Compile','CompileBatch','ServerModules','ServerVendor','ServerShims','ServerRuntime','DevLWC','DevVF']
selectors=['default','GLADE_HOME','GLADE_ROOT','GLADE_HOME_relative','GLADE_ROOT_relative','GLADE_HOME_parent','GLADE_ROOT_parent','GLADE_HOME_symlink','GLADE_ROOT_symlink','GLADE_HOME_case','GLADE_ROOT_case']
expected={'install_then_'+route+'/'+selector for route in routes for selector in selectors}
assert set(c['splitTests'][pkg][name])==expected
runs=[r for b in m.plan(c)['include'] for r in b['runs'] if r['package']==pkg]
paths=[n for r in runs for n in r['tests'] if n.startswith(name+'/')]
assert set(paths)=={name+'/'+p for p in expected} and len(paths)==len(expected)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("measured race routes: %v\n%s", err, out)
	}
}
