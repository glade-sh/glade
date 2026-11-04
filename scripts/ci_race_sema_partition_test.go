package scripts

import (
	"os/exec"
	"testing"
)

func TestCIRacePushSemaRepositoryPartitions(t *testing.T) {
	program := `
import importlib.util,sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
c=m.scope();pkg='./internal/sema';parent='TestSemaIgnoredTextMatchesReferenceOnRepositoryApex'
runs=[r for b in m.plan(c)['include'] for r in b['runs'] if r['package']==pkg]
assert sum(r.get('excludeTests')==[parent] for r in runs)==1,runs
leaves=[n for r in runs for n in (r['tests'] or []) if n.startswith(parent+'/')]
assert sorted(leaves)==[parent+'/part_'+str(i) for i in range(8)],leaves
assert all(r['estimatedSeconds']>0 for r in runs)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("sema repository partitions: %v\n%s", err, out)
	}
}
