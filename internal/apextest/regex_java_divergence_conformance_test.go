package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// regex_java_divergence holds native Salesforce observations of java.util.regex
// behavior that Glade's translator layer must follow: \p property names, the
// exception type and text of malformed patterns, replacement strings, line
// terminators, case folding, octal and surrogate escapes, quoting, lookbehind
// limits, stack overflow, and Pattern/Matcher text.
//
// cases.tsv (id, group, snippet) is the Apex each row runs. org.tsv (api,
// context, id, value) is the captured observation, an anonymous run and an
// @IsTest run per row at API 62.0 and 67.0, repeated in two scratch orgs. A row
// that Salesforce answered differently on the two repeats keeps both values.
// carried.tsv lists the rows that still differ: each pins today's Glade value,
// so the test fails the moment a carried row changes, and states why.
const regexJavaDivergenceFixture = "testdata/regex_java_divergence"

const regexJavaDivergenceWrap = "try { %s } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\\n','\\\\n'); }"

type regexDivergenceRow struct {
	api, context, id string
	values           []string
}

type regexDivergenceCarry struct {
	value, reason string
}

// readRegexDivergenceTSV splits lines on tabs without CSV quoting: every value
// is a JSON string, so a leading quote must reach the decoder intact.
func readRegexDivergenceTSV(t *testing.T, name string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(regexJavaDivergenceFixture, name))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for n, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if n == 0 || line == "" {
			continue
		}
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows
}

func loadRegexDivergence(t *testing.T) (snippets map[string]string, rows []regexDivergenceRow, carried map[string]regexDivergenceCarry) {
	t.Helper()
	snippets = map[string]string{}
	for _, row := range readRegexDivergenceTSV(t, "cases.tsv") {
		var snippet string
		if err := json.Unmarshal([]byte(row[2]), &snippet); err != nil {
			t.Fatal(err)
		}
		snippets[row[0]] = snippet
	}
	byKey := map[string]*regexDivergenceRow{}
	var order []string
	for _, row := range readRegexDivergenceTSV(t, "org.tsv") {
		var value string
		if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
			t.Fatal(err)
		}
		key := row[0] + "|" + row[1] + "|" + row[2]
		if byKey[key] == nil {
			byKey[key] = &regexDivergenceRow{api: row[0], context: row[1], id: row[2]}
			order = append(order, key)
		}
		byKey[key].values = append(byKey[key].values, value)
	}
	for _, key := range order {
		rows = append(rows, *byKey[key])
	}
	carried = map[string]regexDivergenceCarry{}
	for _, row := range readRegexDivergenceTSV(t, "carried.tsv") {
		var value string
		if err := json.Unmarshal([]byte(row[2]), &value); err != nil {
			t.Fatal(err)
		}
		carried[row[0]+"|"+row[1]] = regexDivergenceCarry{value: value, reason: row[3]}
	}
	return snippets, rows, carried
}

// checkRegexDivergenceRow compares one observation with its native values.
func checkRegexDivergenceRow(t *testing.T, row regexDivergenceRow, observed string, carried map[string]regexDivergenceCarry, citation string) {
	t.Helper()
	carry, isCarried := carried[row.id+"|"+row.context]
	if !isCarried {
		carry, isCarried = carried[row.id+"|*"]
	}
	matches := false
	for _, want := range row.values {
		matches = matches || observed == want
	}
	if isCarried {
		if matches {
			t.Fatalf("%s now matches Salesforce; remove it from carried.tsv", row.id)
		}
		if observed != carry.value {
			t.Fatalf("carried row %s changed\n got  %q\n held %q\n native %q\n (%s)", row.id, observed, carry.value, row.values, carry.reason)
		}
		t.Logf("CARRIED %s: %s", row.id, carry.reason)
		return
	}
	if !matches {
		t.Fatalf("%s\n got    %q\n native %q", citation, observed, row.values)
	}
}

func TestRegexJavaDivergenceNativeAnonymousRows(t *testing.T) {
	snippets, rows, carried := loadRegexDivergence(t)
	if len(snippets) < 900 {
		t.Fatalf("cases: %d", len(snippets))
	}
	count := 0
	for _, row := range rows {
		if row.context != "anonymous" {
			continue
		}
		snippet, ok := snippets[row.id]
		if !ok {
			t.Fatalf("org row %s has no case", row.id)
		}
		count++
		t.Run(row.api+"/"+row.id, func(t *testing.T) {
			if carry, held := carried[row.id+"|anonymous"]; held && strings.HasPrefix(carry.reason, "SLOW") && row.api != "67.0" {
				t.Skip("slow carried row runs at 67.0 only")
			}
			body := "String observation;\n" + fmt.Sprintf(regexJavaDivergenceWrap, snippet)
			observed := runRegexDivergenceAnonymous(t, body, row.api)
			checkRegexDivergenceRow(t, row, observed, carried, fmt.Sprintf("NATIVE %s %s/org.tsv anonymous %s", row.id, regexJavaDivergenceFixture, row.api))
		})
	}
	if count < 1500 {
		t.Fatalf("anonymous rows: %d", count)
	}
}

// runRegexDivergenceAnonymous runs one anonymous block and returns what it
// stored in observation. A compile or runtime failure reads as the native
// COMPILE or FAIL text.
func runRegexDivergenceAnonymous(t *testing.T, body, api string) string {
	t.Helper()
	program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
	if err != nil {
		return "COMPILE " + err.Error()
	}
	machine := vm.New(nil)
	if _, err := machine.Execute(program); err != nil {
		return "FAIL " + strings.TrimPrefix(err.Error(), "System.")
	}
	// The native debug log drops NUL characters, so the anonymous rows hold the
	// text without them; the @IsTest rows keep them.
	return strings.ReplaceAll(machine.Globals["observation"].Text, "\x00", "")
}

func TestRegexJavaDivergenceNativeIsTestRows(t *testing.T) {
	snippets, rows, carried := loadRegexDivergence(t)
	byAPI := map[string][]regexDivergenceRow{}
	for _, row := range rows {
		if row.context == "isTest" {
			if _, ok := snippets[row.id]; !ok {
				t.Fatalf("org row %s has no case", row.id)
			}
			byAPI[row.api] = append(byAPI[row.api], row)
		}
	}
	apis := make([]string, 0, len(byAPI))
	for api := range byAPI {
		apis = append(apis, api)
	}
	sort.Strings(apis)
	if strings.Join(apis, ",") != "62.0,67.0" {
		t.Fatalf("apis: %v", apis)
	}
	for _, api := range apis {
		api := api
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(regexJavaDivergenceFixture, "project"))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			suffix := api[:2]
			var classes []string
			entries, err := os.ReadDir(filepath.Join(root, "force-app/main/default/classes"))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasSuffix(name, ".cls") && strings.HasSuffix(strings.TrimSuffix(name, ".cls"), suffix) {
					classes = append(classes, strings.TrimSuffix(name, ".cls"))
				}
			}
			sort.Strings(classes)
			if len(classes) < 30 {
				t.Fatalf("test classes at %s: %d", api, len(classes))
			}
			index := loadTestIndex(t, root)
			result := Run(index, Options{SelectedClasses: classes, NoDiskCache: true, Parallelism: 1})
			observations := map[string]string{}
			for _, suite := range result.Suites {
				for _, c := range suite.Cases {
					if c.Problem == nil {
						t.Fatalf("%s.%s passed: the row assertion cannot pass", suite.Name, c.MethodName)
					}
					_, value, ok := strings.Cut(c.Problem.Message, ", Actual: ")
					if !ok {
						value = "TESTOUTCOME " + c.Problem.Message
					}
					observations[strings.ToLower(c.MethodName)] = value
				}
			}
			for _, row := range byAPI[api] {
				row := row
				t.Run(row.id, func(t *testing.T) {
					observed, ok := observations[strings.ToLower(row.id)]
					if !ok {
						t.Fatalf("no observation for %s", row.id)
					}
					checkRegexDivergenceRow(t, row, observed, carried, fmt.Sprintf("NATIVE %s %s/org.tsv isTest %s", row.id, regexJavaDivergenceFixture, api))
				})
			}
		})
	}
}
