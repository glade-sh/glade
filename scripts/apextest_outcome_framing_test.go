package scripts

import (
	"os/exec"
	"testing"
)

// test2json can split a single long diagnostic over several output events.
// Event boundaries and sibling interleaving cannot change the payload multiset.
func TestCIApexTestOutcomePayloadFraming(t *testing.T) {
	program := `
import importlib.util, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('outcomes','apextest-outcomes.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
p='github.com/glade-sh/glade/internal/apextest'
def e(a,n='',text=''):
 return dict(Package=p,Action=a,**({'Test':n} if n else {}),**({'Output':text} if text else {}))
a='    probe_test.go:42: NATIVE|api62|anonymous|'+('x'*2400)+'\nerror: exact diagnostic\n'
b='    probe_test.go:55: NATIVE|api67|istest|2/2\n'
prefix=[e('start'),e('run','TestA'),e('run','TestB')]
suffix=[e('pass','TestA'),e('pass','TestB'),e('pass')]
whole=prefix+[e('output','TestA',a),e('output','TestB',b)]+suffix
for cut in (1,7,1024,2048):
 chunks=prefix+[e('output','TestA',a[:cut]),e('output','TestB',b[:11]),e('output','TestA',a[cut:]),e('output','TestB',b[11:])]+suffix
 m.compare([whole],[chunks],[],registry=['TestA','TestB'])
# Reassembly must retain errors, native text and multiplicity.
bad=prefix+[e('output','TestA',a.replace('exact diagnostic','wrong diagnostic')),e('output','TestB',b)]+suffix
try:m.compare([whole],[bad],[])
except ValueError:pass
else:raise AssertionError('reassembly lost an error payload')
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("outcome framing: %v\n%s", err, out)
	}
}
