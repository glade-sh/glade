package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replays the native LWC action argument conversions
// (../apextest/testdata/conformance/lwc_param_conversion/org.tsv) through the
// two HTTP routes that reach the LWC conversion: the imperative route and the
// wire route (cacheable methods). The Apex class is the capture class itself.
// The VM test in internal/vm covers every cell; this test proves both routes
// pass the decoded JSON arguments to that conversion unchanged.

var lwcParamRouteArgs = map[string]string{
	"missing": ``, "null": `null`, "str_x": `"x"`, "num_42": `42`, "num_frac": `4.5`, "num_big": `3000000000`,
	"bool_true": `true`, "str_42": `"42"`, "str_empty": `""`, "str_date": `"2024-01-02"`,
	"str_datetime_z": `"2024-01-02T03:04:05.000Z"`, "str_datetime_noz": `"2024-01-02T03:04:05.000"`,
	"str_id15": `"001000000000000"`, "str_id18": `"001000000000000AAA"`,
	"obj_name": `{"Name":"n"}`, "obj_sobj": `{"sobjectType":"Account","Name":"n"}`,
	"arr_str": `["a","b"]`, "arr_num": `[1,2]`, "str_json_arr": `"[\"a\"]"`,
}

// Cells the capture answered but Glade does not reproduce yet; the VM test
// carries the same list.
var lwcParamRouteParked = map[string]bool{"Date/str_datetime_z": true}

func lwcParamRouteCells(t *testing.T) map[string]map[string]bool {
	t.Helper()
	file, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", "lwc_param_conversion", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	cells := map[string]map[string]bool{}
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 7 || strings.Contains(fields[6], "aura:systemError") {
			continue
		}
		key := fields[4] + "/" + fields[5]
		if cells[key] == nil {
			cells[key] = map[string]bool{}
		}
		cells[key][fields[6]] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return cells
}

func TestLWCParameterConversionHTTPRoutes(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "conformance", "lwc_param_conversion", "project", "force-app", "main", "default", "classes", "Cv67.cls"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/Cv67.cls"), string(source))
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/Cv67.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sourceMetadata, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	srv := newLightningTestServer(t, &org, sourceMetadata)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatal(srv.runtimeErr)
	}
	checked := 0
	for key, outcomes := range lwcParamRouteCells(t) {
		typeName, argName, _ := strings.Cut(key, "/")
		argJSON, ok := lwcParamRouteArgs[argName]
		if !ok || len(outcomes) != 1 || lwcParamRouteParked[key] {
			continue
		}
		var want string
		for outcome := range outcomes {
			want = outcome
		}
		method := strings.NewReplacer("<", "", ">", "").Replace(typeName)
		params := `{"value":` + argJSON + `}`
		if argJSON == "" {
			params = `{}`
		}
		for _, route := range []struct{ name, method, path, body string }{
			{"imperative", http.MethodPost, "/lightning/apex/Cv67/t" + method, params},
			{"wire", http.MethodPost, "/lightning/wire/apex", `{"className":"Cv67","method":"w` + method + `","params":` + params + `,"cacheable":true}`},
		} {
			t.Run(route.name+"/"+key, func(t *testing.T) {
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
				}
				var out lwcbrowser.WireResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(want, "error:") {
					message := strings.TrimPrefix(want, "error:")
					if out.Error == nil {
						t.Fatalf("native rejected %s; got data %#v", argJSON, out.Data)
					}
					if strings.HasPrefix(message, "Value provided is invalid for action parameter") || message == "Unable to read SObject" {
						if out.Error.Body == nil || out.Error.Body.Message != message {
							t.Fatalf("error = %#v, want %s", out.Error, message)
						}
					}
					return
				}
				if out.Error != nil {
					t.Fatalf("native accepted %s as %s; got %#v", argJSON, want, out.Error)
				}
				if out.Data != strings.TrimPrefix(want, "ok:") {
					t.Fatalf("data = %#v, want %s", out.Data, strings.TrimPrefix(want, "ok:"))
				}
			})
			checked++
		}
	}
	if checked < 300 {
		t.Fatalf("expected at least 300 route cells, got %d", checked)
	}
}
