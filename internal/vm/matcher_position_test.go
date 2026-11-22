package vm

import (
	"reflect"
	"testing"
)

func TestMatcherPositions(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		bytes       []int
		inside      []bool
	}{
		{"ascii", "abc", []int{0, 1, 2, 3}, []bool{false, false, false, false}},
		{"bmp", "aéz", []int{0, 1, 3, 4}, []bool{false, false, false, false}},
		{"supplementary", "a😀b", []int{0, 1, 1, 5, 6}, []bool{false, false, true, false, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newMatcherPositions(tc.input)
			for unit, wantByte := range tc.bytes {
				at, inside, ok := p.byteAtUnit(unit)
				if !ok || at != wantByte || inside != tc.inside[unit] {
					t.Fatalf("unit %d: got %d, %v, %v; want %d, %v, true", unit, at, inside, ok, wantByte, tc.inside[unit])
				}
				if !inside {
					got, ok := p.unitAtByte(at)
					if !ok || got != unit {
						t.Fatalf("byte %d: got %d, %v; want %d, true", at, got, ok, unit)
					}
				}
			}
			if _, _, ok := p.byteAtUnit(len(tc.bytes)); ok {
				t.Fatal("accepted unit beyond end")
			}
		})
	}
}

func TestMatcherMidPairView(t *testing.T) {
	subject := newMatcherMidPairSubject("😀ab𐐷c😃")
	original := append([]rune(nil), subject.runes...)
	for _, at := range []int{0, 3, 5, 3, 0, 0} {
		r := subject.runes[at] - 0x10000
		want := append([]rune{}, subject.runes[:at]...)
		want = append(want, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
		want = append(want, subject.runes[at+1:]...)
		if got := subject.splitAtRune(at); !reflect.DeepEqual(got, want) {
			t.Fatalf("split at %d: got %U, want %U", at, got, want)
		}
		if !reflect.DeepEqual(subject.runes, original) {
			t.Fatal("split view mutated the subject")
		}
	}
}

func TestMatcherMidPairPlanGate(t *testing.T) {
	subject := newMatcherMidPairSubject("😀")
	for _, tc := range []struct {
		pattern string
		want    bool
	}{
		{"", true}, {"x*", true}, {"(?=.)", false},
		{"(?=.)|$", false}, {"^", false}, {"x*", true},
	} {
		plan, err := compileRegexp2PlanForInput("Pattern.compile", tc.pattern, "😀")
		if err != nil {
			t.Fatal(err)
		}
		got, err := subject.allows(plan)
		if err != nil || got != tc.want {
			t.Fatalf("%q: got %v, err %v; want %v", tc.pattern, got, err, tc.want)
		}
	}
}

// The group and cursor state remain internally consistent across a split.
func TestMatcherSingleLowSurrogateRegion(t *testing.T) {
	pattern, err := patternCompile([]Value{String(".")})
	if err != nil {
		t.Fatal(err)
	}
	matcher, _, _, _, err := callPatternMember(pattern, "matcher", []Value{String("😀")})
	if err != nil {
		t.Fatal(err)
	}
	var cache preparedMatcherCache
	for _, call := range []struct {
		method string
		args   []Value
	}{
		{"region", []Value{Int(1), Int(2)}},
		{"find", nil},
	} {
		got, _, _, _, err := callMatcherMemberWithCache(matcher, call.method, call.args, &cache)
		if err != nil || (call.method == "find" && !got.Bool) {
			t.Fatalf("%s = %v, err %v", call.method, got, err)
		}
	}
	for _, tc := range []struct {
		method string
		args   []Value
		want   int64
	}{
		{"regionStart", nil, 1}, {"regionEnd", nil, 2},
		{"start", nil, 1}, {"end", nil, 2},
	} {
		got, _, _, _, err := callMatcherMemberWithCache(matcher, tc.method, tc.args, &cache)
		if err != nil || got.Int != tc.want {
			t.Fatalf("%s = %v, err %v; want %d", tc.method, got, err, tc.want)
		}
	}
	group, _, _, _, err := callMatcherMemberWithCache(matcher, "group", nil, &cache)
	if err != nil || group.Text != apexStringFromUTF16Units([]uint16{0xde00}) {
		t.Fatalf("group = %q, err %v", group.Text, err)
	}
	found, _, _, _, err := callMatcherMemberWithCache(matcher, "find", nil, &cache)
	if err != nil || found.Bool {
		t.Fatalf("second find = %v, err %v", found, err)
	}
}
