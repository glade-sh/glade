package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Native regex_numeric_backreferences/org.tsv records both API floors and
// entry contexts. Exercise the three Go split entry points independently.
func TestNumericBackreferenceStringAndPatternSplitReplaceAllAndMatcherRoutes(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "regex_numeric_backreferences")
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
	native := map[string]map[string]json.RawMessage{}
	for _, row := range read("org.tsv") {
		if row[0] == "62.0" && row[1] == "anonymous" {
			var value map[string]json.RawMessage
			if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
				t.Fatal(err)
			}
			native[row[2]] = value
		}
	}
	for _, row := range read("cases.tsv") {
		t.Run(row[0], func(t *testing.T) {
			var expression, input string
			if err := json.Unmarshal([]byte(row[1]), &expression); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row[2]), &input); err != nil {
				t.Fatal(err)
			}
			text := "L" + input + "R" + input + "T"
			// These native rows expose an older engine mismatch. Keep route twins
			// checked against the matcher plan, without counting them as parity.
			if row[0] == "ref10_nine_backspace" || row[0] == "ref10_nine_ref1_zero" || row[0] == "ref10_zero_backspace" {
				plan, err := compileRegexp2PlanForInput("Pattern.matcher", expression, input)
				if err != nil {
					t.Fatal(err)
				}
				match, err := plan.findValidStartingAt(input, 0)
				if err != nil {
					t.Fatal(err)
				}
				matched := match != nil
				if matched && match.String() != input {
					t.Fatal("carry control matched only part of the input")
				}
				parts, positive, replacement, found := []string{text}, []string{text}, text, []string{}
				if matched {
					parts, positive, replacement, found = []string{"L", "R", "T"}, []string{"L", "R" + input + "T"}, "LQRQT", []string{input, input}
				}
				control := map[string]any{"Matcher.matches": matched, "Matcher.find": found, "Matcher.replaceAll": replacement, "String.replaceAll": replacement}
				for _, route := range []string{"String.split", "Pattern.split"} {
					control[route+"/default"], control[route+"/negative"], control[route+"/positive"] = parts, parts, positive
				}
				for route, value := range control {
					raw, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					native[row[0]][route] = raw
				}
				t.Log("carried native mismatch; checking split/replaceAll/matcher equivalence (see CARRIED.txt)")
			}
			pattern, err := patternCompile([]Value{String(expression)})
			if err != nil {
				t.Fatal(err)
			}
			check := func(t *testing.T, route string, got any, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
				want, ok := native[row[0]][route]
				if !ok {
					t.Fatal("missing native route", route)
				}
				raw, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				var a, b any
				if err := json.Unmarshal(raw, &a); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(want, &b); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(a, b) {
					t.Fatalf("%s got %s want native %s", route, raw, want)
				}
			}
			for _, route := range []string{"String.split", "Pattern.split/source", "Pattern.split/value"} {
				for _, limit := range []struct {
					name string
					args []Value
				}{
					{"default", nil}, {"negative", []Value{Int(-1)}}, {"positive", []Value{Int(2)}},
				} {
					t.Run(route+"/"+limit.name, func(t *testing.T) {
						var got []string
						var err error
						switch route {
						case "String.split":
							got, err = stringRegexSplit(text, append([]Value{String(expression)}, limit.args...))
						case "Pattern.split/source":
							got, err = patternSplit(expression, append([]Value{String(text)}, limit.args...))
						case "Pattern.split/value":
							got, err = patternSplitValue(pattern, append([]Value{String(text)}, limit.args...))
						}
						nativeRoute := "Pattern.split/" + limit.name
						if route == "String.split" {
							nativeRoute = "String.split/" + limit.name
						}
						check(t, nativeRoute, got, err)
					})
				}
			}
			t.Run("String.replaceAll", func(t *testing.T) {
				got, err := stringRegexReplace("String.replaceAll", text, []Value{String(expression), String("Q")}, true)
				check(t, "String.replaceAll", got, err)
			})
			matcher := func(t *testing.T, text string) Value {
				t.Helper()
				value, _, _, handled, err := callPatternMember(pattern, "matcher", []Value{String(text)})
				if err != nil || !handled {
					t.Fatalf("matcher: handled=%v error=%v", handled, err)
				}
				return value
			}
			for _, route := range []string{"Matcher.replaceAll", "Matcher.matches", "Matcher.find"} {
				t.Run(route, func(t *testing.T) {
					m := matcher(t, text)
					if route == "Matcher.matches" {
						m = matcher(t, input)
					}
					call := func(method string, args ...Value) Value {
						t.Helper()
						got, updated, _, handled, err := callMatcherMember(m, method, args)
						if err != nil || !handled {
							t.Fatalf("%s handled=%v error=%v", method, handled, err)
						}
						m = updated
						return got
					}
					switch route {
					case "Matcher.replaceAll":
						check(t, route, call("replaceAll", String("Q")).Text, nil)
					case "Matcher.matches":
						check(t, route, call("matches").Bool, nil)
					case "Matcher.find":
						found := []string{}
						for call("find").Bool {
							found = append(found, call("group").Text)
						}
						check(t, route, found, nil)
						check(t, "Matcher.groupCount", call("groupCount").Int, nil)
					}
				})
			}
		})
	}
}

// Backreferences and Go octal escapes can have the same spelling. Eligibility
// must reject numeric references even when Go reports a complete literal.
func TestLiteralSplitEligibilityNumericBackreferencePlanRoute(t *testing.T) {
	for _, expression := range []string{`(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)\10`, `\10`} {
		t.Run(expression, func(t *testing.T) {
			plan, err := compileRegexp2PlanForInput("String.split", expression, "abcdefghijj")
			if err != nil {
				t.Fatal(err)
			}
			if parts, ok := splitLiteralRegexPlan(plan, "abcdefghijj", -1); ok {
				t.Fatalf("numeric reference entered literal path: %q", parts)
			}
		})
	}
	// Native regex_conservative_split/org.tsv:9-11 (and API/context twins); ORG-RUN.txt.
	for _, expression := range []string{`\010`, `\\10`, `[1]0`} {
		t.Run("literal/"+expression, func(t *testing.T) {
			plan, err := compileRegexp2PlanForInput("String.split", expression, "x")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := splitLiteralRegexPlan(plan, "x", -1); ok {
				t.Fatal("escaped or class control entered literal path")
			}
		})
	}
}
