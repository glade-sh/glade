package scripts

import (
	"os/exec"
	"testing"
)

// The early gladecli race route spends its single processor on race
// instrumentation, and the runner caps simultaneous test execution owners at
// GOMAXPROCS, so the parallel route's two-tests-in-flight contract can only
// report that shortfall there. The race lane summary defers exactly the one
// declared identity and reason; altered reasons, other identities, other
// packages and every normal lane keep rejecting the skip, so lanes that run
// with two processors must still observe the overlap themselves.
func TestCIParallelRouteProcsSkipPolicy(t *testing.T) {
	program := `
import json,pathlib,subprocess,tempfile
pkg='github.com/glade-sh/glade/internal/gladecli'
test='TestRunTestMemoryLimitsParallelMethodAndClassRoutes'
reason='the parallel route needs two processors for two tests in flight; testExecutionParallelism caps execution owners to GOMAXPROCS'
file='test_memory_limits_routes_test.go'
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp); events=d/'events'; metadata=d/'metadata'; summary=d/'summary'
 def check(name,lane,package,test_name,cause,valid):
  rows=[dict(Action='output',Package=package,Test=test_name,Output='    '+file+':54: '+cause+'\n'),dict(Action='skip',Package=package,Test=test_name),dict(Action='pass',Package=package)]
  events.write_text(''.join(json.dumps(row)+'\n' for row in rows)); metadata.write_text(package+'\thas-tests\n')
  r=subprocess.run(['bash','ci-go-test.sh','package-summary',lane,'fixture',str(events),str(summary),str(metadata),package],capture_output=True,text=True)
  assert (r.returncode==0)==valid,(name,r.stdout,r.stderr)
  if valid:
   data=json.loads(summary.read_text())
   assert data['deferred'][0]['test']==test_name,(name,data)
 check('declared race-lane shortfall is deferred','race',pkg,test,reason,True)
 check('altered reason stays an error','race',pkg,test,reason+' altered',False)
 check('other identity stays an error','race',pkg,'TestRunTestMemoryLimitsProjectRoutes',reason,False)
 check('other package stays an error','race','github.com/glade-sh/glade/internal/storage',test,reason,False)
 check('normal lanes must run the route','gladecli-ci',pkg,test,reason,False)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("parallel route processor skip policy: %v\n%s", err, out)
	}
}
