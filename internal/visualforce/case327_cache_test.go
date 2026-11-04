package visualforce

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestCase327ExpressionSyntaxCacheKeepsValuesLiveAndBounded(t *testing.T) {
	cache := &expressionSyntaxCache{}
	ctx := &ExpressionContext{Variables: map[string]vm.Value{"item": vm.String("one")}, syntax: cache}
	for _, value := range []string{"one", "two", "three"} {
		ctx.Variables["item"] = vm.String(value)
		got, err := EvaluateExpression("item", ctx)
		if err != nil || got != value {
			t.Fatalf("value %q: got %q, %v", value, got, err)
		}
	}
	for i := 0; i < 300; i++ {
		_, _ = cache.parse(fmt.Sprintf("%d", i))
	}
	if len(cache.entries) > 256 || cache.bytes > 64*1024 {
		t.Fatalf("cache exceeded bounds: %d entries, %d bytes", len(cache.entries), cache.bytes)
	}
	before := len(cache.entries)
	_, _ = cache.parse(strings.Repeat("x", 65*1024))
	if len(cache.entries) != before {
		t.Fatal("oversized expression was retained")
	}
}
