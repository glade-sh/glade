package vm

import "testing"

func TestJavaClassAlgebraLowering(t *testing.T) {
	tests := []struct {
		name   string
		source string
		input  string
		want   bool
	}{
		{name: "chain accepts overlap", source: `[a-z&&[m-p]&&[n-o]]+`, input: "no", want: true},
		{name: "chain rejects outside final overlap", source: `[a-z&&[m-p]&&[n-o]]+`, input: "mp", want: false},
		{name: "positive negative accepts", source: `[a-z&&[m-p]&&[^o]]+`, input: "mnp", want: true},
		{name: "positive negative rejects", source: `[a-z&&[m-p]&&[^o]]+`, input: "o", want: false},
		{name: "nested accepts", source: `[a-z&&[m-p&&[^o]]]+`, input: "mnp", want: true},
		{name: "nested rejects", source: `[a-z&&[m-p&&[^o]]]+`, input: "o", want: false},
		{name: "nested union accepts first range", source: `[a-c[x-z]]+`, input: "abc", want: true},
		{name: "nested union accepts nested range", source: `[a-c[x-z]]+`, input: "xyz", want: true},
		{name: "nested union rejects outside ranges", source: `[a-c[x-z]]+`, input: "m", want: false},
		{name: "property intersection", source: `[\p{L}&&[\p{Ll}]&&[^x]]+`, input: "abcé", want: true},
		{name: "property intersection rejects", source: `[\p{L}&&[\p{Ll}]&&[^x]]+`, input: "x", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, re, err := compileRegexp2Pattern("Pattern.compile", tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			match, err := re.FindStringMatchStartingAt(tc.input, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := match != nil && match.Index == 0 && match.Length == len([]rune(tc.input))
			if got != tc.want {
				t.Fatalf("match = %v, want %v", got, tc.want)
			}
		})
	}
}
