package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseLockstepBuild(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, product, source, diagnostic string
		pass                              bool
	}{
		{"compatible", "package glade\nfunc Candidate() {}\n", "package main\nimport g \"github.com/glade-sh/glade\"\nfunc main(){g.Candidate()}\n", "", true},
		{"incompatible-candidate", "package glade\nfunc Other() {}\n", "package main\nimport g \"github.com/glade-sh/glade\"\nfunc main(){g.Candidate()}\n", "undefined: g.Candidate", false},
		{"broken-tools", "package glade\nfunc Candidate() {}\n", "package main\nfunc main(){missing()}\n", "undefined: missing", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			product := filepath.Join(d, "candidate")
			tools := filepath.Join(d, "tools")
			files := map[string]string{
				filepath.Join(product, "go.mod"):                               "module github.com/glade-sh/glade\n\ngo 1.26\n",
				filepath.Join(product, "glade.go"):                             tc.product,
				filepath.Join(product, "third_party/glade-apex-parser/go.mod"): "module github.com/glade-sh/apex-parser\n\ngo 1.26\n",
				filepath.Join(tools, "go.mod"):                                 "module example.com/tools\n\ngo 1.26\n\nrequire github.com/glade-sh/glade v0.0.0\nreplace github.com/glade-sh/glade => ../unrelated\n",
				filepath.Join(tools, "go.sum"):                                 "",
				filepath.Join(tools, "cmd/glade-tools/main.go"):                tc.source,
			}
			for path, data := range files {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(data), 0644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", filepath.Join(root, "scripts/release-tools-lockstep.sh"), product, tools)
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v err=%v output=%s", tc.pass, err, out)
			}
			if tc.diagnostic != "" && !strings.Contains(string(out), tc.diagnostic) {
				t.Fatalf("build failure must contain %q, got err=%v output=%s", tc.diagnostic, err, out)
			}
			for path, want := range files {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatalf("changed input %s: %v", path, err)
				}
			}
		})
	}
}

func TestReleaseLockstepRunsBeforeReleaseGates(t *testing.T) {
	data, err := os.ReadFile("release-check.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	lock := strings.Index(script, "scripts/release-tools-lockstep.sh")
	for _, gate := range []string{"npm run release:check", "scripts/ci-go-test.sh", "scripts/smoke.sh"} {
		if lock < 0 || strings.Index(script, gate) < lock {
			t.Errorf("lockstep must run before %s", gate)
		}
	}
}
