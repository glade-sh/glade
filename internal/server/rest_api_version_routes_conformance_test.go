package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// restRouteAnswer is one native GET answer: the status, and either the error,
// the versions list, the top-level keys, or picked nested values.
type restRouteAnswer struct {
	Status    int            `json:"status"`
	ErrorCode string         `json:"errorCode,omitempty"`
	Message   string         `json:"message,omitempty"`
	Versions  []string       `json:"versions,omitempty"`
	Keys      []string       `json:"keys,omitempty"`
	Picked    map[string]any `json:"picked,omitempty"`
}

type restRouteCase struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	// KeyMode "exact" compares the local top-level keys with the native keys;
	// "subset" requires every local key to be native. GladeOnlyKeys and
	// NativeOnlyKeys are explicit, version-independent shape differences.
	KeyMode        string   `json:"keyMode,omitempty"`
	GladeOnlyKeys  []string `json:"gladeOnlyKeys,omitempty"`
	NativeOnlyKeys []string `json:"nativeOnlyKeys,omitempty"`
	KeyReason      string   `json:"keyReason,omitempty"`
	// Owner tracks closing a recorded shape difference or local rejection.
	Owner string `json:"owner,omitempty"`
	// Note explains a native answer that no local behavior depends on yet.
	Note string `json:"note,omitempty"`
	// LocalUnsupported is the explicit local rejection inside the window for a
	// route the local server does not model.
	LocalUnsupported *struct {
		Status    int    `json:"status"`
		ErrorCode string `json:"errorCode"`
		Reason    string `json:"reason"`
	} `json:"localUnsupported,omitempty"`
	Native map[string]restRouteAnswer `json:"native"`
}

type restRouteTable struct {
	APIVersions []string `json:"apiVersions"`
	// Floor cases (ids ending in _floor) repeat a route at the oldest version
	// Salesforce serves and the version below it.
	FloorAPIVersions []string `json:"floorAPIVersions"`
	// Native key lists keep at most this many sorted keys.
	NativeKeyLimit int `json:"nativeKeyLimit"`
	// Versions Salesforce serves that are outside the local endpoint window.
	WindowExclusions struct {
		Versions []string `json:"versions"`
		Reason   string   `json:"reason"`
	} `json:"windowExclusions"`
	// Window exclusions keep the native NOT_FOUND status and errorCode but name
	// the excluded version, because Salesforce serves it. Versions Salesforce
	// does not serve get the native body exactly.
	WindowExclusionMessage string          `json:"windowExclusionMessage"`
	Cases                  []restRouteCase `json:"cases"`
}

// TestRESTAPIVersionRouteConformance replays native GET answers for every
// version-gated /services/data route at the endpoint window boundaries:
// below and between window entries, the window entries, and the first
// version above the window. GLADE_REST_ROUTES_REPORT=1 also logs every local
// answer; drift fails either way.
func TestRESTAPIVersionRouteConformance(t *testing.T) {
	report := os.Getenv("GLADE_REST_ROUTES_REPORT") == "1"
	data, err := os.ReadFile("testdata/rest_api_version_routes.json")
	if err != nil {
		t.Fatal(err)
	}
	var table restRouteTable
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&table); err != nil {
		t.Fatal(err)
	}
	window := storage.SupportedRESTAPIVersions
	newest := window[len(window)-1]
	above := fmt.Sprintf("%d.0", restRouteMajor(t, newest)+1)
	if !slices.Contains(table.APIVersions, newest) || !slices.Contains(table.APIVersions, above) {
		t.Fatalf("route table must cover the newest window version %s and %s above it: %v", newest, above, table.APIVersions)
	}
	if len(table.Cases) != 21 || len(table.FloorAPIVersions) != 2 || table.NativeKeyLimit <= 0 || table.WindowExclusionMessage == "" || table.WindowExclusions.Reason == "" {
		t.Fatalf("incomplete route table: %d cases", len(table.Cases))
	}
	// Versions Salesforce serves come from the native versions list.
	var nativeServed []string
	routePaths := map[string]bool{}
	for _, c := range table.Cases {
		if !strings.Contains(c.Path, "%s") {
			nativeServed = c.Native["68.0"].Versions
		} else if !strings.HasSuffix(c.ID, "_floor") {
			routePaths[c.Path] = true
		}
	}
	if len(nativeServed) == 0 || !slices.Contains(nativeServed, table.FloorAPIVersions[1]) || slices.Contains(nativeServed, table.FloorAPIVersions[0]) {
		t.Fatalf("floor versions %v must straddle the oldest native version: %v", table.FloorAPIVersions, nativeServed)
	}
	org := testOrg()
	handler := New(&org)
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	seen := map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || !strings.HasPrefix(c.Path, "/services/data/") || len(c.Native) == 0 {
			t.Fatalf("invalid or duplicate route case %q", c.ID)
		}
		seen[c.ID] = true
		t.Run(c.ID, func(t *testing.T) {
			switch {
			case !strings.Contains(c.Path, "%s"):
				assertRESTVersionsList(t, c, get(c.Path), window, above)
			case strings.Contains(c.Path, "/ui-api/"):
				assertUIAPIURLsEchoRequestVersion(t, c)
			case strings.HasSuffix(c.ID, "_floor"):
				if !routePaths[c.Path] {
					t.Fatalf("floor case %s has no matching route case", c.ID)
				}
				assertRESTRouteAcrossVersions(t, table, c, table.FloorAPIVersions, nativeServed, get, window, report)
			default:
				assertRESTRouteAcrossVersions(t, table, c, table.APIVersions, nativeServed, get, window, report)
			}
		})
	}
}

func assertRESTVersionsList(t *testing.T, c restRouteCase, rec *httptest.ResponseRecorder, window []string, above string) {
	t.Helper()
	native, ok := c.Native["68.0"]
	if !ok || native.Status != http.StatusOK || len(native.Versions) == 0 {
		t.Fatalf("versions list row needs a native 200 answer: %#v", c.Native)
	}
	if rec.Code != native.Status {
		t.Fatalf("status = %d, native %d; body=%s", rec.Code, native.Status, rec.Body.String())
	}
	var entries []apiVersionEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, entry.Version)
		if !slices.Contains(native.Versions, entry.Version) {
			t.Fatalf("local version %s is not served natively: %v", entry.Version, native.Versions)
		}
	}
	if !slices.Equal(got, window) {
		t.Fatalf("local versions = %v, want the endpoint window %v", got, window)
	}
	nativeNewest := ""
	for _, version := range native.Versions {
		if nativeNewest == "" || restRouteMajor(t, version) > restRouteMajor(t, nativeNewest) {
			nativeNewest = version
		}
	}
	if nativeNewest != window[len(window)-1] || slices.Contains(native.Versions, above) {
		t.Fatalf("native newest version %s, local newest %s; %s must be absent natively", nativeNewest, window[len(window)-1], above)
	}
}

// Direct UI API calls echo the request version in body URLs. Local list wire
// reads have no request version and keep the org release's version instead
// (orgReleaseListRESTAPIVersion); the local server serves no REST ui-api route.
func assertUIAPIURLsEchoRequestVersion(t *testing.T, c restRouteCase) {
	t.Helper()
	urls := 0
	for api, native := range c.Native {
		if native.Status != http.StatusOK {
			if native.Status != http.StatusNotFound || native.ErrorCode != "NOT_FOUND" {
				t.Fatalf("API %s: unexpected native answer %#v", api, native)
			}
			continue
		}
		for path, value := range native.Picked {
			text, ok := value.(string)
			if !ok || !strings.HasPrefix(text, "/services/data/") {
				continue
			}
			urls++
			if !strings.HasPrefix(text, "/services/data/v"+api+"/") {
				t.Fatalf("API %s: native %s = %s does not echo the request version", api, path, text)
			}
		}
	}
	if urls == 0 {
		t.Fatal("no native list resource URL in the row")
	}
	if !slices.Contains(storage.SupportedRESTAPIVersions, orgReleaseListRESTAPIVersion) {
		t.Fatalf("org release list version %s is outside the endpoint window", orgReleaseListRESTAPIVersion)
	}
}

func assertRESTRouteAcrossVersions(t *testing.T, table restRouteTable, c restRouteCase, versions, nativeServed []string, get func(string) *httptest.ResponseRecorder, window []string, report bool) {
	t.Helper()
	if len(c.Native) != len(versions) {
		t.Fatalf("native answers %d, want one per version %v", len(c.Native), versions)
	}
	bodies := map[string][]byte{}
	for _, api := range versions {
		native, ok := c.Native[api]
		if !ok {
			t.Fatalf("missing native answer at API %s", api)
		}
		path := fmt.Sprintf(c.Path, api)
		rec := get(path)
		if report {
			t.Logf("API %s %s: status=%d keys=%v body=%.200s", api, path, rec.Code, restRouteKeys(rec.Body.Bytes()), rec.Body.String())
		}
		switch {
		case slices.Contains(window, api):
			if native.Status != http.StatusOK {
				t.Fatalf("API %s: window version is not served natively: %#v", api, native)
			}
			bodies[api] = rec.Body.Bytes()
			if unsupported := c.LocalUnsupported; unsupported != nil {
				if unsupported.Reason == "" || c.Owner == "" {
					t.Fatal("local unsupported route needs a reason and an owner")
				}
				if rec.Code == native.Status {
					t.Logf("carry now matches: %s localUnsupported (%s) at API %s", c.ID, c.Owner, api)
					continue
				}
				assertSalesforceError(t, rec, unsupported.Status, unsupported.ErrorCode, "")
				continue
			}
			if rec.Code != native.Status {
				t.Fatalf("API %s: status = %d, native %d; body=%s", api, rec.Code, native.Status, rec.Body.String())
			}
			assertRESTRouteKeys(t, table, c, api, native, restRouteKeys(rec.Body.Bytes()))
		case !slices.Contains(nativeServed, api):
			// Salesforce does not serve this version: the local answer is native.
			if native.Status < 400 || native.ErrorCode == "" || native.Message == "" {
				t.Fatalf("API %s: incomplete native rejection %#v", api, native)
			}
			assertSalesforceError(t, rec, native.Status, native.ErrorCode, "")
			assertRESTRejectionMessage(t, rec, api, native.Message)
		default:
			// Salesforce serves this version; the local endpoint window does not.
			// A native resource-level 404 at such a version needs a note.
			if !slices.Contains(table.WindowExclusions.Versions, api) ||
				(native.Status != http.StatusOK && (native.Status != http.StatusNotFound || native.ErrorCode != "NOT_FOUND" || c.Note == "")) {
				t.Fatalf("API %s: native %d outside the window needs a window exclusion (and a note if not 200)", api, native.Status)
			}
			assertSalesforceError(t, rec, http.StatusNotFound, "NOT_FOUND", "")
			assertRESTRejectionMessage(t, rec, api, fmt.Sprintf(table.WindowExclusionMessage, "v"+api))
		}
	}
	// The newest window version answers exactly like the previous release,
	// apart from the version in resource URLs.
	previous := fmt.Sprintf("%d.0", restRouteMajor(t, window[len(window)-1])-1)
	newest := window[len(window)-1]
	if before, ok := bodies[previous]; ok {
		want := bytes.ReplaceAll(before, []byte("/v"+previous+"/"), []byte("/v"+newest+"/"))
		if !bytes.Equal(bodies[newest], want) {
			t.Fatalf("API %s body differs from API %s beyond the URL version:\n%s\n%s", newest, previous, bodies[newest], want)
		}
		if !slices.Equal(c.Native[previous].Keys, c.Native[newest].Keys) {
			t.Fatalf("native keys changed between %s and %s", previous, newest)
		}
	}
}

func assertRESTRouteKeys(t *testing.T, table restRouteTable, c restRouteCase, api string, native restRouteAnswer, got []string) {
	t.Helper()
	if got == nil {
		t.Fatalf("API %s: local body is not a JSON object", api)
	}
	differs := c.KeyMode == "subset" || len(c.GladeOnlyKeys) > 0 || len(c.NativeOnlyKeys) > 0
	if (c.KeyMode != "exact" && c.KeyMode != "subset") || (differs && (c.KeyReason == "" || c.Owner == "")) {
		t.Fatalf("route case needs keyMode exact or subset, and a reason and an owner for key differences")
	}
	nativeKeys := native.Keys
	// Only keys up to the last recorded native key are comparable.
	truncated := len(nativeKeys) >= table.NativeKeyLimit
	comparable := func(key string) bool { return !truncated || key <= nativeKeys[len(nativeKeys)-1] }
	var problems []string
	// Carried differences are logged when they close, never asserted to persist.
	for _, key := range c.NativeOnlyKeys {
		if !slices.Contains(nativeKeys, key) {
			problems = append(problems, "native-only key "+key+" is not native")
		}
		if slices.Contains(got, key) {
			t.Logf("carry now matches: %s %s (%s) at API %s", c.ID, key, c.Owner, api)
		}
	}
	for _, key := range c.GladeOnlyKeys {
		if comparable(key) && slices.Contains(nativeKeys, key) {
			problems = append(problems, "local-only key "+key+" is native")
		}
		if !slices.Contains(got, key) {
			t.Logf("carry now matches: %s %s (%s) at API %s", c.ID, key, c.Owner, api)
		}
	}
	for _, key := range got {
		if comparable(key) && !slices.Contains(nativeKeys, key) && !slices.Contains(c.GladeOnlyKeys, key) {
			problems = append(problems, "local key "+key+" is not native")
		}
	}
	if c.KeyMode == "exact" {
		for _, key := range nativeKeys {
			if !slices.Contains(got, key) && !slices.Contains(c.NativeOnlyKeys, key) {
				problems = append(problems, "native key "+key+" is missing locally")
			}
		}
	}
	if len(problems) > 0 {
		// Errorf keeps checking the remaining versions so one run reports all drift.
		t.Errorf("API %s: %s; local keys %v, native %v", api, strings.Join(problems, "; "), got, nativeKeys)
	}
}

func assertRESTRejectionMessage(t *testing.T, rec *httptest.ResponseRecorder, api, want string) {
	t.Helper()
	var errors []salesforceError
	if err := json.Unmarshal(rec.Body.Bytes(), &errors); err != nil || len(errors) != 1 {
		t.Fatalf("API %s: rejection body = %s", api, rec.Body.String())
	}
	if errors[0].Message != want {
		t.Fatalf("API %s: rejection message = %q, want %q", api, errors[0].Message, want)
	}
}

func restRouteKeys(body []byte) []string {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return nil
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func restRouteMajor(t *testing.T, version string) int {
	t.Helper()
	var major int
	if _, err := fmt.Sscanf(version, "%d.0", &major); err != nil {
		t.Fatalf("invalid API version %q", version)
	}
	return major
}
