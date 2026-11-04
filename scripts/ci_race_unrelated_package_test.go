package scripts

import (
	"os/exec"
	"testing"
)

func TestCIRacePushUnrelatedPackageDoesNotRequireSemaFixtures(t *testing.T) {
	program := `
import json,pathlib,shutil,subprocess,tempfile
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp);(d/'scripts').mkdir();script=d/'scripts/ci-go-test.sh'
 shutil.copyfile('ci-go-test.sh',script)
 shutil.copyfile('ci-apex-native-gaps.json',d/'scripts/ci-apex-native-gaps.json')
 pkg='github.com/glade-sh/glade/internal/example'
 e=d/'events';s=d/'summary';m=d/'metadata'
 e.write_text(json.dumps(dict(Package=pkg,Action='pass'))+'\n');m.write_text(pkg+'\thas-tests\n')
 p=subprocess.run(['bash',str(script),'package-summary','race','fixture',str(e),str(s),str(m),pkg],capture_output=True,text=True)
 assert p.returncode==0,(p.stdout,p.stderr)
 assert json.loads(s.read_text())['valid']
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("unrelated race package validation: %v\n%s", err, out)
	}
}
