package scripts

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCIRacePushGladehomeToolchainAndCaseRoutes(t *testing.T) {
	job := workflowJobBlocks(t, readWorkflowFile(t, "race.yml"))["race-push"]
	if strings.Count(job, "contains(matrix.packages, './internal/gladehome')") < 2 {
		t.Error("gladehome race jobs need Node and the installed toolchain")
	}
	program := `
import json,pathlib,subprocess,tempfile
script=str(pathlib.Path('ci-go-test.sh').resolve())
pkg='github.com/glade-sh/glade/internal/gladehome'
reason='case alias requires a case-insensitive filesystem'
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp);(d/'caseprobe').touch(); sensitive=not (d/'CASEPROBE').exists()
 e=d/'events';s=d/'summary';m=d/'metadata';m.write_text(pkg+'\thas-tests\n')
 def check(test,cause,expected):
  rows=[dict(Package=pkg,Test=test,Action='output',Output='    readonly.go:81: '+cause+'\n'),dict(Package=pkg,Test=test,Action='skip'),dict(Package=pkg,Action='pass')]
  e.write_text(''.join(json.dumps(row)+'\n' for row in rows))
  p=subprocess.run(['bash',script,'package-summary','race','fixture',str(e),str(s),str(m),pkg],capture_output=True,text=True)
  assert (p.returncode==0)==expected,(test,p.stderr,s.read_text())
 root='TestHomeReadersCompletedReadOnlyHome/absent/Root/'
 check(root+'GLADE_HOME_case',reason,sensitive)
 check(root+'GLADE_HOME',reason,False)
 check(root+'GLADE_HOME_case',reason+' altered',False)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("gladehome race routes: %v\n%s", err, out)
	}
}
