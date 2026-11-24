package apextest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The same fixture is run on origin/main with ROAD541_BASELINE=1 and on this
// branch. Only the positive case should change; all other observations remain
// the unsupported-criteria error from main.
func TestFlowProfileParentRHSDifferential(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement, apexOld, apexNew string }{
		{name: "captured"},
		{name: "update", old: "<recordTriggerType>Create</recordTriggerType>", replacement: "<recordTriggerType>Update</recordTriggerType>", apexOld: "insert cs;", apexNew: "insert cs; update cs;"},
		{name: "inequality", old: "<operator>EqualTo</operator><rightValue>", replacement: "<operator>NotEqualTo</operator><rightValue>"},
		{name: "two_parents", old: "$Record.Account.Name", replacement: "$Record.Account.Parent.Name"},
		{name: "setup_parent", old: "$Record.Account.Name", replacement: "$Record.Owner.Name"},
		{name: "other_parent_field", old: "$Record.Account.Name", replacement: "$Record.Account.AccountNumber", apexOld: "new Account(Name=p)", apexNew: "new Account(Name=p, AccountNumber=p)"},
		{name: "case_difference", apexOld: "new Account(Name=p)", apexNew: "new Account(Name=p.toLowerCase())"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := filepath.Join("testdata", "conformance", "flow_profile_parent_rhs", "api62")
			root := t.TempDir()
			if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				dest := filepath.Join(root, rel)
				if entry.IsDir() {
					return os.MkdirAll(dest, 0755)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(data)
				if strings.HasSuffix(path, ".flow-meta.xml") && tc.old != "" {
					if !strings.Contains(text, tc.old) {
						t.Fatalf("missing Flow mutation %q", tc.old)
					}
					text = strings.Replace(text, tc.old, tc.replacement, 1)
				}
				if filepath.Base(path) == "ProfileProbeCase.cls" && tc.apexOld != "" {
					if !strings.Contains(text, tc.apexOld) {
						t.Fatalf("missing Apex mutation %q", tc.apexOld)
					}
					text = strings.Replace(text, tc.apexOld, tc.apexNew, 1)
				}
				return os.WriteFile(dest, []byte(text), 0644)
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("fixture diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := capturedFixtureOrg(t, index, "62.0")
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions("System.assert(false, 'OBS=' + ProfileProbeCase.run());", vm.CompileOptions{APIVersion: "62.0"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = runner.newMachine().Execute(program)
			var runtimeErr *vm.RuntimeError
			if !errors.As(err, &runtimeErr) {
				t.Fatalf("expected an exception, got %v", err)
			}
			if tc.name == "captured" && os.Getenv("ROAD541_BASELINE") != "1" {
				if flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" || !strings.Contains(runtimeErr.ExceptionMessage(), "OBS=yes|no|no") {
					t.Fatalf("captured behavior: %v", err)
				}
			} else if !strings.Contains(runtimeErr.Error(), "unsupported criteria") {
				t.Fatalf("main behavior changed: %v", err)
			}
		})
	}
}
