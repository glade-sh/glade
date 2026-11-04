package scripts

import (
	"os/exec"
	"testing"
)

func TestCIRacePushApexObservedDurations(t *testing.T) {
	program := `
import importlib.util,sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
c=m.scope();pkg='./internal/apextest';weights=c['testSeconds'][pkg]
# Lower bounds are sums of completed per-API durations in push run 40329751796.
for name,seconds in [('TestCollectionIncrementOrgConformance',491),('TestCollectionRecordCastViewsConformance',527),('TestCronTimezoneNativeConformance',210)]:
 assert weights[name]>=seconds,(name,weights[name])
 assert any(name in (r['tests'] or []) for b in m.plan(c)['include'] for r in b['runs'] if r['package']==pkg)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("Apex race measurements: %v\n%s", err, out)
	}
}
