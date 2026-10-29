package scripts

import (
	"os/exec"
	"testing"
)

func TestCIApexTestOutcomeEvidence(t *testing.T) {
	program := `
import importlib.util, sys, copy
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('outcomes','apextest-outcomes.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
p='github.com/glade-sh/glade/internal/apextest'
def e(a,n='',text=''):
 return dict(Package=p,Action=a,**({'Test':n} if n else {}),**({'Output':text} if text else {}))
def log(name,action='pass'):
 return [e('start'),e('run',name),e('run',name+'/api62'),e('output',name+'/api62','NATIVE|ready\n'),e(action,name+'/api62'),e(action,name),e('fail' if action=='fail' else 'pass')]
a=log('TestA');b=log('TestB','skip');both=a[:-1]+b[1:]
m.compare([both],[a,b],[])
m.compare([both],[a,b,[e("start"),e("pass")]],[])
try:m.compare([[e("start"),e("pass")]],[[e("start"),e("pass")]],[])
except ValueError:pass
else:raise AssertionError("accepted an entirely empty comparison")
for kind in ('truncated','unfinished','duplicate','newskip','payload','addition','missing'):
 bad=copy.deepcopy(both)
 if kind=='truncated':bad=bad[:-1]
 if kind=='unfinished':bad=[x for x in bad if not(x.get('Test')=='TestA/api62' and x['Action']=='pass')]
 if kind=='duplicate':bad.insert(-1,e('pass','TestA'))
 if kind=='newskip':bad[4]['Action']='skip'
 if kind=='payload':bad[3]['Output']='NATIVE|wrong\n'
 if kind=='addition':bad[-1:-1]=[e('run','TestNew'),e('pass','TestNew')]
 if kind=='missing':bad=[x for x in bad if not x.get('Test','').startswith('TestB')]
 try:m.compare([both],[bad],[])
 except ValueError:pass
 else:raise AssertionError('accepted '+kind)
# Expected new guard tests are explicit, and cannot hide changes to old outcomes.
new=both[:-1]+[e('run','TestNew'),e('pass','TestNew'),both[-1]]
m.compare([both],[new],['TestNew'])
# A failing package with all terminal tests remains a valid comparison.
m.compare([log('TestFailure','fail')],[log('TestFailure','fail')],[])
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("outcome evidence: %v\n%s", err, out)
	}
}
