package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// userSelfUpdateIDPattern matches the 15 and 18 character User ids that the
// native messages carry; ids differ per org.
var userSelfUpdateIDPattern = regexp.MustCompile(`005[A-Za-z0-9]{12}([A-Za-z0-9]{3})?`)

// userSelfUpdateObservation reduces a test or anonymous message to the
// comparable observation: the CAP payload as parsed JSON when present,
// otherwise the exception text, with User ids replaced.
func userSelfUpdateObservation(t *testing.T, message string) any {
	t.Helper()
	message = userSelfUpdateIDPattern.ReplaceAllString(message, "<id>")
	at := strings.Index(message, "CAP|")
	if at < 0 {
		return message
	}
	parts := strings.SplitN(message[at:], "|", 3)
	if len(parts) != 3 {
		t.Fatalf("invalid CAP transport %q", message)
	}
	var payload any
	if err := json.Unmarshal([]byte(parts[2]), &payload); err != nil {
		t.Fatalf("invalid CAP payload %q: %v", message, err)
	}
	return map[string]any{"probe": parts[1], "payload": payload}
}

// TestUserSelfUpdateNativeConformance replays the laneC1 capture of a runAs
// Standard User updating its own User record (C1SelfUpdate, API 62.0 and
// 67.0) and the anonymous administrator control.
func TestUserSelfUpdateNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "user_self_update")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 65 || strings.Join(rows[0], ",") != "source_api,route,method,outcome,message" {
		t.Fatalf("incomplete native matrix: %d rows", len(rows))
	}
	anonymous, err := os.ReadFile(filepath.Join(dir, "anon", "admin_self.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyPermsetMutingProject(t, filepath.Join(dir, "project"), api)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"C1SelfUpdate"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 {
				t.Fatalf("expected one suite: %+v", run.Summary())
			}
			observed := map[string]string{}
			for _, result := range run.Suites[0].Cases {
				if result.Problem == nil {
					observed[result.MethodName] = ""
					continue
				}
				// Glade names built-in exception types without the System
				// namespace the native test result prints.
				name := result.Problem.Type
				if !strings.Contains(name, ".") {
					name = "System." + name
				}
				observed[result.MethodName] = name + ": " + result.Problem.Message
			}
			compared := 0
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					var got string
					if row[1] == "anonymous" {
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: func() *storage.OrgState {
							org := orgFromIndex(index)
							org.APIVersion = api
							return &org
						}()})
						// The native route logs the payload; the replay asserts it.
						source := strings.Replace(string(anonymous), "System.debug(LoggingLevel.ERROR, ", "System.assert(false, ", 1)
						_, err := runAnonymousObservation(t, runner, index, source, api)
						got = anonymousAssertionMessage(t, err)
					} else {
						var ok bool
						if got, ok = observed[row[2]]; !ok {
							t.Fatalf("no Glade result for %s", row[2])
						}
					}
					want := userSelfUpdateObservation(t, row[4])
					if have := userSelfUpdateObservation(t, got); !reflect.DeepEqual(have, want) {
						wantJSON, _ := json.Marshal(want)
						haveJSON, _ := json.Marshal(have)
						t.Fatalf("native=%s\nGlade=%s", wantJSON, haveJSON)
					}
				})
				compared++
			}
			if compared != 32 {
				t.Fatalf("compared %d native rows at %s, want 32", compared, api)
			}
		})
	}
}
