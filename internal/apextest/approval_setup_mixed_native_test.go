package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestApprovalSetupMixedNativeConformance(t *testing.T) {
	fixture := "testdata/conformance/approval_setup_mixed"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				sources := map[string]string{}
				if err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, e fs.DirEntry, err error) error {
					if err != nil || e.IsDir() {
						return err
					}
					rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
					if err != nil {
						return err
					}
					b, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if strings.HasSuffix(rel, "-meta.xml") {
						b = []byte(strings.ReplaceAll(string(b), "62.0", api))
					}
					sources[rel] = string(b)
					writeFile(t, filepath.Join(root, rel), string(b))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				source := sources["anonymous.apex"]
				if route == "istest" {
					source = "ApprovalSetupCaptureTest.capture();"
					machine.EnableTestContext()
					if err := RegisterTestMethodForRequest(machine, index, "ApprovalSetupCaptureTest", "capture"); err != nil {
						t.Fatal(err)
					}
				}
				p, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				payload := ""
				if route == "anonymous" {
					r, err := machine.Execute(p)
					if err != nil {
						t.Fatal(err)
					}
					for _, line := range r.Debug {
						if _, v, ok := strings.Cut(line, "SETUP_ROWS|"); ok {
							payload = v
							break
						}
					}
				} else {
					_, err := machine.ExecuteInClass(p, "ApprovalSetupCaptureTest")
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) {
						t.Fatalf("missing observation: %v", err)
					}
					_, payload, _ = strings.Cut(fault.ExceptionMessage(), "SETUP_ROWS|")
				}
				got := map[string]string{}
				for _, pair := range strings.Split(payload, ";") {
					if pair != "" {
						k, v, ok := strings.Cut(pair, "=")
						if !ok {
							t.Fatal(pair)
						}
						got[k] = v
					}
				}
				capture := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), route)
				run, err := os.ReadFile(filepath.Join(capture, "RUN-FACTS.txt"))
				if err != nil {
					t.Fatal(err)
				}
				for _, fact := range []string{"api=" + api, "context=" + route, "rows=2", "native_complete=true", "deployment_success=true"} {
					if !strings.Contains("\n"+string(run), "\n"+fact+"\n") {
						t.Fatalf("missing %s", fact)
					}
				}
				for name, src := range sources {
					if !strings.Contains(string(run), fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256([]byte(src)))) {
						t.Fatalf("source changed %s", name)
					}
				}
				b, err := os.ReadFile(filepath.Join(capture, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				lines := strings.Split(strings.TrimSpace(string(b)), "\n")
				if len(lines) != 3 {
					t.Fatalf("rows %d", len(lines)-1)
				}
				for _, line := range lines[1:] {
					key, raw, ok := strings.Cut(line, "\t")
					if !ok {
						t.Fatal(line)
					}
					var want string
					if err := json.Unmarshal([]byte(raw), &want); err != nil {
						t.Fatal(err)
					}
					if got[key] != want {
						t.Errorf("%s got %q want %q", key, got[key], want)
					}
				}
			})
		}
	}
}
