package scripts

import (
	"os/exec"
	"testing"
)

// Race reports platform-inapplicable routes and captured gaps as deferred;
// it must reject unrelated skips, altered reasons, and unsupported identities.
func TestCIRacePushSkipPolicy(t *testing.T) {
	program := `
import csv,json,pathlib,subprocess,tempfile
script=str(pathlib.Path('ci-go-test.sh').resolve())
root=pathlib.Path('../internal/apextest/testdata/conformance/qualified_receivers')
with (root/'cases.tsv').open() as f: cases={r['case']:r for r in csv.DictReader(f,delimiter='\t')}
with (root/'compile.tsv').open() as f: rows=list(csv.DictReader(f,delimiter='\t'))
r=next(r for r in rows if r['route']=='anonymous' and r['compiled']=='false' and cases[r['case']]['kind']=='known_gap' and r['case']!='standard_schema_fields_map')
key='/'.join((r['source_api'],r['route'],r['case']))
base='TestAnalyzeQualifiedReceiverNativeControls/KNOWN_GAP/'
reason='KNOWN GAP: local analysis accepts '+cases[r['case']]['expression']+'; native compile.tsv row '+key+' rejects it: '+r['problem']
pkg='github.com/glade-sh/glade/internal/sema'
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp); e=d/'events'; s=d/'summary'; meta=d/'metadata'
 def check(package,test,cause,valid,lane='race'):
  file='process.go' if 'HomeProjectRoutes' in test else 'qualified_receiver_matrix_test.go'
  events=[dict(Package=package,Test=test,Action='output',Output='    '+file+':137: '+cause+'\n'),dict(Package=package,Test=test,Action='skip'),dict(Package=package,Action='pass')]
  e.write_text(''.join(json.dumps(x)+'\n' for x in events));meta.write_text(package+'\thas-tests\n')
  p=subprocess.run(['bash',script,'package-summary',lane,'test',str(e),str(s),str(meta),package],capture_output=True,text=True)
  data=json.loads(s.read_text());assert (p.returncode==0)==valid,(test,cause,p.stdout,p.stderr,data)
  if valid: assert data['deferred'][0]['test']==test
 check(pkg,base+key,reason,True)
 check(pkg,base+key,reason+' altered',False)
 check(pkg,base+key+'/extra',reason,False)
 check(pkg+'x',base+key,reason,False)
 check(pkg,base+key,reason,False,'remaining-go')
 case='TestHomeProjectRoutesTwoProcessesPendingInstall/install_then_Compile/GLADE_HOME_case'
 cause='case-variant alias requires a case-insensitive filesystem; this filesystem is case-sensitive'
 f=d/'caseprobe';f.write_text('x');case_sensitive=not (d/'CASEPROBE').exists()
 cpkg='github.com/glade-sh/glade/internal/gladecli'
 check(cpkg,case,cause,case_sensitive)
 check(cpkg,case.replace('GLADE_HOME_case','GLADE_HOME'),cause,False)
 check(cpkg,case,cause+' altered',False)
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("race skip policy: %v\n%s", err, out)
	}
}
