package scripts

import (
	"os/exec"
	"testing"
)

// Budget the selected work, including descendants of indivisible slow families.
func TestCIRacePushBudgetedSelections(t *testing.T) {
	program := `
import importlib.util, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
for total in (20,459,700):
 names=['TestCase%04d'%i for i in range(total)]
 cfg={'packages':{'./internal/apextest':names},'estimatedSeconds':{'./internal/apextest':total*30},
      'testSeconds':{'./internal/apextest':{n:30 for n in names}}}
 p=m.plan(cfg)['include']
 selected=[]
 for b in p:
  assert b['estimatedSeconds']<=900,b
  for r in b['runs']:
   assert r['estimatedSeconds']<=660,r
   selected.extend(r['tests'])
 assert sorted(selected)==names
 assert p==m.plan(cfg)['include']
# Slow parents must be explicitly partitioned, never silently underbudgeted.
cfg={'packages':{'./internal/apextest':['TestLarge','TestSmall']},
 'estimatedSeconds':{'./internal/apextest':2000},
 'testSeconds':{'./internal/apextest':{'TestLarge':1500,'TestSmall':10}},
 'splitTests':{'./internal/apextest':{'TestLarge':{
  '62.0/one':300,'62.0/two':300,'67.0/one':300,'67.0/two':300,'68.0/carry':20}}}}
p=m.plan(cfg)['include']; paths=[]
for b in p:
 assert b['estimatedSeconds']<=900
 for r in b['runs']:
  assert r['estimatedSeconds']<=660
  paths.extend(r['tests'])
assert sorted(paths)==sorted(['TestSmall']+['TestLarge/'+n for n in cfg['splitTests']['./internal/apextest']['TestLarge']])
for mutation in ('missing','oversized','overlap'):
 import copy
 bad=copy.deepcopy(cfg)
 if mutation=='missing': del bad['splitTests']
 if mutation=='oversized':bad['splitTests']['./internal/apextest']['TestLarge']['62.0/one']=1000
 if mutation=='overlap':bad['splitTests']['./internal/apextest']['TestLarge']['62.0']=10
 try:m.plan(bad)
 except ValueError:pass
 else:raise AssertionError('accepted '+mutation)

# The checked split inventory must contain every existing native case at each API.
import json, pathlib
real=m.scope();pkg='./internal/apextest'
fixture=json.loads(pathlib.Path('../internal/apextest/testdata/conformance/heap_limits.json').read_text())
expected={api+'/'+row['ID'] for row in fixture['Cases'] for api in row['Native']}
actual=set(real['splitTests'][pkg]['TestHeapLimitsOrgConformance'])
assert actual==expected,(sorted(expected-actual),sorted(actual-expected))
assert len(real['testSeconds'][pkg])==len(real['packages'][pkg])
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("budgeted plan: %v\n%s", err, out)
	}
}

func TestCIRacePushSplitExecution(t *testing.T) {
	program := `
import importlib.util,json,os,pathlib,sys,tempfile
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('push','ci-race-push.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
with tempfile.TemporaryDirectory() as root:
 m.ROOT=pathlib.Path(root);fake=m.ROOT/'go'
 fake.write_text('''#!/usr/bin/env python3
import json,os,sys
if '-list' in sys.argv:print('TestLarge');sys.exit(0)
p='github.com/glade-sh/glade/internal/example'
def emit(a,n=''):
 if os.getenv('FIXTURE_MODE')=='package-start' and a=='start':return
 if os.getenv('FIXTURE_MODE')=='test-start' and a=='run':return
 print(json.dumps(dict(Package=p,Action=a,**({'Test':n} if n else {}))))
regex=sys.argv[sys.argv.index('-run')+1]
assert regex=='^TestLarge$/^62\\\\.0$/^(?:one|two)$',regex
mode=os.getenv('FIXTURE_MODE','pass')
emit('start');emit('run','TestLarge');emit('run','TestLarge/62.0')
for suffix in ['one','two']:
 n='TestLarge/62.0/'+suffix
 if mode=='missing' and suffix=='two':continue
 emit('run',n);emit('skip' if mode=='skip' else 'pass',n)
 if mode=='duplicate':emit('pass',n)
if mode=='extra':emit('run','TestLarge/62.0/other');emit('pass','TestLarge/62.0/other')
emit('pass','TestLarge/62.0');emit('pass','TestLarge');emit('pass')
''');fake.chmod(0o700);m.GO=str(fake)
 pkg='./internal/example'
 cfg={'packages':{pkg:['TestLarge']},'estimatedSeconds':{pkg:1000},'testSeconds':{pkg:{'TestLarge':1000}},'splitTests':{pkg:{'TestLarge':{'62.0/one':300,'62.0/two':300}}}}
 m.run(cfg,0)
 for mode in ('missing','extra','skip','duplicate','package-start','test-start'):
  os.environ['FIXTURE_MODE']=mode
  try:m.run(cfg,0)
  except ValueError:pass
  else:raise AssertionError('accepted '+mode)
  assert not (m.ROOT/'ci-artifacts/race-push/internal-example/validation-summary.json').exists()
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("split execution: %v\n%s", err, out)
	}
}
