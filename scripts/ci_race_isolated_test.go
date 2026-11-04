package scripts

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCIRacePushIsolatesMeasuredSlowTests(t *testing.T) {
	job := workflowJobBlocks(t, readWorkflowFile(t, "race.yml"))["race-push"]
	if !strings.Contains(job, "timeout-minutes: ${{ matrix.timeoutMinutes }}") {
		t.Error("race push job must leave time for setup, an extended test deadline, and artifact upload")
	}
	program := `
import importlib.util,json,os,pathlib,sys,tempfile
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
pkg='./internal/example'
cfg={'packages':{pkg:None},'estimatedSeconds':{pkg:100},'isolatedTests':{pkg:{'TestSlow':800}}}
plan=m.plan(cfg)['include'];runs=[r for b in plan for r in b['runs']]
assert len(runs)==2,runs
assert sum(r['tests']==['TestSlow'] for r in runs)==1,runs
assert sum(r.get('excludeTests')==['TestSlow'] for r in runs)==1,runs
for bad in ({'Missing':800},{'TestSlow':0},{'TestSlow':901}):
 x={'packages':{pkg:None},'estimatedSeconds':{pkg:100},'isolatedTests':{pkg:bad}}
 try: m.plan(x)
 except ValueError: pass
 else: raise AssertionError(('accepted',bad))
with tempfile.TemporaryDirectory() as root:
 m.ROOT=pathlib.Path(root);fake=m.ROOT/'go'
 fake.write_text('''#!/usr/bin/env python3
import json,sys
p='github.com/glade-sh/glade/internal/example'
if '-list' in sys.argv: print('TestFast\\nTestSlow');sys.exit(0)
if '-skip' in sys.argv:
 assert sys.argv[sys.argv.index('-skip')+1]=='^(?:TestSlow)$'
 names=['TestFast']
else:
 assert sys.argv[sys.argv.index('-run')+1]=='^(?:TestSlow)$'
 names=['TestSlow']
print(json.dumps(dict(Package=p,Action='start')))
for name in names:
 print(json.dumps(dict(Package=p,Action='run',Test=name)))
 print(json.dumps(dict(Package=p,Action='pass',Test=name)))
print(json.dumps(dict(Package=p,Action='pass')))
''');fake.chmod(0o700);m.GO=str(fake)
 for b in plan:m.run(cfg,b['index'])
 summaries=list((m.ROOT/'ci-artifacts/race-push').rglob('validation-summary.json'))
 assert sorted(n for f in summaries for n in json.loads(f.read_text())['selected'])==['TestFast','TestSlow']
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("isolated race plan: %v\n%s", err, out)
	}
}
