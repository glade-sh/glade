package vm

import (
	"strings"
	"testing"
)

func TestHeapUserBlobAndPlatformBlobRootRoutes(t *testing.T) {
	for _, identity := range []string{"user", "platform"} {
		for _, route := range []string{"global", "static", "list", "set", "map", "object"} {
			t.Run(identity+"/"+route, func(t *testing.T) {
				machine := New(nil)
				held := NewBlobValue(strings.Repeat("x", 100000))
				if identity == "user" {
					if err := machine.RegisterClass(Class{Name: "Blob", Access: "public", Fields: map[string]Field{"value": {Name: "value", Type: "String"}}}); err != nil {
						t.Fatal(err)
					}
					var err error
					held, err = machine.constructValue("Blob", nil, nil, &Result{})
					if err != nil {
						t.Fatal(err)
					}
					if !held.classInstance {
						t.Fatal("missed user class constructor route")
					}
					held.Fields["value"] = String(strings.Repeat("x", 100000))
				}
				switch route {
				case "static":
					machine.Classes["HeapRegistry"] = Class{Name: "HeapRegistry", StaticFields: map[string]Field{"held": {Name: "held", Static: true, Value: held}}}
				default:
					switch route {
					case "list":
						held = List(held, held)
					case "set":
						held = Set(held)
					case "map":
						envelope := Map()
						envelope.Map["held"] = held
						held = envelope
					case "object":
						envelope := Object("Envelope")
						envelope.Fields["held"] = held
						held = envelope
					}
					machine.Globals["held"] = held
				}
				got := machine.currentHeapSize()
				if identity == "user" && (got < 100000 || got > 101000) {
					t.Fatalf("user Blob retained heap=%d; want one ordinary field payload", got)
				}
				if identity == "platform" && got > 1024 {
					t.Fatalf("platform Blob handle retained heap=%d", got)
				}
			})
		}
	}
}

func TestApproxValueSizeUserBlobDirectRoute(t *testing.T) {
	held := Object("Blob")
	held.classInstance = true
	held.Fields["value"] = String(strings.Repeat("x", 100000))
	if got := approxValueSize(held); got < 100000 {
		t.Fatalf("user class fields omitted: %d", got)
	}
	if got := approxValueSize(NewBlobValue(strings.Repeat("x", 100000))); got != 8 {
		t.Fatalf("platform handle size=%d; want 8", got)
	}
}
