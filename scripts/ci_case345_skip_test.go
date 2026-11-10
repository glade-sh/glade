package scripts

import (
	"os/exec"
	"testing"
)

func TestCICase345NativeGapSkipPolicy(t *testing.T) {
	program := `
import json,pathlib,subprocess,tempfile
pkg='github.com/glade-sh/glade/internal/apextest'
reason='captured native observation has a known local gap'
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp); events=d/'events'; metadata=d/'metadata'; summary=d/'summary'
 # Registered leaves come from the live ledger, so clearing entries never
 # strands this fixture; the first two registered leaves exercise the policy.
 for test in json.loads(pathlib.Path('ci-apex-native-gaps.json').read_text())[:2]:
  cases=[(pkg,test,reason,True), (pkg,test.rsplit('/',1)[0]+'/unregistered-control',reason,False),
         (pkg,test+'/nested',reason,False), (pkg,test.split('/')[0],reason,False),
         (pkg+'other',test,reason,False), (pkg,test,'different reason',False)]
  for package,name,cause,valid in cases:
   rows=[dict(Action='output',Package=package,Test=name,Output='    fixture_test.go:23: '+cause+'\n'),
         dict(Action='skip',Package=package,Test=name),dict(Action='pass',Package=package)]
   events.write_text(''.join(json.dumps(row)+'\n' for row in rows)); metadata.write_text(package+'\thas-tests\n')
   r=subprocess.run(['bash','ci-go-test.sh','package-summary','apextest','fixture',str(events),str(summary),str(metadata),package],capture_output=True,text=True)
   assert (r.returncode==0)==valid,(package,name,cause,r.stdout,r.stderr)
   if valid:
    data=json.loads(summary.read_text()); assert len(data['deferred'])==1 and data['deferred'][0]['test']==name,data
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("native gap skip policy: %v\n%s", err, out)
	}
}
