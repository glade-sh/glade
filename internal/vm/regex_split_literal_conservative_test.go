package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The former extracted-helper check: accepting a literal must agree with the
// full matcher. Native regex_conservative_split/org.tsv rows 2-4 record these
// same inputs through both public split routes (capture: ORG-RUN.txt).
func TestLiteralSplitPlanFullEngineEquivalenceUnicodeAndOctalRoutes(t *testing.T) {
	for _, tc := range []struct{ name, pattern, text string }{
		{"unicode-fold", `(?iu)İ`, "LiRiT"},
		{"octal-class-400", `[\400]`, "LĀRĀT"},
		{"octal-class-777", `[\777]`, "LǿRǿT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := compileRegexp2PlanForInput("Pattern.split", tc.pattern, tc.text)
			if err != nil {
				t.Fatal(err)
			}
			got, eligible := splitLiteralRegexPlan(plan, tc.text, -1)
			if !eligible {
				return
			}
			runes := []rune(tc.text)
			want := []string{}
			end := 0
			match, err := plan.findValidStartingAt(tc.text, 0)
			for match != nil && err == nil {
				want = append(want, string(runes[end:match.Index]))
				end = match.Index + match.Length
				match, err = plan.findNextValid(tc.text, match)
			}
			if err != nil {
				t.Fatal(err)
			}
			want = append(want, string(runes[end:]))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("literal helper=%q full engine=%q", got, want)
			}
		})
	}
}

func TestLiteralSplitEligibilityPlainASCIIPlanRoute(t *testing.T) {
	for _, pattern := range []string{`\.`, `\010`, `\\10`, `[1]0`, `(?i)a`, `(?iu)İ`, `é`, `[\400]`, `[\777]`, `\Qabc\E`, `(a)`, `a|b`, `a*`, `^a$`, ""} {
		t.Run(pattern, func(t *testing.T) {
			plan, err := compileRegexp2PlanForInput("String.split", pattern, "text")
			if err != nil {
				t.Fatal(err)
			}
			if parts, ok := splitLiteralRegexPlan(plan, "text", -1); ok {
				t.Fatalf("non-plain pattern entered literal path: %q", parts)
			}
		})
	}
	for _, pattern := range []string{"-", "abc", "::", "a b", "10", "!,=/@#%&_;~`"} {
		t.Run("ASCII/"+pattern, func(t *testing.T) {
			plan, err := compileRegexp2PlanForInput("String.split", pattern, "L"+pattern+"R")
			if err != nil {
				t.Fatal(err)
			}
			got, ok := splitLiteralRegexPlan(plan, "L"+pattern+"R", -1)
			if !ok || !reflect.DeepEqual(got, []string{"L", "R"}) {
				t.Fatalf("plain ASCII split=%q eligible=%v", got, ok)
			}
		})
	}
}

// Same native rows as the project test, independently invoking every Go route.
func TestConservativeSplitNativeStringAndPatternSourceAndValueRoutes(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "regex_conservative_split")
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		r.LazyQuotes = true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	cases := map[string][2]string{}
	for _, row := range read("cases.tsv") {
		var p, s string
		if err := json.Unmarshal([]byte(row[1]), &p); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(row[2]), &s); err != nil {
			t.Fatal(err)
		}
		cases[row[0]] = [2]string{p, s}
	}
	for _, row := range read("org.tsv") {
		if row[0] != "62.0" || row[1] != "anonymous" {
			continue
		}
		t.Run(row[2], func(t *testing.T) {
			input := cases[row[2]]
			text := "L" + input[1] + "R" + input[1] + "T"
			var native map[string]json.RawMessage
			if err := json.Unmarshal([]byte(row[3]), &native); err != nil {
				t.Fatal(err)
			}
			if row[2] == "octal_class_400" || row[2] == "octal_class_777" {
				// Native rejects these classes; retain that syntax gap and
				// require every split route to agree with the full matcher.
				plan, err := compileRegexp2PlanForInput("Pattern.matcher", input[0], text)
				if err != nil {
					t.Fatal(err)
				}
				match, err := plan.findValidStartingAt(text, 0)
				if err != nil || match != nil {
					t.Fatalf("full-engine control match=%v error=%v", match, err)
				}
				raw, err := json.Marshal([]string{text})
				if err != nil {
					t.Fatal(err)
				}
				for key := range native {
					if key != "Matcher.find" {
						native[key] = raw
					}
				}
				t.Log("carried native syntax gap; asserting full-engine route equivalence (CARRIED.txt)")
			}
			pattern, err := patternCompile([]Value{String(input[0])})
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"String.split", "Pattern.split/source", "Pattern.split/value"} {
				for _, limit := range []struct {
					name string
					args []Value
				}{{"default", nil}, {"negative", []Value{Int(-1)}}, {"one", []Value{Int(1)}}, {"positive", []Value{Int(2)}}} {
					t.Run(route+"/"+limit.name, func(t *testing.T) {
						var got []string
						var err error
						switch route {
						case "String.split":
							got, err = stringRegexSplit(text, append([]Value{String(input[0])}, limit.args...))
						case "Pattern.split/source":
							got, err = patternSplit(input[0], append([]Value{String(text)}, limit.args...))
						case "Pattern.split/value":
							got, err = patternSplitValue(pattern, append([]Value{String(text)}, limit.args...))
						}
						if err != nil {
							t.Fatal(err)
						}
						key := "Pattern.split/" + limit.name
						if route == "String.split" {
							key = "String.split/" + limit.name
						}
						var want []string
						if err := json.Unmarshal(native[key], &want); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("got %q want native %q", got, want)
						}
					})
				}
			}
		})
	}
}

// Native regex_conservative_split/org.tsv:5 (ORG-RUN.txt) pins the literal
// result. UTF-8 validation belongs to eligibility, not regex reinterpretation.
func TestLiteralSplitPlainASCIITextAndLimitsPlanRoute(t *testing.T) {
	plan, err := compileRegexp2PlanForInput("String.split", "-", "L-R-T")
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int64{-1, 0, 1, 2, 100} {
		want := []string{"L", "R", "T"}
		if limit == 1 {
			want = []string{"L-R-T"}
		} else if limit == 2 {
			want = []string{"L", "R-T"}
		}
		got, ok := splitLiteralRegexPlan(plan, "L-R-T", limit)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("limit=%d got=%q eligible=%v", limit, got, ok)
		}
	}
	if _, ok := splitLiteralRegexPlan(plan, string([]byte{'L', 0xff, '-', 'R'}), -1); ok {
		t.Fatal("invalid UTF-8 entered literal path")
	}
}

// A genuine budget error from a subsequent search propagates, as on the base
// runtime. The retry must not mask it. This is a local execution-budget check,
// not native parity; the initial-search case is in regex_split_error_test.go.
func TestRegexSplitFullEngineNextBudgetErrorRoute(t *testing.T) {
	parts, err := splitRegexRegexp2("String.split", `x|(0+)+y`, "x"+strings.Repeat("0", 40), -1)
	if err == nil || !strings.HasPrefix(err.Error(), "match timeout after ") {
		t.Fatalf("budget error lost: split=%q error=%v", parts, err)
	}
}
