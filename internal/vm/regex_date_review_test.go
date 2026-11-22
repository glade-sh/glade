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
	alias := m
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
 Integer replacedLength = '😀'.replaceAll('','-').length();`
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
	for name, want := range map[string]int64{"splitSize": 1, "replacedLength": 4} {
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
