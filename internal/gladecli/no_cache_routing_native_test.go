package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestDaemonRoutingPoisonPreservesProjectMetadata(t *testing.T) {
	for _, helper := range []struct {
		name   string
		poison func(*testing.T, string)
	}{
		{"direct_poison", poisonDaemonRoutingLocalLoad},
		{"equivalence_fixture", addDaemonEquivalenceLocalTest},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(helper.name+"/"+api, func(t *testing.T) {
				testDaemonRoutingPoisonPreservesProjectMetadata(t, api, helper.poison)
			})
		}
	}
}

func testDaemonRoutingPoisonPreservesProjectMetadata(t *testing.T, api string, poison func(*testing.T, string)) {
	fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{
		sourceAPIVersion: api,
		classes:          map[string]string{"AlphaTest": `@isTest private class AlphaTest { @isTest static void passes() { System.assert(true); } }`},
	})
	manifest := filepath.Join(fixture.projectRoot, "sfdx-project.json")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	poison(t, fixture.projectRoot)
	after, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("routing canary corrupted project metadata needed by the server runtime")
	}
	classFile := filepath.Join(fixture.baseRoot, "classes.txt")
	writeDaemonEquivalenceFile(t, classFile, "AlphaTest\n")
	for _, route := range []struct {
		name string
		args []string
	}{
		{"all_tests", nil},
		{"selected_class", []string{"--class", "AlphaTest"}},
		{"selected_method", []string{"--class", "AlphaTest", "--method", "passes"}},
		{"filter", []string{"--filter", "AlphaTest"}},
		{"class_file", []string{"--class-file", classFile}},
		{"execution_shard", []string{"--class", "AlphaTest", "--shard-count", "1", "--shard-index", "0"}},
	} {
		t.Run(route.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"test", "--project", fixture.projectRoot, "--no-serve", "--no-cache", "--json", "--no-progress"}
			code := Run(context.Background(), append(args, route.args...), &stdout, &stderr)
			var envelope struct{ Data testreport.Run }
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode local canary: %v; stdout=%s stderr=%s", err, stdout.String(), stderr.String())
			}
			if code == 0 || envelope.Data.Summary().CompileErrors == 0 {
				t.Fatalf("local route bypassed canary: exit=%d summary=%+v stderr=%s", code, envelope.Data.Summary(), stderr.String())
			}
		})
	}
}

// Use the matrix's actual server fixture, then force its client down the local
// route. Successful selection must never conceal the fixture's routing poison.
func TestDaemonEquivalenceSelectedRoutesRejectLocalFallback(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct {
			name        string
			args        []string
			clientInput bool
		}{
			{"selected_class", []string{"--class", "AlphaTest"}, false},
			{"selected_method", []string{"--class", "AlphaTest", "--method", "passes"}, false},
			{"filter", []string{"--filter", "AlphaTest"}, false},
			{"class_file", nil, true},
			{"execution_shard", []string{"--class", "AlphaTest", "--shard-count", "1", "--shard-index", "0"}, false},
		} {
			t.Run(api+"/"+route.name, func(t *testing.T) {
				scenario := daemonEquivalenceScenario{
					name: route.name, sourceAPIVersion: api,
					classes: map[string]string{
						"AlphaTest": `@isTest private class AlphaTest { @isTest static void passes() { System.assert(true); } }`,
					},
					args:         append([]string{"--no-cache"}, route.args...),
					clientInputs: route.clientInput, classFileEntries: []string{"AlphaTest"},
					wantExit: 0, wantTotal: 1, wantPass: 1, wantCase: "AlphaTest.passes", wantEnvelopeStatus: "passed",
				}
				for _, mode := range []daemonEquivalenceMode{
					{name: "local", args: []string{"--no-serve"}},
					{name: "auto_connect", realServer: true},
					{name: "explicit_connect", args: []string{"--connect"}, realServer: true},
					{name: "forced_local", args: []string{"--no-serve"}, realServer: true},
				} {
					t.Run(mode.name, func(t *testing.T) {
						invocation := scenario
						if mode.name == "auto_connect" {
							// --no-cache deliberately suppresses automatic connection.
							invocation.args = route.args
						}
						got := runDaemonEquivalenceInvocation(t, invocation, mode)
						if mode.name == "forced_local" {
							if got.Exit == 0 || got.Summary.CompileErrors == 0 {
								t.Fatalf("matrix local fallback escaped poison: exit=%d summary=%+v", got.Exit, got.Summary)
							}
						} else {
							assertDaemonEquivalenceScenario(t, invocation, mode, got)
						}
					})
				}
			})
		}
	}
}

func poisonDaemonRoutingLocalLoad(t *testing.T, root string) {
	t.Helper()
	// Preserve project metadata, but reject local execution even when class
	// selection excludes the additional failing test.
	addDaemonEquivalenceUnpublishedTest(t, root)
	writeDaemonEquivalenceFile(t, filepath.Join(root, "force-app/main/default/triggers/LocalRoutingCanary.trigger"),
		`trigger LocalRoutingCanary on Account (before insert) { !!! }`)
}

func TestConnectedRunUsesPublishedProjectSnapshot(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{
				sourceAPIVersion: api,
				classes:          map[string]string{"AlphaTest": `@isTest private class AlphaTest { @isTest static void passes() { System.assert(true); } }`},
			})
			stopServer := startDaemonEquivalenceServer(t, fixture.projectRoot)
			defer stopServer()
			writeDaemonEquivalenceFile(t, filepath.Join(fixture.projectRoot, "sfdx-project.json"), "{\n")
			classFile := filepath.Join(fixture.baseRoot, "classes.txt")
			writeDaemonEquivalenceFile(t, classFile, "AlphaTest\n")
			for _, route := range []struct {
				name string
				args []string
			}{
				{"explicit_connect", []string{"--connect"}},
				{"auto_connect", nil},
				{"selected_class", []string{"--connect", "--class", "AlphaTest"}},
				{"class_file", []string{"--connect", "--class-file", classFile}},
				{"execution_shard", []string{"--connect", "--shard-count", "1", "--shard-index", "0"}},
			} {
				t.Run(route.name, func(t *testing.T) {
					args := route.args
					if route.name != "auto_connect" {
						args = append([]string{"--no-cache"}, args...)
					}
					got := runNoCacheRoutingInvocation(t, fixture.projectRoot, args...)
					assertNoCacheRoutingResult(t, got, 1, "AlphaTest.passes")
				})
			}
		})
	}
}

func TestNoCacheRoutingNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "nocache_routing_native")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 11 || strings.Join(rows[0], ",") != "source_api,route,case,observation" {
		t.Fatal("incomplete native matrix")
	}
	classes := make(map[string]string)
	for _, name := range []string{"AlphaTest", "BetaTest", "GammaTest", "DeltaTest"} {
		source, err := os.ReadFile(filepath.Join(dir, "project", "force-app", "main", "default", "classes", name+".cls"))
		if err != nil {
			t.Fatal(err)
		}
		classes[name] = string(source)
	}
	anonymous, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		var wantCases []string
		wantAnonymous := ""
		for _, row := range rows[1:] {
			if row[0] != api {
				continue
			}
			switch row[1] {
			case "isTest":
				if row[3] != "Pass" {
					t.Fatalf("unexpected native outcome: %v", row)
				}
				wantCases = append(wantCases, row[2])
			case "anonymous":
				wantAnonymous = row[3]
			default:
				t.Fatalf("unknown native route: %v", row)
			}
		}
		if len(wantCases) != 4 || wantAnonymous != "4" {
			t.Fatalf("incomplete native rows for API %s", api)
		}
		t.Run(api+"/anonymous", func(t *testing.T) {
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{classes: classes, sourceAPIVersion: api})
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", fixture.projectRoot, "--json", string(anonymous)}, &stdout, &stderr)
			var envelope struct{ Data struct{ Debug []string } }
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if code != 0 || !strings.Contains(strings.Join(envelope.Data.Debug, "\n"), "OBSERVED|assertions|"+wantAnonymous) {
				t.Fatalf("anonymous native=%s exit=%d stdout=%s stderr=%s", wantAnonymous, code, stdout.String(), stderr.String())
			}
		})
		for _, mode := range []daemonEquivalenceMode{
			{name: "local", args: []string{"--no-serve"}},
			{name: "in_process_daemon", args: []string{"--daemon"}},
			{name: "explicit_connect", args: []string{"--connect"}, realServer: true},
		} {
			t.Run(api+"/isTest/"+mode.name, func(t *testing.T) {
				scenario := daemonEquivalenceScenario{classes: classes, sourceAPIVersion: api, clientInputs: true,
					args:     []string{"--no-cache", "--parallelism", "4", "--parallel-methods"},
					wantExit: 0, wantTotal: len(wantCases), wantPass: len(wantCases), wantEnvelopeStatus: "passed"}
				got := runDaemonEquivalenceInvocation(t, scenario, mode)
				assertDaemonEquivalenceScenario(t, scenario, mode, got)
				assertNoCacheRoutingResult(t, got.Run, len(wantCases), wantCases...)
			})
		}
	}
}
