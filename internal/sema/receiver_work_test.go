package sema

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestOversizedReceiverFallbackWork(t *testing.T) {
	t.Run("small constructor", func(t *testing.T) {
		body := "new List<String>().size()"
		pos := strings.Index(body, ".size") + 1
		receiver, method, ok := semaChainedCallReceiver(body, pos, map[string]string{}, semaTypeMemberViewFromMembers(map[string]typeMembers{}), "Example")
		if !ok || receiver != "List<String>" || method != "size" {
			t.Fatalf("receiver = (%q, %q, %v)", receiver, method, ok)
		}
	})
	for _, count := range []int{12, 24, 48} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var b strings.Builder
			b.WriteString("Factory.start()\n")
			for i := 0; i < count; i++ {
				b.WriteString("  .when(Factory.Services.Example.generate((Example.Request) anyValue()))\n  .then(result)\n")
			}
			b.WriteString("  .when(Factory.Services.Last.generate((Example.Request) anyValue()))\n  .then(lastResult)\n  .stop();")
			body := b.String()
			pos := strings.LastIndex(body, ".then(lastResult)") + 1
			work := &semaReceiverWork{Limit: 32*len(body) + 512}
			type result struct {
				receiver, method string
				ok               bool
				panicValue       any
			}
			done := make(chan result, 1)
			go func() {
				var got result
				defer func() { got.panicValue = recover(); done <- got }()
				got.receiver, got.method, got.ok = semaChainedCallReceiverWithWork(body, pos, map[string]string{}, semaTypeMemberViewFromMembers(map[string]typeMembers{}), "Example", work)
			}()
			select {
			case got := <-done:
				if got.panicValue != nil || got.receiver != "" || got.method != "" || got.ok {
					t.Fatalf("receiver result = %#v work=%+v", got, work)
				}
				if work.Calls != 1 || work.InferenceEntries != 0 || work.StopSite != "oversized-dotted-fallback" || work.Units > work.Limit {
					t.Fatalf("unexpected work: %+v", work)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("receiver did not finish")
			}
		})
	}
}
