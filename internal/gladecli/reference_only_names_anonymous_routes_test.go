package gladecli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dap"
)

// context-org.tsv records a named helper called from native Tooling
// executeAnonymous. Inside the helper, Type.forName stays null at API 62 and
// 67, so glade exec and an anonymous DAP launch must not use the named-test
// reflection boundary.
func referenceOnlyAnonymousContextRows(t *testing.T) (string, [][]string) {
	t.Helper()
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "reference_only_names")
	body, err := os.ReadFile(filepath.Join(dir, "context-org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 5 {
			t.Fatalf("invalid context row: %q", line)
		}
		if row[1] == "anonymous" {
			if row[2] != "warm_named_availability" || row[3] != "RUNTIME_ERROR" || row[4] == "" {
				t.Fatalf("invalid anonymous context row: %q", line)
			}
			rows = append(rows, row)
		}
	}
	if len(rows) != 2 || rows[0][0] != "62.0" || rows[1][0] != "67.0" {
		t.Fatalf("anonymous context rows=%v", rows)
	}
	return dir, rows
}

func writeReferenceOnlyContextProject(t *testing.T, dir, api string) (string, string) {
	t.Helper()
	helperName := "RefOnlyAvailabilityWarm" + strings.TrimSuffix(api, ".0")
	helper, err := os.ReadFile(filepath.Join(dir, helperName+".cls"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, helperName+".apex"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	writeTestFile(t, filepath.Join(classes, helperName+".cls"), string(helper))
	writeTestFile(t, filepath.Join(classes, helperName+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	return root, string(body)
}

func TestExecReferenceOnlyNameAnonymousContextNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	dir, rows := referenceOnlyAnonymousContextRows(t)
	for _, row := range rows {
		t.Run(row[0], func(t *testing.T) {
			root, body := writeReferenceOnlyContextProject(t, dir, row[0])
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, body}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit=%d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), row[4]+"\n") && !strings.HasSuffix(stderr.String(), row[4]) {
				t.Fatalf("native=%q; Glade stderr=%q", row[4], stderr.String())
			}
		})
	}
}

func TestRunDAPLaunchReferenceOnlyNameAnonymousContextNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	dir, rows := referenceOnlyAnonymousContextRows(t)
	for _, row := range rows {
		t.Run(row[0], func(t *testing.T) {
			root, body := writeReferenceOnlyContextProject(t, dir, row[0])
			inR, inW := io.Pipe()
			outR, outW := io.Pipe()
			done := make(chan error, 1)
			go func() {
				defer outW.Close()
				done <- runDAP(context.Background(), []string{"--project", root}, inR, outW)
			}()
			messages := make(chan map[string]any, 16)
			go readDAPMessages(t, outR, messages)
			writeDAPRequest(t, inW, dap.CommandInitialize, 1, nil)
			writeDAPRequest(t, inW, dap.CommandLaunch, 2, map[string]any{"source": body})
			writeDAPRequest(t, inW, dap.CommandConfigurationDone, 3, nil)
			stderrOutput := waitForDAPTerminatedAndStderr(t, messages)
			writeDAPRequest(t, inW, dap.CommandDisconnect, 4, nil)
			_ = inW.Close()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(dapTestTimeout):
				t.Fatal("DAP server did not stop")
			}
			if !strings.Contains(stderrOutput, row[4]) {
				t.Fatalf("native=%q; Glade DAP stderr=%q", row[4], stderrOutput)
			}
		})
	}
}
