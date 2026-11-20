package vm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestPreparedMatcherEquivalence(t *testing.T) {
	cases := []struct {
		pattern, input string
		operations     []string
	}{
		{"(a)?(b)", "x😀abb😀b", []string{"find", "find", "find", "find", "reset", "find"}},
		{"(?=a)|$", "a😀aa😀a", []string{"find", "find", "find", "find", "find", "find"}},
		{"(?<=😀)(a)|b$", "😀a😀ab", []string{"find", "find", "find", "find"}},
		{"a", "aa\xed\xa0\x80aaaaa", []string{"find", "find", "find"}},
		{"a", "aaaaaaaa", []string{"find", "find", "region", "find", "resetInput", "find", "pattern", "find", "bounds", "find", "anchoring", "find0", "replaceAll", "replaceFirst"}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q/%q", tc.pattern, tc.input), func(t *testing.T) {
			var matchers [2]Value
			var cache preparedMatcherCache
			for i := range matchers {
				pattern, err := patternCompile([]Value{String(tc.pattern)})
				if err != nil {
					t.Fatal(err)
				}
				m, _, _, _, err := callPatternMember(pattern, "matcher", []Value{String(tc.input)})
				if err != nil {
					t.Fatal(err)
				}
				matchers[i] = m
			}
			for _, op := range tc.operations {
				var args []Value
				method := op
				switch op {
				case "region":
					args = []Value{Int(1), Int(5)}
				case "resetInput":
					method = "reset"
					args = []Value{String("bbbbbaaaa")}
				case "pattern":
					method = "usePattern"
					p, err := patternCompile([]Value{String("b+")})
					if err != nil {
						t.Fatal(err)
					}
					args = []Value{p}
				case "find0":
					method = "find"
					args = []Value{Int(0)}
				case "replaceAll", "replaceFirst":
					args = []Value{String("x")}
				case "anchoring":
					method = "useAnchoringBounds"
					args = []Value{Bool(false)}
				case "bounds":
					method = "useTransparentBounds"
					args = []Value{Bool(true)}
				}
				var got [2]Value
				var errors [2]string
				for i := range matchers {
					var err error
					var c *preparedMatcherCache
					if i == 1 {
						c = &cache
					}
					got[i], matchers[i], _, _, err = callMatcherMemberWithCache(matchers[i], method, args, c)
					if err != nil {
						errors[i] = err.Error()
					}
				}
				got[0].Ref, got[1].Ref = 0, 0
				left, _ := json.Marshal(matchers[0].Fields)
				right, _ := json.Marshal(matchers[1].Fields)
				gotLeft, _ := json.Marshal(got[0])
				gotRight, _ := json.Marshal(got[1])
				if string(gotLeft) != string(gotRight) || errors[0] != errors[1] || string(left) != string(right) {
					t.Fatalf("%s: results %v/%v errors %q/%q fields %v/%v", op, got[0], got[1], errors[0], errors[1], matchers[0].Fields, matchers[1].Fields)
				}
			}
		})
	}
}

func TestPreparedMatcherCacheInvalidation(t *testing.T) {
	var cache preparedMatcherCache
	first := cache.use(1, "a😀aaaaaa")
	if first == nil || first.prepared != nil {
		t.Fatal("first search should defer decoding")
	}
	first = cache.use(1, "a😀aaaaaa")
	if first.prepared == nil {
		t.Fatal("second search did not prepare input")
	}
	if got := first.prepared.unitAtRune(2); got != 3 {
		t.Fatalf("UTF-16 offset: %d", got)
	}
	cache.forget(1)
	if cache.lookup(1, "a😀aaaaaa") != nil {
		t.Fatal("forgotten entry retained")
	}
	cache.use(1, "aaaaaaaa")
	cache.use(1, "aaaaaaaa")
	if cache.lookup(1, "bbbbbbbb") != nil {
		t.Fatal("changed input reused")
	}
	cache.use(2, "bbbbbbbb")
	cache.use(3, "cccccccc")
	if cache.lookup(1, "aaaaaaaa") != nil {
		t.Fatal("old entry not evicted")
	}
	cache.release()
	if cache.lookup(2, "bbbbbbbb") != nil || cache.lookup(3, "cccccccc") != nil {
		t.Fatal("transaction retained inputs")
	}
	cache.use(4, "a\xed\xa0\x80aaaaa")
	if entry := cache.use(4, "a\xed\xa0\x80aaaaa"); entry.prepared != nil {
		t.Fatal("invalid UTF-8 was prepared")
	}
}

func TestPreparedMatcherTimeoutRetry(t *testing.T) {
	defer func() { regexp2SearchFault = nil }()
	var outputs [2][]int
	var calls [2]int
	for mode := range 2 {
		p, err := patternCompile([]Value{String("a")})
		if err != nil {
			t.Fatal(err)
		}
		m, _, _, _, err := callPatternMember(p, "matcher", []Value{String("aaaaaaaa")})
		if err != nil {
			t.Fatal(err)
		}
		var cache preparedMatcherCache
		var c *preparedMatcherCache
		if mode == 1 {
			c = &cache
		}
		for i := 0; i < 2; i++ {
			_, m, _, _, err = callMatcherMemberWithCache(m, "find", nil, c)
			if err != nil {
				t.Fatal(err)
			}
		}
		regexp2SearchFault = func(string) error {
			calls[mode]++
			if calls[mode] == 1 {
				return fmt.Errorf("match timeout after simulated delay")
			}
			return nil
		}
		result, updated, _, _, err := callMatcherMemberWithCache(m, "find", nil, c)
		regexp2SearchFault = nil
		if err != nil || !result.Bool {
			t.Fatalf("mode %d: %v %v", mode, result, err)
		}
		groups := updated.Fields["groups"]
		for _, group := range groups.List {
			outputs[mode] = append(outputs[mode], int(group.Int))
		}
	}
	if !reflect.DeepEqual(outputs[0], outputs[1]) || calls[0] != calls[1] || calls[0] < 2 {
		t.Fatalf("outcomes %v/%v calls %v", outputs[0], outputs[1], calls)
	}
}

func TestPreparedMatcherConcurrent(t *testing.T) {
	const workers = 4
	errors := make(chan error, workers)
	for range workers {
		go func() {
			p, err := patternCompile([]Value{String("(a)")})
			if err != nil {
				errors <- err
				return
			}
			m, _, _, _, err := callPatternMember(p, "matcher", []Value{String("aaaaaaaa")})
			if err != nil {
				errors <- err
				return
			}
			var cache preparedMatcherCache
			for i := 0; i < 8; i++ {
				result, updated, _, _, err := callMatcherMemberWithCache(m, "find", nil, &cache)
				if err != nil || !result.Bool {
					errors <- fmt.Errorf("find %d: %v", i, err)
					return
				}
				m = updated
			}
			errors <- nil
		}()
	}
	for range workers {
		if err := <-errors; err != nil {
			t.Error(err)
		}
	}
}
