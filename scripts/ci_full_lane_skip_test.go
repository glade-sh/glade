package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCIFullLaneAuditsSkips(t *testing.T) {
	for _, tc := range []struct {
		name, lane, pkg, action, terminal string
		ok                                bool
	}{
		{"passed", "sema", "sema", "pass", "pass", true},
		{"unexpected skip", "sema", "sema", "skip", "pass", false},
		{"missing package", "sema", "sema", "pass", "", false},
		{"service passed", "server-process-builder", "server", "pass", "pass", true},
		{"service skip", "server-process-builder", "server", "skip", "pass", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fake := `#!/bin/bash
printf '%s\n' '{"Action":"` + tc.action + `","Package":"github.com/glade-sh/glade/internal/` + tc.pkg + `","Test":"TestExample"}'
` + func() string {
				if tc.terminal == "" {
					return ""
				}
				return `printf '%s\n' '{"Action":"` + tc.terminal + `","Package":"github.com/glade-sh/glade/internal/` + tc.pkg + `"}'`
			}() + "\n"
			if err := os.WriteFile(filepath.Join(dir, "go"), []byte(fake), 0700); err != nil {
				t.Fatal(err)
			}
			renderer := filepath.Join(dir, "testlog")
			if err := os.WriteFile(renderer, []byte("#!/bin/bash\ncat >/dev/null\n"), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-go-test.sh", "lane", tc.lane)
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "CI_GO_COMMAND="+realGoCommand(t), "CI_TESTLOG_RENDERER="+renderer, "CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "evidence"))
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("lane err=%v output=%s", err, out)
			}
		})
	}
}
