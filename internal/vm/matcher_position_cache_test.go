package vm

import (
	"strings"
	"testing"
)

func TestMatcherPositionCacheInvalidation(t *testing.T) {
	var cache preparedMatcherCache
	first := matcherPositionsFor(&cache, 1, "a😀b")
	if again := matcherPositionsFor(&cache, 1, "a😀b"); again != first {
		t.Fatal("same subject did not reuse positions")
	}
	view := matcherMidPairSubjectFor(&cache, 1, "a😀b")
	if again := matcherMidPairSubjectFor(&cache, 1, "a😀b"); again != view {
		t.Fatal("same subject did not reuse search view")
	}
	view.splitAtRune(1)
	if other := matcherPositionsFor(&cache, 1, "aé😀b"); other == first || other.length == first.length {
		t.Fatal("changed subject reused positions")
	}
	if other := matcherMidPairSubjectFor(&cache, 1, "aé😀b"); other == view || other.split != nil {
		t.Fatal("changed subject reused search view")
	}
	if other := matcherPositionsFor(&cache, 2, "a😀b"); other == first {
		t.Fatal("different matcher reused positions")
	}
	cache.forget(2)
	if cache.positions[2] != nil {
		t.Fatal("forget retained subject")
	}
	matcherPositionsFor(&cache, 3, "😀")
	matcherPositionsFor(&cache, 4, "😀")
	matcherPositionsFor(&cache, 5, "😀")
	if cache.positions[3] == nil {
		t.Fatal("additional matchers displaced a subject")
	}
	cache.reset()
	if len(cache.positions) != 0 {
		t.Fatal("reset retained subjects")
	}
	matcherPositionsFor(&cache, 6, "😀")
	cache.release()
	if len(cache.positions) != 0 {
		t.Fatal("release retained subject")
	}
	long := strings.Repeat("a", 64<<10) + "😀"
	large := matcherPositionsFor(&cache, 7, long)
	if matcherPositionsFor(&cache, 7, long) != large {
		t.Fatal("large subject did not reuse positions")
	}
}

func TestMatcherPositionCacheMutators(t *testing.T) {
	for _, method := range []string{"reset", "resetInput", "region", "usePattern", "useAnchoringBounds", "useTransparentBounds"} {
		t.Run(method, func(t *testing.T) {
			pattern, err := patternCompile([]Value{String(".")})
			if err != nil {
				t.Fatal(err)
			}
			matcher, _, _, _, err := callPatternMember(pattern, "matcher", []Value{String("a😀b")})
			if err != nil {
				t.Fatal(err)
			}
			var cache preparedMatcherCache
			matcherMidPairSubjectFor(&cache, matcher.Ref, "a😀b")
			operation := method
			var args []Value
			switch method {
			case "resetInput":
				operation, args = "reset", []Value{String("😀c")}
			case "region":
				args = []Value{Int(0), Int(1)}
			case "usePattern":
				args = []Value{pattern}
			case "useAnchoringBounds", "useTransparentBounds":
				args = []Value{Bool(false)}
			}
			_, _, _, _, err = callMatcherMemberWithCache(matcher, operation, args, &cache)
			if err != nil {
				t.Fatal(err)
			}
			if cache.positions[matcher.Ref] != nil {
				t.Fatalf("%s retained the subject cache", method)
			}
		})
	}
}
