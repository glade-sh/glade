package vm

import (
	"strings"
	"testing"
)

// Native heap_throw_point rows retain their collections in user statics.
func TestHeapRetainedGlobalAndStaticRoutes(t *testing.T) {
	for _, route := range []string{"global", "static"} {
		t.Run(route, func(t *testing.T) {
			machine := New(nil)
			payload := List(String(strings.Repeat("x", 100000)))
			switch route {
			case "global":
				machine.Globals["held"] = payload
			case "static":
				machine.Classes["HeapHolder"] = Class{Name: "HeapHolder", StaticFields: map[string]Field{"held": {Name: "held", Static: true, Value: payload}}}
			}
			if got := machine.currentHeapSize(); got < 100000 {
				t.Fatalf("retained %s heap=%d; missing payload", route, got)
			}
		})
	}
}

func TestHeapSharedGlobalStaticAliasRouteCountsPayloadOnce(t *testing.T) {
	machine := New(nil)
	payload := List(String(strings.Repeat("x", 100000)))
	machine.Globals["held"] = payload
	machine.Classes["HeapHolder"] = Class{Name: "HeapHolder", StaticFields: map[string]Field{"held": {Name: "held", Static: true, Value: payload}}}
	if got := machine.currentHeapSize(); got < 100000 || got > 110000 {
		t.Fatalf("aliased heap=%d; want one retained payload", got)
	}
}

func TestHeapBlobHandleListAndMapRoutes(t *testing.T) {
	for _, route := range []string{"list", "map"} {
		t.Run(route, func(t *testing.T) {
			blob := Object("Blob")
			blob.Fields["data"] = String(strings.Repeat("x", 100000))
			root := List(blob)
			if route == "map" {
				root = Map()
				root.Map["item"] = blob
			}
			machine := New(nil)
			machine.Globals["held"] = root
			if got := machine.currentHeapSize(); got > 1024 {
				t.Fatalf("Blob payload counted as heap: %d", got)
			}
		})
	}
}
