package scripts

import (
	"os/exec"
	"testing"
)

func TestCIRacePushCLIReadonlyCaseSkip(t *testing.T) {
	program := `
import json,os,pathlib,subprocess,tempfile
script=str(pathlib.Path('ci-go-test.sh').resolve());pkg='github.com/glade-sh/glade/internal/gladecli'
reason='case alias requires a case-insensitive filesystem'
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp);e=d/'events';s=d/'summary';meta=d/'metadata';meta.write_text(pkg+'\thas-tests\n')
 # Simulate a case-sensitive temporary filesystem even on a case-insensitive laptop.
 (d/'sitecustomize.py').write_text("from pathlib import Path\nold=Path.exists\nPath.exists=lambda p: False if p.name=='CASEPROBE' else old(p)\n")
 env=dict(os.environ,PYTHONPATH=str(d))
 def check(test,cause,valid,lane='race'):
  rows=[dict(Package=pkg,Test=test,Action='output',Output='    readonly.go:81: '+cause+'\n'),dict(Package=pkg,Test=test,Action='skip'),dict(Package=pkg,Action='pass')]
  e.write_text(''.join(json.dumps(row)+'\n' for row in rows))
  p=subprocess.run(['bash',script,'package-summary',lane,'fixture',str(e),str(s),str(meta),pkg],capture_output=True,text=True,env=env)
  assert (p.returncode==0)==valid,(test,p.stderr,s.read_text())
  if valid: assert json.loads(s.read_text())['deferred'][0]['test']==test
 a='TestHomeProjectRoutesCompletedReadOnlyHome/absent/DevVF/GLADE_HOME_case'
 b='TestHomeCLIStatusRoutesCompletedReadOnlyHome/read-only/DoctorCLI/GLADE_ROOT_case'
 for test in (a,b):check(test,reason,True)
 check(a.replace('DevVF','unknown'),reason,False)
 check(a,reason+' altered',False)
 check(a,reason,False,'remaining-go')
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("CLI readonly race skip: %v\n%s", err, out)
	}
}
