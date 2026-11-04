package scripts

import (
	"os/exec"
	"testing"
)

func TestCIRacePushRetainsJobTimeoutFloor(t *testing.T) {
	program := `
import importlib.util, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py'); m=importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
for package in ('./internal/sema','./internal/gladecli'):
 cfg={'packages':{package:None},'estimatedSeconds':{package:1}}
 batch=m.plan(cfg)['include'][0]
 assert batch['timeoutMinutes'] >= 100, (package,batch['timeoutMinutes'])
for batch in m.plan(m.scope())['include']:
 assert batch['timeoutMinutes'] >= 100, batch
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("race push job timeout floor: %v\n%s", err, out)
	}
}
