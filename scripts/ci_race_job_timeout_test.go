package scripts

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCIRacePushJobTimeoutCoversSelections(t *testing.T) {
	job := workflowJobBlocks(t, readWorkflowFile(t, "race.yml"))["race-push"]
	if !strings.Contains(job, "timeout-minutes: ${{ matrix.timeoutMinutes }}") {
		t.Error("race job timeout must come from the planned selections")
	}
	program := `
import importlib.util,math,sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
for batch in m.plan(m.scope())['include']:
 required=720+sum(max(1200,(r['estimatedSeconds']*3+1)//2+120)+60 for r in batch['runs'])
 assert batch['timeoutMinutes']>=math.ceil(required/60),(batch,required)
 assert batch['timeoutMinutes']<=360,batch
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("race job timeout budget: %v\n%s", err, out)
	}
}
