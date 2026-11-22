package vm

import (
	"strings"
	"testing"
)

// The same uncaptured optional-pattern outcome is required on both trees.
func BenchmarkMatcherShortLivedLargeSubject(b *testing.B) {
	input := strings.Repeat("é", 4096) + "😀"
	pattern, err := patternCompile([]Value{String("z?")})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cache preparedMatcherCache
		for n := 0; n < 32; n++ {
			matcher, _, _, _, err := callPatternMember(pattern, "matcher", []Value{String(input)})
			if err != nil {
				b.Fatal(err)
			}
			for j := 0; j < 2; j++ {
				got, _, _, _, err := callMatcherMemberWithCache(matcher, "find", nil, &cache)
				if err != nil || !got.Bool {
					b.Fatalf("find %v %v", got, err)
				}
			}
			start, _, _, _, err := callMatcherMemberWithCache(matcher, "start", nil, &cache)
			if err != nil || start.Int != 1 {
				b.Fatalf("start %v %v", start, err)
			}
		}
	}
}
