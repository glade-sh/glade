package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The anonymous rows in testdata/conformance/string_equality_locale_change_c14
// (captured 2026-11-07 at API 62.0 and 67.0): an en_US admin updates its own
// LocaleSidKey to tr_TR with DML. UserInfo.getLocale() and a re-query report
// tr_TR at once, but String == keeps the transaction-start locale ('I' == 'i'
// stays true) until the next transaction, where it follows tr_TR; restoring
// en_US in its own transaction restores it. The anon scripts run in the
// captured order against one org.
func TestExecNativeStringEqualityLocaleChangeRows(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "string_equality_locale_change_c14")
	rows := readLocaleChangeRows(t, filepath.Join(dir, "org.tsv"))
	sameTxn := inlineLocaleProbeSteps(t, readFixture(t, filepath.Join(dir, "anon", "n6_same_txn.apex")))
	eval := readFixture(t, filepath.Join(dir, "anon", "n6_eval.apex"))
	restore := strings.ReplaceAll(readFixture(t, filepath.Join(dir, "anon", "set_locale.apex")), "__LOCALE__", "en_US")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "User")
			admin := storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{
				"Id": storage.IDValue("005000000000001"), "LocaleSidKey": storage.StringValue("en_US"), "LanguageLocaleKey": storage.StringValue("en_US"),
				"TimeZoneSidKey": storage.StringValue("America/Los_Angeles")}}
			users := org.Objects["User"]
			users.Records[admin.ID] = admin
			org.Objects["User"] = users
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(admin)
			observed := map[string]map[string]any{}
			machine.SetDebugOutputSink(func(event DebugEvent) {
				rest, ok := strings.CutPrefix(event.Message, "CAP|n6.p1.")
				if !ok {
					return
				}
				step, body, _ := strings.Cut(rest, "|")
				var got map[string]any
				if err := json.Unmarshal([]byte(body), &got); err != nil {
					t.Fatal(err)
				}
				observed[step] = got
			})
			run := func(source string) {
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			}
			run(sameTxn)
			run(eval)
			observed["new_txn_tr"] = observed["new_txn"]
			run(restore)
			run(eval)
			observed["new_txn_en"] = observed["new_txn"]
			checked := 0
			for _, row := range rows {
				if row[0] != api || row[1] != "anonymous" {
					continue
				}
				step, expression, observation := row[3], row[4], row[5]
				want := localeChangeObservation(observation)
				got, ok := observed[step][expression]
				if !ok {
					t.Fatalf("step %s emitted no %s", step, expression)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s/%s: native=%s; Glade=%v", step, expression, observation, got)
				}
				checked++
			}
			if checked != 35 {
				t.Fatalf("capture shape changed: %d anonymous rows", checked)
			}
		})
	}
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func readLocaleChangeRows(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 311 {
		t.Fatalf("invalid native observations: %v (%d rows)", err, len(rows))
	}
	for _, row := range rows[1:] {
		if len(row) != 6 {
			t.Fatalf("invalid row %q", row)
		}
	}
	return rows[1:]
}

// inlineLocaleProbeSteps expands the captured script's local step(name) method
// at each call site: the anonymous compiler here does not accept method
// declarations inside an anonymous block. Each call becomes the method body in
// its own block with name bound to the call's literal.
func inlineLocaleProbeSteps(t *testing.T, source string) string {
	t.Helper()
	lines := strings.Split(source, "\n")
	var body, out []string
	inBody := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Map<String,Object> step(String name) {"):
			inBody = true
		case inBody && trimmed == "}":
			inBody = false
		case inBody:
			if trimmed != "return m;" {
				body = append(body, line)
			}
		case strings.HasPrefix(trimmed, "step('") && strings.HasSuffix(trimmed, "');"):
			name := strings.TrimSuffix(strings.TrimPrefix(trimmed, "step("), ");")
			out = append(out, "{")
			for _, bodyLine := range body {
				out = append(out, strings.ReplaceAll(bodyLine, "' + name + '", "' + "+name+" + '"))
			}
			out = append(out, "}")
		default:
			out = append(out, line)
		}
	}
	if len(body) == 0 {
		t.Fatal("probe has no step method")
	}
	return strings.Join(out, "\n")
}

// localeChangeObservation decodes an org.tsv observation: JSON for booleans and
// unit lists, plain text for locale names and messages.
func localeChangeObservation(text string) any {
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return text
	}
	return value
}

// The same captured rule (String == keeps the transaction-start locale after a
// DML update of the running user's LocaleSidKey) when no comparison ran before
// the update: the locale is fixed at transaction start, not at the first ==.
func TestStringEqualityLocaleFixedBeforeFirstComparison(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "User")
	admin := storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{
		"Id": storage.IDValue("005000000000001"), "LocaleSidKey": storage.StringValue("en_US"), "LanguageLocaleKey": storage.StringValue("en_US"),
		"TimeZoneSidKey": storage.StringValue("America/Los_Angeles")}}
	users := org.Objects["User"]
	users.Records[admin.ID] = admin
	org.Objects["User"] = users
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(admin)
	run := func(source string) {
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			t.Fatal(err)
		}
	}
	run(`update new User(Id = UserInfo.getUserId(), LocaleSidKey = 'tr_TR');
String upperI = String.fromCharArray(new List<Integer>{73});
String lowerI = String.fromCharArray(new List<Integer>{105});
System.assertEquals('tr_TR', UserInfo.getLocale());
System.assertEquals(true, upperI == lowerI);`)
	run(`String upperI = String.fromCharArray(new List<Integer>{73});
String lowerI = String.fromCharArray(new List<Integer>{105});
System.assertEquals(false, upperI == lowerI);`)
}
