package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v09Fixture struct {
	Files             map[string]string `json:"files"`
	ControllerPath    string            `json:"controller_path"`
	CompileController string            `json:"compile_controller"`
	RuntimeController string            `json:"runtime_controller"`
}

type v09Observation struct {
	URL         *string `json:"url"`
	Navigate    bool    `json:"navigate"`
	RecordParam bool    `json:"record_param"`
	TargetRole  string  `json:"target_role,omitempty"`
}

type v09Case struct {
	ID            string                     `json:"id"`
	Group         string                     `json:"group"`
	Kind          string                     `json:"kind"`
	Inputs        map[string]v04Input        `json:"inputs"`
	Expected      map[string]string          `json:"expected"`
	Diagnostics   map[string][]v04Diagnostic `json:"diagnostics"`
	Observation   v09Observation             `json:"observation"`
	Owner         string                     `json:"owner,omitempty"`
	Reason        string                     `json:"reason,omitempty"`
	Qualified     bool                       `json:"-"`
	Widget        bool                       `json:"-"`
	DeniedControl bool                       `json:"-"`
	Parameters    map[string]string          `json:"-"`
}

// TestV09SalesforceConformance replays all 146 compile/metadata and 132 DOM
// rows at API 59/67. The final floor DOM capture is browser-recapture/
// api59; floor metadata is from api59, and both ceiling captures are from api67.
// Compile rows use V04's shared compiler adapter. DOM rows use the product HTTP
// handler and its same-principal lookup destinations, without a browser.
// The observer follows the native ID/role projection and DOM property fields.
// Every answer and diagnostic is compared as exact text, retaining whitespace,
// raw null, current-date affordances and generated script text. Expected values
// never seed execution. Only the owned Account ID is replaced in fixture source.
// GLADE_V09_CAPTURE=1 records all row differences without failing on mismatches;
// GLADE_V09_REPORT selects the optional per-row TSV output path.
var v09APIVersions = []string{"59.0", "67.0"}

func TestV09SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V09_CAPTURE") != ""
	// Preserve a stricter host limit and restore the process setting afterwards.
	// Rows run serially; no browser, page, handler or DOM survives a row.
	const memoryLimit = 1536 << 20
	if previous := debug.SetMemoryLimit(-1); previous > memoryLimit {
		debug.SetMemoryLimit(memoryLimit)
		defer debug.SetMemoryLimit(previous)
	}
	// These disposable project generations must not publish disk-cache work.
	restoreDiskCache := apextest.DisableDiskCacheForTesting()
	defer restoreDiskCache()
	v09ReleaseRowState(t)
	defer v09ReleaseRowState(t)
	data, err := os.ReadFile("testdata/v09_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		OracleCommit string                `json:"oracle_commit"`
		Fixtures     map[string]v09Fixture `json:"fixtures"`
		Cases        []v09Case             `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.OracleCommit != "capture-revision-013768982597356981632269094543802952453226531052006184415422506951984144623109" || len(table.Cases) != 278 {
		t.Fatalf("V09 requires all 278 cases from the final native oracle, got %d at %s", len(table.Cases), table.OracleCommit)
	}
	controls := v09LoadWidgetControls(t)
	proofs := map[string]bool{}
	for _, row := range controls.Cases {
		if row.Kind == "runtime" {
			for _, original := range table.Cases {
				if original.ID != row.SourceRow {
					continue
				}
				identical := true
				for _, api := range []string{"59.0", "67.0"} {
					pageContainsInput := false
					for path, source := range original.Inputs[api].Files {
						if strings.HasSuffix(path, ".page") && strings.Contains(source, row.Markup) {
							pageContainsInput = true
						}
					}
					identical = identical && pageContainsInput
				}
				if identical {
					proofs[row.SourceRow] = true
				}
			}
		}
	}
	if len(proofs) != 19 {
		t.Fatalf("V09 needs 19 identical-input control proofs, got %d", len(proofs))
	}
	for i := range table.Cases {
		table.Cases[i].Qualified = proofs[table.Cases[i].ID]
	}
	for _, api := range v09APIVersions {
		fixture := table.Fixtures[api]
		if len(fixture.Files) != 22 || fixture.Files["sfdx-project.json"] == "" ||
			fixture.Files["force-app/main/default/objects/V09FieldFixture__c/V09FieldFixture__c.object-meta.xml"] == "" ||
			fixture.ControllerPath == "" || fixture.CompileController == "" || fixture.RuntimeController == "" {
			t.Fatalf("incomplete V09 captured scaffold at API %s", api)
		}
	}
	counts, seen, groups := map[string]int{}, map[string]bool{}, map[string]int{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate V09 case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, groups[c.Group]+1
		counts[c.Kind]++
		if (c.Owner == "") != (c.Reason == "") || c.Owner != "" && c.Owner != "V14 / authorization and request context" {
			t.Fatalf("%s: carries require an active V14 owner and one-line reason", c.ID)
		}
		for _, api := range v09APIVersions {
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) != 2 ||
				input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete captured input at API %s", c.ID, api)
			}
			answer, exists := c.Expected[api]
			diagnostics, diagnosticExists := c.Diagnostics[api]
			if !exists || !diagnosticExists || diagnostics == nil {
				t.Fatalf("%s: missing native answer/diagnostics at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				if (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") || (answer == "COMPILE_ERROR") != (len(diagnostics) > 0) {
					t.Fatalf("%s: inconsistent native compile answer at API %s", c.ID, api)
				}
				for _, d := range diagnostics {
					if d.Resource == "" || d.Type == "" || d.Message == "" {
						t.Fatalf("%s: incomplete native diagnostic at API %s", c.ID, api)
					}
				}
			} else {
				var native map[string]json.RawMessage
				if !strings.HasPrefix(answer, "DOM|") || json.Unmarshal([]byte(strings.TrimPrefix(answer, "DOM|")), &native) != nil || len(diagnostics) != 0 {
					t.Fatalf("%s: invalid native DOM answer at API %s", c.ID, api)
				}
				for _, key := range []string{"text", "labels", "controls", "links", "tags", "rich"} {
					if native[key] == nil {
						t.Fatalf("%s: missing native %s at API %s", c.ID, key, api)
					}
				}
				if c.Observation.Navigate && native["destination"] == nil {
					t.Fatalf("%s: missing captured destination at API %s", c.ID, api)
				}
				if c.Observation.Navigate && c.Observation.TargetRole != "Account" && c.Observation.TargetRole != "User" {
					t.Fatalf("%s: missing captured destination role at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 146 || counts["runtime"] != 132 || len(groups) != 5 ||
		groups["field-types"] != 166 || groups["labels"] != 54 || groups["requiredness"] != 24 ||
		groups["overrides"] != 20 || groups["lookup-destinations"] != 14 {
		t.Fatalf("V09 requires 146 compile/metadata and 132 DOM rows in all five groups, got %v, %v", counts, groups)
	}

	var report io.Writer = io.Discard
	if path := os.Getenv("GLADE_V09_REPORT"); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		}()
		report = file
	}
	tsv := csv.NewWriter(report)
	tsv.Comma = '\t'
	writeRow := func(t *testing.T, fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
		// Keep only csv.Writer's fixed buffer, and retain completed rows even if
		// validation is interrupted before the final count.
		tsv.Flush()
		if err := tsv.Error(); err != nil {
			t.Fatal(err)
		}
	}
	writeRow(t, "api", "id", "status", "actual", "expected", "group", "diagnostic", "owner", "reason", "acceptance_status", "diagnostic_status", "expected_diagnostic")
	matches, compileMatches, acceptanceMatches, diagnosticMatches, domMatches := 0, 0, 0, 0, 0
	apiPasses := 0
	for _, api := range v09APIVersions {
		t.Run(api, func(t *testing.T) {
			apiPasses++
			apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches := 0, 0, 0, 0, 0
			// Keep the compile and runtime controllers in separate stable projects.
			// Build the runtime's immutable metadata and request template once per
			// API; every row still installs only its own page and gets fresh records.
			compileRoot, runtimeRoot := t.TempDir(), t.TempDir()
			defer visualforce.V09ReleaseRenderEnvironmentsForTest(runtimeRoot)
			fixture := table.Fixtures[api]
			for root, controller := range map[string]string{
				compileRoot: fixture.CompileController, runtimeRoot: fixture.RuntimeController,
			} {
				v09WriteFiles(t, root, fixture.Files)
				v09WriteFiles(t, root, map[string]string{fixture.ControllerPath: controller})
			}
			renderer := v09NewRenderer(t, runtimeRoot)
			for row, c := range table.Cases {
				func() {
					input := c.Inputs[api]
					root := compileRoot
					if c.Kind == "runtime" {
						root = runtimeRoot
					}
					defer func() {
						for path := range input.Files {
							if err := os.Remove(filepath.Join(root, path)); err != nil && !os.IsNotExist(err) {
								t.Error(err)
							}
						}
					}()
					v09WriteFiles(t, root, input.Files)
					got, diagnostics := "", []v04Diagnostic{}
					if c.Kind == "compile" {
						got, diagnostics = v04Execute(t, root, input.Name, v04Case{Kind: "compile"})
					} else {
						got = renderer.render(t, input.Name, c)
					}
					want := c.Expected[api]
					if c.Kind == "runtime" && c.Qualified {
						got, want = v09QualifyOriginal(t, got), v09QualifyOriginal(t, want)
					}
					actualDiagnostic, expectedDiagnostic := v04Encode(t, diagnostics), v04Encode(t, c.Diagnostics[api])
					acceptanceMatch := got == want
					diagnosticMatch := c.Kind != "compile" || actualDiagnostic == expectedDiagnostic
					status, acceptanceStatus, diagnosticStatus := "MISMATCH", "MISMATCH", "NOT_APPLICABLE"
					if acceptanceMatch {
						acceptanceStatus = "MATCH"
						if c.Kind == "compile" {
							apiAcceptanceMatches++
						}
					}
					if c.Kind == "compile" {
						diagnosticStatus = "MISMATCH"
						if diagnosticMatch {
							diagnosticStatus = "MATCH"
							apiDiagnosticMatches++
						}
					}
					if acceptanceMatch && diagnosticMatch {
						status = "MATCH"
						apiMatches++
						if c.Kind == "compile" {
							apiCompileMatches++
						} else {
							apiDOMMatches++
						}
					}
					// Hide temporary paths only in the report, after exact comparison.
					owner, reason := "", ""
					if status != "MATCH" && c.Owner != "" {
						owner, reason = c.Owner, c.Reason
						t.Logf("V09 API %s carry %s: %s: %s", api, c.ID, owner, reason)
						if owner == "V14 / authorization and request context" && !v09EqualOutsideDestination(t, got, want) && !capture {
							t.Errorf("API %s %s: V14 destination carry cannot exempt owned DOM: actual <%s> expected <%s>", api, c.ID, got, want)
						}
					}
					writeRow(t, api, c.ID, status, strings.ReplaceAll(got, root, "<fixture>"), want, c.Group,
						strings.ReplaceAll(actualDiagnostic, root, "<fixture>"), owner, reason, acceptanceStatus, diagnosticStatus, expectedDiagnostic)
					if !capture && owner == "" && (!acceptanceMatch || !diagnosticMatch) {
						t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, got, want, actualDiagnostic, expectedDiagnostic)
					}
				}()
				v09ReleaseRowState(t, runtimeRoot)
				if (row+1)%20 == 0 || row+1 == len(table.Cases) {
					var memory runtime.MemStats
					runtime.ReadMemStats(&memory)
					t.Logf("V09 API %s completed %d/278; retained heap %d MiB", api, row+1, memory.HeapAlloc>>20)
				}
			}
			writeRow(t, "API_TOTAL", api, fmt.Sprintf("%d/278", apiMatches))
			writeRow(t, "COMPILE_API_TOTAL", api, fmt.Sprintf("%d/146", apiCompileMatches))
			writeRow(t, "COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/146", apiAcceptanceMatches))
			writeRow(t, "DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/146", apiDiagnosticMatches))
			writeRow(t, "DOM_API_TOTAL", api, fmt.Sprintf("%d/132", apiDOMMatches))
			t.Logf("V09 API %s exact %d/278; compile %d/146, acceptance %d/146, diagnostics %d/146; DOM %d/132", api, apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches)
			// Drop the single API template before building the next one.
			renderer = nil
			v09ReleaseRowState(t, runtimeRoot)
			matches += apiMatches
			compileMatches += apiCompileMatches
			acceptanceMatches += apiAcceptanceMatches
			diagnosticMatches += apiDiagnosticMatches
			domMatches += apiDOMMatches
			if api == "67.0" {
				write := func(fields ...string) { writeRow(t, fields...) }
				v09RunWidgetControls(t, controls, capture, write)
				v09RunReviewControls(t, capture, write)
				v09RunDeniedControls(t, capture, write)
			}
		})
	}
	writeRow(t, "COMPILE_TOTAL", fmt.Sprintf("%d/%d", compileMatches, apiPasses*146))
	writeRow(t, "COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/%d", acceptanceMatches, apiPasses*146))
	writeRow(t, "DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/%d", diagnosticMatches, apiPasses*146))
	writeRow(t, "DOM_TOTAL", fmt.Sprintf("%d/%d", domMatches, apiPasses*132))
	writeRow(t, "TOTAL", fmt.Sprintf("%d/%d", matches, apiPasses*278))
	t.Logf("V09 exact matches %d/556; compile %d/292; DOM %d/264", matches, compileMatches, domMatches)
}

// The page-content environment registry and request-runtime cache both retain
// VMs independently of the handler. Drop only V09's finished render environments,
// drain cache work before invalidating it, then collect after the row's execution
// frame has returned. Keep the lane guard as protection against future retention.
func v09ReleaseRowState(t *testing.T, roots ...string) {
	t.Helper()
	for _, root := range roots {
		visualforce.V09ReleaseRenderEnvironmentsForTest(root)
	}
	apextest.WaitForRuntimeCacheWork()
	apextest.InvalidateRuntimeCaches()
	debug.FreeOSMemory()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	if memory.HeapAlloc > 2<<30 {
		t.Fatalf("V09 retained heap exceeds 2 GiB after row cleanup: %d bytes", memory.HeapAlloc)
	}
}

func v09WriteFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, source := range files {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
			t.Fatalf("invalid V09 fixture path %q", path)
		}
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

const v09AccountID = "001000000000004AAA"
const v09AccountName = "V09 owned & <lookup>"
const v09OwnerName = "V09 Current Principal"

// The native fixture was an administrator-owned Account plus the deployed
// V09 object/field permission set. Reuse V04's positive principal fixture and
// load the custom field definitions through the shared project schema path.
func v09Org(index typesys.Index) (storage.OrgState, storage.ID) {
	org, user := v04Org()
	projectOrg := apextest.OrgFromIndex(index)
	org.Objects["V09FieldFixture__c"] = projectOrg.Objects["V09FieldFixture__c"]
	user.Fields["Name"] = storage.StringValue(v09OwnerName)
	// Bound principal facts captured identically at both APIs and browser zones.
	user.Fields["LocaleSidKey"] = storage.StringValue("en_US")
	user.Fields["LanguageLocaleKey"] = storage.StringValue("en_US")
	user.Fields["TimeZoneSidKey"] = storage.StringValue("America/Los_Angeles")
	users := org.Objects["User"]
	users.Records[user.ID] = user
	org.Objects["User"] = users
	accounts := org.Objects["Account"]
	record := accounts.Records[storage.ID(v09AccountID)]
	record.Fields = map[string]storage.Value{
		"Name": storage.StringValue(v09AccountName), "Description": storage.StringValue("field and input rendering owned capture record"),
		"OwnerId": storage.IDValue(user.ID),
	}
	accounts.Records[record.ID] = record
	org.Objects["Account"] = accounts
	profileID := user.Fields["ProfileId"].ID
	objects, fields := org.Objects["ObjectPermissions"], org.Objects["FieldPermissions"]
	const objectID storage.ID = "110000000000009"
	objects.Records[objectID] = storage.Record{ID: objectID, Object: "ObjectPermissions", Fields: map[string]storage.Value{
		"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("V09FieldFixture__c"),
		"PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true),
		"PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true),
	}}
	names := make([]string, 0, len(org.Objects["V09FieldFixture__c"].Definition.Fields))
	for field := range org.Objects["V09FieldFixture__c"].Definition.Fields {
		names = append(names, field)
	}
	sort.Strings(names)
	for i, field := range names {
		id := storage.ID(fmt.Sprintf("12000000009%04d", i+1))
		fields.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("V09FieldFixture__c"),
			"Field": storage.StringValue("V09FieldFixture__c." + field), "PermissionsRead": storage.BooleanValue(true),
			"PermissionsEdit": storage.BooleanValue(field != "Formula__c"),
		}}
	}
	org.Objects["ObjectPermissions"], org.Objects["FieldPermissions"] = objects, fields
	return org, user.ID
}

// v09Renderer keeps one immutable source/index/runtime/record template per API.
// It never holds a response body, parsed DOM, live controller or row org.
type v09Renderer struct {
	root       string
	source     server.SourceMetadata
	index      typesys.Index
	runtime    *vm.VM
	runtimeErr error
	org        storage.RuntimeTemplate
	principal  storage.ID
}

func v09NewRenderer(t *testing.T, root string) *v09Renderer {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, sch)
	org, principal := v09Org(index)
	compiled, runtimeErr := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	machine := vm.New(nil)
	if runtimeErr == nil {
		runtimeErr = apextest.RegisterCompiledProjectRuntimeForRequest(machine, compiled)
	}
	return &v09Renderer{root: root, source: source, index: index, runtime: machine,
		runtimeErr: runtimeErr, org: storage.NewRuntimeTemplate(org), principal: principal}
}

func (renderer *v09Renderer) render(t *testing.T, name string, c v09Case) string {
	t.Helper()
	p, err := project.Load(renderer.root)
	if err != nil {
		return v09DOM(t, map[string]any{"localError": err.Error()})
	}
	// The HTTP handler uses the current project's sole page. The shared
	// tooling metadata contains only the unchanged controller/object scaffold.
	source := renderer.source
	source.Project = p
	org, principal := renderer.org.CloneRuntimeOrg(), renderer.principal
	handler := server.NewWithSource(&org, source)
	handler.VisualforceHTMLUserID = principal
	machine := renderer.runtime.CloneRuntime(nil)
	// Match RegisterProjectRuntimeForRequest's strict Page-reference catalog
	// for this row, without recompiling identical Apex and schema generations.
	if pages, err := visualforce.LoadProject(p); err == nil {
		for _, page := range pages.Pages {
			machine.RegisterPageReference(page.Name)
		}
	}
	handler.SetProjectRuntime(renderer.index, machine, renderer.runtimeErr)
	query := url.Values{"ownedCase": {c.ID}}
	for name, value := range c.Parameters {
		query.Set(name, value)
	}
	if c.Observation.URL != nil {
		query.Set("ownedURL", *c.Observation.URL)
	}
	if c.Observation.RecordParam {
		query.Set("id", v09AccountID)
	}
	request := httptest.NewRequest(http.MethodGet, "https://v09.example.test/apex/"+name+"?"+query.Encode(), nil)
	response := httptest.NewRecorder()
	started := time.Now()
	handler.ServeHTTP(response, request)
	ended := time.Now()
	doc, err := html.Parse(strings.NewReader(response.Body.String()))
	if err != nil {
		return v09DOM(t, map[string]any{"localError": err.Error()})
	}
	if c.DeniedControl {
		return v09ObserveDeniedControl(t, doc, request, response.Code)
	}
	if c.Widget {
		return v09ObserveWidget(t, doc, handler, request, response.Code, c, string(principal), started, ended)
	}
	observed := v09ObserveDOM(t, doc, handler, request, response.Code, c, string(principal))
	if c.Qualified {
		observed = v09VerifyDynamicFacts(t, observed, doc, c, started, ended)
	}
	return observed
}

func v09Stable(text, ownerID string) string {
	for _, role := range [][2]string{{v09OwnerName, "OWNER_NAME"}, {v09AccountID, "OWNED_ACCOUNT_ID"}, {ownerID, "OWNER_ID"}} {
		text = strings.ReplaceAll(text, role[0], role[1])
		if role[1] != "OWNER_NAME" {
			text = strings.ReplaceAll(text, role[0][:15], role[1])
		}
	}
	return text
}

// querySelectorAll and textContent on the native clone omit entire control,
// script and style subtrees. Original labels and anchors retain their raw text.
func v09WalkClone(n *html.Node, visit func(*html.Node)) {
	if n == nil {
		return
	}
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "input", "select", "textarea":
			return
		}
	}
	visit(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		v09WalkClone(child, visit)
	}
}

func v09Text(n *html.Node) string {
	var text strings.Builder
	v04Walk(n, func(child *html.Node) {
		if child.Type == html.TextNode {
			text.WriteString(child.Data)
		}
	})
	return text.String()
}

func v09ObserveDOM(t *testing.T, doc *html.Node, handler http.Handler, request *http.Request, status int, c v09Case, ownerID string) string {
	t.Helper()
	marker := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-v09-case") == c.ID })
	if marker == nil {
		// Reduce the actual local render error with the native semantic tokens.
		errorNode := v04Find(doc, func(n *html.Node) bool {
			classes := " " + v04Attr(n, "class") + " "
			return strings.Contains(classes, " errorMsg ") || strings.Contains(classes, " messageText ") || v04Attr(n, "id") == "errorTitle"
		})
		if errorNode == nil {
			overlay := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "class") == "overlay" })
			errorNode = v04Find(overlay, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "p" })
		}
		if errorNode == nil {
			return v09DOM(t, map[string]any{"localError": "rendered marker/error missing", "status": status})
		}
		text := strings.ToLower(v09Text(errorNode))
		return v09DOM(t, map[string]any{"renderError": true, "label": strings.Contains(text, "label"),
			"null": strings.Contains(text, "null"), "field": strings.Contains(text, "field"), "value": strings.Contains(text, "value")})
	}
	idmap := map[string]string{}
	nextID := 0
	v04Walk(marker, func(n *html.Node) {
		if id := v04Attr(n, "id"); id != "" {
			idmap[id] = fmt.Sprintf("owned-id-%d", nextID)
			nextID++
		}
	})
	stable := func(text string) string { return v09Stable(text, ownerID) }
	controls, labels, links, tags := []any{}, []any{}, []any{}, []any{}
	var firstLink *html.Node
	v04Walk(marker, func(n *html.Node) {
		if n.Type != html.ElementNode || n == marker {
			return
		}
		switch n.Data {
		case "input", "select", "textarea":
			kind := v09ControlType(n)
			if kind == "hidden" {
				return
			}
			value, options := v09ControlValue(n, kind)
			var tabindex any
			if v04HasAttr(n, "tabindex") {
				tabindex = v04Attr(n, "tabindex")
			}
			controls = append(controls, map[string]any{"tag": n.Data, "type": kind, "value": stable(value),
				"disabled": v04HasAttr(n, "disabled"), "required": v04HasAttr(n, "required"), "id": idmap[v04Attr(n, "id")],
				"class": v04Attr(n, "class"), "style": v04Attr(n, "style"), "tabindex": tabindex, "options": options})
		case "label":
			labels = append(labels, map[string]any{"text": v09Text(n), "for": idmap[v04Attr(n, "for")]})
		case "a":
			if v04HasAttr(n, "href") {
				if firstLink == nil {
					firstLink = n
				}
				href := v04Attr(n, "href")
				if parsed, err := url.Parse(href); err == nil {
					target := request.URL.ResolveReference(parsed)
					if target.Scheme == request.URL.Scheme && target.Host == request.URL.Host {
						href = target.EscapedPath()
						if href == "" {
							href = "/"
						}
					}
				}
				links = append(links, map[string]any{"text": stable(v09Text(n)), "href": stable(href)})
			}
		}
	})
	var text strings.Builder
	rich := false
	v09WalkClone(marker, func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		} else if n.Type == html.ElementNode && n != marker {
			tags = append(tags, n.Data)
			switch n.Data {
			case "b", "strong", "i", "em":
				rich = true
			}
		}
	})
	observed := map[string]any{"text": stable(text.String()), "labels": labels, "controls": controls, "links": links, "tags": tags, "rich": rich}
	if c.Observation.Navigate {
		observed["destination"] = v09Destination(t, handler, request, firstLink, c, ownerID)
	}
	return v09DOM(t, observed)
}

func v09ControlType(n *html.Node) string {
	if n.Data == "textarea" {
		return "textarea"
	}
	if n.Data == "select" {
		if v04HasAttr(n, "multiple") {
			return "select-multiple"
		}
		return "select-one"
	}
	kind := strings.ToLower(v04Attr(n, "type"))
	switch kind {
	case "button", "checkbox", "color", "date", "datetime-local", "email", "file", "hidden", "image", "month",
		"number", "password", "radio", "range", "reset", "search", "submit", "tel", "text", "time", "url", "week":
		return kind
	default:
		return "text"
	}
}

var v09OptionSpace = regexp.MustCompile(`[\t\n\f\r ]+`)
var v09NumberValue = regexp.MustCompile(`^-?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func v09ControlValue(n *html.Node, kind string) (string, []any) {
	options := []any{}
	if n.Data == "textarea" {
		return v09Text(n), options
	}
	if n.Data != "select" {
		value := v04Attr(n, "value")
		if (kind == "checkbox" || kind == "radio") && !v04HasAttr(n, "value") {
			value = "on"
		}
		// Read browser value properties, including HTML input sanitization.
		switch kind {
		case "number":
			if value != "" && !v09NumberValue.MatchString(value) {
				value = ""
			}
		case "date":
			if parsed, err := time.Parse("2006-01-02", value); err != nil || parsed.Format("2006-01-02") != value {
				value = ""
			}
		case "datetime-local":
			value = strings.ReplaceAll(value, " ", "T")
			parsed, err := time.Parse("2006-01-02T15:04:05.999999999", value)
			if err != nil {
				parsed, err = time.Parse("2006-01-02T15:04", value)
			}
			if err != nil {
				value = ""
			} else {
				value = parsed.Format("2006-01-02T15:04")
				if parsed.Second() != 0 || parsed.Nanosecond() != 0 {
					value = parsed.Format("2006-01-02T15:04:05.999999999")
				}
			}
		case "text", "search", "tel", "password", "email", "url":
			value = strings.ReplaceAll(strings.ReplaceAll(value, "\r", ""), "\n", "")
			if kind == "email" || kind == "url" {
				value = strings.Trim(value, "\t\n\f\r ")
			}
		}
		return value, options
	}
	nodes := []*html.Node{}
	selected := map[*html.Node]bool{}
	var lastSelected *html.Node
	v04Walk(n, func(option *html.Node) {
		if option.Type == html.ElementNode && option.Data == "option" {
			nodes = append(nodes, option)
			if v04HasAttr(option, "selected") {
				selected[option], lastSelected = true, option
			}
		}
	})
	if kind == "select-one" {
		selected = map[*html.Node]bool{}
		if lastSelected == nil && (v04Attr(n, "size") == "" || v04Attr(n, "size") == "0" || v04Attr(n, "size") == "1") {
			for _, option := range nodes {
				if !v04HasAttr(option, "disabled") && !(option.Parent.Data == "optgroup" && v04HasAttr(option.Parent, "disabled")) {
					lastSelected = option
					break
				}
			}
		}
		selected[lastSelected] = true
	}
	value, hasValue := "", false
	for _, option := range nodes {
		text := strings.Trim(v09OptionSpace.ReplaceAllString(v09Text(option), " "), " ")
		optionValue := v04Attr(option, "value")
		if !v04HasAttr(option, "value") {
			optionValue = text
		}
		options = append(options, map[string]any{"text": text, "value": optionValue, "selected": selected[option]})
		if selected[option] && !hasValue {
			value, hasValue = optionValue, true
		}
	}
	return value, options
}

func v09Destination(t *testing.T, handler http.Handler, request *http.Request, anchor *html.Node, c v09Case, ownerID string) map[string]any {
	t.Helper()
	if anchor == nil {
		return map[string]any{"linkPresent": false}
	}
	parsed, err := url.Parse(v04Attr(anchor, "href"))
	if err != nil {
		return map[string]any{"localError": "lookup URL invalid"}
	}
	target := request.URL.ResolveReference(parsed)
	if target.Scheme != request.URL.Scheme || target.Host != request.URL.Host {
		return map[string]any{"localError": "lookup destination is external"}
	}
	roles := map[string][2]string{"Account": {v09AccountID, v09AccountName}, "User": {ownerID, v09OwnerName}}
	role, ok := roles[c.Observation.TargetRole]
	if !ok {
		return map[string]any{"localError": "lookup destination role missing"}
	}
	expectedID, expectedName := role[0], role[1]
	current := target
	for redirects := 0; redirects <= 5; redirects++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, current.String(), nil))
		if response.Code >= 300 && response.Code < 400 && response.Header().Get("Location") != "" {
			location, err := url.Parse(response.Header().Get("Location"))
			if err != nil {
				return map[string]any{"localError": "lookup redirect URL invalid"}
			}
			current = current.ResolveReference(location)
			if current.Scheme != request.URL.Scheme || current.Host != request.URL.Host {
				return map[string]any{"localError": "lookup redirect is external"}
			}
			continue
		}
		doc, err := html.Parse(strings.NewReader(response.Body.String()))
		if err != nil {
			return map[string]any{"localError": err.Error()}
		}
		body := v04Find(doc, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "body" })
		return map[string]any{"status": response.Code, "targetMatches": strings.Contains(target.String(), expectedID[:15]),
			"destinationMatches": strings.Contains(current.String(), expectedID[:15]), "nameVisible": strings.Contains(v04Text(body), expectedName)}
	}
	return map[string]any{"localError": "lookup redirect limit exceeded"}
}

// Match the native TSV's Python sort_keys/ensure_ascii JSON framing without
// decoding or rewriting expected strings. Nil properties encode as raw null.
func v09DOM(t *testing.T, value any) string {
	t.Helper()
	encoded := v04Encode(t, value)
	var out strings.Builder
	out.WriteString("DOM|")
	for _, r := range encoded {
		if r < 127 {
			out.WriteRune(r)
			continue
		}
		for _, unit := range utf16.Encode([]rune{r}) {
			fmt.Fprintf(&out, "\\u%04x", unit)
		}
	}
	return out.String()
}
