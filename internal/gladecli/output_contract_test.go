package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestCLIOutputContractJSONProgressSplit(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/PassingTest.cls"), `@IsTest
private class PassingTest {
  @IsTest static void ok() {
    System.assertEquals(1, 1);
  }
}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)

	cases := []struct {
		name string
		args []string
	}{
		{name: "check", args: []string{"check", "--project", root, "--json", "--progress-json"}},
		{name: "schema load", args: []string{"schema", "load", "--project", root, "--json", "--progress-json"}},
		{name: "test", args: []string{"test", "--project", root, "--class", "PassingTest", "--json", "--progress-json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tc.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
			}
			if !json.Valid(stdout.Bytes()) {
				t.Fatalf("stdout is not JSON:\n%s", stdout.String())
			}
			assertNDJSONLines(t, stderr.String())
		})
	}
}

func TestCLIOutputContractDBSeedJSONProgressSplit(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "glade.db")
	fixturePath := filepath.Join(root, "fixture.json")
	writeTestFile(t, fixturePath, `{
  "version":"glade.storage.v1",
  "objects":[{"name":"Account","records":[{"alias":"acme","fields":{"Name":{"kind":"string","string":"Acme"}}}]}]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--db", dbPath, "--project", root, "--json", "--progress-json", fixturePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not JSON:\n%s", stdout.String())
	}
	assertNDJSONLines(t, stderr.String())
}

func TestCLIOutputContractNoProgressSuppressesStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Hello.cls"), "public class Hello {}")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "symbols", "--project", root, "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestCLIOutputContractDevLWCBoundsRoutesAndKeepsReadyFileComplete(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	for i := 0; i < 82; i++ {
		name := strings.ToLower("routeProbe"+string(rune('A'+(i%26)))) + strings.Repeat("x", i/26)
		writeTestFile(t, filepath.Join(root, "force-app/main/default/lwc", name, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>true</isExposed></LightningComponentBundle>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	printDevLWCStartupSummary(&out, "127.0.0.1:7357", p, devLWCSelection{})
	if !strings.Contains(out.String(), "... 2 more routes omitted.") {
		t.Fatalf("summary did not cap routes:\n%s", out.String())
	}
	readyPath := filepath.Join(t.TempDir(), "ready.json")
	if err := writeDevLWCReadyFile(readyPath, "127.0.0.1:7357", p, devLWCSelection{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(readyPath)
	if err != nil {
		t.Fatal(err)
	}
	var ready struct {
		Routes []string `json:"routes"`
	}
	if err := json.Unmarshal(data, &ready); err != nil {
		t.Fatal(err)
	}
	if len(ready.Routes) != 82 {
		t.Fatalf("ready routes = %d, want 82", len(ready.Routes))
	}
}

func assertNDJSONLines(t *testing.T, raw string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 || strings.TrimSpace(raw) == "" {
		t.Fatalf("stderr has no NDJSON events")
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("stderr line is not JSON: %q\nall stderr:\n%s", line, raw)
		}
	}
}
