package vm

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The rows in testdata/conformance/stdlib_string_equality_c6267 (captured
// 2026-11-07 at API 62.0 and 67.0 under en_US and tr_TR users): String ==
// compares the texts upper-cased with the full Unicode mapping in the running
// user's locale, so 'Straße' == 'STRASSE' and 'ﬁ' == 'FI' hold everywhere,
// while under Turkish dotting 'I' == 'i' and 'ﬁ' == 'fi' are false. Each row
// re-runs the probe expression from probe.apex under both user routes.
func TestExecNativeStringEqualityRows(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "stdlib_string_equality_c6267")
	exprs := stringEqualityProbeExpressions(t, filepath.Join(dir, "probe.apex"))
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 137 {
		t.Fatalf("invalid native observations: %v (%d rows)", err, len(rows))
	}
	for _, row := range rows[1:] {
		api, locale, name, observation := row[0], row[1], row[2], row[3]
		expr, ok := exprs[name]
		if !ok {
			t.Fatalf("row %s has no probe expression", name)
		}
		source := fmt.Sprintf(`System.assertEquals('%s', String.valueOf(%s));`, observation, expr)
		userLocale := map[string]string{"en": "en_US", "tr": "tr_TR"}[locale]
		for _, route := range []string{"anonymous", "test_context"} {
			t.Run(api+"/"+locale+"/"+name+"/"+route, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "test_context" {
					machine.EnableTestContext()
				}
				machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(userLocale), "LanguageLocaleKey": storage.StringValue("en_US"), "TimeZoneSidKey": storage.StringValue("GMT")}})
				if route == "test_context" {
					machine.testContext.CurrentUser = machine.executionUser
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("native %s: %v", observation, err)
				}
			})
		}
	}
}

// stringEqualityProbeExpressions maps each probe name to the Apex expression
// probe.apex appended under it: lines of the form o += '<name>=' + <expr> + ';';
func stringEqualityProbeExpressions(t *testing.T, path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	exprs := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		rest, ok := strings.CutPrefix(line, "o += '")
		if !ok {
			continue
		}
		name, rest, ok := strings.Cut(rest, "=' + ")
		if !ok {
			continue
		}
		expr, ok := strings.CutSuffix(rest, " + ';';")
		if !ok {
			t.Fatalf("unexpected probe line %q", line)
		}
		exprs[name] = expr
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return exprs
}

// The locale decision is cached per user; switching the current user, the
// test-context user and the org must each refresh it.
func TestExecStringEqualityLocaleCacheFollowsUser(t *testing.T) {
	user := func(id, locale string) storage.Record {
		return storage.Record{ID: storage.ID(id), Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(locale)}}
	}
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(user("005000000000001", "en_US"))
	if !machine.apexStringEqualFold("I", "i") || !machine.apexStringEqualFold("Straße", "STRASSE") {
		t.Fatal("en_US: expected I == i and Straße == STRASSE")
	}
	machine.SetCurrentUser(user("005000000000002", "tr_TR"))
	if machine.apexStringEqualFold("I", "i") || !machine.apexStringEqualFold("ı", "I") || !machine.apexStringEqualFold("İ", "i") || machine.apexStringEqualFold("ﬁ", "fi") || !machine.apexStringEqualFold("ﬁ", "FI") {
		t.Fatal("tr_TR: expected I != i, ı == I, İ == i, ﬁ != fi, ﬁ == FI")
	}
	// Same Id, changed locale: SetCurrentUser resets the cache.
	machine.SetCurrentUser(user("005000000000002", "en_US"))
	if !machine.apexStringEqualFold("I", "i") {
		t.Fatal("after SetCurrentUser back to en_US: expected I == i")
	}
	machine.EnableTestContext()
	machine.testContext.CurrentUser = vmValueFromRecord(user("005000000000003", "az_AZ"))
	if machine.apexStringEqualFold("I", "i") {
		t.Fatal("az_AZ test user: expected I != i")
	}
	machine.testContext.CurrentUser = vmValueFromRecord(user("005000000000004", "de_DE"))
	if !machine.apexStringEqualFold("I", "i") || machine.apexStringEqualFold("ß", "ẞ") || machine.apexStringEqualFold("Ünïcode Straat", "ünïcode STRAAT 7") {
		t.Fatal("de_DE test user: expected I == i, ß != ẞ, and unequal texts without expansion runes to differ")
	}
}
