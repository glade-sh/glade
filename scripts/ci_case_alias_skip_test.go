package scripts

import (
	"os/exec"
	"testing"
)

func TestCICaseAliasNestedSkipPolicy(t *testing.T) {
	program := `
import json,pathlib,subprocess,tempfile
pkg='github.com/glade-sh/glade/internal/gladecli'
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp); events=d/'events'; metadata=d/'metadata'; summary=d/'summary'; metadata.write_text(pkg+'\thas-tests\n')
 cases=[('TestHomeProjectRoutesCompletedReadOnlyHome/absent/Compile/GLADE_HOME_case','readonly.go:81: case alias requires a case-insensitive filesystem',True),('TestHomeCLIStatusRoutesTwoProcessesPendingInstall/install_then_ToolchainStatusCLI/GLADE_ROOT_case','process.go:113: case-variant alias requires a case-insensitive filesystem; this filesystem is case-sensitive',True),('TestHomeProjectRoutesCompletedReadOnlyHome/absent/Compile/GLADE_HOME_case','readonly.go:81: unrelated reason',False),('TestOther/absent/Compile/GLADE_HOME_case','readonly.go:81: case alias requires a case-insensitive filesystem',False),('TestHomeCLIStatusRoutesCompletedReadOnlyHome/absent/Compile/GLADE_HOME_case','readonly.go:81: case alias requires a case-insensitive filesystem',False),('TestHomeProjectRoutesTwoProcessesPendingInstall/install_then_DoctorCLI/GLADE_ROOT_case','process.go:113: case-variant alias requires a case-insensitive filesystem; this filesystem is case-sensitive',False),('TestHomeProjectRoutesCompletedReadOnlyHome/absent/Compile/GLADE_HOME_relative','readonly.go:81: case alias requires a case-insensitive filesystem',False)]
 for name,reason,valid in cases:
  root=name.split('/')[0]; rows=[dict(Action='output',Package=pkg,Test=name,Output='    '+reason+'\n'),dict(Action='skip',Package=pkg,Test=name),dict(Action='pass',Package=pkg,Test=root),dict(Action='pass',Package=pkg)]
  events.write_text(''.join(json.dumps(row)+'\n' for row in rows)); r=subprocess.run(['bash','ci-go-test.sh','package-summary','gladecli-ci','fixture',str(events),str(summary),str(metadata),pkg],capture_output=True,text=True)
  assert (r.returncode==0)==valid,(name,reason,r.stdout,r.stderr)
  if valid:
   unsharded=subprocess.run(['bash','ci-go-test.sh','package-summary','gladecli','fixture',str(events),str(summary),str(metadata),pkg],capture_output=True,text=True)
   assert unsharded.returncode!=0, ('unsharded policy weakened',name)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("case alias skip policy: %v\n%s", err, out)
	}
}
