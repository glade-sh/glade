package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
)

func TestSemanticBinaryIdentityProjectRoutes(t *testing.T) {
	for _, command := range []string{"check", "test"} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(command+"/api"+api, func(t *testing.T) {
				t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
				t.Cleanup(apextest.EnableDiskCacheForTesting())
				reset := func() {
					checkSemanticResults.Reset()
					apextest.InvalidateRuntimeCaches()
				}
				reset()
				t.Cleanup(reset)
				root := t.TempDir()
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				classPath := filepath.Join(root, "force-app/main/default/classes/BinaryCacheTest.cls")
				writeTestFile(t, classPath, `@IsTest private class BinaryCacheTest { @IsTest static void passes() { System.assertEquals(1, 1); } }`)
				writeTestFile(t, classPath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				perfPath := filepath.Join(t.TempDir(), "perf.json")
				args := []string{command, "--project", root, "--json", "--no-progress", "--perf-json", perfPath}
				if command == "test" {
					args = append(args, "--no-serve", "--parallelism", "1", "--no-parallel-methods")
				}
				run := func() map[string]any {
					t.Helper()
					var stdout, stderr bytes.Buffer
					if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
						t.Fatalf("%s code = %d; stdout=%s stderr=%s", command, code, &stdout, &stderr)
					}
					perf := readPerfJSONObject(t, perfPath)
					if command == "test" {
						perf = requireJSONObject(t, perf, "apexPerf")
					}
					return requireJSONObject(t, perf, "semanticCache")
				}
				cold := run()
				if cold["source"] != "build" {
					t.Fatalf("cold semantic cache = %#v", cold)
				}
				fields := requireJSONObject(t, cold, "identity")
				if digest, ok := fields["buildFingerprint"].(string); !ok || len(digest) != 64 {
					t.Errorf("%s identity lacks executable SHA-256", command)
				}
				data, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				var identity semanticcache.Identity
				if err := json.Unmarshal(data, &identity); err != nil {
					t.Fatal(err)
				}
				cachePath := filepath.Join(".glade", "semantic", "result-"+identity.OptionsFingerprint+".json")
				if _, err := os.Stat(filepath.Join(root, cachePath)); err != nil {
					t.Fatal(err)
				}
				reset()
				if warm := run(); warm["source"] != "disk" {
					t.Fatalf("same binary semantic cache = %#v, want disk hit", warm)
				}
				fields["buildFingerprint"] = strings.Repeat("0", 64)
				data, err = json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &identity); err != nil {
					t.Fatal(err)
				}
				stale := sema.Result{Diagnostics: []diagnostic.Diagnostic{{Severity: diagnostic.Error, Code: "GLADESEMA006", Message: "stale binary diagnostic", File: classPath}}}
				if err := semanticcache.Store(root, cachePath, identity, stale); err != nil {
					t.Fatal(err)
				}
				reset()
				fresh := run()
				if fresh["source"] != "build" || fresh["missReason"] != "identity_mismatch" {
					t.Fatalf("different binary semantic cache = %#v, want identity miss and rebuild", fresh)
				}
			})
		}
	}
}
