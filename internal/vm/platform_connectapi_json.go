package vm

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/typesys"
)

// ConnectApi output classes serialize through a native writer, not through the
// Apex object writer. Native rows (testdata/conformance/capi4_output_json, all
// 4749 type names at class APIs 62.0, 67.0 and 68.0):
//   - the key set does not depend on the API version: every property that was
//     not removed, including those added at 68.0, ascending by name; a few
//     classes keep another order or skip properties that compile;
//   - 1241 classes drop null properties, 64 serialize as the JSON text null;
//   - an unassigned (or null) property of some classes serializes as false, 0,
//     [] or {} and a discriminator property as its constant;
//   - JSON.serialize(x, true) changes none of this.
//
// The deviations come from the captured shape (typesys.ConnectAPIJSONProfile).

var connectAPIJSONKeyCache sync.Map

// connectAPIJSONKeys returns the serialized keys of an output class in order.
func connectAPIJSONKeys(typeName string, profile *typesys.ConnectAPIJSON) []string {
	if profile != nil && len(profile.Keys) > 0 {
		return profile.Keys
	}
	cacheKey := strings.ToLower(typeName)
	if cached, ok := connectAPIJSONKeyCache.Load(cacheKey); ok {
		return cached.([]string)
	}
	if keys, ok := typesys.ConnectAPIJSONKeys(typeName); ok {
		connectAPIJSONKeyCache.Store(cacheKey, keys)
		return keys
	}
	seen := map[string]bool{}
	var keys []string
	for _, generated := range connectAPITypeChain(typeName) {
		for _, name := range generated.FieldOrder {
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			if rng, ok := typesys.ConnectAPIMemberAvailability(generated.Name, name); ok && rng.Until > 0 {
				continue
			}
			keys = append(keys, generated.Fields[name].Name)
		}
	}
	sort.Strings(keys)
	connectAPIJSONKeyCache.Store(cacheKey, keys)
	return keys
}

// connectAPIJSONDefault parses a captured default literal into the value
// shapes the JSON writer accepts.
func connectAPIJSONDefault(literal string) (any, bool) {
	var decoded any
	if err := json.Unmarshal([]byte(literal), &decoded); err != nil {
		return nil, false
	}
	return connectAPIJSONNative(decoded), true
}

func connectAPIJSONNative(value any) any {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = connectAPIJSONNative(item)
		}
		return out
	case map[string]any:
		if len(typed) == 0 {
			return map[string]any{}
		}
		names := make([]string, 0, len(typed))
		for name := range typed {
			names = append(names, name)
		}
		sort.Strings(names)
		out := orderedJSONObject{}
		for _, name := range names {
			out = append(out, orderedJSONField{name: name, value: connectAPIJSONNative(typed[name])})
		}
		return out
	}
	return value
}

// connectAPIOutputJSON serializes a ConnectApi output object the way the
// native writer does. The second result is false for any other value.
func (vm *VM) connectAPIOutputJSON(value Value, suppressObjectNulls bool, state *jsonSerializationState) (any, bool) {
	if value.Kind != ValueObject || !hasTypePrefixFold(value.Type, "ConnectApi") {
		return nil, false
	}
	if isExceptionType(value.Type) {
		// Native rows (capi4_output_json, Exception rows at APIs 62.0..67.0): the
		// ConnectApi exception classes cannot be serialized. Through API 62.0 the
		// message is the ConnectApi one; from 63.0 it names the class.
		if major, _ := apexversion.Major(vm.connectAPIAPIVersion()); major < 63 {
			panic(jsonConversionFailure{err: newExceptionError("System.JSONException", "Only output types from ConnectApi support serialization")})
		}
		simple := value.Type[strings.LastIndex(value.Type, ".")+1:]
		panic(jsonConversionFailure{err: newExceptionError("System.JSONException", "Type unsupported in JSON: "+simple)})
	}
	profile, profiled := typesys.ConnectAPIJSONProfile(value.Type)
	if !profiled && !vm.connectAPIObjectValue(value) {
		return nil, false
	}
	if profile != nil && profile.Fail != nil && profile.Fail.Phase == "serialize" {
		panic(jsonConversionFailure{err: newExceptionError(profile.Fail.Exception, profile.Fail.Message)})
	}
	if profile != nil && profile.Whole != "" {
		whole, ok := connectAPIJSONDefault(profile.Whole)
		return whole, ok
	}
	value = connectAPISegmentJSONView(value)
	out := orderedJSONObject{}
	for _, key := range connectAPIJSONKeys(value.Type, profile) {
		item := Null
		if _, current, ok := objectFieldValue(value, key); ok {
			item = current
		} else {
			for name, current := range value.Fields {
				if strings.EqualFold(name, key) {
					item = current
					break
				}
			}
		}
		if profile != nil {
			if literal, ok := profile.Defaults[key]; ok && (item.Kind == ValueNull || profile.Constants[key]) {
				if literal == `"<uuid>"` {
					// A default generated per instance.
					if generated, err := vm.randomUUID(); err == nil {
						if text, ok := platformScalarObjectText(generated); ok {
							out = append(out, orderedJSONField{name: key, value: text})
							continue
						}
					}
				}
				if parsed, ok := connectAPIJSONDefault(literal); ok {
					out = append(out, orderedJSONField{name: key, value: parsed})
					continue
				}
			}
		}
		if item.Kind == ValueNull {
			if profile != nil {
				if profile.OmitNulls {
					continue
				}
			}
			out = append(out, orderedJSONField{name: key, value: nil})
			continue
		}
		out = append(out, orderedJSONField{name: key, value: vm.jsonFromValueForSerializeWithState(item, suppressObjectNulls, state)})
	}
	return out, true
}

// connectAPIReflectionFailure reports the captured native failure of
// Type.forName(name) (the type does not exist, so the lookup answers null) or
// of Type.forName(name).newInstance() for a ConnectApi type.
func connectAPIReflectionFailure(typeName, phase string) (*typesys.ConnectAPIFailure, string, bool) {
	if !hasTypePrefixFold(typeName, "ConnectApi") {
		return nil, "", false
	}
	profile, ok := typesys.ConnectAPIJSONProfile(typeName)
	if !ok || profile.Fail == nil || profile.Fail.Phase != phase {
		return nil, "", false
	}
	canonical := typeName
	if generated, ok := generatedPlatformTypes()[strings.ToLower(typeName)]; ok {
		canonical = generated.Name
	}
	return profile.Fail, strings.ReplaceAll(profile.Fail.Message, "{T}", canonical), true
}
