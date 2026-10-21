package scripts

import (
	"os/exec"
	"testing"
)

// A package whose whole test list is selected must not put every test name in
// one -run argument: Linux rejects a single argument over 128 KiB, and the vm
// package passed that size.
func TestCIRacePushRunsLargeFullPackageWithoutOversizedArgument(t *testing.T) {
	program := `
import importlib.util, json, os, pathlib, tempfile, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location("race_push", "ci-race-push.py")
m=importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
with tempfile.TemporaryDirectory() as root:
 m.ROOT=pathlib.Path(root)
 fake=m.ROOT/"go"
 fake.write_text('''#!/usr/bin/env python3
import json, sys
p="github.com/glade-sh/glade/internal/vm"
names=["TestLargePackage%05d_%s"%(i,"x"*30) for i in range(5000)]
if "-list" in sys.argv:
 print("\\n".join(names)+"\\nok  "+p+" 0.01s");sys.exit(0)
if any(len(a)>100000 for a in sys.argv):
 sys.stderr.write("argument list too long\\n");sys.exit(7)
for n in names: print(json.dumps({"Action":"pass","Package":p,"Test":n}))
print(json.dumps({"Action":"pass","Package":p}))
''')
 fake.chmod(0o700);m.GO=str(fake)
 m.run({"packages":{"./internal/vm":None},"estimatedSeconds":{"./internal/vm":1}},0)
 summary=json.loads((m.ROOT/"ci-artifacts/race-push/internal-vm/validation-summary.json").read_text())
 assert len(summary["passed"])==5000, len(summary["passed"])
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("large package runner fixture: %v\n%s", err, out)
	}
}
