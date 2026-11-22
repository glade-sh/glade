package vm

import "testing"

// The Matcher route shares replacement with the String observations in
// regex_java_divergence/org.tsv:758-759,1694-1695,2505-2506,3491-3492.
func TestMatcherReplacementUTF16Route(t *testing.T) {
	input := "a😀b"
	want, err := stringRegexReplace("String.replaceAll", input, []Value{String(""), String("-")}, true)
	if err != nil || apexStringLength(want) != 9 {
		t.Fatalf("String replacement = %q, err %v", want, err)
	}
	pattern, err := patternCompile([]Value{String("")})
	if err != nil {
		t.Fatal(err)
	}
	matcher, _, _, _, err := callPatternMember(pattern, "matcher", []Value{String(input)})
	if err != nil {
		t.Fatal(err)
	}
	var cache preparedMatcherCache
	got, _, _, _, err := callMatcherMemberWithCache(matcher, "replaceAll", []Value{String("-")}, &cache)
	if err != nil || got.Text != want {
		t.Fatalf("Matcher replacement = %q, err %v; want %q", got.Text, err, want)
	}
}
