package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

type ownedIterableCapture struct {
	Group                     string            `json:"group"`
	Route                     string            `json:"route"`
	APIVersion                string            `json:"apiVersion"`
	Source                    string            `json:"source"`
	SourceSHA256              string            `json:"sourceSHA256"`
	Observations              string            `json:"observations"`
	ObservationsSHA256        string            `json:"observationsSHA256"`
	CompileObservations       string            `json:"compileObservations"`
	CompileObservationsSHA256 string            `json:"compileObservationsSHA256"`
	SourceManifest            string            `json:"sourceManifest"`
	SourceManifestSHA256      string            `json:"sourceManifestSHA256"`
	NativeResults             string            `json:"nativeResults"`
	NativeResultsSHA256       string            `json:"nativeResultsSHA256"`
	NamedSourceSHA256         map[string]string `json:"namedSourceSHA256"`
}

// Anonymous rows retain their captured transaction: R004 sees R003's Account.
// Named rows retain their independent @IsTest methods and terminal assertion
// transport. Every route compares its own native API62/API67 observations.
func TestOwnedIterableCollectionViewsConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	fixtureRoot := filepath.Join("..", "..", "testdata", "local-tests", "iterable-collection-views")
	var fixture struct {
		Captures []ownedIterableCapture `json:"captures"`
	}
	data, err := os.ReadFile(filepath.Join(fixtureRoot, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	wantMatrix := make(map[string]bool)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, group := range []string{"runtime", "controls"} {
				wantMatrix[api+"/"+route+"/"+group] = true
			}
		}
	}
	for _, capture := range fixture.Captures {
		name := capture.APIVersion + "/" + capture.Route + "/" + capture.Group
		if !wantMatrix[name] {
			t.Fatalf("unexpected or duplicate native capture: %s", name)
		}
		delete(wantMatrix, name)
		t.Run(name, func(t *testing.T) {
			prefix, count := "R", 9
			if capture.Group == "controls" {
				prefix, count = "V", 11
			}
			native := ownedIterableCaptureFile(t, filepath.Join(fixtureRoot, capture.Observations), capture.ObservationsSHA256)
			rows := ownedIterableRows(t, native, prefix, count)
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(
				`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`,
				capture.APIVersion))
			if capture.Route == "anonymous" {
				source := ownedIterableCaptureFile(t, filepath.Join(fixtureRoot, capture.Source), capture.SourceSHA256)
				ownedIterableAnonymous(t, root, source, rows, prefix)
			} else {
				ownedIterableNamed(t, fixtureRoot, root, capture, rows, prefix)
			}
		})
	}
	if len(wantMatrix) != 0 {
		t.Fatalf("missing native captures: %v", wantMatrix)
	}
}

func ownedIterableAnonymous(t *testing.T, root, source string, rows []string, prefix string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var result struct {
		Debug []string `json:"debug"`
	}
	envelope := decodeCLIEnvelopeData(t, stdout.Bytes(), "exec", &result)
	if envelope.Status != "passed" || envelope.ExitCode != 0 {
		t.Fatalf("unexpected anonymous envelope: %+v", envelope)
	}
	if len(result.Debug) != len(rows) {
		t.Fatalf("executed %d anonymous rows, want %d: %q", len(result.Debug), len(rows), result.Debug)
	}
	for index, observed := range rows {
		id := fmt.Sprintf("%s%03d", prefix, index+1)
		t.Run(id, func(t *testing.T) {
			want := "P|" + id + "|" + observed
			if result.Debug[index] != want {
				t.Errorf("anonymous observation=%q, native=%q", result.Debug[index], want)
			}
		})
	}
}

func ownedIterableNamed(t *testing.T, fixtureRoot, root string, capture ownedIterableCapture, rows []string, prefix string) {
	t.Helper()
	manifest := ownedIterableCaptureFile(t, filepath.Join(fixtureRoot, capture.SourceManifest), capture.SourceManifestSHA256)
	var sources []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Method string `json:"method"`
		Route  string `json:"route"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(manifest), &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources) != len(rows) || len(capture.NamedSourceSHA256) != len(rows) {
		t.Fatalf("named source matrix: sources=%d hashes=%d rows=%d", len(sources), len(capture.NamedSourceSHA256), len(rows))
	}
	compiled := ownedIterableCaptureFile(t, filepath.Join(fixtureRoot, capture.CompileObservations), capture.CompileObservationsSHA256)
	for index, outcome := range ownedIterableRows(t, compiled, prefix, len(rows)) {
		if outcome != "compiled" {
			t.Fatalf("native named source %d did not compile: %q", index, outcome)
		}
	}
	var nativeResults []struct {
		FullName   string
		MethodName string
		Outcome    string
		Message    string
	}
	native := ownedIterableCaptureFile(t, filepath.Join(fixtureRoot, capture.NativeResults), capture.NativeResultsSHA256)
	if err := json.Unmarshal([]byte(native), &nativeResults); err != nil {
		t.Fatal(err)
	}
	if len(nativeResults) != len(rows) {
		t.Fatalf("named native results=%d, want %d", len(nativeResults), len(rows))
	}
	nativeMessages := make(map[string]string)
	for _, result := range nativeResults {
		if result.Outcome != "Fail" || result.Message == "" || nativeMessages[result.FullName] != "" || !strings.HasSuffix(result.FullName, "."+result.MethodName) {
			t.Fatalf("invalid captured terminal assertion: %+v", result)
		}
		nativeMessages[result.FullName] = result.Message
	}
	want := make(map[string]string)
	for index, source := range sources {
		id := fmt.Sprintf("%s%03d", prefix, index+1)
		if source.ID != id || source.Method != id || source.Name == "" || source.Route != "runtime" || source.Source != id+".cls" {
			t.Fatalf("unexpected named source manifest row: %+v", source)
		}
		fullName := source.Name + "." + source.Method
		message := nativeMessages[fullName]
		if message != "System.AssertException: Assertion Failed: P|"+id+"|"+rows[index] {
			t.Fatalf("native assertion differs from org.tsv for %s: %q", fullName, message)
		}
		want[fullName] = message
		path := filepath.Join(fixtureRoot, filepath.Dir(capture.SourceManifest), source.Source)
		body := ownedIterableCaptureFile(t, path, capture.NamedSourceSHA256[id])
		classPath := filepath.Join(root, "force-app", "main", "default", "classes", source.Name+".cls")
		writeTestFile(t, classPath, body)
		writeTestFile(t, classPath+"-meta.xml", fmt.Sprintf(
			`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, capture.APIVersion))
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("terminal assertions require exit=1; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	run, err := decodeTestRunJSON(stdout.Bytes())
	if err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	seen := make(map[string]bool)
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			fullName := result.ClassName + "." + result.MethodName
			message, expected := want[fullName]
			if !expected || seen[fullName] {
				t.Fatalf("unexpected or duplicate named method: %+v", result)
			}
			seen[fullName] = true
			t.Run(result.MethodName, func(t *testing.T) {
				if result.Status != testreport.StatusFail || result.Reason != testreport.ReasonAssertion || result.Problem == nil {
					t.Fatalf("captured terminal assertion did not execute: %+v", result)
				}
				if got := result.Problem.Type + ": " + result.Problem.Message; got != message {
					t.Errorf("named observation=%q, native=%q", got, message)
				}
			})
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("executed %d named rows, want %d", len(seen), len(want))
	}
}

func ownedIterableRows(t *testing.T, native, prefix string, count int) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(native, "\n"), "\n")
	if len(lines) != count {
		t.Fatalf("native rows=%d, want %d", len(lines), count)
	}
	rows := make([]string, len(lines))
	for index, line := range lines {
		id, observed, ok := strings.Cut(line, "\t")
		if !ok || id != fmt.Sprintf("%s%03d", prefix, index+1) || observed == "" || observed == "?" || observed == "MISSING" {
			t.Fatalf("missing or out-of-order native observation: %q", line)
		}
		rows[index] = observed
	}
	return rows
}

func ownedIterableCaptureFile(t *testing.T, path, expectedSHA256 string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != expectedSHA256 {
		t.Fatalf("file differs from native capture export: %s sha256=%s want=%s", path, got, expectedSHA256)
	}
	return string(data)
}
