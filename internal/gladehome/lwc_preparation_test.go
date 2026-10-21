package gladehome

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyLWCToolchainPreparation(t *testing.T) {
	const current, legacy = "prepare-lwc-shared.mjs", "apply-lwc-shared-api67.mjs"
	type preparationCase struct {
		name       string
		hook       string   // script named by the postinstall hook, if any
		present    []string // script files in the tree
		checkErr   error
		wantScript string // script passed to the check
		wantErr    string
	}
	cases := []preparationCase{
		{name: "no hook and no script"},
		{name: "no hook uses legacy script", present: []string{legacy}, wantScript: legacy},
		{name: "no hook prefers current script", present: []string{legacy, current}, wantScript: current},
		// A declared hook is authoritative: a leftover script never stands in for it.
		{name: "current hook with only legacy script", hook: current, present: []string{legacy}, wantErr: "script missing"},
		{name: "current hook with both scripts", hook: current, present: []string{legacy, current}, wantScript: current},
		{name: "legacy hook with both scripts", hook: legacy, present: []string{legacy, current}, wantScript: legacy},
	}
	// The current script and the earlier API 67-only script must both be checked.
	for _, scriptName := range []string{current, legacy} {
		cases = append(cases,
			preparationCase{name: scriptName + "/missing script", hook: scriptName, wantErr: "script missing"},
			preparationCase{name: scriptName + "/prepared", hook: scriptName, present: []string{scriptName}, wantScript: scriptName},
			preparationCase{name: scriptName + "/unprepared", hook: scriptName, present: []string{scriptName}, checkErr: errors.New("preimage requires preparation"), wantScript: scriptName, wantErr: "not prepared"},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			toolchain := filepath.Join(root, "third_party", "lwc")
			if err := os.MkdirAll(filepath.Join(toolchain, "scripts"), 0o755); err != nil {
				t.Fatal(err)
			}
			pkg := `{"scripts":{}}`
			if tc.hook != "" {
				pkg = `{"scripts":{"postinstall":"node scripts/` + tc.hook + `"}}`
			}
			if err := os.WriteFile(filepath.Join(toolchain, "package.json"), []byte(pkg), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, name := range tc.present {
				if err := os.WriteFile(filepath.Join(toolchain, "scripts", name), []byte("// owned fixture\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var checked []string
			err := verifyLWCToolchainPreparation(root, func(gotScript, gotRoot string) error {
				checked = append(checked, gotScript)
				if gotRoot != toolchain {
					t.Fatalf("wrong preparation root: %s", gotRoot)
				}
				return tc.checkErr
			})
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error=%v, want %q", err, tc.wantErr)
			}
			var want []string
			if tc.wantScript != "" {
				want = []string{filepath.Join(toolchain, "scripts", tc.wantScript)}
			}
			if strings.Join(checked, ",") != strings.Join(want, ",") {
				t.Fatalf("checked scripts=%v, want %v", checked, want)
			}
		})
	}
}
