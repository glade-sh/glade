package scripts

import (
	"os/exec"
	"testing"
)

func TestCIPRPackagePrerequisiteSkips(t *testing.T) {
	program := `
import json, os, pathlib, subprocess, tempfile
module='github.com/glade-sh/glade'
home=module+'/internal/gladehome'; dml=module+'/internal/dml'; soql=module+'/internal/soql'
readers=('EnsureRoot','Root','LWCToolchainDir','ShimsDir','RuntimeAssetDir','ToolchainStatus')
selectors=('GLADE_HOME_case','GLADE_ROOT_case')
readonly='readonly.go:81: case alias requires a case-insensitive filesystem'
pending='process.go:113: case-variant alias requires a case-insensitive filesystem; this filesystem is case-sensitive'
cases=[(home,'TestHomeReadersCompletedReadOnlyHome/'+mode+'/'+reader+'/'+selector,readonly) for mode in ('absent','inaccessible','read-only') for reader in readers for selector in selectors]
cases += [(home,'TestHomeReadersTwoProcessesPendingInstall/install_then_'+reader+'/'+selector,pending) for reader in readers for selector in selectors]
cases += [(home,'TestReadLeaseFilesystemIdentity/case','lease_identity_review_test.go:42: filesystem is case-sensitive'),(dml,'TestFuzzyConfidenceDump','fuzzy_arch_identity_test.go:53: GLADE_FUZZY_DUMP is not set'),(soql,'TestLocaleCollationColdQuery','locale_collation_cold_test.go:26: per-process locale required'),(soql,'TestLocaleCollationGeneratorReproducesTables','locale_collation_state_test.go:201: not run: pinned ICU 78.1 / CLDR 48 generation dependency unavailable: exit status 1')]
assert len(cases)==52
with tempfile.TemporaryDirectory() as tmp:
 d=pathlib.Path(tmp); events=d/'events'; metadata=d/'metadata'; summary=d/'summary'
 def check(pkg,name,reason,valid,extra=None,lane='remaining-go'):
  metadata.write_text(pkg+'\thas-tests\n')
  rows=[dict(Action='output',Package=pkg,Test=name,Output='    '+reason+'\n'),dict(Action='skip',Package=pkg,Test=name),dict(Action='pass',Package=pkg)]
  events.write_text(''.join(json.dumps(row)+'\n' for row in rows))
  env=dict(os.environ,RUNNER_OS='Linux'); env.pop('GLADE_FUZZY_DUMP',None); env.pop('GLADE_COLD_LOCALE',None)
  if extra: env.update(extra)
  r=subprocess.run(['bash','ci-go-test.sh','package-summary',lane,'fixture',str(events),str(summary),str(metadata),pkg],env=env,capture_output=True,text=True)
  assert (r.returncode==0)==valid,(pkg,name,reason,r.stdout,r.stderr)
  if valid:
   deferred=json.loads(summary.read_text())['deferred']; assert len(deferred)==1 and deferred[0]['test']==name and 'owner:' in deferred[0]['reason'], deferred
 for case in cases: check(*case,True)
 for pkg,name,reason in cases[::12]+cases[-4:]:
  check(pkg,name,'other_test.go:1: unexpected reason',False)
  check(pkg,name+'Extra',reason,False)
  check(pkg,name,reason,False,{'RUNNER_OS':'Windows'})
 check(home,'TestHomeReadersCompletedReadOnlyHome/absent/Root/GLADE_HOME_relative',readonly,False)
 check(home,'TestHomeReadersCompletedReadOnlyHome/absent/Other/GLADE_HOME_case',readonly,False)
 check(home,'TestReadLeaseFilesystemIdentity/hardlink','lease_identity_review_test.go:42: filesystem is case-sensitive',False)
 check(dml,'TestFuzzyConfidenceDump','fuzzy_arch_identity_test.go:53: GLADE_FUZZY_DUMP is not set',False,{'GLADE_FUZZY_DUMP':'configured'})
 check(soql,'TestLocaleCollationColdQuery','locale_collation_cold_test.go:26: per-process locale required',False,{'GLADE_COLD_LOCALE':'en_US'})
 check(module+'/internal/other','TestFuzzyConfidenceDump','fuzzy_arch_identity_test.go:53: GLADE_FUZZY_DUMP is not set',False)
 check(home,'TestReadLeaseFilesystemIdentity/case','lease_identity_review_test.go:42: filesystem is case-sensitive',False,lane='gladecli-ci')
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("PR prerequisite skip policy: %v\n%s", err, out)
	}
}
