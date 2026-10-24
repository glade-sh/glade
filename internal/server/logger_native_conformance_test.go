package server

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// The compiler and caller-visible logger results come from the same owned
// component deployed at APIs 59, 62 and 67. Hosted event delivery is not observed.
// CI replays the retained rows without accessing Salesforce.
func TestLoggerSalesforceConformanceHostRoutes(t *testing.T) {
	if os.Getenv("GLADE_LWC_COMPILE") == "" {
		if _, err := os.Stat(filepath.Join("..", "..", "third_party", "lwc", "node_modules")); err != nil {
			t.Skip("install the LWC compiler or set GLADE_LWC_COMPILE=1")
		}
	}
	const fixture = "testdata/logger_native"
	data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 43 || !reflect.DeepEqual(rows[0], []string{"api", "case_id", "expected"}) {
		t.Fatal("expected 42 native compile/runtime observations and the TSV header")
	}
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			expected := map[string]string{}
			for _, row := range rows[1:] {
				if row[0] == api {
					if _, duplicate := expected[row[1]]; duplicate {
						t.Fatalf("duplicate native row %s", row[1])
					}
					expected[row[1]] = row[2]
				}
			}
			if len(expected) != 14 || expected["compile"] != "COMPILE_OK" {
				t.Fatal("incomplete native observations")
			}
			root := t.TempDir()
			name := "telemetryProbe" + strings.TrimSuffix(api, ".0")
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", name)
			for _, file := range []struct{ source, target string }{
				{"probe.js", name + ".js"}, {"probe.html", name + ".html"}, {"observations.js", "observations.js"},
			} {
				contents, err := os.ReadFile(filepath.Join(fixture, file.source))
				if err != nil {
					t.Fatal(err)
				}
				writeLightningFixtureFile(t, filepath.Join(bundle, file.target), string(contents))
			}
			writeLightningFixtureFile(t, filepath.Join(bundle, name+".js-meta.xml"),
				`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>true</isExposed><targets><target>lightning__Tab</target></targets></LightningComponentBundle>`)
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"),
				`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/pages/TelemetryHost.page"),
				`<apex:page showHeader="false" sidebar="false"><apex:includeLightning /><div id="telemetry"></div><script>$Lightning.use("c:telemetryOut", function() { $Lightning.createComponent("c:`+name+`", {}, "telemetry"); });</script></apex:page>`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/aura/telemetryOut/telemetryOut.app"),
				`<aura:application access="GLOBAL" extends="ltng:outApp"><aura:dependency resource="c:`+name+`"/></aura:application>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			handler := newVisualforceHTMLTestServer(t, &org, source)
			t.Cleanup(handler.resetLightningCacheLocked)
			t.Run("Compile", func(t *testing.T) {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/modules/c/"+name+"/"+name+".js", nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("native %s; local HTTP %d: %s", expected["compile"], rec.Code, rec.Body.String())
				}
				if !strings.Contains(rec.Body.String(), "lightning/logger") {
					t.Fatal("compiled component lost its logger import")
				}
			})
			assertRuntime := func(t *testing.T, moduleURL string) {
				t.Helper()
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, moduleURL, nil))
				if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
					t.Fatalf("logger module HTTP %d: %s", rec.Code, rec.Body.String())
				}
				observations, err := os.ReadFile(filepath.Join(fixture, "observations.js"))
				if err != nil {
					t.Fatal(err)
				}
				moduleJSON, _ := json.Marshal(rec.Body.String())
				observationsJSON, _ := json.Marshal(string(observations))
				script := fmt.Sprintf(`
const load = source => import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
const { log } = await load(%s);
const { observeLogger } = await load(%s);
process.stdout.write(JSON.stringify(observeLogger(log)));
`, moduleJSON, observationsJSON)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				output, err := exec.CommandContext(ctx, "node", "--input-type=module", "-e", script).CombinedOutput()
				if err != nil {
					t.Fatalf("logger execution: %v\n%s", err, output)
				}
				var actual [][2]string
				if err := json.Unmarshal(output, &actual); err != nil {
					t.Fatalf("decode runtime rows: %v\n%s", err, output)
				}
				if len(actual) != len(expected)-1 {
					t.Fatalf("runtime observations %d, want %d", len(actual), len(expected)-1)
				}
				seen := map[string]bool{}
				for _, row := range actual {
					want, exists := expected[row[0]]
					if !exists || seen[row[0]] || row[1] != want {
						t.Errorf("%s: local %q, native %q (observed=%t duplicate=%t)", row[0], row[1], want, exists, seen[row[0]])
					}
					seen[row[0]] = true
				}
			}
			t.Run("Runtime", func(t *testing.T) {
				assertRuntime(t, "/lightning/shims/lightning/logger.js")
			})
			// Both host routes must resolve the logger through their emitted import
			// map, including when the registry entry points at a wrong URL.
			for _, route := range []struct{ name, path string }{
				{"LWCShellRoute", "/lwc/preview/component/c/" + name},
				{"VisualforceLightningOutRoute", "/apex/TelemetryHost"},
			} {
				t.Run(route.name, func(t *testing.T) {
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route.path, nil))
					if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
						t.Fatalf("host %s HTTP %d: %s", route.path, rec.Code, rec.Body.String())
					}
					matches := regexp.MustCompile(`<script type="importmap">([^<]+)</script>`).FindAllStringSubmatch(rec.Body.String(), -1)
					if len(matches) != 1 {
						t.Fatalf("host %s emitted %d import maps, want 1", route.path, len(matches))
					}
					var importMap struct {
						Imports map[string]string `json:"imports"`
					}
					if err := json.Unmarshal([]byte(matches[0][1]), &importMap); err != nil {
						t.Fatal(err)
					}
					componentURL := importMap.Imports["c/"+name]
					if componentURL == "" {
						t.Fatalf("host %s has no import mapping for c/%s", route.path, name)
					}
					component := httptest.NewRecorder()
					handler.ServeHTTP(component, httptest.NewRequest(http.MethodGet, componentURL, nil))
					if component.Code != http.StatusOK || !strings.Contains(component.Body.String(), "lightning/logger") {
						t.Fatalf("host component %s HTTP %d: %s", componentURL, component.Code, component.Body.String())
					}
					loggerURL := importMap.Imports["lightning/logger"]
					if loggerURL == "" {
						t.Fatalf("host %s has no import mapping for lightning/logger", route.path)
					}
					assertRuntime(t, loggerURL)
				})
			}
		})
	}
}
