package vm

import (
	"reflect"
	"testing"
)

// literal_splits/org.tsv captures these limit and trailing-empty results at
// API 62/67, on both anonymous and named String.split/Pattern.split routes.
func TestLiteralSplitCapturedLimitStringAndPatternRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, pattern, text string
		limit               int64
		want                []string
	}{
		{"dot-limited", `\.`, "0.0.0.0", 2, []string{"0", "0.0.0"}},
		{"dot-zero", `\.`, "0.0.0.0", 0, []string{"0", "0", "0", "0"}},
		{"quoted-trim", `\Q.\E`, "..0..", 0, []string{"", "", "0"}},
		{"captured-keep", `(\.)`, "a.b.", -1, []string{"a", "b", ""}},
		{"class-one", `[.]`, "0.0.0.0", 1, []string{"0.0.0.0"}},
		{"multi-keep", "::", "::north::::", 5, []string{"", "north", "", ""}},
	} {
		for _, route := range []string{"String.split", "Pattern.split/source", "Pattern.split/value"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				var parts []string
				var err error
				switch route {
				case "String.split":
					parts, err = stringRegexSplit(tc.text, []Value{String(tc.pattern), Int(tc.limit)})
				case "Pattern.split/source":
					parts, err = patternSplit(tc.pattern, []Value{String(tc.text), Int(tc.limit)})
				case "Pattern.split/value":
					var pattern Value
					pattern, err = patternCompile([]Value{String(tc.pattern)})
					if err == nil {
						parts, err = patternSplitValue(pattern, []Value{String(tc.text), Int(tc.limit)})
					}
				}
				if err != nil || !reflect.DeepEqual(parts, tc.want) {
					t.Fatalf("parts=%q want=%q error=%v", parts, tc.want, err)
				}
			})
		}
	}
}
