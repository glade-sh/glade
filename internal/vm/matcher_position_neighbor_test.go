package vm

import (
	"reflect"
	"testing"
)

// These uncaptured searches retain the origin/main positions.
func TestMatcherPositionUncapturedNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name, pattern, input string
		want                 []int64
	}{
		{"asciiEmpty", "", "ab", []int64{0, 1, 2}},
		{"bmpEmpty", "", "aéb", []int64{0, 1, 2, 3}},
		{"lookahead", "(?=.)", "😀a", []int64{0, 2}},
		{"lookaheadAlternative", "(?=.)|$", "😀a", []int64{0, 2, 3}},
		{"consuming", ".", "😀a", []int64{0, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pattern, err := patternCompile([]Value{String(tc.pattern)})
			if err != nil {
				t.Fatal(err)
			}
			matcher, _, _, _, err := callPatternMember(pattern, "matcher", []Value{String(tc.input)})
			if err != nil {
				t.Fatal(err)
			}
			var starts []int64
			for {
				found, _, _, _, err := callMatcherMember(matcher, "find", nil)
				if err != nil {
					t.Fatal(err)
				}
				if !found.Bool {
					break
				}
				start, _, _, _, err := callMatcherMember(matcher, "start", nil)
				if err != nil {
					t.Fatal(err)
				}
				starts = append(starts, start.Int)
			}
			if !reflect.DeepEqual(starts, tc.want) {
				t.Fatalf("got %v, want %v", starts, tc.want)
			}
		})
	}
}

func TestMatcherRegionUncapturedNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name, input, pattern string
		start, end           int64
		wantError            bool
	}{
		{"ascii", "abc", ".", 1, 2, false},
		{"bmp", "aéb", ".", 1, 2, false},
		{"wholePair", "a😀b", ".", 1, 3, false},
		{"highHalf", "😀", ".", 0, 1, true},
		{"literalLowHalf", "😀", "x", 1, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pattern, err := patternCompile([]Value{String(tc.pattern)})
			if err != nil {
				t.Fatal(err)
			}
			matcher, _, _, _, err := callPatternMember(pattern, "matcher", []Value{String(tc.input)})
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, _, err = callMatcherMember(matcher, "region", []Value{Int(tc.start), Int(tc.end)})
			if tc.wantError {
				if err == nil {
					t.Fatal("region unexpectedly accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			found, _, _, _, err := callMatcherMember(matcher, "find", nil)
			if err != nil || !found.Bool {
				t.Fatalf("find = %v, err %v", found, err)
			}
			start, _, _, _, err := callMatcherMember(matcher, "start", nil)
			if err != nil || start.Int != tc.start {
				t.Fatalf("start = %v, err %v; want %d", start, err, tc.start)
			}
		})
	}
}
