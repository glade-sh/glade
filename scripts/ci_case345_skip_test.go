package scripts

import (
	"os/exec"
	"testing"
)

func TestCICase345NativeGapSkipPolicy(t *testing.T) {
	program := `
import json,pathlib,shutil,subprocess,tempfile
pkg='github.com/glade-sh/glade/internal/apextest'
reason='captured native observation has a known local gap'
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp); events=d/'events'; metadata=d/'metadata'; summary=d/'summary'
 scriptdir=d/'scripts'; scriptdir.mkdir(); script=scriptdir/'ci-go-test.sh'
 shutil.copyfile('ci-go-test.sh',script)
 ledger=scriptdir/'ci-apex-native-gaps.json'
 registered=['TestSynthetic/62.0/gap','TestSynthetic/67.0/gap']; ledger.write_text(json.dumps(registered))
 for test in registered:
  cases=[(pkg,test,reason,True), (pkg,test.rsplit('/',1)[0]+'/unregistered-control',reason,False),
         (pkg,test+'/nested',reason,False), (pkg,test.split('/')[0],reason,False),
         (pkg+'other',test,reason,False), (pkg,test,'different reason',False)]
  for package,name,cause,valid in cases:
   rows=[dict(Action='output',Package=package,Test=name,Output='    fixture_test.go:23: '+cause+'\n'),
         dict(Action='skip',Package=package,Test=name),dict(Action='pass',Package=package)]
   events.write_text(''.join(json.dumps(row)+'\n' for row in rows)); metadata.write_text(package+'\thas-tests\n')
   r=subprocess.run(['bash',str(script),'package-summary','apextest','fixture',str(events),str(summary),str(metadata),package],capture_output=True,text=True)
   assert (r.returncode==0)==valid,(package,name,cause,r.stdout,r.stderr)
   if valid:
    data=json.loads(summary.read_text()); assert len(data['deferred'])==1 and data['deferred'][0]['test']==name,data
 # With an empty live ledger no previously registered gap is accepted.
 ledger.write_text('[]')
 rows=[dict(Action='output',Package=pkg,Test=registered[0],Output='    fixture_test.go:23: '+reason+'\n'),dict(Action='skip',Package=pkg,Test=registered[0]),dict(Action='pass',Package=pkg)]
 events.write_text(''.join(json.dumps(row)+'\n' for row in rows)); metadata.write_text(pkg+'\thas-tests\n')
 r=subprocess.run(['bash',str(script),'package-summary','apextest','fixture',str(events),str(summary),str(metadata),pkg],capture_output=True,text=True)
 assert r.returncode!=0,(r.stdout,r.stderr)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("native gap skip policy: %v\n%s", err, out)
	}
}
