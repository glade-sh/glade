package vm

import (
	"fmt"
	"testing"
)

// Both containment entry points and replacement share the scalar prefix path.
func TestScalarAliasContainmentCachedProbeAndReplacementRoutes(t *testing.T) {
	for _, container := range []string{"list", "set", "object", "map_value", "map_key"} {
		for _, route := range []string{"cached", "probe", "replacement"} {
			t.Run(container+"/"+route, func(t *testing.T) {
				target := Value{Kind: ValueString, Text: "before", Ref: newValueRef()}
				previous := snapshotAlias(target)
				children := make([]Value, 201)
				for i := range children {
					children[i] = String("noise")
				}
				children[200] = target
				root := Object("Envelope")
				switch container {
				case "list":
					root = List(children...)
				case "set":
					root = Set()
					root.Set = children
				case "object":
					root.Fields["item"] = target
				case "map_value":
					root = Map()
					root.Map["item"] = target
				case "map_key":
					root = Map()
					root.MapKeys["item"] = target
					root.Map["item"] = Int(1)
				}
				machine := New(nil)
				switch route {
				case "cached":
					if !machine.valueContainsAliasRefCached(root, previous, make(map[uint64]bool)) {
						t.Fatal("lost scalar reference")
					}
				case "probe":
					if !machine.valueContainsAliasRefCachedWithProbe(root, previous, make(map[uint64]bool), &scopeAliasProbe{}) {
						t.Fatal("lost scalar reference")
					}
				case "replacement":
					updated := target
					updated.Text = "after"
					got, changed := replaceValueAliasRef(root, previous, updated, make(map[uint64]bool))
					if !changed {
						t.Fatal("lost scalar replacement")
					}
					var value Value
					switch container {
					case "list":
						value = got.List[200]
					case "set":
						value = got.Set[200]
					case "object":
						value = got.Fields["item"]
					case "map_value":
						value = got.Map["item"]
					case "map_key":
						value = got.MapKeys["item"]
					}
					if value.Text != "after" {
						t.Fatalf("replacement = %v", value)
					}
				}
			})
		}
	}
}

func TestScalarAliasPrefixListSetCachedAndProbeRoutesBoundVisits(t *testing.T) {
	for _, width := range []int{200, 800} {
		for _, container := range []string{"list", "set"} {
			for _, route := range []string{"cached", "probe"} {
				t.Run(fmt.Sprintf("%s/%s/%d", container, route, width), func(t *testing.T) {
					target := Object("Leaf")
					values := make([]Value, width+1)
					for i := range width {
						values[i] = String("noise")
					}
					values[width] = target
					root := List(values...)
					if container == "set" {
						root = Set()
						root.Set = values
					}
					machine := New(nil)
					recorder := NewPerfRecorder()
					machine.SetPerfRecorder(recorder)
					found := false
					probe := &scopeAliasProbe{}
					if route == "cached" {
						found = machine.valueContainsAliasRefCached(root, snapshotAlias(target), make(map[uint64]bool))
					} else {
						found = machine.valueContainsAliasRefCachedWithProbe(root, snapshotAlias(target), make(map[uint64]bool), probe)
					}
					if !found {
						t.Fatal("lost reference after scalar prefix")
					}
					if visits := probe.recursiveVisits; route == "probe" && visits > 2 {
						t.Fatalf("visits=%d; want <=2", visits)
					}
				})
			}
		}
	}
}
