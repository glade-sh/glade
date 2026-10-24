package scripts

import (
	"os/exec"
	"testing"
)

func TestCIPackageSummaryNativeBoundaryCLILocalReleaseAndRaceRoutes(t *testing.T) {
	program := `
import json, pathlib, subprocess, tempfile
script=str(pathlib.Path("ci-go-test.sh").resolve())
pkg="github.com/glade-sh/glade/internal/vm"
root="TestSOQLSafeNavigationBindNativeConformance"
cause="native boundary is captured but outside this bind replay"
with tempfile.TemporaryDirectory() as directory:
 d=pathlib.Path(directory); events=d/"events"; metadata=d/"metadata"; summary=d/"summary"
 for route in ("cli", "local-release"):
  for api in ("62.0", "67.0"):
   identity=root+"/"+api+"/left_member"
   cases=[("declared",pkg,identity,cause,"skip","pass",True),
          ("wrong package",pkg+"other",identity,cause,"skip","pass",False),
          ("other row",pkg,identity.replace("left_member","simple_member"),cause,"skip","pass",False),
          ("other API",pkg,identity.replace(api,"66.0"),cause,"skip","pass",False),
          ("nested",pkg,identity+"/nested",cause,"skip","pass",False),
          ("parent",pkg,root,cause,"skip","pass",False),
          ("wrong reason",pkg,identity,"unrelated runtime issue","skip","pass",False),
          ("missing reason",pkg,identity,"","skip","pass",False),
          ("failure",pkg,identity,cause,"fail","pass",False),
          ("missing package",pkg,identity,cause,"skip",None,False)]
   for label, package, test, reason, action, terminal, valid in cases:
    rows=[]
    if reason: rows.append(dict(Action="output",Package=package,Test=test,Output="    soql_safe_navigation_bind_conformance_test.go:55: "+reason+"\n"))
    rows.append(dict(Action=action,Package=package,Test=test))
    if test!=root: rows.append(dict(Action="pass",Package=package,Test=root))
    if terminal: rows.append(dict(Action=terminal,Package=package))
    events.write_text("".join(json.dumps(row)+"\n" for row in rows))
    metadata.write_text(package+"\thas-tests\n")
    args=[str(events),str(summary),str(metadata),package]
    if route=="cli": command=["bash",script,"package-summary","race","fixture"]+args
    else: command=["bash","-c",'source "$1"; shift; validate_local_release_package_summary remaining-go fixture "$@"',"fixture",script]+args
    result=subprocess.run(command,capture_output=True,text=True)
    data=json.loads(summary.read_text())
    assert (result.returncode==0)==valid, (route,api,label,result.stdout,result.stderr,data)
    assert data["valid"]==valid, data
    if label=="declared":
     assert len(data["deferred"])==1, data
     entry=data["deferred"][0]
     assert entry["package"]==pkg and entry["test"]==identity and entry["skipReason"]==cause, entry
     assert "native boundary" in entry["reason"] and entry["reason"] in result.stdout, result.stdout
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("native boundary package summary fixture: %v\n%s", err, out)
	}
}

func TestCIRacePushNativeBoundarySubtestsRemainVisible(t *testing.T) {
	program := `
import importlib.util, json, pathlib, tempfile, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location("race_push", "ci-race-push.py")
m=importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
with tempfile.TemporaryDirectory() as directory:
 m.ROOT=pathlib.Path(directory)
 fake=m.ROOT/"go"
 fake.write_text('''#!/usr/bin/env python3
import json, sys
p="github.com/glade-sh/glade/internal/vm"
n="TestSOQLSafeNavigationBindNativeConformance"
if "-list" in sys.argv:
 print(n+"\\nok  "+p+" 0.01s");sys.exit(0)
for api in ("62.0", "67.0"):
 child=n+"/"+api+"/left_member"
 print(json.dumps(dict(Action="output",Package=p,Test=child,Output="    soql_safe_navigation_bind_conformance_test.go:55: native boundary is captured but outside this bind replay\\n")))
 print(json.dumps(dict(Action="skip",Package=p,Test=child)))
print(json.dumps(dict(Action="pass",Package=p,Test=n)))
print(json.dumps(dict(Action="pass",Package=p)))
''')
 fake.chmod(0o700);m.GO=str(fake)
 m.run({"packages":{"./internal/vm":None},"estimatedSeconds":{"./internal/vm":1}},0)
 summary=json.loads((m.ROOT/"ci-artifacts/race-push/internal-vm/validation-summary.json").read_text())
 assert summary["passed"]==["TestSOQLSafeNavigationBindNativeConformance"], summary
 assert summary["deferred"]==["TestSOQLSafeNavigationBindNativeConformance/62.0/left_member", "TestSOQLSafeNavigationBindNativeConformance/67.0/left_member"], summary
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("native boundary race-push fixture: %v\n%s", err, out)
	}
}
