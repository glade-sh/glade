package scripts

import (
	"os/exec"
	"testing"
)

// The package-summary validator defers exactly the captured native boundary
// rows on the local-release lane: the declared identity passes, and every
// altered identity, reason or terminal outcome stays an error.
func TestCIPackageSummaryNativeBoundaryLocalReleaseRoute(t *testing.T) {
	program := `
import json, pathlib, subprocess, tempfile
script=str(pathlib.Path("ci-go-test.sh").resolve())
pkg="github.com/glade-sh/glade/internal/vm"
root="TestSOQLSafeNavigationBindNativeConformance"
cause="native boundary is captured but outside this bind replay"
with tempfile.TemporaryDirectory() as directory:
 d=pathlib.Path(directory); events=d/"events"; metadata=d/"metadata"; summary=d/"summary"
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
   command=["bash","-c",'source "$1"; shift; validate_local_release_package_summary remaining-go fixture "$@"',"fixture",script]+args
   result=subprocess.run(command,capture_output=True,text=True)
   data=json.loads(summary.read_text())
   assert (result.returncode==0)==valid, (api,label,result.stdout,result.stderr,data)
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
