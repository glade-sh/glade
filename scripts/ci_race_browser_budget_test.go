package scripts

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCIRacePushBrowserSetupBudget(t *testing.T) {
	job := workflowJobBlocks(t, readWorkflowFile(t, "race.yml"))["race-push"]
	condition := "contains(matrix.packages, './internal/gladecli') || contains(matrix.packages, './internal/server') || contains(matrix.packages, './internal/gladehome')"
	for _, step := range []struct{ name, timeout string }{
		{"Install LWC toolchain", "5"},
		{"Install browser conformance dependencies", "5"},
		{"Install conformance Chromium", "65"},
	} {
		block := workflowStepBlockText(job, "      - name: "+step.name)
		if !strings.Contains(block, condition) || !strings.Contains(block, "timeout-minutes: "+step.timeout+"\n") {
			t.Errorf("%s must retain the browser condition and %s-minute setup budget", step.name, step.timeout)
		}
	}
	program := `
import importlib.util, math, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py'); m=importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
browser={'./internal/gladecli','./internal/gladehome','./internal/server'}
for package in sorted(browser | {'./internal/sema'}):
 config={'packages':{package:None},'estimatedSeconds':{package:100}}
 batch=m.plan(config)['include'][0]
 setup=720+(75*60 if package in browser else 0)
 required=setup+sum(m.run_timeout(r['estimatedSeconds'])+60 for r in batch['runs'])
 assert batch['timeoutMinutes'] >= math.ceil(required/60), (package,batch['timeoutMinutes'],required)
for batch in m.plan(m.scope())['include']:
 setup=720+(75*60 if browser.intersection(batch['packages']) else 0)
 required=setup+sum(m.run_timeout(r['estimatedSeconds'])+60 for r in batch['runs'])
 assert batch['timeoutMinutes'] >= math.ceil(required/60), (batch,required)
 assert batch['timeoutMinutes'] <= 360, batch
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("race browser setup budget: %v\n%s", err, out)
	}
}
