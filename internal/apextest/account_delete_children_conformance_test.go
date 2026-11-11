package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

const accountDeleteFixture = "testdata/native-capture/account_delete_children"

// TestAccountDeleteChildrenNativeConformance replays the
// account_delete_children capture (fresh Developer scratch org, API 62.0 and
// 67.0, anonymous and isTest): Account delete with Draft, In Approval,
// Activated and past-end Contracts, open and Closed Won Opportunities, open
// and closed Cases, Contacts and Draft Orders, and undelete.
func TestAccountDeleteChildrenNativeConformance(t *testing.T) {
	native := accountCaptureNativeRows(t, accountDeleteFixture)
	anonymous, err := os.ReadFile(filepath.Join(accountDeleteFixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := accountCaptureProject(t, accountDeleteFixture, api)
			accountCaptureExactTailMasks(t, filepath.Join(root, "force-app/main/default/classes/LpDeleteProbe.cls"))
			index := loadTestIndex(t, root)
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			program, err := vm.CompileAnonymousWithOptions(string(anonymous), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, runErr := runner.execute(program)
			if runErr != nil {
				t.Fatalf("anonymous run: %v", runErr)
			}
			got := accountCaptureDebugRows(result.Debug)
			for _, row := range native[api]["anonymous"] {
				if got[row[0]] != row[1] {
					t.Errorf("anonymous %s: native <%s> glade <%s>", row[0], row[1], got[row[0]])
				}
			}
			if len(native[api]["anonymous"]) != 55 {
				t.Errorf("native anonymous rows = %d; want 55", len(native[api]["anonymous"]))
			}
			run := Run(index, Options{SelectedClasses: []string{"LpDeleteProbeTest"}, NoDiskCache: true, Parallelism: 1})
			gotTest := accountCaptureTestRows(t, run)
			for _, row := range native[api]["istest"] {
				if gotTest[row[0]] != row[1] {
					t.Errorf("isTest %s: native <%s> glade <%s>", row[0], row[1], gotTest[row[0]])
				}
			}
		})
	}
}

// accountCaptureExactTailMasks narrows the probe's record-number masks to the list
// after the final ".: " of a DELETE_FAILED message. Natively the masked
// numbers (eight digits) never occur elsewhere in a row; Glade's shorter
// CaseNumber would also match inside record names and other numbers.
func accountCaptureExactTailMasks(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const native = "for (String key : masks.keySet()) text = text.replace(key, masks.get(key));\n        return text;"
	const replay = "Integer at = text.lastIndexOf('.: '); if (at < 0) return text; String tail = text.substring(at + 3).trim(); return masks.containsKey(tail) ? text.substring(0, at + 3) + masks.get(tail) : text;"
	if !strings.Contains(string(raw), native) {
		t.Fatal("probe mask loop not found")
	}
	writeFile(t, path, strings.Replace(string(raw), native, replay, 1))
}
