package gladehome

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyLWCToolchainPreparation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		script    bool
		hook      bool
		checkErr  error
		wantErr   string
		wantCalls int
	}{
		{name: "legacy", wantCalls: 0},
		{name: "missing current script", hook: true, wantErr: "script missing"},
		{name: "prepared", script: true, hook: true, wantCalls: 1},
		{name: "unprepared", script: true, hook: true, checkErr: errors.New("preimage requires preparation"), wantErr: "not prepared", wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			toolchain := filepath.Join(root, "third_party", "lwc")
			if err := os.MkdirAll(filepath.Join(toolchain, "scripts"), 0o755); err != nil {
				t.Fatal(err)
			}
			pkg := `{"scripts":{}}`
			if tc.hook {
				pkg = `{"scripts":{"postinstall":"node scripts/apply-lwc-shared-api67.mjs"}}`
			}
			if err := os.WriteFile(filepath.Join(toolchain, "package.json"), []byte(pkg), 0o644); err != nil {
				t.Fatal(err)
			}
			script := filepath.Join(toolchain, "scripts", "apply-lwc-shared-api67.mjs")
			if tc.script {
				if err := os.WriteFile(script, []byte("// owned fixture\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			err := verifyLWCToolchainPreparation(root, func(gotScript, gotRoot string) error {
				calls++
				if gotScript != script || gotRoot != toolchain {
					t.Fatalf("wrong preparation roots: %s %s", gotScript, gotRoot)
				}
				return tc.checkErr
			})
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error=%v, want %q", err, tc.wantErr)
			}
			if calls != tc.wantCalls {
				t.Fatalf("check calls=%d, want %d", calls, tc.wantCalls)
			}
		})
	}
}
