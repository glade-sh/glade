package scripts

import (
	"os/exec"
	"testing"
)

// Registry coverage needs completed families, and partition ownership is by
// test identity regardless of its result. Comparing two equally corrupt logs
// must not turn missing parents or duplicate executions into valid evidence.
func TestCIApexTestOutcomePartitionOwnership(t *testing.T) {
	program := `
import importlib.util, itertools, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('outcomes','apextest-outcomes.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
p='github.com/glade-sh/glade/internal/apextest'
def e(action,name=''):
 return dict(Package=p,Action=action,**({'Test':name} if name else {}))
def log(action):
 return [e('start'),e('run','TestA'),e(action,'TestA'),e('fail' if action=='fail' else 'pass')]
accepted=[]
for actions in itertools.combinations(('pass','fail','skip'),2):
 logs=[log(action) for action in actions]
 try:m.compare(logs,logs,[],registry=['TestA'])
 except ValueError:pass
 else:accepted.append(actions)
assert not accepted, 'accepted duplicate family ownership with different results: '+repr(accepted)
# A real failure and a real skip remain valid when each family executes once.
for action in ('pass','fail','skip'):
 m.compare([log(action)],[log(action)],[],registry=['TestA'])
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("partition ownership: %v\n%s", err, out)
	}
}

func TestCIApexTestOutcomeFamilyLifecycle(t *testing.T) {
	program := `
import importlib.util, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('outcomes','apextest-outcomes.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
p='github.com/glade-sh/glade/internal/apextest'
def e(action,name='',text=''):
 return dict(Package=p,Action=action,**({'Test':name} if name else {}),**({'Output':text} if text else {}))
valid=[e('start'),e('run','TestA'),e('run','TestA/api62'),e('run','TestA/api67'),
 e('run','TestA/api62/anonymous'),e('output','TestA/api62/anonymous','NATIVE|1/1\n'),
 e('pass','TestA/api67'),e('pass','TestA/api62/anonymous'),e('pass','TestA/api62'),e('pass','TestA'),e('pass')]
m.compare([valid],[valid],[],registry=['TestA'])
# A t.Run label may contain slashes: JSON has no explicit immediate-parent
# identity. Parallel siblings may even have overlapping slash-delimited names.
labels=[e('start'),e('run','TestA'),e('run','TestA/api62'),
 e('run','TestA/api62/anonymous'),e('pass','TestA/api62'),
 e('pass','TestA/api62/anonymous'),e('pass','TestA'),e('pass')]
m.compare([labels],[labels],[],registry=['TestA'])
slash=[e('start'),e('run','TestA'),e('run','TestA/api62/anonymous'),
 e('pass','TestA/api62/anonymous'),e('pass','TestA'),e('pass')]
m.compare([slash],[slash],[],registry=['TestA'])
bad={
 'missing family': [e('start'),e('run','TestA/api62'),e('pass','TestA/api62'),e('pass')],
 'parent ends before child': [e('start'),e('run','TestA'),e('run','TestA/api62'),e('pass','TestA'),e('pass','TestA/api62'),e('pass')],
 'child starts after parent': [e('start'),e('run','TestA'),e('pass','TestA'),e('run','TestA/api62'),e('pass','TestA/api62'),e('pass')],
 'payload without execution': valid[:-1]+[e('output','TestA/never-ran','NATIVE|1/1\n'),e('pass')],
}
accepted=[]
for name,events in bad.items():
 try:m.compare([events],[events],[],registry=['TestA'])
 except ValueError:pass
 else:accepted.append(name)
assert not accepted, 'accepted invalid family lifecycles: '+repr(accepted)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("family lifecycle: %v\n%s", err, out)
	}
}
