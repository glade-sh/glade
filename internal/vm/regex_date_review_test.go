package vm

import (
	"reflect"
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
