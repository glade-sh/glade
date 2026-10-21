package repoguard

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestPrivateNamespaceContextPrefilterCost(t *testing.T) {
	// Ordinary identifiers mention the public context words without opening a
	// namespace value. The fast path should avoid a regexp scan over this text.
	text := strings.Repeat("public String namespaceValue; public void forNameHelper() {}\n", 4096)
	expression := regexp.MustCompile(`(?i)(namespace"\s*:\s*"|Namespace:\s*"|Namespace\s*=\s*")|(SetCurrentNamespace\(")|(Type\.forName\(')`)
	patterns := []privatePackagePattern{newPrivateNamePattern("stand-in", privateNameNamespace, "ks")}
	scan := privateNameScan{text: text, patterns: patterns, prefilter: true, matched: make([]bool, 1)}
	// Compare best-of-five batches to reduce scheduling noise. The relative
	// budget also follows the host's CPU speed instead of fixing a duration.
	measure := func(run func()) time.Duration {
		best := time.Duration(1<<63 - 1)
		for range 5 {
			start := time.Now()
			for range 3 {
				run()
			}
			best = min(best, time.Since(start))
		}
		return best
	}
	plain := measure(func() {
		if matches := expression.FindAllStringSubmatchIndex(text, -1); len(matches) != 0 {
			t.Fatalf("unexpected context openings: %v", matches)
		}
	})
	filtered := measure(func() { scan.scanNamespaceContexts([]int{0}) })
	if scan.matched[0] {
		t.Fatal("ordinary identifiers matched a namespace value")
	}
	t.Logf("namespace scan: filtered=%s plain-regexp=%s", filtered, plain)
	if filtered*4 > plain*3 {
		t.Fatalf("filtered namespace scan = %s, want at least 25%% below plain regexp %s", filtered, plain)
	}
}
