package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCIFullLaneAuditsSkips(t *testing.T) {
	for _, tc := range []struct {
		name, action, terminal string
		ok                     bool
	}{
		{"passed", "pass", "pass", true}, {"unexpected skip", "skip", "pass", false}, {"missing package", "pass", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fake := `#!/bin/bash
printf '%s\n' '{"Action":"` + tc.action + `","Package":"github.com/glade-sh/glade/internal/sema","Test":"TestExample"}'
` + func() string {
				if tc.terminal == "" {
					return ""
				}
				return `printf '%s\n' '{"Action":"` + tc.terminal + `","Package":"github.com/glade-sh/glade/internal/sema"}'`
			}() + "\n"
			if err := os.WriteFile(filepath.Join(dir, "go"), []byte(fake), 0700); err != nil {
				t.Fatal(err)
			}
			renderer := filepath.Join(dir, "testlog")
			if err := os.WriteFile(renderer, []byte("#!/bin/bash\ncat >/dev/null\n"), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "ci-go-test.sh", "lane", "sema")
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "CI_GO_COMMAND="+realGoCommand(t), "CI_TESTLOG_RENDERER="+renderer, "CI_GO_TEST_ARTIFACT_DIR="+filepath.Join(dir, "evidence"))
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("lane err=%v output=%s", err, out)
			}
		})
	}
}
