package visualforce_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
)

type v13Input struct {
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
}

type v13Diagnostic struct {
	Resource string `json:"resource"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

type v13Case struct {
	ID               string                     `json:"id"`
	Group            string                     `json:"group"`
	Kind             string                     `json:"kind"`
	Action           map[string]any             `json:"action"`
	Inputs           map[string]v13Input        `json:"inputs"`
	Expected         map[string]string          `json:"expected"`
	Diagnostics      map[string][]v13Diagnostic `json:"diagnostics"`
	Owner            string                     `json:"remaining_owner,omitempty"`
	Reason           string                     `json:"remaining_reason,omitempty"`
	DiagnosticOwner  string                     `json:"remaining_diagnostic_owner,omitempty"`
	DiagnosticReason string                     `json:"remaining_diagnostic_reason,omitempty"`
}

type v13Observation struct {
	ID         string `json:"id"`
	Actual     string `json:"actual"`
	Diagnostic string `json:"diagnostic"`
}

type v13WorkerRequest struct {
	API    string   `json:"api"`
	IDs    []string `json:"ids"`
	Output string   `json:"output"`
}

const v13WorkerLimit = 16

// TestV13SalesforceConformance compares all 278 native rows at API 59 and 67.
// The fixture exports captured compile sources/metadata, complete diagnostics,
// and full native browser observations. Browser inputs contain only owned source
// and action controls, never native answers. The local product server executes
// requests and generated scripts; the saved native observer measures callbacks,
// synchronous errors, model methods, and the same bounded no-callback result.
// Every row uses exact text equality, including every diagnostic type/message.
// Sequential child processes bound native parser allocations that are outside
// the Go heap limit. They execute this compiled test, with identical fixtures
// and observers; only the parent compares answers and counts matches.
// GLADE_V13_CAPTURE=1 records differences without failing; GLADE_V13_REPORT
// selects the per-row TSV. CI needs no Salesforce credentials.
func TestV13SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V13_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v13_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source                    string            `json:"source"`
		Runs                      map[string]string `json:"runs"`
		VersionDifferences        []json.RawMessage `json:"version_differences"`
		BrowserVersionDifferences []json.RawMessage `json:"browser_version_differences"`
		Cases                     []v13Case         `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned Ajax action observations at API 59.0 and 67.0." || len(table.Cases) != 278 {
		t.Fatalf("Ajax actions requires 278 final native cases, got %d at %s", len(table.Cases), table.Source)
	}
	if table.VersionDifferences == nil || table.BrowserVersionDifferences == nil || len(table.VersionDifferences)+len(table.BrowserVersionDifferences) != 0 {
		t.Fatal("Ajax actions final capture requires explicit zero floor/ceiling differences")
	}
	versions := []string{"59.0", "67.0"}
	seen, counts, groups := map[string]bool{}, map[string]int{}, map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid Ajax actions row %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		counts[c.Kind]++
		groups[c.Group] = true
		for _, carry := range [][2]string{{c.Owner, c.Reason}, {c.DiagnosticOwner, c.DiagnosticReason}} {
			if carry[0] != "" || carry[1] != "" {
				t.Fatalf("%s: Ajax actions requires no carried rows, got %q %q", c.ID, carry[0], carry[1])
			}
		}
		if c.Action["id"] != c.ID || c.Action["kind"] != c.Kind {
			t.Fatalf("%s: missing action controls", c.ID)
		}
		for _, api := range versions {
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) != 5 || input.Files["sfdx-project.json"] == "" || input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" || input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete API %s input", c.ID, api)
			}
			diagnostics, ok := c.Diagnostics[api]
			if !ok || diagnostics == nil {
				t.Fatalf("%s: missing API %s diagnostics", c.ID, api)
			}
			answer := c.Expected[api]
			if c.Kind == "compile" {
				if (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") || (answer == "COMPILE_OK" && len(diagnostics) != 0) || (answer == "COMPILE_ERROR" && len(diagnostics) == 0) {
					t.Fatalf("%s: inconsistent API %s native compile answer", c.ID, api)
				}
				for _, d := range diagnostics {
					if d.Resource == "" || d.Type == "" || d.Message == "" {
						t.Fatalf("%s: incomplete native diagnostic", c.ID)
					}
				}
			} else {
				var native map[string]any
				decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(answer, "DOM|")))
				decoder.UseNumber()
				if !strings.HasPrefix(answer, "DOM|") || decoder.Decode(&native) != nil || len(diagnostics) != 0 || "DOM|"+v13JSON(t, native) != answer {
					t.Fatalf("%s: invalid or changed API %s native DOM text", c.ID, api)
				}
				if native["before"] == nil || native["after"] == nil || native["outcomes"] == nil {
					t.Fatalf("%s: incomplete native DOM observation", c.ID)
				}
			}
		}
	}
	if counts["compile"] != 156 || counts["runtime"] != 122 || len(groups) != 4 || !groups["invocation"] || !groups["options"] || !groups["error-contract"] || !groups["remote-objects"] {
		t.Fatalf("incomplete Ajax actions case coverage: %v %v", counts, groups)
	}
	if request := os.Getenv("GLADE_V13_WORKER"); request != "" {
		v13RunWorker(t, table.Cases, request)
		return
	}
	var report strings.Builder
	writer := csv.NewWriter(&report)
	writer.Comma = '\t'
	write := func(fields ...string) {
		if err := writer.Write(fields); err != nil {
			t.Fatal(err)
		}
	}
	write("api", "id", "status", "actual", "expected", "group", "kind", "diagnostic", "expected_diagnostic", "acceptance_status", "diagnostic_status", "owner", "reason")
	saveReport := func() {
		writer.Flush()
		if err := writer.Error(); err != nil {
			t.Fatal(err)
		}
		if path := os.Getenv("GLADE_V13_REPORT"); path != "" {
			if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	saveReport()
	total, compileTotal, domTotal := 0, 0, 0
	for _, api := range versions {
		if !strings.Contains(table.Runs[api], "api="+api+"\n") || !strings.Contains(table.Runs[api], "observed=278\n") || !strings.Contains(table.Runs[api], "total_differences=0\n") {
			t.Fatalf("missing API %s final native run evidence", api)
		}
		observed := v13ObserveAPI(t, table.Cases, api)
		matches, compileMatches, domMatches, acceptanceMatches, diagnosticMatches := 0, 0, 0, 0, 0
		for _, c := range table.Cases {
			actual, actualDiagnostic := observed[c.ID].Actual, observed[c.ID].Diagnostic
			expectedDiagnostic := v13JSON(t, c.Diagnostics[api])
			acceptanceMatch := actual == c.Expected[api]
			diagnosticMatch := actualDiagnostic == expectedDiagnostic
			status, acceptanceStatus, diagnosticStatus := "MISMATCH", "MISMATCH", "NOT_APPLICABLE"
			if acceptanceMatch {
				acceptanceStatus = "MATCH"
				if c.Kind == "compile" {
					acceptanceMatches++
				}
			}
			if c.Kind == "compile" {
				diagnosticStatus = "MISMATCH"
				if diagnosticMatch {
					diagnosticStatus = "MATCH"
					diagnosticMatches++
				}
			}
			if acceptanceMatch && diagnosticMatch {
				status = "MATCH"
				matches++
				if c.Kind == "compile" {
					compileMatches++
				} else {
					domMatches++
				}
			}
			owner, reason := "", ""
			if !acceptanceMatch {
				owner, reason = c.Owner, c.Reason
			} else if !diagnosticMatch {
				owner, reason = c.DiagnosticOwner, c.DiagnosticReason
			}
			write(api, c.ID, status, actual, c.Expected[api], c.Group, c.Kind, actualDiagnostic, expectedDiagnostic, acceptanceStatus, diagnosticStatus, owner, reason)
			if !acceptanceMatch {
				if c.Owner != "" {
					t.Logf("API %s %s: remaining mismatch owned by %s: %s; actual <%s> expected <%s>", api, c.ID, c.Owner, c.Reason, actual, c.Expected[api])
				} else if !capture {
					t.Errorf("API %s %s: actual <%s> expected <%s>", api, c.ID, actual, c.Expected[api])
				}
			}
			if !diagnosticMatch {
				if c.DiagnosticOwner != "" {
					t.Logf("API %s %s: remaining diagnostic owned by %s: %s; actual <%s> expected <%s>", api, c.ID, c.DiagnosticOwner, c.DiagnosticReason, actualDiagnostic, expectedDiagnostic)
				} else if !capture {
					t.Errorf("API %s %s: diagnostics <%s> expected <%s>", api, c.ID, actualDiagnostic, expectedDiagnostic)
				}
			}
		}
		write("API_TOTAL", api, fmt.Sprintf("%d/278", matches))
		write("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/156", compileMatches))
		write("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/156", acceptanceMatches))
		write("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/156", diagnosticMatches))
		write("DOM_API_TOTAL", api, fmt.Sprintf("%d/122", domMatches))
		// Preserve completed rows if a later API run is interrupted. A partial
		// report has no TOTAL and does not establish a valid before count.
		saveReport()
		t.Logf("Ajax actions API %s exact %d/278; compile %d/156 (acceptance %d/156, diagnostics %d/156); DOM %d/122", api, matches, compileMatches, acceptanceMatches, diagnosticMatches, domMatches)
		total += matches
		compileTotal += compileMatches
		domTotal += domMatches
	}
	write("COMPILE_TOTAL", fmt.Sprintf("%d/312", compileTotal))
	write("DOM_TOTAL", fmt.Sprintf("%d/244", domTotal))
	write("TOTAL", fmt.Sprintf("%d/556", total))
	saveReport()
	t.Logf("Ajax actions exact matches %d/556; compile %d/312; DOM %d/244", total, compileTotal, domTotal)
}

// Re-enter the compiled test in one guarded lane, never through go test or a
// build command. Native parser allocations are reclaimed at process exit.
func v13ObserveAPI(t *testing.T, cases []v13Case, api string) map[string]v13Observation {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	observed := make(map[string]v13Observation, len(cases))
	root := t.TempDir()
	for start := 0; start < len(cases); start += v13WorkerLimit {
		end := min(start+v13WorkerLimit, len(cases))
		request := v13WorkerRequest{API: api, Output: filepath.Join(root, fmt.Sprintf("%03d.json", start))}
		for _, c := range cases[start:end] {
			request.IDs = append(request.IDs, c.ID)
		}
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestV13SalesforceConformance$", "-test.timeout=10m")
		for _, value := range os.Environ() {
			if !strings.HasPrefix(value, "GLADE_V13_WORKER=") {
				cmd.Env = append(cmd.Env, value)
			}
		}
		cmd.Env = append(cmd.Env, "GLADE_V13_WORKER="+string(encoded))
		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("Ajax actions API %s rows %d-%d: %v: %s", api, start+1, end, err, output)
		}
		data, err := os.ReadFile(request.Output)
		if err != nil {
			t.Fatal(err)
		}
		var rows []v13Observation
		if err := json.Unmarshal(data, &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != len(request.IDs) {
			t.Fatalf("Ajax actions API %s worker returned %d/%d rows", api, len(rows), len(request.IDs))
		}
		for i, row := range rows {
			if row.ID != request.IDs[i] || row.Actual == "" || row.Diagnostic == "" {
				t.Fatalf("Ajax actions API %s missing or changed worker row %q", api, request.IDs[i])
			}
			observed[row.ID] = row
		}
		t.Logf("Ajax actions API %s observed rows %d-%d/%d", api, start+1, end, len(cases))
	}
	return observed
}

func v13RunWorker(t *testing.T, cases []v13Case, encoded string) {
	t.Helper()
	var request v13WorkerRequest
	if err := json.Unmarshal([]byte(encoded), &request); err != nil {
		t.Fatal(err)
	}
	if (request.API != "59.0" && request.API != "67.0") || len(request.IDs) == 0 || len(request.IDs) > v13WorkerLimit || !filepath.IsAbs(request.Output) {
		t.Fatal("invalid Ajax actions worker request")
	}
	byID := make(map[string]v13Case, len(cases))
	for _, c := range cases {
		byID[c.ID] = c
	}
	chosen := make([]v13Case, 0, len(request.IDs))
	seen := map[string]bool{}
	hasRuntime := false
	for _, id := range request.IDs {
		c, ok := byID[id]
		if !ok || seen[id] {
			t.Fatalf("invalid Ajax actions worker row %q", id)
		}
		seen[id] = true
		chosen = append(chosen, c)
		hasRuntime = hasRuntime || c.Kind == "runtime"
	}
	var browserRows map[string]any
	if hasRuntime {
		browserRows = v13BrowserRows(t, chosen, request.API)
	}
	rows := make([]v13Observation, 0, len(chosen))
	for _, c := range chosen {
		row := v13Observation{ID: c.ID, Diagnostic: "[]"}
		if c.Kind == "compile" {
			_, _, _, diagnostics := v13Load(t, c, request.API, true)
			row.Actual = "COMPILE_OK"
			if len(diagnostics) != 0 {
				row.Actual = "COMPILE_ERROR"
			}
			row.Diagnostic = v13JSON(t, diagnostics)
		} else {
			row.Actual = "DOM|" + v13JSON(t, browserRows[c.ID])
		}
		rows = append(rows, row)
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(request.Output, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func v13Load(t *testing.T, c v13Case, api string, analyze bool) (project.Project, typesys.Index, error, []v13Diagnostic) {
	return v13LoadProject(v13WriteInput(t, c, api), analyze)
}

func v13WriteInput(t *testing.T, c v13Case, api string) string {
	t.Helper()
	root := t.TempDir()
	for path, source := range c.Inputs[api].Files {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
			t.Fatalf("%s: invalid fixture path %q", c.ID, path)
		}
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func v13LoadProject(root string, analyze bool) (project.Project, typesys.Index, error, []v13Diagnostic) {
	diagnostics := []v13Diagnostic{}
	fail := func(p project.Project, index typesys.Index, err error, resource string) (project.Project, typesys.Index, error, []v13Diagnostic) {
		diagnostics = append(diagnostics, v13Diagnostic{Resource: resource, Type: "Error", Message: err.Error()})
		return p, index, err, diagnostics
	}
	p, err := project.Load(root)
	if err != nil {
		return fail(p, typesys.Index{}, err, "ApexPage")
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		return fail(p, typesys.Index{}, err, "ApexClass")
	}
	index := typesys.Build(p, sch)
	if analyze {
		result := sema.AnalyzeWithOptions(apextest.SemanticAnalysisIndex(index), sema.AnalyzeOptions{Diagnostics: true, SuppressPerformanceDiagnostics: true})
		for _, d := range result.Diagnostics {
			if d.Severity == diagnostic.Error {
				// A04/A05 publish measured compiler text in NativeMessage;
				// Message may retain an editor explanation of the same error.
				message := d.NativeMessage
				if message == "" {
					message = d.Message
				}
				diagnostics = append(diagnostics, v13Diagnostic{Resource: "ApexClass", Type: "Error", Message: message})
			}
		}
	}
	loader := visualforce.LoadProjectForRender
	if analyze {
		loader = visualforce.LoadProject
	}
	if _, err := loader(p); err != nil {
		diagnostics = append(diagnostics, v13Diagnostic{Resource: "ApexPage", Type: "Error", Message: err.Error()})
	}
	sort.SliceStable(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
		if a.Resource != b.Resource {
			return a.Resource < b.Resource
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Message < b.Message
	})
	return p, index, nil, diagnostics
}

func v13BrowserRows(t *testing.T, cases []v13Case, api string) map[string]any {
	t.Helper()
	roots := map[string]string{}
	specs := []map[string]any{}
	for _, c := range cases {
		if c.Kind != "runtime" {
			continue
		}
		path := "/apex/" + c.Inputs[api].Name
		roots[path] = v13WriteInput(t, c, api)
		// Whitelist action controls: expectations and capture provenance never reach JS.
		action := map[string]any{}
		for _, key := range []string{"id", "mode", "args", "steps", "callback", "options", "operation", "value", "field", "criteria"} {
			if value, ok := c.Action[key]; ok {
				action[key] = value
			}
		}
		specs = append(specs, map[string]any{"id": c.ID, "path": path, "action": action})
	}
	// The observer visits one page at a time. Keep that page's org, source
	// metadata and index through its endpoint requests, then retire them on
	// navigation. Retaining 122 full indices defeats the runner's heap bound.
	var mu sync.Mutex
	var currentPath string
	var current *server.Server
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/remoting") || strings.HasSuffix(path, "/remoteObjects") {
			path = path[:strings.LastIndex(path, "/")]
		}
		root, ok := roots[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if currentPath != path {
			current = nil
			currentPath = ""
			srv, err := v13RuntimeServer(root, api)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			current, currentPath = srv, path
		}
		current.ServeHTTP(w, r)
	}))
	defer local.Close()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		t.Fatal(err)
	}
	observer, err := filepath.Abs("testdata/v13_browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Ajax actions API %s local browser: %v: %s", api, err, stderr.String())
	}
	var observed map[string]any
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.UseNumber()
	if err := decoder.Decode(&observed); err != nil {
		t.Fatal(err)
	}
	if len(observed) != len(specs) {
		t.Fatalf("Ajax actions browser returned %d/%d rows at API %s", len(observed), len(specs), api)
	}
	for _, spec := range specs {
		if observed[spec["id"].(string)] == nil {
			t.Fatalf("Ajax actions browser omitted %s at API %s", spec["id"], api)
		}
	}
	return observed
}

func v13RuntimeServer(root, api string) (*server.Server, error) {
	p, index, err, _ := v13LoadProject(root, false)
	if err != nil {
		return nil, err
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		return nil, err
	}
	org := storage.NewOrgState()
	org.APIVersion = api
	for _, object := range []string{"Account", "User", "Profile"} {
		storage.EnsureStandardObject(&org, object)
	}
	userID, profileID := storage.ID("005000000000013"), storage.ID("00e000000000013")
	users := org.Objects["User"]
	users.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"Name": storage.StringValue("Ajax actions Owned User"), "Username": storage.StringValue("v13@example.test"), "IsActive": storage.BooleanValue(true), "ProfileId": storage.IDValue(profileID)}}
	org.Objects["User"] = users
	profiles := org.Objects["Profile"]
	profiles.Records[profileID] = storage.Record{ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Ajax actions Owned Profile")}}
	org.Objects["Profile"] = profiles
	srv := server.NewWithSource(&org, source)
	srv.VisualforceHTMLUserID = userID
	// Build request runtimes on demand rather than retaining a VM template.
	srv.SetProjectRuntime(index, nil, nil)
	return srv, nil
}

// Match the captured Python json.dumps(sort_keys=True, ensure_ascii=True)
// framing, including spaces. This changes serialization only, never observations.
func v13JSON(t *testing.T, value any) string {
	t.Helper()
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	quoted, escaped := false, false
	for _, r := range strings.TrimSuffix(encoded.String(), "\n") {
		if r == '"' && !escaped {
			quoted = !quoted
		}
		if r <= 127 {
			out.WriteRune(r)
		} else {
			for _, unit := range utf16.Encode([]rune{r}) {
				fmt.Fprintf(&out, "\\u%04x", unit)
			}
		}
		if !quoted && (r == ':' || r == ',') {
			out.WriteByte(' ')
		}
		if r == '\\' && !escaped {
			escaped = true
		} else {
			escaped = false
		}
	}
	return out.String()
}
