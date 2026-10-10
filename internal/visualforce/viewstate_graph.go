package visualforce

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/glade-sh/glade/internal/vm"
)

// The local state format assigns its own graph IDs. VM reference numbers are
// request-local and must never be reused by the next request. Shared collection
// and object nodes stay shared; equal but independent nodes stay independent.
type viewStateGraph struct {
	Payload    ViewStatePayload `json:"payload"`
	Nodes      []viewStateNode  `json:"nodes"`
	Controller map[string]int   `json:"controller,omitempty"`
	Extensions []map[string]int `json:"extensions,omitempty"`
}

type viewStateNode struct {
	Value           vm.Value       `json:"value"`
	Fields          map[string]int `json:"fields,omitempty"`
	List            []int          `json:"list,omitempty"`
	Set             []int          `json:"set,omitempty"`
	Map             map[string]int `json:"map,omitempty"`
	MapKeys         map[string]int `json:"keys,omitempty"`
	MapOrder        []string       `json:"order,omitempty"`
	MapIdentityKeys []string       `json:"identityKeys,omitempty"`
	Static          string         `json:"static,omitempty"`
	Runtime         string         `json:"runtime,omitempty"`
	ExplicitScale   bool           `json:"scale,omitempty"`
}

func marshalViewStateGraph(payload ViewStatePayload) ([]byte, error) {
	graph := viewStateGraph{Payload: payload, Nodes: []viewStateNode{}}
	seen := map[uint64]int{}
	var add func(vm.Value) int
	var fields func(map[string]vm.Value) map[string]int
	fields = func(values map[string]vm.Value) map[string]int {
		if values == nil {
			return nil
		}
		out := make(map[string]int, len(values))
		keys := make([]string, 0, len(values))
		for name := range values {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			out[name] = add(values[name])
		}
		return out
	}
	items := func(values []vm.Value) []int {
		if values == nil {
			return nil
		}
		out := make([]int, len(values))
		for i, value := range values {
			out[i] = add(value)
		}
		return out
	}
	add = func(value vm.Value) int {
		if value.Ref != 0 {
			if id, ok := seen[value.Ref]; ok {
				return id
			}
		}
		id := len(graph.Nodes)
		graph.Nodes = append(graph.Nodes, viewStateNode{})
		if value.Ref != 0 {
			seen[value.Ref] = id
		}
		node := viewStateNode{Value: value, Static: value.Static, Runtime: value.Runtime, ExplicitScale: value.ExplicitScale, MapOrder: value.MapOrder}
		node.Value.Fields, node.Value.List, node.Value.Set, node.Value.Map, node.Value.MapKeys = nil, nil, nil, nil, nil
		node.Fields = fields(value.Fields)
		node.List, node.Set = items(value.List), items(value.Set)
		node.Map, node.MapKeys = fields(value.Map), fields(value.MapKeys)
		for name, key := range value.MapKeys {
			if _, present := value.Map[name]; present && key.Kind == vm.ValueObject && key.Ref != 0 && name == viewStateIdentityMapKey(key) {
				node.MapIdentityKeys = append(node.MapIdentityKeys, name)
			}
		}
		sort.Strings(node.MapIdentityKeys)
		graph.Nodes[id] = node
		return id
	}
	graph.Controller = fields(payload.ControllerValues)
	if payload.ExtensionValues != nil {
		graph.Extensions = make([]map[string]int, len(payload.ExtensionValues))
		for i, values := range payload.ExtensionValues {
			graph.Extensions[i] = fields(values)
		}
	}
	graph.Payload.ControllerValues, graph.Payload.ExtensionValues = nil, nil
	// Typed state supersedes the legacy string fallback. Avoid storing each
	// field twice, especially lists and the controller's observation string.
	if payload.ControllerValues != nil {
		graph.Payload.ControllerFields = viewStateFallbackFields(payload.ControllerFields, payload.ControllerValues)
	}
	if payload.ExtensionValues != nil {
		graph.Payload.ExtensionFields = make([]map[string]string, len(payload.ExtensionFields))
		for i, fields := range payload.ExtensionFields {
			if i < len(payload.ExtensionValues) {
				graph.Payload.ExtensionFields[i] = viewStateFallbackFields(fields, payload.ExtensionValues[i])
			} else {
				graph.Payload.ExtensionFields[i] = fields
			}
		}
	}
	return json.Marshal(graph)
}

func viewStateFallbackFields(fields map[string]string, values map[string]vm.Value) map[string]string {
	var out map[string]string
	for name, text := range fields {
		if _, typed := values[name]; typed {
			continue
		}
		if out == nil {
			out = make(map[string]string)
		}
		out[name] = text
	}
	return out
}

// VM map dispatch uses this storage key for objects without a custom hashCode.
// Only exact identity keys are marked during encoding. Recomputing all keys
// would discard retained insertion hashes when a value-based key has mutated.
func viewStateIdentityMapKey(key vm.Value) string {
	return fmt.Sprintf("object:%s:ref:%d", key.Type, key.Ref)
}

func unmarshalViewStateGraph(data []byte) (ViewStatePayload, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ViewStatePayload{}, err
	}
	if _, ok := envelope["payload"]; !ok {
		// Existing authenticated local envelopes remain readable.
		var payload ViewStatePayload
		err := json.Unmarshal(data, &payload)
		return payload, err
	}
	var graph viewStateGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return ViewStatePayload{}, err
	}
	values := make([]vm.Value, len(graph.Nodes))
	for i, node := range graph.Nodes {
		value := node.Value
		switch value.Kind {
		case vm.ValueObject:
			value.Ref = vm.Object(value.Type).Ref
			value.Fields = make(map[string]vm.Value, len(node.Fields))
		case vm.ValueList:
			value.Ref = vm.List().Ref
			value.List = make([]vm.Value, len(node.List))
		case vm.ValueSet:
			value.Ref = vm.Set().Ref
			value.Set = make([]vm.Value, len(node.Set))
		case vm.ValueMap:
			value.Ref = vm.Map().Ref
			value.Map = make(map[string]vm.Value, len(node.Map))
			value.MapKeys = make(map[string]vm.Value, len(node.MapKeys))
		case vm.ValueNull, vm.ValueInt, vm.ValueDecimal, vm.ValueBool, vm.ValueString:
		default:
			return ViewStatePayload{}, fmt.Errorf("invalid state value kind %q", value.Kind)
		}
		switch value.Kind {
		case vm.ValueList, vm.ValueSet, vm.ValueMap:
			// Collections can carry VM metadata, such as a SOQL list's query.
			// Allocate it before resolving references so aliases and cycles share it.
			if node.Fields != nil {
				value.Fields = make(map[string]vm.Value, len(node.Fields))
			}
		}
		value.Static, value.Runtime, value.ExplicitScale, value.MapOrder = node.Static, node.Runtime, node.ExplicitScale, node.MapOrder
		values[i] = value
	}
	resolve := func(id int) (vm.Value, error) {
		if id < 0 || id >= len(values) {
			return vm.Null, fmt.Errorf("invalid state graph reference")
		}
		return values[id], nil
	}
	fill := func(dst map[string]vm.Value, src map[string]int, names map[string]string) error {
		for name, id := range src {
			value, err := resolve(id)
			if err != nil {
				return err
			}
			if replacement, ok := names[name]; ok {
				name = replacement
			}
			if _, exists := dst[name]; exists {
				return fmt.Errorf("invalid state graph duplicate map key")
			}
			dst[name] = value
		}
		return nil
	}
	for i, node := range graph.Nodes {
		value := &values[i]
		var mapNames map[string]string
		if len(node.MapIdentityKeys) != 0 {
			mapNames = make(map[string]string, len(node.MapIdentityKeys))
			for _, name := range node.MapIdentityKeys {
				keyID, present := node.MapKeys[name]
				if _, entry := node.Map[name]; !present || !entry || value.Kind != vm.ValueMap {
					return ViewStatePayload{}, fmt.Errorf("invalid state graph identity map key")
				}
				key, err := resolve(keyID)
				if err != nil {
					return ViewStatePayload{}, err
				}
				if key.Kind != vm.ValueObject || key.Ref == 0 {
					return ViewStatePayload{}, fmt.Errorf("invalid state graph identity map key")
				}
				mapNames[name] = viewStateIdentityMapKey(key)
			}
			for j, name := range value.MapOrder {
				if replacement, ok := mapNames[name]; ok {
					value.MapOrder[j] = replacement
				}
			}
		}
		for _, pair := range []struct {
			dst   map[string]vm.Value
			src   map[string]int
			names map[string]string
		}{{value.Fields, node.Fields, nil}, {value.Map, node.Map, mapNames}, {value.MapKeys, node.MapKeys, mapNames}} {
			if len(pair.src) != 0 && pair.dst == nil {
				return ViewStatePayload{}, fmt.Errorf("invalid state graph container")
			}
			if err := fill(pair.dst, pair.src, pair.names); err != nil {
				return ViewStatePayload{}, err
			}
		}
		for _, pair := range []struct {
			dst []vm.Value
			src []int
		}{{value.List, node.List}, {value.Set, node.Set}} {
			if len(pair.dst) != len(pair.src) {
				return ViewStatePayload{}, fmt.Errorf("invalid state graph collection")
			}
			for j, id := range pair.src {
				item, err := resolve(id)
				if err != nil {
					return ViewStatePayload{}, err
				}
				pair.dst[j] = item
			}
		}
	}
	payload := graph.Payload
	if graph.Controller != nil {
		payload.ControllerValues = make(map[string]vm.Value, len(graph.Controller))
		if err := fill(payload.ControllerValues, graph.Controller, nil); err != nil {
			return ViewStatePayload{}, err
		}
	}
	if graph.Extensions != nil {
		payload.ExtensionValues = make([]map[string]vm.Value, len(graph.Extensions))
		for i, fields := range graph.Extensions {
			if fields == nil {
				continue
			}
			payload.ExtensionValues[i] = make(map[string]vm.Value, len(fields))
			if err := fill(payload.ExtensionValues[i], fields, nil); err != nil {
				return ViewStatePayload{}, err
			}
		}
	}
	return payload, nil
}
