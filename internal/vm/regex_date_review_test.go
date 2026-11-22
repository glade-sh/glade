package vm

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These expectations pin origin/main for shapes not present in the native rows.
func TestReviewRegexDateUncaptured(t *testing.T) {
	p, err := patternCompile([]Value{String("z?")})
	if err != nil {
		t.Fatal(err)
	}
	m, _, _, _, err := callPatternMember(p, "matcher", []Value{String("😀")})
	if err != nil {
		t.Fatal(err)
	}
	var starts []int64
	for {
		found, _, _, _, e := callMatcherMember(m, "find", nil)
		if e != nil {
			t.Fatal(e)
		}
		if !found.Bool {
			break
		}
		start, _, _, _, e := callMatcherMember(m, "start", nil)
		if e != nil {
			t.Fatal(e)
		}
		starts = append(starts, start.Int)
	}
	if !reflect.DeepEqual(starts, []int64{0, 2}) {
		t.Errorf("optional starts: %v", starts)
	}
	got, err := stringRegexReplace("String.replaceAll", "😀", []Value{String("z?"), String("-")}, true)
	if err != nil || got != "-😀-" {
		t.Errorf("optional replacement: %q, %v", got, err)
	}
	dot, err := patternCompile([]Value{String(".")})
	if err != nil {
		t.Fatal(err)
	}
	m, _, _, _, err = callPatternMember(dot, "matcher", []Value{String("a😀")})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err = callMatcherMember(m, "region", []Value{Int(2), Int(3)})
	if err == nil || err.Error() != "string index splits a surrogate pair" {
		t.Errorf("region error: %v", err)
	}
	for _, locale := range []string{"fr_FR", "de_DE"} {
		t.Run(locale, func(t *testing.T) {
			program, e := CompileAnonymousWithOptions("String observation = Datetime.newInstanceGmt(1500,2,28,12,0,0).format('EEEE|u','GMT');", CompileOptions{APIVersion: "66.0"})
			if e != nil {
				t.Fatal(e)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(locale), "TimeZoneSidKey": storage.StringValue("GMT")}})
			if _, e = machine.Execute(program); e != nil {
				t.Fatal(e)
			}
			if got := machine.Globals["observation"].Text; got != "Wednesday|3" {
				t.Errorf("got %q", got)
			}
		})
	}
}

func TestReviewRegexDateCacheEvictionAlias(t *testing.T) {
	var cache preparedMatcherCache
	p, err := patternCompile([]Value{String("")})
	if err != nil {
		t.Fatal(err)
	}
	m, _, _, _, err := callPatternMember(p, "matcher", []Value{String("😀")})
	if err != nil {
		t.Fatal(err)
	}
	alias, _, _, _, err := callMatcherMemberWithCache(m, "usePattern", []Value{p}, &cache)
	if err != nil || alias.Ref == m.Ref {
		t.Fatalf("alias identity: %v %v", alias, err)
	}
	found, _, _, _, err := callMatcherMemberWithCache(m, "find", nil, &cache)
	if err != nil || !found.Bool {
		t.Fatalf("first find %v %v", found, err)
	}
	matcherMidPairSubjectFor(&cache, m.Ref, "😀")
	for i := uint64(1000); i < 1100; i++ {
		matcherMidPairSubjectFor(&cache, i, "😀")
	}
	if len(cache.positions) > 8 {
		t.Errorf("cache retains %d identities", len(cache.positions))
	}
	found, _, _, _, err = callMatcherMemberWithCache(alias, "find", nil, &cache)
	if err != nil || !found.Bool {
		t.Fatalf("alias find %v %v", found, err)
	}
	start, _, _, _, err := callMatcherMemberWithCache(m, "start", nil, &cache)
	if err != nil || start.Int != 1 {
		t.Errorf("alias start %v %v", start, err)
	}
}

// API 66 has no rows for the new UTF-16 paths. These values are from origin/main.
func TestReviewRegexDateUncapturedAPI(t *testing.T) {
	source := `String positions = ''; Matcher m = Pattern.compile('').matcher('😀'); while (m.find()) { positions += m.start() + ','; }
 Integer splitSize = '😀'.split('').size();
 Integer replacedLength = '😀'.replaceAll('','-').length();
 Integer patternSplitSize = Pattern.compile('').split('😀').size();
 Integer matcherReplaceLength = Pattern.compile('').matcher('😀').replaceAll('-').length();`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "66.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"positions": "0,2,"} {
		if got := machine.Globals[name].Text; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	for name, want := range map[string]int64{"splitSize": 1, "replacedLength": 4, "patternSplitSize": 1, "matcherReplaceLength": 4} {
		if got := machine.Globals[name].Int; got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
	region, err := CompileAnonymousWithOptions("Pattern.compile('.').matcher('😀').region(1,2);", CompileOptions{APIVersion: "66.0"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(nil).Execute(region)
	if err == nil || !strings.Contains(err.Error(), "string index splits a surrogate pair") {
		t.Errorf("region error: %v", err)
	}

}

func TestReviewRegexDateStarFindNeighbor(t *testing.T) {
	p, err := patternCompile([]Value{String("x*")})
	if err != nil {
		t.Fatal(err)
	}
	m, _, _, _, err := callPatternMember(p, "matcher", []Value{String("😀")})
	if err != nil {
		t.Fatal(err)
	}
	var starts []int64
	for {
		found, _, _, _, e := callMatcherMember(m, "find", nil)
		if e != nil {
			t.Fatal(e)
		}
		if !found.Bool {
			break
		}
		start, _, _, _, e := callMatcherMember(m, "start", nil)
		if e != nil {
			t.Fatal(e)
		}
		starts = append(starts, start.Int)
	}
	if !reflect.DeepEqual(starts, []int64{0, 2}) {
		t.Errorf("star find = %v", starts)
	}
}

func TestReviewRegexDateSubjectNeighbors(t *testing.T) {
	for _, tc := range []struct {
		input       string
		starts      []int64
		replacement string
		split       []string
	}{
		{"😀a", []int64{0, 2, 3}, "-😀-a-", []string{"😀", "a"}},
		{"😀😀", []int64{0, 2, 4}, "-😀-😀-", []string{"😀", "😀"}},
		{"aa😀b", []int64{0, 1, 2, 4, 5}, "-a-a-😀-b-", []string{"a", "a", "😀", "b"}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			p, err := patternCompile([]Value{String("")})
			if err != nil {
				t.Fatal(err)
			}
			m, _, _, _, err := callPatternMember(p, "matcher", []Value{String(tc.input)})
			if err != nil {
				t.Fatal(err)
			}
			var starts []int64
			for {
				found, _, _, _, e := callMatcherMember(m, "find", nil)
				if e != nil {
					t.Fatal(e)
				}
				if !found.Bool {
					break
				}
				start, _, _, _, e := callMatcherMember(m, "start", nil)
				if e != nil {
					t.Fatal(e)
				}
				starts = append(starts, start.Int)
			}
			if !reflect.DeepEqual(starts, tc.starts) {
				t.Errorf("find = %v, want %v", starts, tc.starts)
			}
			got, err := stringRegexReplace("String.replaceAll", tc.input, []Value{String(""), String("-")}, true)
			if err != nil || got != tc.replacement {
				t.Errorf("replace = %q, %v; want %q", got, err, tc.replacement)
			}
			parts := splitStringCharacters(tc.input, 0)
			if !reflect.DeepEqual(parts, tc.split) {
				t.Errorf("split = %q, want %q", parts, tc.split)
			}
		})
	}
}

func TestReviewRegexDateUncapturedRoutes(t *testing.T) {
	got, err := stringRegexReplace("String.replaceAll", "a😀b", []Value{String("x*"), String("-")}, true)
	if err != nil || got != "-a-😀-b-" {
		t.Errorf("star replacement = %q, %v", got, err)
	}
	program, err := CompileAnonymousWithOptions("Integer count = Pattern.compile('').split('😀').size();", CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if got := machine.Globals["count"].Int; got != 1 {
		t.Errorf("pattern split = %d, want 1", got)
	}
}

// Empty replacement on a lone pair is not among the captured replacement rows.
func TestReviewRegexDateEmptyReplacementSinglePair(t *testing.T) {
	source := `String stringResult = '😀'.replaceAll('','-'); String matcherResult = Pattern.compile('').matcher('😀').replaceAll('-');`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"stringResult", "matcherResult"} {
		if got := machine.Globals[name].Text; got != "-😀-" {
			t.Errorf("%s = %q, want %q", name, got, "-😀-")
		}
	}
}

func TestReviewRegexDatePatternSwitchNeighbor(t *testing.T) {
	optional, err := patternCompile([]Value{String("z?")})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := patternCompile([]Value{String("")})
	if err != nil {
		t.Fatal(err)
	}
	matcher, _, _, _, err := callPatternMember(optional, "matcher", []Value{String("😀")})
	if err != nil {
		t.Fatal(err)
	}
	found, _, _, _, err := callMatcherMember(matcher, "find", nil)
	if err != nil || !found.Bool {
		t.Fatalf("first find %v, %v", found, err)
	}
	if _, _, _, _, err = callMatcherMember(matcher, "usePattern", []Value{empty}); err != nil {
		t.Fatal(err)
	}
	found, _, _, _, err = callMatcherMember(matcher, "find", nil)
	if err != nil || !found.Bool {
		t.Fatalf("second find %v, %v", found, err)
	}
	start, _, _, _, err := callMatcherMember(matcher, "start", nil)
	if err != nil || start.Int != 2 {
		t.Fatalf("start %v, %v; want 2", start, err)
	}
}

func TestReviewRegexDateVersionSwitchNeighbor(t *testing.T) {
	pattern, err := patternCompile([]Value{String("")})
	if err != nil {
		t.Fatal(err)
	}
	matcher, _, _, _, err := callPatternMember(pattern, "matcher", []Value{String("😀")})
	if err != nil {
		t.Fatal(err)
	}
	var cache preparedMatcherCache
	found, _, _, _, err := callMatcherMemberWithPolicy(matcher, "find", nil, &cache, true)
	if err != nil || !found.Bool {
		t.Fatalf("first find %v, %v", found, err)
	}
	found, _, _, _, err = callMatcherMemberWithPolicy(matcher, "find", nil, &cache, false)
	if err != nil || !found.Bool {
		t.Fatalf("second find %v, %v", found, err)
	}
	start, _, _, _, err := callMatcherMemberWithPolicy(matcher, "start", nil, &cache, false)
	if err != nil || start.Int != 2 {
		t.Fatalf("start %v, %v; want 2", start, err)
	}
}
