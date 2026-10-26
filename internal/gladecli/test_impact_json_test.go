package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Decode the public artifact independently so this contract compiles before
// the opt-in implementation exists.
type impactJSONContract struct {
	Version      int      `json:"version"`
	SharedInputs []string `json:"sharedInputs"`
	Tests        []struct {
		ClassName       string   `json:"className"`
		MethodName      string   `json:"methodName"`
		SourceFile      string   `json:"sourceFile"`
		SourceFiles     []string `json:"sourceFiles"`
		Classes         []string `json:"classes"`
		Methods         []string `json:"methods"`
		Operations      []string `json:"operations"`
		FallbackReasons []string `json:"fallbackReasons"`
	} `json:"tests"`
}

func TestImpactJSONCLIFlags(t *testing.T) {
	for _, mode := range []string{"--connect", "--daemon", "--watch", "--watch-once", "--wizard", "--debug"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "impact.json")
			var out, progress bytes.Buffer
			_, err := runTest(context.Background(), []string{"--impact-json", path, mode}, &out, &progress)
			if err == nil || !strings.Contains(err.Error(), "--impact-json cannot be combined with "+mode) {
				t.Fatalf("mode rejection = %v", err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("artifact written before rejection: %v", err)
			}
		})
	}
	t.Run("artifact-collision", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "shared.json")
		var out, progress bytes.Buffer
		_, err := runTest(context.Background(), []string{"--impact-json", path, "--perf-json", path, "--no-serve"}, &out, &progress)
		if err == nil || !strings.Contains(err.Error(), "same path") {
			t.Fatalf("collision = %v", err)
		}
	})
}

func TestImpactJSONProjectNamedTestRunnerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, parallelism := range []string{"1", "2"} {
			t.Run(api+"/workers="+parallelism, func(t *testing.T) {
				root := writeImpactJSONProject(t, api)
				args := []string{"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache", "--read-only", "--parallelism", parallelism}
				code, stdout, stderr := runPerfCLI(t, context.Background(), args...)
				identity := perfTestResultIdentity(t, stdout)
				if len(identity) != 3 {
					t.Fatalf("baseline test count: %s", stdout)
				}
				for _, row := range identity {
					want := "pass"
					if row["methodName"] == "rejection" {
						want = "fail"
					}
					if row["status"] != want {
						t.Fatalf("baseline route did not execute: %s", stdout)
					}
				}
				path := filepath.Join(t.TempDir(), "artifact", "impact.json")
				tracedCode, tracedOut, tracedErr := runPerfCLI(t, context.Background(), append(args, "--impact-json", path)...)
				if tracedCode != code || !bytes.Equal(stderr, tracedErr) {
					t.Fatalf("outcome changed: baseline=%d %s; impact=%d %s", code, stderr, tracedCode, tracedErr)
				}
				if got, want := perfTestResultIdentity(t, tracedOut), perfTestResultIdentity(t, stdout); !reflect.DeepEqual(got, want) {
					t.Fatalf("result identity changed: got=%#v want=%#v", got, want)
				}
				var artifact impactJSONContract
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &artifact); err != nil {
					t.Fatal(err)
				}
				if artifact.Version != 1 || len(artifact.Tests) != 3 {
					t.Fatalf("artifact = %s", data)
				}
				assertPathPermissions(t, filepath.Dir(path), 0o700)
				if !sort.StringsAreSorted(artifact.SharedInputs) {
					t.Errorf("unstable shared input order")
				}
				assertPathPermissions(t, path, 0o600)
				for _, file := range []string{"sfdx-project.json", "force-app/main/default/classes/ImpactWorker.cls", "force-app/main/default/classes/ImpactWorker.cls-meta.xml", "force-app/main/default/labels/CustomLabels.labels-meta.xml"} {
					if !containsImpactString(artifact.SharedInputs, file) {
						t.Errorf("shared inputs omit %s: %v", file, artifact.SharedInputs)
					}
				}
				for _, row := range artifact.Tests {
					for _, values := range [][]string{row.SourceFiles, row.Classes, row.Methods, row.Operations} {
						if !sort.StringsAreSorted(values) {
							t.Errorf("unstable set order: %v", values)
						}
						for i := 1; i < len(values); i++ {
							if values[i-1] == values[i] {
								t.Errorf("duplicate dependency: %v", values)
							}
						}
					}
					if row.SourceFile == "" || !containsImpactString(row.SourceFiles, row.SourceFile) {
						t.Errorf("missing entry source: %#v", row)
					}
					if !containsImpactString(row.Classes, row.ClassName) {
						t.Errorf("missing entry class: %#v", row)
					}
					if row.MethodName == "viaHelper" {
						if !containsImpactString(row.Classes, "ImpactWorker") || !containsImpactString(row.Methods, "ImpactWorker.value") {
							t.Errorf("missing helper route: %#v", row)
						}
						if containsImpactString(row.Classes, "ImpactQuiet") {
							t.Errorf("cross-test trace leak: %#v", row)
						}
					}
					if row.MethodName == "viaTrigger" && (!containsImpactString(row.Operations, "dml:insert") || !containsImpactString(row.SourceFiles, "force-app/main/default/triggers/ImpactAccount.trigger")) {
						t.Errorf("missing trigger route: %#v", row)
					}
				}
				if strings.Contains(string(tracedOut), `"trace"`) || strings.Contains(string(tracedOut), `"profile"`) {
					t.Fatalf("impact leaked profiling into results: %s", tracedOut)
				}
			})
		}
	}
}

func TestImpactJSONProjectCompileErrorBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) { testImpactJSONProjectCompileErrorBoundary(t, api) })
	}
}

func testImpactJSONProjectCompileErrorBoundary(t *testing.T, api string) {
	root := writeImpactJSONProject(t, api)
	writeImpactJSONFile(t, filepath.Join(root, "force-app/main/default/classes/ImpactWorker.cls"), `public class ImpactWorker { public static Integer value() { return missingName; } }`)
	path := filepath.Join(t.TempDir(), "impact.json")
	code, stdout, stderr := runPerfCLI(t, context.Background(), "test", "--project", root, "--no-progress", "--no-serve", "--no-cache", "--json", "--read-only", "--impact-json", path)
	if code == 0 {
		t.Fatalf("compile error passed: %s %s", stdout, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var artifact impactJSONContract
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if len(artifact.Tests) != 3 {
		t.Fatalf("compile boundary dropped cases: %s", data)
	}
	for _, row := range artifact.Tests {
		if len(row.FallbackReasons) == 0 {
			t.Errorf("unexecuted test claims complete impact: %#v", row)
		}
		if row.SourceFile != "" || len(row.Classes) != 0 {
			t.Errorf("unexecuted test claims executed source: %#v", row)
		}
	}
}

func TestImpactJSONProjectChangedRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := writeImpactJSONProject(t, api)
			writeImpactJSONFile(t, filepath.Join(root, "force-app/main/default/classes/ImpactIndependentTests.cls"), `@IsTest class ImpactIndependentTests { @IsTest static void independent() { System.assertEquals(9, ImpactQuiet.value()); } }`)
			runGitCLI(t, root, "init", "--quiet")
			runGitCLI(t, root, "add", ".")
			runGitCLI(t, root, "-c", "user.name=Glade Test", "-c", "user.email=glade@example.test", "commit", "--quiet", "-m", "synthetic baseline")
			writeImpactJSONFile(t, filepath.Join(root, "force-app/main/default/classes/ImpactWorker.cls"), `public class ImpactWorker { public static Integer value() { return 3+4; } }`)
			for _, route := range [][]string{{"test", "--changed-since", "HEAD"}, {"test", "changed", "--since", "HEAD"}} {
				args := append(append([]string{}, route...), "--project", root, "--json", "--no-progress")
				baseline := []string{"test", "--changed-since", "HEAD", "--project", root, "--json", "--no-progress", "--no-serve"}
				code, stdout, stderr := runPerfCLI(t, context.Background(), baseline...)
				path := filepath.Join(t.TempDir(), "impact.json")
				tracedCode, tracedOut, tracedErr := runPerfCLI(t, context.Background(), append(args, "--impact-json", path)...)
				if tracedCode != code || !bytes.Equal(stderr, tracedErr) {
					t.Fatalf("changed route outcome: %d %s / %d %s", code, stderr, tracedCode, tracedErr)
				}
				if !reflect.DeepEqual(perfTestResultIdentity(t, stdout), perfTestResultIdentity(t, tracedOut)) {
					t.Fatalf("changed route result identity: %s / %s", stdout, tracedOut)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var artifact impactJSONContract
				if err := json.Unmarshal(data, &artifact); err != nil {
					t.Fatal(err)
				}
				if len(artifact.Tests) != 3 {
					t.Fatalf("changed selection = %s", data)
				}
				for _, row := range artifact.Tests {
					if row.ClassName != "ImpactTests" {
						t.Errorf("unselected test traced: %#v", row)
					}
				}
			}
		})
	}
}

func TestImpactJSONProjectSelectionAndRequestedTraceRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := writeImpactJSONProject(t, api)
			// One method per class exercises the runner's no-setup execution path.
			writeImpactJSONFile(t, filepath.Join(root, "force-app/main/default/classes/ImpactTests.cls"), `@IsTest class ImpactTests { @IsTest static void viaHelper() { System.assertEquals(7, ImpactWorker.value()); } }`)
			classFile := filepath.Join(t.TempDir(), "classes.txt")
			writeImpactJSONFile(t, classFile, "ImpactTests\n")
			before := snapshotReadOnlyProject(t, root)
			for _, traceFlag := range []string{"", "--trace-blockers", "--trace"} {
				path := filepath.Join(t.TempDir(), "impact.json")
				args := []string{"test", "--project", root, "--no-progress", "--read-only", "--json", "--parallelism", "2", "--class-file", classFile, "--impact-json", path}
				tracePath := filepath.Join(t.TempDir(), "trace.json")
				if traceFlag != "" {
					args = append(args, traceFlag)
				}
				if traceFlag == "--trace" {
					args = append(args, tracePath)
				}
				code, out, errOut := runPerfCLI(t, context.Background(), args...)
				if code != 0 {
					t.Fatalf("%s: %d %s %s", traceFlag, code, out, errOut)
				}
				var artifact impactJSONContract
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &artifact); err != nil {
					t.Fatal(err)
				}
				if len(artifact.Tests) != 1 || artifact.Tests[0].MethodName != "viaHelper" {
					t.Fatalf("selection changed: %s", data)
				}
				if traceFlag == "--trace" {
					data, err := os.ReadFile(tracePath)
					if err != nil || !bytes.Contains(data, []byte("ImpactWorker.value")) {
						t.Fatalf("requested trace lost: %s %v", data, err)
					}
				} else if bytes.Contains(out, []byte(`"trace"`)) {
					t.Fatalf("unrequested trace leaked: %s", out)
				}
			}
			if after := snapshotReadOnlyProject(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("impact run modified read-only project")
			}
		})
	}
}

func TestImpactJSONProjectTruncationBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) { testImpactJSONProjectTruncationBoundary(t, api) })
	}
}

func testImpactJSONProjectTruncationBoundary(t *testing.T, api string) {
	root := writeImpactJSONProject(t, api)
	writeImpactJSONFile(t, filepath.Join(root, "force-app/main/default/classes/ImpactTests.cls"), `@IsTest class ImpactTests { @IsTest static void longMethod() { Integer total=0; for(Integer i=0;i<12000;i++){total++;} System.assertEquals(12000,total); } }`)
	path := filepath.Join(t.TempDir(), "impact.json")
	code, out, errOut := runPerfCLI(t, context.Background(), "test", "--project", root, "--no-progress", "--read-only", "--json", "--class", "ImpactTests", "--impact-json", path)
	if code != 0 {
		t.Fatalf("truncation changed outcome: %d %s %s", code, out, errOut)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var artifact impactJSONContract
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if len(artifact.Tests) != 1 || !containsImpactString(artifact.Tests[0].FallbackReasons, "trace-truncated") {
		t.Fatalf("truncated trace claims complete coverage: %s", data)
	}
}

func BenchmarkImpactJSONTestRunner(b *testing.B) {
	root := writeImpactJSONProject(b, "67.0")
	writeImpactJSONFile(b, filepath.Join(root, "force-app/main/default/classes/ImpactTests.cls"), `@IsTest class ImpactTests { @IsTest static void measured() { Integer total=0; for(Integer i=0;i<100;i++){total+=ImpactWorker.value();} System.assertEquals(700,total); } }`)
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		b.Run(name, func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "impact.json")
			args := []string{"--project", root, "--no-progress", "--read-only", "--json", "--no-serve", "--parallelism", "1"}
			if enabled {
				args = append(args, "--impact-json", path)
			}
			run := func() {
				var out, progress bytes.Buffer
				report, err := runTest(context.Background(), args, &out, &progress)
				if err != nil || report.Summary().Passed != 1 {
					b.Fatalf("run: %v %#v %s", err, report.Summary(), &out)
				}
			}
			run()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				run()
			}
			b.StopTimer()
			if enabled {
				data, err := os.ReadFile(path)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(len(data)), "artifact-B")
			}
		})
	}
}

func writeImpactJSONFile(t testing.TB, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeImpactJSONProject(t testing.TB, api string) string {
	t.Helper()
	root := t.TempDir()
	writeImpactJSONFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	classes := map[string]string{
		"ImpactWorker": `public class ImpactWorker { public static Integer value() { return 7; } }`,
		"ImpactQuiet":  `public class ImpactQuiet { public static Integer value() { return 9; } }`,
		"ImpactTests": `@IsTest class ImpactTests {
			@TestSetup static void seed() { insert new Account(Name='setup'); }
			@IsTest static void viaHelper() { System.assertEquals(7, ImpactWorker.value()); System.assertEquals('ready', System.Label.ImpactReady); }
			@IsTest static void viaTrigger() { insert new Account(Name='method'); System.assertEquals(2, [SELECT COUNT() FROM Account]); }
			@IsTest static void rejection() { System.assertEquals(0, ImpactQuiet.value()); }
		}`,
	}
	for name, source := range classes {
		file := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeImpactJSONFile(t, file, source)
		writeImpactJSONFile(t, file+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	}
	trigger := filepath.Join(root, "force-app/main/default/triggers/ImpactAccount.trigger")
	writeImpactJSONFile(t, trigger, `trigger ImpactAccount on Account (before insert) { for(Account row : Trigger.new) { row.Description=String.valueOf(ImpactWorker.value()); } }`)
	writeImpactJSONFile(t, trigger+"-meta.xml", `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
	writeImpactJSONFile(t, filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels-meta.xml"), `<CustomLabels xmlns="http://soap.sforce.com/2006/04/metadata"><labels><fullName>ImpactReady</fullName><value>ready</value><language>en_US</language><protected>false</protected><shortDescription>Ready</shortDescription></labels></CustomLabels>`)
	return root
}

func containsImpactString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
