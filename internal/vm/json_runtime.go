package vm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

const jsonCyclicConversionMessage = "Glade: cyclic object reference cannot be converted to JSON (native behaviour not yet captured)"

var jsonChildRelationshipIndexBuilds atomic.Uint64

// childRelationshipObjectStateCopies counts ObjectState values copied into a
// temporary candidate map. Parent-plan construction must stay at zero.
var childRelationshipObjectStateCopies atomic.Uint64

// jsonConversionFailure crosses legacy converters and overload scoring whose
// signatures cannot return an Apex exception. VM execution boundaries recover
// only this failure and hand its exception to the normal Apex error path.
type jsonConversionFailure struct{ err error }

func recoverJSONConversionError(err *error) {
	if recovered := recover(); recovered != nil {
		failure, ok := recovered.(jsonConversionFailure)
		if !ok {
			panic(recovered)
		}
		*err = failure.err
	}
}

func jsonFromValue(value Value, suppressObjectNulls bool) any {
	out, err := jsonFromValueChecked(value, suppressObjectNulls)
	if err != nil {
		panic(jsonConversionFailure{err: err})
	}
	return out
}

// jsonFromValueChecked returns no partial JSON when the graph contains a cycle.
// Native behaviour for the cyclic cases is captured on
// both routes at APIs 59/62/67/68; this boundary is not itself a parity claim.
// This Glade-specific exception is a safety boundary, not a native parity claim.
// Legacy display/DTO conversion retains its bounded null cut separately from
// the captured public JSON.serialize class-cycle boundary.
func jsonFromValueChecked(value Value, suppressObjectNulls bool) (out any, err error) {
	defer recoverJSONConversionError(&err)
	walker := jsonValueWalker{}
	out = walker.convert(value, suppressObjectNulls)
	return out, nil
}

// jsonValueWalker tracks the composite values on the active conversion path.
// The table is allocated only once a non-empty composite value is entered.
type jsonValueWalker struct {
	active map[jsonSerializationIdentity]bool
}

func (w *jsonValueWalker) convert(value Value, suppressObjectNulls bool) any {
	value = value.currentMapKeySet()
	if jsonValueMayRecurse(value) {
		if identity, ok := jsonSerializationToken(value); ok {
			if w.active[identity] {
				panic(jsonConversionFailure{err: newExceptionError("System.JSONException", jsonCyclicConversionMessage)})
			}
			if w.active == nil {
				w.active = make(map[jsonSerializationIdentity]bool)
			}
			w.active[identity] = true
			defer delete(w.active, identity)
		}
	}
	switch value.Kind {
	case ValueNull:
		return nil
	case ValueInt:
		return value.Int
	case ValueDecimal:
		if strings.TrimSpace(value.Text) != "" {
			return json.Number(value.Text)
		}
		return value.Decimal
	case ValueBool:
		return value.Bool
	case ValueString:
		return value.Text
	case ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, w.convert(item, suppressObjectNulls))
		}
		return out
	case ValueSet:
		out := make([]any, 0, len(value.Set))
		for _, item := range value.Set {
			out = append(out, w.convert(item, suppressObjectNulls))
		}
		return out
	case ValueMap:
		if len(value.MapOrder) > 0 {
			out := orderedJSONObject{}
			seen := map[string]bool{}
			for _, key := range orderedJSONMapKeys(value) {
				item, ok := value.Map[key]
				if !ok || seen[key] {
					continue
				}
				out = append(out, orderedJSONField{name: jsonMapKeyText(mapStoredKey(value, key)), value: w.convert(item, suppressObjectNulls)})
				seen[key] = true
			}
			for _, key := range sortedMapKeys(value.Map) {
				if seen[key] {
					continue
				}
				out = append(out, orderedJSONField{name: jsonMapKeyText(mapStoredKey(value, key)), value: w.convert(value.Map[key], suppressObjectNulls)})
			}
			return out
		}
		out := make(map[string]any, len(value.Map))
		for key, item := range value.Map {
			out[jsonMapKeyText(mapStoredKey(value, key))] = w.convert(item, suppressObjectNulls)
		}
		return out
	case ValueObject:
		if compound, ok := jsonCompoundFromValue(value, suppressObjectNulls, w.convert); ok {
			return compound
		}
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			return scalar
		}
		return jsonSObjectFromValue(value, suppressObjectNulls, w.convert, storage.DefaultRESTAPIVersion, value.Type)
	default:
		return nil
	}
}

// jsonValueMayRecurse reports whether converting value can descend into
// another runtime value. Scalars and empty collections skip cycle tracking.
func jsonValueMayRecurse(value Value) bool {
	switch value.Kind {
	case ValueList:
		return len(value.List) > 0
	case ValueSet:
		return len(value.Set) > 0
	case ValueMap:
		return len(value.Map) > 0
	case ValueObject:
		return len(value.Fields) > 0
	default:
		return false
	}
}

const jsonListReferenceKind ValueKind = "list-reference"

type jsonSerializationIdentity struct {
	kind  ValueKind
	token uint64
}

// jsonSerializationToken identifies the backing composite value while it is
// on the active serialization path. Values are copied frequently by the VM,
// so Ref is not sufficient for aliases that share a backing map or slice.
func jsonSerializationToken(value Value) (jsonSerializationIdentity, bool) {
	if value.Kind != ValueList && value.Kind != ValueSet && value.Kind != ValueMap && value.Kind != ValueObject {
		return jsonSerializationIdentity{}, false
	}
	var pointer uintptr
	switch value.Kind {
	case ValueList, ValueSet:
		if value.List != nil {
			pointer = reflect.ValueOf(value.List).Pointer()
		} else if value.Set != nil {
			pointer = reflect.ValueOf(value.Set).Pointer()
		}
	case ValueMap:
		if value.Map != nil {
			pointer = reflect.ValueOf(value.Map).Pointer()
		}
	case ValueObject:
		if value.Fields != nil {
			pointer = reflect.ValueOf(value.Fields).Pointer()
		}
	}
	if pointer != 0 {
		return jsonSerializationIdentity{kind: value.Kind, token: uint64(pointer)}, true
	}
	if value.Ref != 0 {
		return jsonSerializationIdentity{kind: value.Kind, token: value.Ref}, true
	}
	return jsonSerializationIdentity{}, false
}

// jsonRootMapHasNullKey checks the two runtime encodings for a null map key.
// The second encoding is used by maps populated from unsaved SObjects.
func jsonRootMapHasNullKey(value Value) bool {
	if value.Kind != ValueMap {
		return false
	}
	value = value.currentMapKeySet()
	for _, key := range [...]string{mapKey(Null), "null:sobject-list-id"} {
		if _, ok := value.Map[key]; ok && mapStoredKey(value, key).Kind == ValueNull {
			return true
		}
	}
	return false
}

func (vm *VM) jsonFromValueForSerialize(value Value, suppressObjectNulls bool) any {
	// Most JSON.serialize calls are scalar or shallow values. Allocate the
	// active-path table only once a composite value actually needs cycle
	// tracking; the previous eager allocation made every scalar serialization
	// pay for a map that it never used.
	return vm.jsonFromValueForSerializeWithActive(value, suppressObjectNulls, nil)
}

// Public JSON entry points reject circular collections and source objects.
// The legacy converter also serves display/DTO APIs with separate behavior.
func (vm *VM) jsonFromValueForJSONSerialize(value Value, suppressObjectNulls bool) any {
	return vm.jsonFromValueForSerializeWithState(value, suppressObjectNulls, &jsonSerializationState{rejectCycles: true, rejectUnsupportedTypes: true, root: true, rootValue: value})
}

type jsonSerializationState struct {
	active                 map[jsonSerializationIdentity]bool
	rejectCycles           bool
	root                   bool
	rootValue              Value
	generator              bool
	rootKey                string
	rootIndex              int
	rejectUnsupportedTypes bool
}

func (vm *VM) jsonFromValueForSerializeWithActive(value Value, suppressObjectNulls bool, active map[jsonSerializationIdentity]bool) any {
	return vm.jsonFromValueForSerializeWithState(value, suppressObjectNulls, &jsonSerializationState{active: active})
}

func (vm *VM) jsonFromValueForSerializeWithState(value Value, suppressObjectNulls bool, state *jsonSerializationState) any {
	value = value.currentMapKeySet()
	root := state.root
	state.root = false
	// List mutation can leave an earlier slice header inside the same logical
	// list. Track its reference even when that snapshot has no elements.
	if state.rejectCycles && value.Kind == ValueList && value.Ref != 0 {
		identity := jsonSerializationIdentity{kind: jsonListReferenceKind, token: value.Ref}
		if state.active[identity] {
			panic(jsonConversionFailure{err: vm.jsonCircularReferenceError(value, state)})
		}
		if len(value.List) > 0 {
			if state.active == nil {
				state.active = make(map[jsonSerializationIdentity]bool)
			}
			state.active[identity] = true
			defer delete(state.active, identity)
		}
	}
	if encoded, captured := capturedDoubleSingletonJSON(value); captured {
		return encoded
	}
	// Empty collections cannot recurse, so they cannot participate in a cycle.
	// Preserve the ordered-object representation for maps while avoiding the
	// reflection and active-table work for these common no-op values.
	switch value.Kind {
	case ValueList:
		if len(value.List) == 0 {
			return []any{}
		}
	case ValueSet:
		if len(value.Set) == 0 {
			return []any{}
		}
	case ValueMap:
		if len(value.Map) == 0 {
			if len(value.MapOrder) > 0 {
				return orderedJSONObject{}
			}
			return map[string]any{}
		}
	}
	// Native collection diagnostics identify the repeated child graph; the
	// public root container is an envelope around those values.
	if identity, ok := jsonSerializationToken(value); ok && !(root && value.Kind != ValueObject) {
		if state.active[identity] {
			if state.rejectCycles && (value.Kind != ValueObject || value.classInstance && value.projectClass) {
				panic(jsonConversionFailure{err: vm.jsonCircularReferenceError(value, state)})
			}
			// Record aliases and legacy display conversion retain their bounded cut.
			return nil
		}
		if state.active == nil {
			state.active = make(map[jsonSerializationIdentity]bool)
		}
		state.active[identity] = true
		defer delete(state.active, identity)
	}
	switch value.Kind {
	case ValueList:
		out := make([]any, 0, len(value.List))
		for index, item := range value.List {
			if root {
				state.rootIndex = index
			}
			out = append(out, vm.jsonFromValueForSerializeWithState(item, suppressObjectNulls, state))
		}
		return out
	case ValueSet:
		out := make([]any, 0, len(value.Set))
		for index, item := range value.Set {
			if root {
				state.rootIndex = index
			}
			out = append(out, vm.jsonFromValueForSerializeWithState(item, suppressObjectNulls, state))
		}
		return out
	case ValueMap:
		if len(value.MapOrder) > 0 {
			out := orderedJSONObject{}
			seen := map[string]bool{}
			for _, key := range orderedJSONMapKeys(value) {
				item, ok := value.Map[key]
				if !ok || seen[key] {
					continue
				}
				name := vm.jsonMapKeyText(mapStoredKey(value, key))
				if root {
					state.rootKey = name
				}
				out = append(out, orderedJSONField{name: name, value: vm.jsonFromValueForSerializeWithState(item, suppressObjectNulls, state)})
				seen[key] = true
			}
			for _, key := range sortedMapKeys(value.Map) {
				if seen[key] {
					continue
				}
				if root {
					state.rootKey = vm.jsonMapKeyText(mapStoredKey(value, key))
				}
				out = append(out, orderedJSONField{name: vm.jsonMapKeyText(mapStoredKey(value, key)), value: vm.jsonFromValueForSerializeWithState(value.Map[key], suppressObjectNulls, state)})
			}
			return out
		}
		out := make(map[string]any, len(value.Map))
		for key, item := range value.Map {
			if root {
				state.rootKey = vm.jsonMapKeyText(mapStoredKey(value, key))
			}
			out[vm.jsonMapKeyText(mapStoredKey(value, key))] = vm.jsonFromValueForSerializeWithState(item, suppressObjectNulls, state)
		}
		return out
	case ValueObject:
		if (state.rejectUnsupportedTypes || state.generator) && isAccessLevelValue(value) && !accessLevelHasCustomScope(value) {
			message := "Type unsupported in JSON: common.apex.methods.AccessLevelEnum"
			if state.generator {
				switch state.rootValue.Kind {
				case ValueMap:
					message += fmt.Sprintf(" (through reference chain: java.util.LinkedHashMap[%q])", state.rootKey)
				case ValueList:
					message += fmt.Sprintf(" (through reference chain: java.util.ArrayList[%d])", state.rootIndex)
				}
			}
			panic(jsonConversionFailure{err: newExceptionError("System.JSONException", message)})
		}
		if state.rejectUnsupportedTypes && !value.classInstance && strings.EqualFold(value.Type, "JSONParser") {
			panic(jsonConversionFailure{err: newExceptionError("System.JSONException", "Type unsupported in JSON: common.apex.json.ApexJsonParser")})
		}
		value = vm.picklistEntryJSONView(value, suppressObjectNulls)
		if strings.EqualFold(value.Type, "Database.QueryLocator") {
			return &queryLocatorJSONError{}
		}
		if strings.EqualFold(value.Type, "Database.Cursor") {
			return vm.databaseCursorJSONValue(value, suppressObjectNulls)
		}
		if vm.isEnumObjectValue(value) {
			return value.Text
		}
		if !vm.jsonAccessAllowed(value.Type, "serializable") {
			return jsonAccessSerializationFailure{}
		}
		if strings.EqualFold(value.Type, "ApexPages.Message") {
			return apexPagesMessageJSONFailure{}
		}
		if strings.EqualFold(value.Type, "Datetime") || strings.EqualFold(value.Type, "DateTime") {
			if t, err := parsePlatformDatetime(value); err == nil {
				return t.UTC().Format("2006-01-02T15:04:05.000Z")
			}
		}
		if compound, ok := jsonCompoundFromValue(value, suppressObjectNulls, func(item Value, suppress bool) any {
			return vm.jsonFromValueForSerializeWithState(item, suppress, state)
		}); ok {
			return compound
		}
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			return scalar
		}
		if serialized, handled := vm.invocableFlowActionJSONValue(value, suppressObjectNulls, state.active); handled {
			return serialized
		}
		if out, captured := vm.stubNamespaceJSON(value); captured {
			return out
		}
		if value.Type == "" || sObjectValueType(value.Type) {
			version := storage.DefaultRESTAPIVersion
			if vm != nil && vm.Org != nil {
				version = vm.Org.APIVersion
			}
			urlType := value.Type
			if value.Type == "Name" && (value.Fields[jsonCapturedOwnerProjectionField].Bool || value.Fields[jsonSObjectQueryProjectionField].Runtime == "Name") {
				// The observed projection retains Name as its SObject type, while
				// its REST URL names the concrete object identified by its ID.
				if id, ok := idValueText(value.Fields["Id"]); ok {
					if concrete, known := vm.sObjectNameForID(id); known && (concrete == "User" || value.Fields[jsonSObjectQueryProjectionField].Runtime == "Name") {
						urlType = concrete
					}
				}
			}
			// Salesforce preserves null SObject fields for JSON.serialize, even
			// when the overload's Boolean argument is true. That flag only
			// suppresses nulls on Apex objects and collections.
			value = vm.jsonCompoundSObjectValue(value)
			return jsonSObjectFromValue(value, false, func(item Value, suppress bool) any {
				return vm.jsonFromValueForSerializeWithState(item, suppress, state)
			}, version, urlType)
		}
		base := orderedJSONObject{}
		seen := map[string]bool{}
		getterNames := vm.jsonSerializableGetterNameSet(value.Type)
		for _, fieldName := range vm.jsonSerializableFieldNames(value.Type) {
			fieldKey := strings.ToLower(fieldName)
			field, owner, fieldOK := vm.lookupField(value.Type, fieldName)
			if fieldOK {
				fieldKey = strings.ToLower(field.Name)
				if jsonFieldIsTransient(field) {
					seen[fieldKey] = true
					continue
				}
				if field.Getter != nil && !field.Static {
					getterValue, err := vm.callGetter(vm.getterOwner(owner, field), field, value)
					if err == nil && !(suppressObjectNulls && getterValue.Kind == ValueNull) {
						base = append(base, orderedJSONField{name: field.Name, value: vm.jsonFromValueForSerializeWithState(getterValue, suppressObjectNulls, state)})
						seen[fieldKey] = true
					}
					continue
				}
				if _, shadowed := getterNames[fieldKey]; shadowed {
					continue
				}
			}
			actualName, item, ok := objectFieldValue(value, fieldName)
			if !ok {
				continue
			}
			if isInternalSObjectField(actualName) || (suppressObjectNulls && item.Kind == ValueNull) {
				continue
			}
			base = append(base, orderedJSONField{name: actualName, value: vm.jsonFromValueForSerializeWithState(item, suppressObjectNulls, state)})
			seen[strings.ToLower(actualName)] = true
		}
		var extras []string
		for field := range value.Fields {
			fieldKey := strings.ToLower(field)
			if isInheritedInstanceFieldStorage(field) || isInternalSObjectField(field) || seen[fieldKey] {
				continue
			}
			if classField, _, ok := vm.lookupField(value.Type, field); ok && jsonFieldIsTransient(classField) {
				continue
			}
			extras = append(extras, field)
		}
		sort.Strings(extras)
		for _, field := range extras {
			item := value.Fields[field]
			if suppressObjectNulls && item.Kind == ValueNull {
				continue
			}
			base = append(base, orderedJSONField{name: field, value: vm.jsonFromValueForSerializeWithState(item, suppressObjectNulls, state)})
			seen[strings.ToLower(field)] = true
		}
		for _, field := range vm.jsonSerializableGetterFields(value.Type) {
			if field.Getter == nil || field.Static {
				continue
			}
			name := field.Name
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			getterValue, err := vm.callGetter(vm.getterOwner(value.Type, field), field, value)
			if err != nil || (suppressObjectNulls && getterValue.Kind == ValueNull) {
				continue
			}
			base = append(base, orderedJSONField{name: name, value: vm.jsonFromValueForSerializeWithState(getterValue, suppressObjectNulls, state)})
			seen[strings.ToLower(name)] = true
		}
		if !seen["urls"] {
			if urls, ok := vm.describeJSONURLs(value); ok {
				base = append(base, orderedJSONField{name: "urls", value: urls})
			}
		}
		return base
	default:
		return jsonFromValue(value, suppressObjectNulls)
	}
}

func jsonFieldIsTransient(field Field) bool {
	for _, modifier := range field.Modifiers {
		if strings.EqualFold(modifier, "transient") {
			return true
		}
	}
	return false
}

func (vm *VM) isEnumObjectValue(value Value) bool {
	if value.Kind != ValueObject || strings.TrimSpace(value.Text) == "" {
		return false
	}
	switch {
	case strings.EqualFold(value.Type, "Schema.DisplayType"),
		strings.EqualFold(value.Type, "DisplayType"),
		strings.EqualFold(value.Type, "Schema.SOAPType"),
		strings.EqualFold(value.Type, "SOAPType"),
		strings.EqualFold(value.Type, "LoggingLevel"),
		strings.EqualFold(value.Type, "RoundingMode"),
		strings.EqualFold(value.Type, "AccessType"),
		strings.EqualFold(value.Type, "TriggerOperation"),
		strings.EqualFold(value.Type, "StatusCode"),
		strings.EqualFold(value.Type, "Metadata.DeployStatus"),
		strings.EqualFold(value.Type, "Metadata.MetadataType"):
		return true
	}
	if generated, ok := generatedPlatformTypes()[strings.ToLower(value.Type)]; ok && generated.Kind == apexast.DeclarationEnum {
		return true
	}
	if _, ok := vm.resolveEnumClass(value.Type); ok {
		return true
	}
	return false
}

// Salesforce indents object fields, while array delimiters and separators stay
// inline. Native rows include sobject_json queryChildren and
// json_circular_references dag/deep (source APIs 62 and 67).
func formatSalesforcePrettyJSON(data []byte) string {
	var out bytes.Buffer
	out.Grow(len(data))
	var containers []byte
	depth := 0
	previous := byte(0)
	inString, escaped := false, false
	newline := func() {
		out.WriteByte('\n')
		for i := 0; i < depth; i++ {
			out.WriteString("  ")
		}
	}
	for _, b := range data {
		if inString {
			out.WriteByte(b)
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			previous = b
			continue
		}
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}
		if previous == '{' && b != '}' {
			newline()
		}
		switch b {
		case '"':
			inString = true
			out.WriteByte(b)
		case '{':
			containers = append(containers, b)
			depth++
			out.WriteByte(b)
		case '}':
			depth--
			if previous == '{' {
				out.WriteByte(' ')
			} else {
				newline()
			}
			containers = containers[:len(containers)-1]
			out.WriteByte(b)
		case '[':
			containers = append(containers, b)
			out.WriteString("[ ")
		case ']':
			if previous != '[' {
				out.WriteByte(' ')
			}
			containers = containers[:len(containers)-1]
			out.WriteByte(b)
		case ',':
			out.WriteByte(b)
			if containers[len(containers)-1] == '{' {
				newline()
			} else {
				out.WriteByte(' ')
			}
		case ':':
			out.WriteString(" : ")
		default:
			out.WriteByte(b)
		}
		previous = b
	}
	return out.String()
}

type orderedJSONField struct {
	name  string
	value any
	// Input positions are retained for typed Apex field conversion errors.
	source                 string
	start, end, inputStart int
	// Enum errors retain the separator before later fields (K070-K084).
	enumStart int
}

type orderedJSONObject []orderedJSONField

func jsonMapKeyText(key Value) string {
	if key.Kind == ValueObject && strings.EqualFold(key.Type, "Blob") {
		if raw, ok := key.Fields["value"]; ok && raw.Kind == ValueString {
			return fmt.Sprintf("Blob[%d]", len(raw.Text))
		}
	}
	return key.String()
}

func (vm *VM) jsonMapKeyText(key Value) string {
	// K097/K126-K129: registered enum keys use the same member text as
	// serialized enum values. Other keys keep their existing rendering.
	if vm.isEnumObjectValue(key) {
		return key.Text
	}
	return jsonMapKeyText(key)
}

// Only actual JSON input carries source positions. Non-JSON coercion callers
// keep their existing Object conversion and enum value lookup behavior.
type jsonTypedInput struct {
	value  any
	source string
	start  int
	root   bool
	// Schema relationship mapping must retain record provenance recursively,
	// even when a loaded user class shadows the relationship's record type.
	sObjectRecord bool
}

func jsonArrayInputStarts(input jsonTypedInput) []int {
	if input.source == "" || input.start < 0 || input.start >= len(input.source) {
		return nil
	}
	start := jsonTokenInputStart(input.source, input.start)
	if start >= len(input.source) {
		return nil
	}
	if input.source[start] != '[' {
		// A field's diagnostic position is its key, while its array items
		// start after the key and colon.
		decoder := json.NewDecoder(strings.NewReader(input.source[start:]))
		var key string
		if err := decoder.Decode(&key); err != nil {
			return nil
		}
		start = jsonTokenInputStart(input.source, start+int(decoder.InputOffset()))
	}
	decoder := json.NewDecoder(strings.NewReader(input.source[start:]))
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		return nil
	}
	var starts []int
	for decoder.More() {
		position := jsonEnumTokenInputStart(input.source, start+int(decoder.InputOffset()))
		var item json.RawMessage
		if err := decoder.Decode(&item); err != nil {
			return nil
		}
		starts = append(starts, position)
	}
	return starts
}

func (object orderedJSONObject) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	for i, field := range object {
		if i > 0 {
			out.WriteByte(',')
		}
		name, err := jsonMarshalNoEscape(field.name)
		if err != nil {
			return nil, err
		}
		value, err := jsonMarshalNoEscape(field.value)
		if err != nil {
			return nil, err
		}
		out.Write(name)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

func jsonMarshalNoEscape(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		var unsupported *queryLocatorJSONError
		if errors.As(err, &unsupported) {
			return nil, unsupported
		}
		return nil, jsonAccessMarshalError(err)
	}
	data := out.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	return data, nil
}

func jsonMarshalNoEscapeIndent(value any, prefix, indent string) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)
	if err := encoder.Encode(value); err != nil {
		var unsupported *queryLocatorJSONError
		if errors.As(err, &unsupported) {
			return nil, unsupported
		}
		return nil, jsonAccessMarshalError(err)
	}
	data := out.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	return data, nil
}

func jsonSObjectFromValue(value Value, suppressObjectNulls bool, convert func(Value, bool) any, apiVersion, urlType string) any {
	value = jsonProjectedSObjectValue(value)
	out := orderedJSONObject{}
	if value.Type != "" {
		jsonType := value.Type
		if value.Fields[jsonSObjectQueryProjectionField].Runtime == "Name" {
			jsonType = "Name"
		}
		attributes := map[string]any{"type": jsonType}
		if id, ok := value.Fields["Id"]; ok && !strings.Contains(value.Type, ".") {
			version, err := storage.ResolveRESTAPIVersion(apiVersion)
			if idText, ok := idValueText(id); err == nil && ok && idText != "" {
				urlID := displayIDText(idText)
				if value.Fields[jsonCapturedOwnerProjectionField].Bool && len(idText) == 15 {
					urlID = idText
				}
				attributes["url"] = "/services/data/v" + version + "/sobjects/" + urlType + "/" + urlID
			}
		}
		out = append(out, orderedJSONField{name: "attributes", value: attributes})
	}
	for _, field := range jsonSerializedSObjectFieldNames(value) {
		item := value.Fields[field]
		if suppressObjectNulls && item.Kind == ValueNull {
			continue
		}
		if envelope := item.Fields[jsonChildQueryResultField]; item.Kind == ValueList && envelope.Kind == ValueObject {
			// Queried empty relationships are absent from the serialized
			// SObject. An explicitly deserialized empty envelope is retained.
			if item.Fields[jsonQueriedChildRelationshipField].Bool && len(item.List) == 0 {
				continue
			}
			children := orderedJSONObject{}
			for _, name := range []string{"totalSize", "done"} {
				if metadata, present := envelope.Fields[name]; present {
					children = append(children, orderedJSONField{name: name, value: convert(metadata, false)})
				}
			}
			children = append(children, orderedJSONField{name: "records", value: convert(item, suppressObjectNulls)})
			out = append(out, orderedJSONField{name: field, value: children})
			continue
		}
		out = append(out, orderedJSONField{name: field, value: convert(item, suppressObjectNulls)})
	}
	return out
}

func jsonSObjectFieldNames(value Value) []string {
	regular := make([]string, 0, len(value.Fields))
	system := make([]string, 0, 8)
	for field, item := range value.Fields {
		if isInternalSObjectField(field) || isPicklistJSONValidForField(value, field) || isDefaultedSObjectField(value, field) {
			continue
		}
		if isImplicitFalseIsDeleted(value, field, item) {
			continue
		}
		if isImplicitGeneratedSystemField(value, field) {
			continue
		}
		// DML bookkeeping may materialize OwnerId without adding it to the
		// caller's field projection. Explicit and selected owners stay visible.
		if field == "OwnerId" && !isExplicitSObjectField(value, field) {
			if selected, ok := value.Fields[sobjectQueriedFieldsField]; ok && selected.Kind == ValueMap && !jsonSObjectSelectedField(value, field) {
				continue
			}
		}
		if jsonGeneratedSystemField(field) {
			system = append(system, field)
			continue
		}
		regular = append(regular, field)
	}
	sort.Strings(regular)
	sort.Strings(system)
	return jsonOrderedSObjectFieldNames(value, regular, system)
}

// The private explicit-field marker owns insertion history and its JSON origin.
// Its runtime tag is internal and never becomes an Apex field or map entry.
const jsonDeserializedSObjectFieldOrder = "__glade_json_deserialized_sobject_fields"

// Constructed SObjects retain their first assignment order (R222/R223/R368).
// Deserialized SObjects use canonical-name hash buckets, including later writes.
func jsonOrderedSObjectFieldNames(value Value, regular, system []string) []string {
	customMetadata := strings.HasSuffix(strings.ToLower(value.Type), "__mdt") || value.Fields[sobjectExplicitFieldsField].Runtime == jsonDeserializedSObjectFieldOrder
	byFoldedName := make(map[string]string, len(regular)+len(system))
	for _, field := range regular {
		byFoldedName[strings.ToLower(field)] = field
	}
	if customMetadata {
		for _, field := range system {
			byFoldedName[strings.ToLower(field)] = field
		}
	}
	ordered := make([]string, 0, len(byFoldedName))
	seen := make(map[string]bool, len(byFoldedName))
	explicit := explicitSObjectFieldNamesInInsertionOrder(value)
	for _, field := range explicit {
		actual, ok := byFoldedName[strings.ToLower(field)]
		if !ok || seen[actual] {
			continue
		}
		ordered = append(ordered, actual)
		seen[actual] = true
	}
	for _, field := range regular {
		if !seen[field] {
			ordered = append(ordered, field)
		}
	}
	marker := value.Fields[sobjectExplicitFieldsField]
	if marker.Runtime == jsonDeserializedSObjectFieldOrder {
		// R364-R375 and R376-R389: buckets use canonical field names;
		// collisions retain insertion order and 13 fields grow 16 to 32 buckets.
		capacity := uint32(16)
		for len(explicit) > int(capacity*3/4) {
			capacity *= 2
		}
		bucket := func(field string) uint32 {
			hash := uint32(javaStringHashCode(field))
			return (hash ^ (hash >> 16)) & (capacity - 1)
		}
		sort.SliceStable(ordered, func(i, j int) bool {
			return bucket(ordered[i]) < bucket(ordered[j])
		})
	}
	for _, field := range system {
		if !seen[field] {
			ordered = append(ordered, field)
		}
	}
	return ordered
}

func jsonGeneratedSystemField(field string) bool {
	switch field {
	case "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SetupOwnerId", "SystemModstamp":
		return true
	default:
		return false
	}
}

func isImplicitGeneratedSystemField(value Value, field string) bool {
	if !jsonGeneratedSystemField(field) {
		return false
	}
	// Explicit-field keys are normalized; use their shared lookup rather
	// than comparing those keys to the canonical mixed-case field name.
	return !isExplicitSObjectField(value, field) &&
		(dmlAccessibleSObject(value) || !jsonSObjectSelectedField(value, field))
}

func jsonSObjectSelectedField(value Value, field string) bool {
	selected, ok := value.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	flag := selected.Map[mapKey(String(strings.ToLower(field)))]
	return flag.Kind == ValueBool && flag.Bool
}

func isImplicitFalseIsDeleted(value Value, field string, item Value) bool {
	if field != "IsDeleted" || item.Kind != ValueBool || item.Bool {
		return false
	}
	return !isExplicitSObjectField(value, field) && !jsonSObjectSelectedField(value, field)
}

func orderedJSONMapKeys(value Value) []string {
	return reverseMapOrder(value.MapOrder)
}

func jsonObjectMap(raw any) (map[string]any, bool) {
	if fields, ok := raw.(map[string]any); ok {
		return fields, true
	}
	object, ok := raw.(orderedJSONObject)
	if !ok {
		return nil, false
	}
	fields := make(map[string]any, len(object))
	for _, field := range object {
		fields[field.name] = field.value
	}
	return fields, true
}

func jsonObjectFields(raw any) ([]orderedJSONField, bool) {
	if object, ok := raw.(orderedJSONObject); ok {
		out := make([]orderedJSONField, 0, len(object))
		positions := make(map[string]int, len(object))
		for _, field := range object {
			if index, ok := positions[field.name]; ok {
				out[index].value = field.value
				continue
			}
			positions[field.name] = len(out)
			out = append(out, field)
		}
		return out, true
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]orderedJSONField, 0, len(keys))
	for _, key := range keys {
		out = append(out, orderedJSONField{name: key, value: fields[key]})
	}
	return out, true
}

// SObject aliases refer to the same field, so each occurrence must be applied
// in textual order. Keep duplicate occurrences here; generic map conversion
// deliberately retains its existing duplicate-key behavior.
func jsonSObjectFieldsInTextualOrder(raw any) ([]orderedJSONField, bool) {
	if fields, ok := raw.(orderedJSONObject); ok {
		return fields, true
	}
	return jsonObjectFields(raw)
}

func reverseMapOrder(order []string) []string {
	reversed := make([]string, len(order))
	for i, key := range order {
		reversed[len(reversed)-1-i] = key
	}
	return reversed
}

func (vm *VM) jsonSerializableFieldNames(typeName string) []string {
	var fields []string
	seen := make(map[string]struct{})
	var visit func(string)
	visit = func(name string) {
		class, ok := vm.lookupClass(name)
		if !ok {
			return
		}
		if class.SuperClass != "" {
			visit(class.SuperClass)
		}
		current := make([]string, 0, len(class.FieldOrder))
		metadataChild := false
		for _, field := range class.FieldOrder {
			if strings.EqualFold(field, "fullName_type_info") {
				metadataChild = true
				break
			}
		}
		for _, field := range class.FieldOrder {
			key := strings.ToLower(field)
			// WSDL2Apex metadata children redeclare fullName alongside the
			// inherited member; Salesforce emits both occurrences. Ordinary
			// shadowed fields remain deduplicated.
			if _, ok := seen[key]; ok && !(metadataChild && strings.EqualFold(field, "fullName")) {
				continue
			}
			seen[key] = struct{}{}
			current = append(current, field)
		}
		sort.SliceStable(current, func(i, j int) bool {
			return strings.ToLower(current[i]) > strings.ToLower(current[j])
		})
		fields = append(fields, current...)
	}
	visit(typeName)
	// Salesforce reflects Apex object members in case-insensitive descending
	// name order within each inheritance level. This is observable for
	// generated SOAP classes, where private *_type_info members are serialized
	// alongside their public values, while superclass members remain first.
	return fields
}

func (vm *VM) jsonSerializableGetterNameSet(typeName string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, field := range vm.jsonSerializableGetterFields(typeName) {
		if field.Getter == nil || field.Static || field.Name == "" {
			continue
		}
		out[strings.ToLower(field.Name)] = struct{}{}
	}
	return out
}

func (vm *VM) jsonSerializableGetterFields(typeName string) []Field {
	var fields []Field
	var visit func(string)
	visit = func(name string) {
		class, ok := vm.lookupClass(name)
		if !ok {
			return
		}
		if class.SuperClass != "" {
			visit(class.SuperClass)
		}
		for _, fieldName := range class.FieldOrder {
			field, ok := class.Fields[fieldName]
			if ok && field.Getter != nil {
				fields = append(fields, field)
			}
		}
	}
	visit(typeName)
	return fields
}

func (vm *VM) getterOwner(typeName string, field Field) string {
	if field.Getter == nil {
		return typeName
	}
	if dot := strings.LastIndex(field.Getter.Name, "."); dot > 0 {
		return field.Getter.Name[:dot]
	}
	return typeName
}

func decodeJSONValue(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := decodeJSONToken(decoder, text)
	return value, jsonSyntaxErrorAtInput(err, text, int(decoder.InputOffset()))
}

func decodeJSONToken(decoder *json.Decoder, source string) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			out := orderedJSONObject{}
			for decoder.More() {
				start := int(decoder.InputOffset())
				enumStart := jsonEnumTokenInputStart(source, start)
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, fmt.Errorf("expected object field name")
				}
				start = jsonTokenInputStart(source, start)
				item, err := decodeJSONToken(decoder, source)
				if err != nil {
					return nil, err
				}
				// J003/J004 and P001-P009: Integer conversion uses the same
				// input boundary as enum errors, retaining a separating comma.
				out = append(out, orderedJSONField{name: key, value: item, source: source, start: start, end: int(decoder.InputOffset()), enumStart: enumStart, inputStart: enumStart})
			}
			if end, err := decoder.Token(); err != nil {
				return nil, err
			} else if end != json.Delim('}') {
				return nil, fmt.Errorf("expected object end")
			}
			return out, nil
		case '[':
			out := []any{}
			for decoder.More() {
				item, err := decodeJSONToken(decoder, source)
				if err != nil {
					return nil, err
				}
				out = append(out, item)
			}
			if end, err := decoder.Token(); err != nil {
				return nil, err
			} else if end != json.Delim(']') {
				return nil, fmt.Errorf("expected array end")
			}
			return out, nil
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter %q", value)
		}
	default:
		return value, nil
	}
}

func decodeJSONUntypedValue(text string) (Value, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := decodeJSONUntypedToken(decoder, text)
	if err != nil {
		return Null, jsonUnexpectedCharacterError(jsonSyntaxErrorAtInput(err, text, int(decoder.InputOffset())), text)
	}
	return value, nil
}

func decodeJSONUntypedToken(decoder *json.Decoder, source string) (Value, error) {
	token, err := decoder.Token()
	if err != nil {
		return Null, jsonStringEOFError(err, source)
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			out := Map()
			out.Type = "Map<String,Object>"
			completeValueAtEnd := false
			scalarAtEnd := false
			sawEntry := false
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return Null, jsonContainerEOFError(err, source, "OBJECT", sawEntry && !completeValueAtEnd, scalarAtEnd)
				}
				key, ok := keyToken.(string)
				if !ok {
					return Null, fmt.Errorf("expected object field name")
				}
				sawEntry = true
				item, err := decodeJSONUntypedToken(decoder, source)
				if err != nil {
					return Null, jsonContainerEOFError(err, source, "OBJECT", sawEntry && !completeValueAtEnd, scalarAtEnd)
				}
				completeValueAtEnd, scalarAtEnd = jsonValueAtInputEnd(item, decoder, source)
				encoded := mapKey(String(key))
				if _, exists := out.Map[encoded]; !exists {
					out.MapOrder = append(out.MapOrder, encoded)
				}
				out.Map[encoded] = item
				out.MapKeys[encoded] = String(key)
			}
			if end, err := decoder.Token(); err != nil {
				return Null, jsonContainerEOFError(err, source, "OBJECT", sawEntry && !completeValueAtEnd, scalarAtEnd)
			} else if end != json.Delim('}') {
				return Null, fmt.Errorf("expected object end")
			}
			return out, nil
		case '[':
			out := List()
			out.Type = "List<Object>"
			completeValueAtEnd := false
			scalarAtEnd := false
			sawEntry := false
			for decoder.More() {
				item, err := decodeJSONUntypedToken(decoder, source)
				if err != nil {
					return Null, jsonContainerEOFError(err, source, "ARRAY", sawEntry && !completeValueAtEnd, scalarAtEnd)
				}
				sawEntry = true
				completeValueAtEnd, scalarAtEnd = jsonValueAtInputEnd(item, decoder, source)
				out.List = append(out.List, item)
			}
			if end, err := decoder.Token(); err != nil {
				return Null, jsonContainerEOFError(err, source, "ARRAY", sawEntry && !completeValueAtEnd, scalarAtEnd)
			} else if end != json.Delim(']') {
				return Null, fmt.Errorf("expected array end")
			}
			return out, nil
		default:
			return Null, fmt.Errorf("unexpected JSON delimiter %q", value)
		}
	case nil:
		return Null, nil
	case string:
		return String(value), nil
	case bool:
		return Bool(value), nil
	case json.Number:
		if integer, err := strconv.ParseInt(value.String(), 10, 64); err == nil {
			// Untyped JSON retains Long identity outside the Integer range.
			// Captured cases also distinguish small Integer values from Long.
			if integer < math.MinInt32 || integer > math.MaxInt32 {
				return longIntValue(integer), nil
			}
			return Int(integer), nil
		}
		if !strings.ContainsAny(value.String(), ".eE") {
			line, column := jsonNumberStartLineColumn(source, decoder.InputOffset(), value.String())
			return Null, &jsonNumberInputError{text: value.String(), line: line, column: column}
		}
		decimal, err := decimalFromText(value.String())
		if err != nil {
			return Null, err
		}
		return decimal, nil
	default:
		return valueFromJSON(value), nil
	}
}

type jsonContainerInputError struct {
	container     string
	withinEntries bool
	line          int
	column        int
}

func (err *jsonContainerInputError) Error() string {
	if err.withinEntries {
		return fmt.Sprintf("Unexpected end-of-input within/between %s entries at [line:%d, column:%d]", err.container, err.line, err.column)
	}
	return fmt.Sprintf("Unexpected end-of-input: expected close marker for %s (from [line:%d, column:%d]", err.container, err.line, err.column)
}

// jsonValueAtInputEnd distinguishes a complete value at EOF from a value
// whose reported close-marker position includes the input span. Strings use
// the ordinary EOF position, while the other JSON scalar tokens use the
// Salesforce-admitted scalar position.
func jsonValueAtInputEnd(value Value, decoder *json.Decoder, source string) (bool, bool) {
	if decoder.InputOffset() != int64(len(source)) {
		return false, false
	}
	switch value.Kind {
	case ValueNull, ValueBool, ValueInt, ValueDecimal:
		return true, true
	case ValueString:
		return true, false
	}
	return false, false
}

// Observed EOF positions include the input span plus the final line span.
// A bare scalar ending at EOF adds one more input span to the reported column.
func jsonEOFPosition(source string, scalarAtEnd bool) (int, int) {
	line, start := 1, 0
	for i := 0; i < len(source); i++ {
		if source[i] == '\r' {
			line++
			if i+1 < len(source) && source[i+1] == '\n' {
				i++
			}
			start = i + 1
		} else if source[i] == '\n' {
			line++
			start = i + 1
		}
	}
	column := apexStringLength(source) + apexStringLength(source[start:]) + 1
	if scalarAtEnd {
		column += apexStringLength(source)
	}
	return line, column
}

func jsonContainerEOFError(err error, source, container string, withinEntries, scalarAtEnd bool) error {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if err != io.EOF && err != io.ErrUnexpectedEOF &&
		!strings.Contains(message, "unexpected end of JSON input") &&
		!strings.Contains(message, "unexpected EOF") {
		return jsonStringEOFError(err, source)
	}
	line, column := jsonEOFPosition(source, scalarAtEnd)
	return &jsonContainerInputError{container: container, withinEntries: withinEntries, line: line, column: column}
}

type jsonStringInputError struct {
	escape       bool
	line, column int
}

type jsonCharacterInputError struct {
	char         rune
	line, column int
	nested       bool
}

func (err *jsonCharacterInputError) Error() string {
	message := fmt.Sprintf("Unexpected character ('%c' (code %d)): %s", err.char, err.char, jsonExpectedValueDescription(err.char))
	// Non-delimiter failures at the document root use
	// input-location text; nested values and delimiter errors retain line/column.
	if !err.nested && err.char != '}' && err.char != ']' {
		return message + fmt.Sprintf(" at input location [%d,%d]", err.line, err.column)
	}
	return message + fmt.Sprintf(" at [line:%d, column:%d]", err.line, err.column)
}

func jsonExpectedValueDescription(character rune) string {
	// Distinguish an unexpected closing delimiter from
	// an unrecognized value token.
	if character == '}' || character == ']' {
		return "expected a value"
	}
	return "expected a valid value (number, String, array, object, 'true', 'false' or 'null')"
}

func jsonSyntaxErrorAtInput(err error, source string, offset int) error {
	syntax, ok := err.(*json.SyntaxError)
	if !ok || !strings.Contains(syntax.Error(), "looking for beginning of value") || offset < 0 || offset >= len(source) {
		return err
	}
	if !strings.HasPrefix(syntax.Error(), "invalid character "+strconv.QuoteRune(rune(source[offset]))) {
		return err
	}
	// J004/U001-U014: Token's scanner offset excludes structural tokens
	// and previously read object keys. Its input cursor is document-relative.
	// Keep the original error text for the separate root-closing parser path.
	normalized := *syntax
	normalized.Offset = int64(offset + 1)
	return &normalized
}

func jsonUnexpectedCharacterError(err error, source string) error {
	if syntax, ok := err.(*json.SyntaxError); ok && strings.Contains(syntax.Error(), "looking for beginning of value") {
		offset := int(syntax.Offset)
		// Decoder.Token reports an unconsumed delimiter at InputOffset,
		// whereas scalar scanning reports the offset after the bad character.
		consumed := offset > 0 && offset <= len(source) && strings.HasPrefix(syntax.Error(), "invalid character "+strconv.QuoteRune(rune(source[offset-1])))
		if !consumed && offset >= 0 && offset < len(source) && strings.HasPrefix(syntax.Error(), "invalid character "+strconv.QuoteRune(rune(source[offset]))) {
			offset++
		}
		if offset > 0 && offset <= len(source) {
			line, column := jsonInputPosition(source, offset)
			trimmed := strings.TrimSpace(source)
			return &jsonCharacterInputError{char: rune(source[offset-1]), line: line, column: column, nested: strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{")}
		}
	}
	return err
}

func jsonTokenInputStart(source string, offset int) int {
	for offset < len(source) && strings.ContainsRune(" \t\r\n,:", rune(source[offset])) {
		offset++
	}
	return offset
}

func jsonEnumTokenInputStart(source string, offset int) int {
	for offset < len(source) && strings.ContainsRune(" \t\r\n", rune(source[offset])) {
		offset++
	}
	return offset
}

func jsonInputPosition(source string, offset int) (int, int) {
	if offset > len(source) {
		offset = len(source)
	}
	prefix := source[:offset]
	line := strings.Count(prefix, "\n") + 1
	start := strings.LastIndex(prefix, "\n") + 1
	return line, apexStringLength(prefix[start:]) + 1
}

func (err *jsonStringInputError) Error() string {
	context := ": was expecting closing quote for a string value"
	if err.escape {
		context = " in character escape sequence"
	}
	return fmt.Sprintf("Unexpected end-of-input%s at [line:%d, column:%d]", context, err.line, err.column)
}

func jsonStringEOFError(err error, source string) error {
	if err != io.ErrUnexpectedEOF {
		return err
	}
	quoted, escaped, unicodeDigits := false, false, 0
	for _, char := range source {
		if !quoted {
			if char == '"' {
				quoted = true
			}
			continue
		}
		if unicodeDigits > 0 {
			unicodeDigits--
			continue
		}
		if escaped {
			escaped = false
			if char == 'u' {
				unicodeDigits = 4
			}
			continue
		}
		if char == '\\' {
			escaped = true
		} else if char == '"' {
			quoted = false
		}
	}
	if !quoted {
		return err
	}
	line, column := jsonEOFPosition(source, false)
	return &jsonStringInputError{escape: escaped || unicodeDigits > 0, line: line, column: column}
}

type jsonNumberInputError struct {
	text   string
	line   int
	column int
}

func (err *jsonNumberInputError) Error() string {
	return fmt.Sprintf("For input string: %q at [line:%d, column:%d]", err.text, err.line, err.column)
}

func jsonNumberStartLineColumn(source string, endOffset int64, number string) (int, int) {
	start := int(endOffset) - len(number)
	if start < 0 {
		start = 0
	}
	if boundary := jsonNumberErrorBoundary(source, start); boundary >= 0 {
		return sourceLineColumn(source, boundary)
	}
	return sourceLineColumn(source, start)
}

func jsonNumberErrorBoundary(source string, end int) int {
	if end <= 0 {
		return -1
	}
	inString := false
	escaped := false
	boundary := -1
	for index, char := range source[:end] {
		if inString {
			switch {
			case escaped:
				escaped = false
			case char == '\\':
				escaped = true
			case char == '"':
				inString = false
			}
			continue
		}
		if char == '"' {
			inString = true
			continue
		}
		switch char {
		case ',':
			boundary = index
		}
	}
	return boundary
}

func decodeJSONValueForDeserialize(text string, strict bool) (any, error) {
	decoded, err := decodeJSONValue(text)
	if err != nil {
		if character, ok := jsonUnexpectedCharacterError(err, text).(*jsonCharacterInputError); ok &&
			(character.nested || (character.char != '}' && character.char != ']')) {
			// Typed invalid values agree with
			// untyped decoding. Root closing markers (K001-K005/K009-K012)
			// use a separate parser error path, outside this value formatter.
			return nil, character
		}
		if strings.Contains(err.Error(), "unexpected EOF") && strings.HasPrefix(strings.TrimSpace(text), `"`) {
			return nil, fmt.Errorf("malformed JSON: %s", err.Error())
		}
		if strings.Contains(err.Error(), "invalid character '\\\\'") && strings.Contains(text, `\"`) {
			decoded, retryErr := decodeJSONValue(strings.ReplaceAll(text, `\"`, `"`))
			if retryErr == nil {
				return decoded, nil
			}
			return nil, normalizeJSONDeserializeError(retryErr)
		}
		return nil, normalizeJSONDeserializeError(err)
	}
	return decoded, nil
}

func normalizeJSONDeserializeError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if strings.Contains(message, "unexpected EOF") || strings.Contains(message, "unexpected end of JSON input") || message == "EOF" {
		return fmt.Errorf("Unexpected end-of-input: %s", message)
	}
	if strings.HasPrefix(message, "JSON.deserializeStrict") {
		return err
	}
	if hasPrefixFold(message, "malformed json:") {
		return err
	}
	// Salesforce capitalizes the malformed-object-key diagnostic surfaced by
	// JSON.deserialize. Keep the existing lower-case spelling for the other
	// parser errors, whose local and Salesforce contracts are already covered.
	if strings.Contains(message, "after object key") {
		return fmt.Errorf("Malformed JSON: %s", message)
	}
	return fmt.Errorf("malformed JSON: %s", message)
}

func validateJSONNoDuplicateObjectFields(text string) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	return validateJSONValueNoDuplicateObjectFields(decoder)
}

func validateJSONValueNoDuplicateObjectFields(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON.deserializeStrict expected object field name")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("JSON.deserializeStrict found duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := validateJSONValueNoDuplicateObjectFields(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("JSON.deserializeStrict expected object end")
		}
	case '[':
		for decoder.More() {
			if err := validateJSONValueNoDuplicateObjectFields(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("JSON.deserializeStrict expected array end")
		}
	default:
		return fmt.Errorf("JSON.deserializeStrict found unexpected delimiter %q", delim)
	}
	return nil
}

func valueFromJSON(raw any) Value {
	switch v := raw.(type) {
	case nil:
		return Null
	case bool:
		return Bool(v)
	case float64:
		if math.Trunc(v) == v {
			if converted, err := int64FromFloat("JSON number", v); err == nil {
				return Int(converted)
			}
		}
		return Decimal(v)
	case json.Number:
		text := v.String()
		if !strings.ContainsAny(text, ".eE") {
			if converted, err := strconv.ParseInt(text, 10, 64); err == nil {
				return Int(converted)
			}
		}
		if decimal, err := decimalFromText(text); err == nil {
			return decimal
		}
		return String(text)
	case string:
		return String(v)
	case []any:
		out := make([]Value, 0, len(v))
		for _, item := range v {
			out = append(out, valueFromJSON(item))
		}
		return List(out...)
	case map[string]any:
		out := Map()
		for key, item := range v {
			out.Map[mapKey(String(key))] = valueFromJSON(item)
		}
		return out
	case orderedJSONObject:
		out := Map()
		for _, field := range v {
			encoded := mapKey(String(field.name))
			if _, exists := out.Map[encoded]; !exists {
				out.MapOrder = append(out.MapOrder, encoded)
			}
			out.Map[encoded] = valueFromJSON(field.value)
			out.MapKeys[encoded] = String(field.name)
		}
		return out
	default:
		return Null
	}
}

// jsonRootSObjectPayload recognizes the concrete SObject envelope accepted by
// JSON.deserialize. Keep this at that entry point: recursive mapping, strict
// deserialization and JSONParser have independent payload contracts.
func (vm *VM) jsonRootSObjectPayload(typeName string, raw any) any {
	typeName = vm.resolveJSONTypeName(typeName)
	if _, userClass := vm.lookupClass(typeName); userClass {
		return raw
	}
	if !vm.isSObjectLikeType(typeName) {
		return raw
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return raw
	}
	fields, ok := jsonObjectMap(raw)
	if !ok || len(fields) != 1 {
		return raw
	}
	wrapped, ok := fields[objectName]
	if !ok {
		return raw
	}
	if _, ok := jsonObjectMap(wrapped); !ok {
		return raw
	}
	return wrapped
}

// Owned native API62/67/68 rows cover these positioned List<String> inputs.
// Other targets and token kinds retain their existing diagnostic boundary.
func jsonStringListInputError(typeName string, raw any, input jsonTypedInput) error {
	if !strings.EqualFold(typeName, "List<String>") || input.source == "" {
		return jsonTypeMappingError(typeName, raw)
	}
	if input.root {
		if _, text := raw.(string); text {
			return jsonDeserializeException("Malformed JSON: Expected '[' at the beginning of List/Set")
		}
		return jsonTypeMappingError(typeName, raw)
	}
	found := ""
	switch value := raw.(type) {
	case string:
		found = "\"" + value + "\""
	case orderedJSONObject, map[string]any:
		found = "{"
	case json.Number:
		found = "null"
	default:
		return jsonTypeMappingError(typeName, raw)
	}
	line, column := jsonInputPosition(input.source, input.start)
	return jsonDeserializeException("Expected List<String> but found %s at [line:%d, column:%d]", found, line, column)
}

func (vm *VM) typedValueFromJSON(typeName string, raw any, strict bool) (Value, error) {
	input, positioned := raw.(jsonTypedInput)
	if err := vm.jsonRootDatetimeNullError(typeName, input); err != nil {
		return Null, err
	}
	if positioned {
		raw = input.value
	}
	if err := vm.jsonRootDateNullError(typeName, input); err != nil {
		return Null, err
	}
	if value, ok, err := typedScalarFromJSON(typeName, raw); ok || err != nil {
		if input.root && err == nil {
			vm.rememberLocalOnlyObject(value)
		}
		return value, err
	}
	if _, ok := vm.explicitSchemaRecordType(typeName); ok {
		typeName = typeName[len("Schema."):]
		input.sObjectRecord = true
	} else if !input.sObjectRecord {
		typeName = vm.resolveJSONTypeName(typeName)
	}
	if !vm.jsonAccessAllowed(typeName, "deserializable") {
		return Null, jsonDeserializeException("Type cannot be deserialized")
	}
	// R179-R184/N022-N038: query handles have their own closed JSON surface.
	// Raw null has already followed the ordinary scalar-null path above.
	if strings.EqualFold(typeName, "Database.QueryLocator") {
		return Null, jsonDeserializeException("Apex Type unsupported in JSON: Database.QueryLocator")
	}
	if strings.EqualFold(typeName, "Database.Cursor") {
		return vm.databaseCursorFromJSON(raw, strict)
	}
	if value, handled, err := vm.invocableFlowResultFromJSON(typeName, raw, input, strict); handled {
		return value, err
	}
	if collectionBase(typeName) == "List" {
		items, ok := raw.([]any)
		if !ok {
			if records, recordsOK := jsonQueryResultRecords(raw); recordsOK {
				items = records
			} else {
				return Null, jsonStringListInputError(typeName, raw, input)
			}
		}
		elementType, _ := collectionElementType(typeName)
		// K031-K034/K068/K069: Object is unsupported in nonempty typed
		// collections, even when every item is null. Empty collections work.
		if input.source != "" && len(items) != 0 && strings.EqualFold(vm.resolveJSONTypeName(elementType), "Object") {
			return Null, jsonDeserializeException("Apex Type unsupported in JSON: Object")
		}
		out := List()
		out.Type = typeName
		starts := jsonArrayInputStarts(input)
		for i, item := range items {
			if i < len(starts) {
				item = jsonTypedInput{value: item, source: input.source, start: starts[i], sObjectRecord: input.sObjectRecord}
			} else if input.sObjectRecord {
				item = jsonTypedInput{value: item, sObjectRecord: true}
			}
			value, err := vm.typedValueFromJSON(elementType, item, strict)
			if err != nil {
				return Null, err
			}
			vm.markCollectionRefsEscaped(value)
			out.List = append(out.List, value)
		}
		return out, nil
	}
	if collectionBase(typeName) == "Set" {
		items, ok := raw.([]any)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		elementType, _ := collectionElementType(typeName)
		out := Set()
		out.Type = typeName
		starts := jsonArrayInputStarts(input)
		for i, item := range items {
			if i < len(starts) {
				item = jsonTypedInput{value: item, source: input.source, start: starts[i]}
			}
			value, err := vm.typedValueFromJSON(elementType, item, strict)
			if err != nil {
				return Null, err
			}
			if !containsValue(out.Set, value) {
				vm.markCollectionRefsEscaped(value)
				out.Set = append(out.Set, value)
			}
		}
		return out, nil
	}
	if isMapType(typeName) {
		_, ok := jsonObjectMap(raw)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			return Null, jsonTypeMappingError(typeName, raw)
		}
		out := Map()
		out.Type = typeName
		orderedFields, _ := jsonObjectFields(raw)
		unsupportedObjectValue := input.source != "" && strings.EqualFold(vm.resolveJSONTypeName(valueType), "Object")
		for _, field := range orderedFields {
			keyValue, err := vm.typedJSONMapKey(keyType, field.name, jsonTypedInput{source: input.source, start: field.enumStart})
			if err != nil {
				return Null, err
			}
			// K094-K096: native validates a present key before its Object value,
			// while an empty map never attempts either conversion.
			if unsupportedObjectValue {
				return Null, jsonDeserializeException("Apex Type unsupported in JSON: Object")
			}
			value, err := vm.typedValueFromJSON(valueType, jsonTypedInput{value: field.value, source: field.source, start: field.enumStart}, strict)
			if err != nil {
				return Null, err
			}
			encodedKey := vm.mapKey(keyValue)
			if _, exists := out.Map[encodedKey]; !exists {
				out.MapOrder = append(out.MapOrder, encodedKey)
			}
			vm.markCollectionRefsEscaped(keyValue, value)
			out.Map[encodedKey] = value
			out.MapKeys[encodedKey] = keyValue
		}
		return out, nil
	}
	if strings.EqualFold(typeName, "Object") {
		if input.source != "" {
			return Null, jsonDeserializeException("Apex Type unsupported in JSON: Object")
		}
		return valueFromJSON(raw), nil
	}
	if enumValue, ok, err := vm.typedEnumJSONInput(typeName, raw, input); ok || err != nil {
		return enumValue, err
	}
	if strings.EqualFold(typeName, "sObject") {
		value, err := vm.sObjectValueFromJSON(raw, strict)
		if input.root && err == nil {
			vm.rememberLocalOnlyObject(value)
		}
		return value, err
	}
	if !vm.isJSONTypedObjectTarget(typeName) {
		return Null, unsupportedCallError("JSON.deserialize local class/SObject mapping for " + typeName)
	}
	var obj Value
	var err error
	if input.sObjectRecord && vm.isSObjectLikeType(typeName) {
		// A schema relationship always allocates a record, never a same-name
		// class instance or its constructor/field initializers.
		obj = Object(typeName)
	} else {
		obj, err = vm.jsonObjectBaseValue(typeName)
	}
	if err != nil {
		return Null, err
	}
	if input.root {
		// Platform construction already tracks escapes. Source DTOs allocate
		// without construction, before a property setter can publish this root.
		if class, registered := vm.lookupClass(typeName); !registered || !class.PlatformType || !classHasZeroArgConstructor(class) {
			vm.rememberLocalOnlyObject(obj)
		}
	}
	// Class declarations take precedence over same-name SObject metadata,
	// including the scalar-field container checks below.
	_, registeredClass := vm.lookupClass(typeName)
	typedObjectIsSObject := !obj.classInstance && vm.isSObjectLikeType(typeName)
	if typedObjectIsSObject {
		vm.markJSONDeserializedSObjectFields(&obj, typeName)
		if strings.EqualFold(typeName, "User") {
			vm.initializeSObjectSchemaDefaults(&obj, typeName)
		}
	}
	fields, ok := jsonObjectMap(raw)
	if !ok {
		return Null, jsonTypeMappingError(typeName, raw)
	}
	if strict || vm.isPlatformJSONHeader(typeName) {
		if err := vm.jsonStrictObjectFieldsError(typeName, raw); err != nil {
			return Null, err
		}
	}
	_, platformDTO := platformJSONDTOFields(typeName)
	if strings.EqualFold(typeName, "EventBus.ChangeEventHeader") && !vm.isPlatformJSONHeader(typeName) {
		platformDTO = false
	}
	ignoreUnknownClassFields := !strict && registeredClass && !typedObjectIsSObject && !platformDTO
	objectFields := make([]orderedJSONField, 0, len(fields))
	if typedObjectIsSObject {
		objectFields, _ = jsonSObjectFieldsInTextualOrder(raw)
	} else {
		inputFields, _ := jsonObjectFields(raw)
		byName := make(map[string]orderedJSONField, len(inputFields))
		for _, field := range inputFields {
			byName[field.name] = field
		}
		for _, key := range vm.sortedJSONTypedObjectFields(typeName, fields) {
			objectFields = append(objectFields, byName[key])
		}
	}
	for _, inputField := range objectFields {
		key, item := inputField.name, inputField.value
		if input.root {
			if handled, err := vm.skipJSONHeaderNullField(typeName, key, item); handled || err != nil {
				if err != nil {
					return Null, err
				}
				continue
			}
		}
		// Source-class members bind normally, including a declared attributes
		// field. Preserve the metadata skip for records and platform targets.
		if key == "attributes" && !obj.classInstance {
			continue
		}
		if jsonSObjectEmptyCanonicalIDShadowedByLowercase(fields, key) {
			continue
		}
		if jsonSObjectLowercaseIDShadowedByCanonical(fields, key) {
			continue
		}
		if ignoreUnknownClassFields {
			if _, _, declared := vm.lookupField(typeName, key); !declared {
				continue
			}
		}
		if typedObjectIsSObject {
			if handled, err := vm.applyDottedSObjectJSONField(&obj, typeName, key, jsonTypedInput{value: item, source: inputField.source, start: inputField.start}, strict); handled || err != nil {
				if err != nil {
					return Null, err
				}
				continue
			}
		}
		if typedObjectIsSObject {
			if relationshipType, ok := vm.jsonSObjectChildRelationshipType(typeName, key); ok {
				if value, handled, err := vm.jsonSObjectChildValueFromJSON(relationshipType, item, strict); handled {
					if err != nil {
						return Null, err
					}
					vm.markCollectionRefsEscaped(value)
					obj.Fields[key] = value
					vm.registerSObjectAliasField(obj, key, value)
					continue
				}
			}
		}
		if typedObjectIsSObject {
			if relationshipType, ok := vm.jsonSObjectParentRelationshipType(typeName, key); ok {
				value, handled, err := vm.capturedOwnerValueFromJSON(typeName, key, item, strict)
				if !handled {
					value, err = vm.jsonSObjectParentValueFromJSON(typeName, relationshipType, key, item, strict)
				}
				if err != nil {
					return Null, err
				}
				vm.setSObjectParentRelationshipValue(&obj, typeName, key, value)
				continue
			}
		}
		if typedObjectIsSObject {
			if fieldType, ok := vm.jsonSObjectFieldType(typeName, key); ok {
				value, err := vm.typedSchemaSObjectFieldValueFromJSON(typeName, key, fieldType, jsonTypedInput{value: item, source: inputField.source, start: inputField.start}, strict, false)
				if err != nil {
					return Null, err
				}
				if vm.jsonSObjectFieldIsIDLike(typeName, key) {
					value = markJSONSObjectIDValue(value)
				}
				fieldName := vm.resolveSObjectFieldName(typeName, key)
				vm.markCollectionRefsEscaped(value)
				obj.Fields[fieldName] = value
				vm.registerSObjectAliasField(obj, fieldName, value)
				markExplicitSObjectField(&obj, fieldName)
				continue
			}
		}
		if typedObjectIsSObject {
			fieldName := vm.resolveSObjectFieldName(typeName, key)
			if !strings.EqualFold(fieldName, key) {
				if fieldType, ok := vm.jsonSObjectFieldType(typeName, fieldName); ok {
					value, err := vm.typedSchemaSObjectFieldValueFromJSON(typeName, key, fieldType, jsonTypedInput{value: item, source: inputField.source, start: inputField.start}, strict, false)
					if err != nil {
						return Null, err
					}
					if vm.jsonSObjectFieldIsIDLike(typeName, fieldName) {
						value = markJSONSObjectIDValue(value)
					}
					vm.markCollectionRefsEscaped(value)
					obj.Fields[fieldName] = value
					vm.registerSObjectAliasField(obj, fieldName, value)
					markExplicitSObjectField(&obj, fieldName)
					continue
				}
			}
		}
		if field, owner, ok := vm.lookupField(typeName, key); ok && field.Type != "" {
			fieldType := vm.resolveTypeNameInClass(owner, field.Type)
			var value Value
			var err error
			if typedObjectIsSObject {
				value, err = vm.typedSchemaSObjectFieldValueFromJSON(typeName, key, fieldType, jsonTypedInput{value: item, source: inputField.source, start: inputField.start}, strict, false)
			} else {
				value, err = vm.typedApexFieldValueFromJSON(fieldType, inputField, strict)
			}
			if err != nil {
				if unsupported, ok := err.(*RuntimeError); ok && unsupported.Type == "UnsupportedFeature" && !strict {
					value = valueFromJSON(item)
				} else {
					return Null, err
				}
			}
			if field.Setter != nil {
				updated, err := vm.callReceiverSetterReturningReceiver(obj, *field.Setter, value)
				if err != nil {
					return Null, err
				}
				obj = updated
				continue
			}
			fieldName := field.Name
			if fieldName == "" {
				fieldName = key
			}
			vm.markCollectionRefsEscaped(value)
			if typedObjectIsSObject {
				fieldName = vm.resolveSObjectFieldName(typeName, fieldName)
				obj.Fields[fieldName] = value
				vm.registerSObjectAliasField(obj, fieldName, value)
				markExplicitSObjectField(&obj, fieldName)
				continue
			}
			obj.Fields[fieldName] = value
			vm.fillDuplicateFieldSlots(obj, owner, field, fieldName, value)
			continue
		}
		if fieldType, ok := platformJSONDTOFieldType(typeName, key); ok && platformDTO {
			var value Value
			var err error
			// Salesforce treats LeadConvertResult Id fields as opaque values
			// during JSON.deserialize. The value is returned unchanged by the
			// getLeadId/getAccountId/etc. accessors, even when a test fixture
			// uses a synthetic 18-character Id that would fail normal Id
			// validation. Keep strict Id parsing for ordinary Apex and other
			// DTO fields.
			if strings.EqualFold(key, "id") && isOpaqueApprovalResultJSONType(typeName) {
				// Approval result Id values are opaque response text. Salesforce
				// preserves synthetic/non-checksum IDs returned by API fixtures.
				value = valueFromJSON(item)
			} else if strings.EqualFold(key, "id") && isDatabaseResultJSONType(typeName) {
				// Salesforce accepts the DML-mock convention of a quoted
				// "null" id while deserializing a Database result. The invalid
				// text is rejected only when the result's Id is read.
				if text, ok := item.(string); ok && text == "null" {
					value = platformScalar("Id", text)
				} else if text, ok := item.(string); ok && platformJSONDTOAllowsOpaqueID(typeName) && validateApexIDShape(text) == nil {
					// Salesforce preserves shape-valid opaque Id text in these
					// result DTOs, including synthetic values without a checksum.
					// Keep it as response text so getId() does not apply ordinary
					// SObject Id checksum validation to the opaque value.
					value = valueFromJSON(item)
				} else {
					value, err = vm.typedValueFromJSON(fieldType, item, strict)
				}
			} else if isOpaqueLeadConvertResultIDField(typeName, fieldType) {
				if text, ok := item.(string); ok {
					value = platformScalar("Id", text)
				} else {
					value, err = vm.typedValueFromJSON(fieldType, item, strict)
				}
			} else {
				value, err = vm.typedValueFromJSON(fieldType, item, strict)
			}
			if err != nil {
				return Null, err
			}
			vm.markCollectionRefsEscaped(value)
			obj.Fields[platformJSONDTOFieldName(key)] = value
			continue
		}
		actualKey := key
		if typedObjectIsSObject {
			if handled, err := vm.discardUnknownSObjectJSONField(typeName, key, item); handled || err != nil {
				if err != nil {
					return Null, err
				}
				continue
			}
		}
		if existingKey, _, ok := objectFieldValue(obj, key); ok {
			actualKey = existingKey
		}
		obj.Fields[actualKey] = valueFromJSON(item)
		if typedObjectIsSObject {
			vm.registerSObjectAliasField(obj, actualKey, obj.Fields[actualKey])
		}
	}
	vm.hydrateJSONParentLookupFields(obj)
	return obj, nil
}

func isDatabaseResultJSONType(typeName string) bool {
	switch {
	case strings.EqualFold(typeName, "Database.SaveResult"),
		strings.EqualFold(typeName, "Database.DeleteResult"),
		strings.EqualFold(typeName, "Database.UndeleteResult"),
		strings.EqualFold(typeName, "Database.EmptyRecycleBinResult"),
		strings.EqualFold(typeName, "Database.LockResult"),
		strings.EqualFold(typeName, "Database.UnlockResult"),
		strings.EqualFold(typeName, "Database.LeadConvertResult"),
		strings.EqualFold(typeName, "Database.UpsertResult"),
		strings.EqualFold(typeName, "Database.MergeResult"),
		strings.EqualFold(typeName, "Approval.LockResult"),
		strings.EqualFold(typeName, "Approval.UnlockResult"):
		return true
	default:
		return false
	}
}

func isOpaqueApprovalResultJSONType(typeName string) bool {
	return strings.EqualFold(typeName, "Approval.LockResult") || strings.EqualFold(typeName, "Approval.UnlockResult")
}

func platformJSONDTOAllowsOpaqueID(typeName string) bool {
	switch {
	case strings.EqualFold(typeName, "Database.SaveResult"),
		strings.EqualFold(typeName, "Database.DeleteResult"),
		strings.EqualFold(typeName, "Database.EmptyRecycleBinResult"),
		strings.EqualFold(typeName, "Database.MergeResult"),
		strings.EqualFold(typeName, "Database.UndeleteResult"),
		strings.EqualFold(typeName, "Database.UpsertResult"):
		return true
	default:
		return false
	}
}

func isOpaqueLeadConvertResultIDField(typeName, fieldType string) bool {
	return strings.EqualFold(typeName, "Database.LeadConvertResult") && strings.EqualFold(fieldType, "Id")
}

func (vm *VM) callReceiverSetterReturningReceiver(receiver Value, setter Method, value Value) (Value, error) {
	if vm.Globals == nil {
		vm.Globals = make(map[string]Value)
	}
	key := "__glade_json_receiver"
	for {
		if _, exists := vm.Globals[key]; !exists {
			break
		}
		key += "_"
	}
	vm.Globals[key] = receiver
	_, err := vm.callMethodWithReceiver(setter, receiver, []Value{value}, resultForLookup())
	updated := vm.Globals[key]
	delete(vm.Globals, key)
	return updated, err
}

func (vm *VM) sortedJSONTypedObjectFields(typeName string, fields map[string]any) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		leftPriority := vm.jsonTypedObjectFieldPriority(typeName, keys[i])
		rightPriority := vm.jsonTypedObjectFieldPriority(typeName, keys[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return keys[i] < keys[j]
	})
	return keys
}

func (vm *VM) jsonTypedObjectFieldPriority(typeName, key string) int {
	if key == "attributes" {
		return -1
	}
	if field, _, ok := vm.lookupField(typeName, key); ok && field.Setter != nil {
		return 1
	}
	return 0
}

func (vm *VM) jsonObjectBaseValue(typeName string) (Value, error) {
	if class, ok := vm.lookupClass(typeName); ok && classHasZeroArgConstructor(class) {
		if class.PlatformType {
			value, err := vm.constructValue(typeName, nil, nil, resultForLookup())
			if err != nil {
				return Null, err
			}
			if value.Kind == ValueObject {
				vm.initializeJSONHeaderDefaults(&value, typeName)
				return value, nil
			}
		} else if err := vm.ensureClassInitialized(typeName); err != nil {
			return Null, err
		}
	}
	// Native source DTO deserialization bypasses constructors and instance
	// initializers, including ones that throw. Declared fields start at null.
	obj := Object(typeName)
	vm.initializeFields(&obj, typeName)
	obj.pluginBatchableDetached = vm.extendsPluginBatchable(typeName)
	if strings.EqualFold(typeName, "Invocable.Action.Result") {
		// The captured row and RDTO01/02: omitted Result properties are null,
		// including invocationParameters when outputParameters is supplied.
		fields, _ := platformJSONDTOFields(typeName)
		for name := range fields {
			obj.Fields[name] = Null
		}
	}
	vm.initializeJSONHeaderDefaults(&obj, typeName)
	return obj, nil
}

func (vm *VM) isPlatformJSONHeader(typeName string) bool {
	if !strings.EqualFold(typeName, "EventBus.ChangeEventHeader") {
		return false
	}
	class, registered := vm.lookupClass(typeName)
	return !registered || class.PlatformType
}

// Named source-62/67/68 JSON observations initialize omitted header numbers and
// lists, unlike ordinary user DTO fields. Keep these defaults on JSON only;
// constructing a header and namespaced contexts were not observed.
func (vm *VM) initializeJSONHeaderDefaults(value *Value, typeName string) {
	if !vm.isPlatformJSONHeader(typeName) {
		return
	}
	fields, _ := platformJSONDTOFields(typeName)
	for name, fieldType := range fields {
		initial := Null
		switch fieldType {
		case "Integer":
			initial = Int(0)
		case "Long":
			initial = longIntValue(0)
		case "List<String>":
			initial = List()
			initial.Type = fieldType
		}
		value.Fields[name] = initial
	}
}

func (vm *VM) applySObjectMetadataDefaults(record *Value) {
	if vm == nil || vm.Org == nil || record == nil || record.Kind != ValueObject {
		return
	}
	_, definition, ok := vm.describeObjectDefinition(record.Type)
	if !ok || definition.APIName == "" || len(definition.Fields) == 0 {
		return
	}
	for name, field := range definition.Fields {
		if _, _, exists := objectFieldValue(*record, name); exists {
			continue
		}
		defaultValue, ok := vm.defaultValueForNewSObjectField(definition, *record, field)
		if !ok {
			continue
		}
		putVMFieldPath(*record, name, vmValueFromStorage(defaultValue))
	}
}

func (vm *VM) markJSONDeserializedSObjectFields(record *Value, typeName string) {
	if record == nil || record.Kind != ValueObject {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]Value)
	}
	marker, ok := record.Fields[sobjectExplicitFieldsField]
	if !ok || marker.Kind != ValueMap {
		marker = Map()
		marker.Type = "Map<String,Boolean>"
	}
	marker.Runtime = jsonDeserializedSObjectFieldOrder
	record.Fields[sobjectExplicitFieldsField] = marker
	if vm == nil || vm.Org == nil {
		return
	}
	objectName, definition, ok := vm.sObjectDefinitionForRead(typeName)
	if !ok {
		return
	}
	definition = vm.describePreparedDefinition(objectName, definition)
	for name := range definition.Fields {
		markSetSObjectField(record, name)
	}
	for _, name := range []string{"Id", "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById", "SystemModstamp", "OwnerId", "IsDeleted"} {
		markSetSObjectField(record, name)
	}
}

func classHasZeroArgConstructor(class Class) bool {
	for _, ctor := range class.Constructors {
		if len(ctor.Params) == 0 {
			return true
		}
	}
	return false
}

func (vm *VM) sObjectValueFromJSON(raw any, strict bool) (Value, error) {
	fields, ok := jsonObjectMap(raw)
	if !ok {
		return Null, jsonTypeMappingError("sObject", raw)
	}
	typeName := "sObject"
	if attrs, ok := jsonObjectMap(fields["attributes"]); ok {
		if rawType, ok := attrs["type"].(string); ok && strings.TrimSpace(rawType) != "" {
			typeName = strings.TrimSpace(rawType)
		}
	}
	if typeName == "sObject" {
		return Null, jsonDeserializeException("Nested object for polymorphic foreign key must have an attributes field before any other fields.")
	}
	if vm.Org != nil {
		canonical, _, known := vm.sObjectDefinitionForRead(typeName)
		if !known {
			return Null, jsonDeserializeException("")
		}
		typeName = canonical
	}
	if strict {
		if err := vm.jsonStrictObjectFieldsError(typeName, raw); err != nil {
			return Null, err
		}
	}
	obj := Object(typeName)
	vm.initializeFields(&obj, typeName)
	// This entry point explicitly maps an SObject payload. A same-name class
	// may supply field initialization, but cannot turn the record into its instance.
	obj.classInstance = false
	vm.markJSONDeserializedSObjectFields(&obj, typeName)
	if strings.EqualFold(typeName, "User") {
		vm.initializeSObjectSchemaDefaults(&obj, typeName)
	}
	objectFields, _ := jsonSObjectFieldsInTextualOrder(raw)
	for _, field := range objectFields {
		key, item := field.name, field.value
		if key == "attributes" {
			continue
		}
		if !strict && vm.qualifiedTextFormulaJSONShadowsLocal(typeName, key, fields) {
			continue
		}
		if jsonSObjectEmptyCanonicalIDShadowedByLowercase(fields, key) {
			continue
		}
		if jsonSObjectLowercaseIDShadowedByCanonical(fields, key) {
			continue
		}
		if handled, err := vm.applyDottedSObjectJSONField(&obj, typeName, key, jsonTypedInput{value: item, source: field.source, start: field.start}, strict); handled || err != nil {
			if err != nil {
				return Null, err
			}
			continue
		}
		if relationshipType, ok := vm.jsonSObjectChildRelationshipType(typeName, key); ok {
			if value, handled, err := vm.jsonSObjectChildValueFromJSON(relationshipType, item, strict); handled {
				if err != nil {
					return Null, err
				}
				vm.markCollectionRefsEscaped(value)
				obj.Fields[key] = value
				vm.registerSObjectAliasField(obj, key, value)
				continue
			}
		}
		if relationshipType, ok := vm.jsonSObjectParentRelationshipType(typeName, key); ok {
			value, handled, err := vm.capturedOwnerValueFromJSON(typeName, key, item, strict)
			if !handled {
				value, err = vm.jsonSObjectParentValueFromJSON(typeName, relationshipType, key, item, strict)
			}
			if err != nil {
				return Null, err
			}
			vm.setSObjectParentRelationshipValue(&obj, typeName, key, value)
			continue
		}
		if fieldType, ok := vm.jsonSObjectFieldType(typeName, key); ok {
			value, err := vm.typedSchemaSObjectFieldValueFromJSON(typeName, key, fieldType, jsonTypedInput{value: item, source: field.source, start: field.start}, strict, true)
			if err != nil {
				return Null, err
			}
			if vm.jsonSObjectFieldIsIDLike(typeName, key) {
				value = markJSONSObjectIDValue(value)
			}
			fieldName := vm.resolveSObjectFieldName(typeName, key)
			vm.markCollectionRefsEscaped(value)
			obj.Fields[fieldName] = value
			vm.registerSObjectAliasField(obj, fieldName, value)
			markExplicitSObjectField(&obj, fieldName)
			continue
		}
		fieldName := vm.resolveSObjectFieldName(typeName, key)
		if !strings.EqualFold(fieldName, key) {
			if fieldType, ok := vm.jsonSObjectFieldType(typeName, fieldName); ok {
				value, err := vm.typedSchemaSObjectFieldValueFromJSON(typeName, key, fieldType, jsonTypedInput{value: item, source: field.source, start: field.start}, strict, true)
				if err != nil {
					return Null, err
				}
				if vm.jsonSObjectFieldIsIDLike(typeName, fieldName) {
					value = markJSONSObjectIDValue(value)
				}
				vm.markCollectionRefsEscaped(value)
				obj.Fields[fieldName] = value
				vm.registerSObjectAliasField(obj, fieldName, value)
				markExplicitSObjectField(&obj, fieldName)
				continue
			}
		}
		if handled, err := vm.discardUnknownSObjectJSONField(typeName, key, item); handled || err != nil {
			if err != nil {
				return Null, err
			}
			continue
		}
		obj.Fields[key] = valueFromJSON(item)
		vm.registerSObjectAliasField(obj, key, obj.Fields[key])
	}
	vm.hydrateJSONParentLookupFields(obj)
	return obj, nil
}

func (vm *VM) discardUnknownSObjectJSONField(typeName, key string, raw any) (bool, error) {
	if vm.allowOpenSObjectJSONFields(typeName) || jsonAllowedFieldContains(vm.jsonAllowedFields(typeName), key) {
		return false, nil
	}
	// R331/R335/R343-R355: unknown scalar/object fields are discarded, but
	// a nonempty array anywhere in that payload is not an SObject column.
	return true, jsonUnknownSObjectPayloadError(typeName, raw)
}

func jsonUnknownSObjectPayloadError(typeName string, raw any) error {
	switch value := raw.(type) {
	case []any:
		if len(value) > 0 {
			return jsonDeserializeException("No field name specified on column for sobject of type %s", typeName)
		}
	case orderedJSONObject:
		for _, field := range value {
			if err := jsonUnknownSObjectPayloadError(typeName, field.value); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range value {
			if err := jsonUnknownSObjectPayloadError(typeName, item); err != nil {
				return err
			}
		}
	}
	return nil
}

// Native format/roundTrip rows distinguish the currency field type from
// Decimal when a formatted string cannot be read back into an SObject field.
// Decoder twins retain field input boundaries for concrete targets; SObject.class
// reports the captured unavailable-source location instead.
func (vm *VM) typedSchemaSObjectFieldValueFromJSON(typeName, fieldName, fieldType string, input jsonTypedInput, strict, genericSObject bool) (Value, error) {
	if text, ok := input.value.(string); ok && strings.TrimSpace(text) != "" {
		if _, field, known := vm.jsonActiveSObjectField(typeName, fieldName); known && strings.EqualFold(field.DisplayType, "CURRENCY") {
			if _, err := decimalFromText(text); err != nil {
				message := fmt.Sprintf("Cannot deserialize instance of currency from VALUE_STRING value %s or request may be missing a required field", text)
				if !genericSObject && input.source != "" {
					// Currency errors identify the separator before the field,
					// not its key (sobject_json2_decoders rows 18-21). Derive
					// it from the key position shared by both decoder routes.
					boundary := input.start
					for boundary > 0 && strings.ContainsRune(" \t\r\n", rune(input.source[boundary-1])) {
						boundary--
					}
					if boundary > 0 && input.source[boundary-1] == ',' {
						boundary--
					} else {
						boundary = input.start
					}
					line, column := jsonInputPosition(input.source, boundary)
					return Null, jsonDeserializeException("%s at [line:%d, column:%d]", message, line, column)
				}
				return Null, jsonDeserializeException("%s at [Source: N/A; line: -1, column: -1][line:-1, column:-1]", message)
			}
		}
	}
	return vm.typedSObjectFieldValueFromJSON(fieldType, input, strict)
}

func (vm *VM) typedSObjectFieldValueFromJSON(fieldType string, raw any, strict bool) (Value, error) {
	input, positioned := raw.(jsonTypedInput)
	if positioned {
		raw = input.value
	}
	canonical := canonicalJSONScalarType(fieldType)
	switch canonical {
	case "String", "Boolean", "Integer", "Long", "Decimal", "Double", "Date", "Datetime", "Time", "Id", "Blob", "UUID":
		message := ""
		locationSeparator := " "
		switch raw.(type) {
		case map[string]any, orderedJSONObject:
			locationSeparator = " at "
			message = fmt.Sprintf("Cannot deserialize instance of %s from START_OBJECT value { or request may be missing a required field", strings.ToLower(canonical))
		case []any:
			message = fmt.Sprintf("Cannot deserialize instance of %s from START_ARRAY value", strings.ToLower(canonical))
		}
		if message != "" {
			if input.source != "" {
				// SObject scalar diagnostics identify the field key, not the
				// opening delimiter of its object or array value (SC014-SC016).
				line, column := jsonInputPosition(input.source, input.start)
				message += fmt.Sprintf("%s[line:%d, column:%d]", locationSeparator, line, column)
			}
			return Null, jsonDeserializeException("%s", message)
		}
	}
	if text, ok := raw.(string); ok && strings.TrimSpace(text) == "" {
		switch canonical {
		case "Decimal", "Double":
			return decimalFromText("0.0")
		case "Integer", "Long":
			return vm.typedValueFromJSON(fieldType, json.Number("0"), strict)
		case "Datetime":
			return Null, nil
		}
	}
	value, err := vm.typedValueFromJSON(fieldType, jsonTypedInput{value: raw, sObjectRecord: true}, strict)
	if err == nil && value.Kind == ValueDecimal && canonical == "Decimal" {
		// SObject Number/Currency values discard input scale but retain a
		// fractional digit for integral values.
		text := decimalDisplayText(value)
		if strings.Contains(text, ".") {
			text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
		}
		if !strings.Contains(text, ".") {
			text += ".0"
		}
		return decimalFromText(text)
	}
	if err != nil && canonical == "Date" {
		if text, ok := raw.(string); ok {
			return Null, jsonDeserializeException("Cannot deserialize instance of date from VALUE_STRING value %s or request may be missing a required field", text)
		}
	}
	if err == nil || !strings.EqualFold(fieldType, "Blob") {
		return value, err
	}
	text, ok := raw.(string)
	if !ok {
		return value, err
	}
	return platformScalar("Blob", text), nil
}

func (vm *VM) jsonSObjectParentValueFromJSON(containingType, typeName, fieldName string, raw any, strict bool) (Value, error) {
	if raw == nil {
		return Null, nil
	}
	fields, nested := jsonObjectFields(raw)
	if !nested {
		return Null, jsonDeserializeException("The value provided for foreign key reference %s is not a nested SObject", fieldName)
	}
	if !strict && vm.jsonOwnerNameProjection(containingType, fieldName, raw) {
		// A Name envelope retains the common parent schema, including explicit
		// null fields absent from the concrete User or Group target.
		typeName = "Name"
	}

	relation, polymorphic := vm.parentRelationshipMetadata(containingType, fieldName)
	nameProjection := false
	if !strict && polymorphic && relation.Polymorphic {
		if values, ok := jsonObjectMap(raw); ok {
			id, idOK := values["Id"].(string)
			concrete, known := vm.sObjectNameForID(id)
			if attrs, ok := jsonObjectMap(values["attributes"]); ok && idOK && known && validateApexID(id) == nil && relationReferencesObject(relation, concrete) {
				if target, ok := attrs["type"].(string); ok {
					if target == "Name" {
						typeName = "Name"
						nameProjection = true
					} else if strings.EqualFold(target, concrete) && relationReferencesObject(relation, target) {
						typeName = target
					}
				}
			}
		}
	}
	allowed := vm.jsonAllowedFields(typeName)
	// R390-R393: report the first unknown string field in input order.
	// jsonObjectFields also gives map-backed callers a deterministic fallback.
	for _, field := range fields {
		key, item := field.name, field.value
		if key == "attributes" || jsonAllowedFieldContains(allowed, key) {
			continue
		}
		if _, known := vm.jsonSObjectParentRelationshipType(typeName, key); known {
			continue
		}
		if _, known := vm.jsonSObjectChildRelationshipType(typeName, key); known {
			continue
		}
		if text, ok := item.(string); ok && !vm.allowOpenSObjectJSONFields(typeName) {
			return Null, jsonDeserializeException("Cannot deserialize instance of <unknown> from VALUE_STRING value %s or request may be missing a required field", text)
		}
	}
	value, err := vm.typedValueFromJSON(typeName, jsonTypedInput{value: raw, sObjectRecord: true}, strict)
	if err == nil && nameProjection {
		projection := String("")
		projection.Runtime = "Name"
		value.Fields[jsonSObjectQueryProjectionField] = projection
	}
	return value, err
}

func (vm *VM) jsonOwnerNameProjection(containingType, relationship string, raw any) bool {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" || containingType != "Case" || relationship != "Owner" {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	if !apexversion.AtLeast(version, 62) || apexversion.AtLeast(version, 69) {
		return false
	}
	fields, ok := jsonObjectMap(raw)
	if !ok {
		return false
	}
	attributes, ok := jsonObjectMap(fields["attributes"])
	if !ok || attributes["type"] != "Name" {
		return false
	}
	target, ok := fields["Type"].(string)
	if !ok || (target != "User" && target != "Group") {
		return false
	}
	id, ok := fields["Id"].(string)
	if !ok || validateApexID(id) != nil {
		return false
	}
	idType, known := vm.sObjectNameForID(id)
	if !known || idType != target {
		return false
	}
	relation, known := vm.parentRelationshipMetadata(containingType, relationship)
	return known && relation.Polymorphic && relationReferencesObject(relation, target)
}

// Native source62/66/67 observes non-strict object-valued class fields here.
// Nulls, scalar tokens, roots and strict deserialization retain their own paths.
func (vm *VM) jsonAbstractObjectFieldError(fieldType string, raw any, strict bool) error {
	if strict {
		return nil
	}
	if _, object := jsonObjectMap(raw); !object {
		return nil
	}
	typeName := vm.resolveJSONTypeName(fieldType)
	if class, known := vm.lookupClass(typeName); known {
		if class.IsInterface || (class.IsAbstract && !class.PlatformType) {
			name := class.Name
			if class.PlatformType {
				if qualified, system := typeExceptionSystemTypeName(name); system {
					name = qualified
				}
			}
			return newExceptionError("System.TypeException", "Cannot deserialize JSON as abstract type: "+name)
		}
		return nil
	}
	if platform, known := generatedPlatformTypes()[strings.ToLower(typeName)]; known && platform.Kind == apexast.DeclarationInterface {
		name := platform.Name
		if qualified, system := typeExceptionSystemTypeName(name); system {
			name = qualified
		}
		return newExceptionError("System.TypeException", "Cannot deserialize JSON as abstract type: "+name)
	}
	return nil
}

func (vm *VM) typedApexFieldValueFromJSON(fieldType string, field orderedJSONField, strict bool) (Value, error) {
	if err := vm.jsonAbstractObjectFieldError(fieldType, field.value, strict); err != nil {
		return Null, err
	}
	// N01/N09 capture the declared view of JSON-null user-class List fields
	// and automatic properties; the captured row covers ordinary declared fields.
	// Other JSON nulls keep their existing conversion, including root
	// collections and SObject fields.
	if field.value == nil && collectionBase(fieldType) == "List" && !strings.HasSuffix(strings.TrimSpace(fieldType), "[]") {
		if element, closed := collectionElementType(fieldType); closed {
			if class, known := vm.lookupClass(element); known && !class.PlatformType && !class.IsInterface && !class.IsAbstract && len(class.EnumValues) == 0 && !vm.isSObjectLikeType(element) {
				value := Null
				value.Type, value.Static = fieldType, fieldType
				return value, nil
			}
		}
	}
	// R042-R044/R046 and I001-I009: Integer class fields use the scalar token's
	// exact text. Root scalars and SObject fields retain their own conversions.
	if canonicalJSONScalarType(fieldType) == "Integer" {
		text, textual := field.value.(string)
		if number, numeric := field.value.(json.Number); numeric {
			text, textual = number.String(), true
		}
		if value, boolean := field.value.(bool); boolean {
			text, textual = strconv.FormatBool(value), true
		}
		if textual {
			value, err := strconv.ParseInt(text, 10, 32)
			if err == nil {
				return Int(value), nil
			}
			message := fmt.Sprintf("For input string: %q", text)
			if field.source != "" {
				// R043/R044/R046 and J003/J004/P001-P009 identify the
				// field's input boundary before its separating comma.
				line, column := jsonInputPosition(field.source, field.inputStart)
				message += fmt.Sprintf(" at [line:%d, column:%d]", line, column)
			}
			return Null, jsonDeserializeException("%s", message)
		}
	}
	if text, quoted := field.value.(string); quoted {
		message := ""
		offset := field.start
		switch canonicalJSONScalarType(fieldType) {
		case "Boolean":
			message = "Illegal value for boolean: " + text
			offset = field.end
		case "Date":
			if text == "" {
				message = `Invalid format: ""`
			}
		case "Decimal", "Double":
			if text == "" {
				message = "N/A"
			} else if strings.TrimSpace(text) == "" {
				message = fmt.Sprintf("Character %c is neither a decimal digit number, decimal point, nor \"e\" notation exponential mark.", []rune(text)[0])
			}
		}
		if message != "" {
			if field.source != "" {
				line, column := jsonInputPosition(field.source, offset)
				message += fmt.Sprintf(" at [line:%d, column:%d]", line, column)
			}
			return Null, jsonDeserializeException("%s", message)
		}
	}
	return vm.typedValueFromJSON(fieldType, jsonTypedInput{value: field.value, source: field.source, start: field.enumStart}, strict)
}

func (vm *VM) typedEnumJSONInput(typeName string, raw any, input jsonTypedInput) (Value, bool, error) {
	class, enum := vm.resolveEnumClass(typeName)
	if !enum || input.source == "" {
		return vm.typedEnumValueFromJSON(typeName, raw)
	}
	// K040-K043/K054/K055/K066/K067: a root enum expects an object and
	// maps that object to null. Enum fields and array items accept scalars.
	if input.root {
		if _, object := jsonObjectMap(raw); !object {
			return Null, true, jsonDeserializeException("Malformed JSON: Expected '{' at the beginning of object")
		}
		return Null, true, nil
	}
	text := ""
	switch item := raw.(type) {
	case string:
		if value, _, err := vm.typedEnumValueFromJSON(typeName, raw); err == nil {
			return value, true, nil
		}
		text = item
	case json.Number, bool:
		text = fmt.Sprint(item)
	default:
		return Null, true, jsonDeserializeException("Illegal value for primitive")
	}
	line, column := jsonInputPosition(input.source, input.start)
	return Null, true, jsonDeserializeException("The type %s does not have an enum value %s at [line:%d, column:%d]", class.Name, text, line, column)
}

// K098-K121: scalar map-key failures retain the key token's native location.
// Field/array conversion and non-JSON coercion keep their existing diagnostics.
func jsonMapKeyInputError(typeName, key string, input jsonTypedInput, err error) error {
	message := ""
	switch canonicalJSONScalarType(typeName) {
	case "Integer", "Long", "Double":
		message = fmt.Sprintf("For input string: %q", key)
		if strings.EqualFold(typeName, "Double") && strings.TrimSpace(key) == "" {
			message = "empty String"
		}
	case "Decimal":
		message = "N/A"
		if key != "" {
			message = fmt.Sprintf("Character %c is neither a decimal digit number, decimal point, nor \"e\" notation exponential mark.", []rune(key)[0])
		}
	case "Date", "Datetime", "Time":
		message = fmt.Sprintf("Invalid format: %q", key)
	case "Id":
		// K099/K105/K130/K131: map-key diagnostics retain the input text,
		// rather than the StringException wrapped by scalar Id conversion.
		message = "bad id " + key
	}
	if message == "" {
		return err
	}
	line, column := jsonInputPosition(input.source, input.start)
	return jsonDeserializeException("%s at [line:%d, column:%d]", message, line, column)
}

func jsonSObjectLowercaseIDShadowedByCanonical(fields map[string]any, key string) bool {
	if key == "Id" || !strings.EqualFold(key, "Id") {
		return false
	}
	canonical, ok := fields["Id"]
	if !ok {
		return false
	}
	if text, ok := canonical.(string); ok {
		return strings.TrimSpace(text) != ""
	}
	return canonical != nil
}

func jsonSObjectEmptyCanonicalIDShadowedByLowercase(fields map[string]any, key string) bool {
	if key != "Id" {
		return false
	}
	canonical, hasCanonical := fields["Id"]
	lower, hasLower := fields["id"]
	if !hasCanonical || !hasLower {
		return false
	}
	if text, ok := canonical.(string); ok && strings.TrimSpace(text) != "" {
		return false
	}
	if text, ok := lower.(string); ok {
		return strings.TrimSpace(text) != ""
	}
	return lower != nil
}

func (vm *VM) setSObjectParentRelationshipValue(obj *Value, typeName, relationshipName string, relationship Value) {
	if obj == nil || obj.Kind != ValueObject {
		return
	}
	if obj.Fields == nil {
		obj.Fields = make(map[string]Value)
	}
	vm.markCollectionRefsEscaped(relationship)
	obj.Fields[relationshipName] = relationship
	vm.registerSObjectAliasField(*obj, relationshipName, relationship)
	for _, alias := range vm.parentRelationshipValueAliases(typeName, relationshipName) {
		obj.Fields[alias] = relationship
		vm.registerSObjectAliasField(*obj, alias, relationship)
	}
}

func (vm *VM) parentRelationshipValueAliases(typeName, relationshipName string) []string {
	aliases := []string{relationshipName}
	if vm != nil && vm.Org != nil && vm.Org.Namespace != "" {
		aliases = append(aliases, storage.StripNamespaceToken(vm.Org.Namespace, relationshipName))
		aliases = append(aliases, storage.NamespaceTokenName(vm.Org.Namespace, relationshipName))
	}
	if vm == nil || vm.Org == nil {
		return uniqueNonEmptyStrings(aliases)
	}
	_, definition, ok := vm.sObjectDefinitionForRead(typeName)
	if !ok {
		return uniqueNonEmptyStrings(aliases)
	}
	for _, relation := range definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		aliases = append(aliases, relation.ParentRelationship)
		if vm.Org.Namespace != "" {
			aliases = append(aliases, storage.StripNamespaceToken(vm.Org.Namespace, relation.ParentRelationship))
			aliases = append(aliases, storage.NamespaceTokenName(vm.Org.Namespace, relation.ParentRelationship))
		}
	}
	return uniqueNonEmptyStrings(aliases)
}

func uniqueNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func (vm *VM) applyDottedSObjectJSONField(obj *Value, typeName, key string, item any, strict bool) (bool, error) {
	relationshipName, childPath, ok := strings.Cut(key, ".")
	if !ok || strings.TrimSpace(relationshipName) == "" || strings.TrimSpace(childPath) == "" {
		return false, nil
	}
	relationshipType, ok := vm.jsonSObjectParentRelationshipType(typeName, relationshipName)
	if !ok {
		return false, nil
	}
	actualRelationshipName := relationshipName
	relationship, exists := Null, false
	if actual, value, ok := objectFieldValue(*obj, relationshipName); ok {
		actualRelationshipName = actual
		relationship = value
		exists = true
	}
	if !exists || relationship.Kind == ValueNull {
		relationship = Object(relationshipType)
		vm.initializeFields(&relationship, relationshipType)
		relationship.classInstance = false // A dotted schema relationship is a record.
	}
	if relationship.Kind != ValueObject {
		return true, fmt.Errorf("JSON dotted relationship %s on %s is not an SObject", relationshipName, typeName)
	}
	if nested, err := vm.applyDottedSObjectJSONField(&relationship, relationshipType, childPath, item, strict); nested || err != nil {
		if err != nil {
			return true, err
		}
		vm.markCollectionRefsEscaped(relationship)
		obj.Fields[actualRelationshipName] = relationship
		vm.registerSObjectAliasField(*obj, actualRelationshipName, relationship)
		return true, nil
	}
	if fieldType, ok := vm.jsonSObjectFieldType(relationshipType, childPath); ok {
		value, err := vm.typedSObjectFieldValueFromJSON(fieldType, item, strict)
		if err != nil {
			return true, err
		}
		fieldName := vm.resolveSObjectFieldName(relationshipType, childPath)
		vm.markCollectionRefsEscaped(value)
		relationship.Fields[fieldName] = value
		vm.registerSObjectAliasField(relationship, fieldName, value)
		vm.markCollectionRefsEscaped(relationship)
		obj.Fields[actualRelationshipName] = relationship
		vm.registerSObjectAliasField(*obj, actualRelationshipName, relationship)
		return true, nil
	}
	if strict {
		return true, newExceptionError("JSONException", fmt.Sprintf("JSON.deserializeStrict found unknown field %q for %s", childPath, relationshipType))
	}
	if input, positioned := item.(jsonTypedInput); positioned {
		item = input.value
	}
	relationship.Fields[childPath] = valueFromJSON(item)
	vm.registerSObjectAliasField(relationship, childPath, relationship.Fields[childPath])
	vm.markCollectionRefsEscaped(relationship)
	obj.Fields[actualRelationshipName] = relationship
	vm.registerSObjectAliasField(*obj, actualRelationshipName, relationship)
	return true, nil
}

const (
	jsonChildQueryResultField         = "__glade_json_child_query_result"
	jsonQueriedChildRelationshipField = "__glade_json_queried_child_relationship"
)

// Child relationships remain ordinary Apex lists. Their private metadata is
// consumed only when they are serialized as a field of the containing SObject.
// Value cloning and collection coercion preserve Fields independently of List.
func markJSONChildQueryResult(value *Value, metadata map[string]Value, queried bool) {
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	value.Fields[jsonChildQueryResultField] = Value{Kind: ValueObject, Fields: metadata}
	value.Fields[jsonQueriedChildRelationshipField] = Bool(queried)
}

func (vm *VM) jsonSObjectChildValueFromJSON(typeName string, raw any, strict bool) (Value, bool, error) {
	if _, isArray := raw.([]any); isArray {
		return Null, true, jsonDeserializeException("QueryResult must start with '{'")
	}
	fields, isObject := jsonObjectMap(raw)
	if !isObject {
		return Null, false, nil
	}
	records, hasRecords := fields["records"]
	if !hasRecords {
		_, hasTotalSize := fields["totalSize"]
		_, hasDone := fields["done"]
		if !hasTotalSize && !hasDone {
			return Null, false, nil
		}
		// R068 accepts a QueryResult envelope with no records member.
		records = []any{}
	} else if _, isArray := records.([]any); !isArray {
		return Null, false, nil
	}
	value, err := vm.typedValueFromJSON(typeName, jsonTypedInput{value: records, sObjectRecord: true}, strict)
	if err != nil {
		return Null, true, err
	}
	metadata := make(map[string]Value, 2)
	for _, name := range []string{"totalSize", "done"} {
		if item, present := fields[name]; present {
			metadata[name] = valueFromJSON(item)
		}
	}
	// R065-R067 retain the supplied metadata even when it disagrees with
	// the number of records. Do not derive it from the materialized list.
	markJSONChildQueryResult(&value, metadata, false)
	return value, true, nil
}

func jsonQueryResultRecords(raw any) ([]any, bool) {
	fields, ok := jsonObjectMap(raw)
	if !ok {
		return nil, false
	}
	records, ok := fields["records"].([]any)
	return records, ok
}

func (vm *VM) jsonSObjectFieldType(typeName, fieldName string) (string, bool) {
	if strings.EqualFold(fieldName, "Id") {
		return "String", true
	}
	if fieldType, ok := jsonSObjectSystemFieldType(fieldName); ok {
		return fieldType, true
	}
	if vm.Org == nil {
		return "", false
	}
	_, definition, ok := vm.sObjectDefinitionForRead(typeName)
	if !ok {
		return "", false
	}
	fieldName = vm.resolveSObjectFieldName(typeName, fieldName)
	_, field, ok := storage.ResolveFieldDefinition(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return "", false
	}
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return "String", true
	case storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist:
		return "String", true
	case storage.FieldBoolean:
		return "Boolean", true
	case storage.FieldInteger:
		return "Integer", true
	case storage.FieldDecimal:
		return "Decimal", true
	case storage.FieldLocation, storage.FieldAddress:
		return "Object", true
	case storage.FieldBlob:
		return "Blob", true
	case storage.FieldDate:
		return "Date", true
	case storage.FieldDateTime:
		return "Datetime", true
	case storage.FieldCalculated, storage.FieldSummary:
		switch strings.ToUpper(strings.TrimSpace(field.DisplayType)) {
		case "INTEGER":
			return "Integer", true
		case "DECIMAL", "DOUBLE", "CURRENCY", "PERCENT":
			return "Decimal", true
		case "BOOLEAN":
			return "Boolean", true
		case "DATE":
			return "Date", true
		case "DATETIME":
			return "Datetime", true
		case "ID", "REFERENCE":
			return "String", true
		case "STRING", "TEXTAREA", "PICKLIST":
			return "String", true
		}
		return "String", true
	default:
		return "", false
	}
}

func (vm *VM) jsonSObjectFieldIsIDLike(typeName, fieldName string) bool {
	if strings.EqualFold(fieldName, "Id") {
		return true
	}
	_, field, ok := vm.sObjectFieldDefinition(typeName, fieldName)
	if !ok {
		return false
	}
	return field.Type == storage.FieldID || field.Type == storage.FieldReference
}

func markJSONSObjectIDValue(value Value) Value {
	if value.Kind == ValueString {
		value.Type = "Id"
	}
	return value
}

func (vm *VM) typedEnumValueFromJSON(typeName string, raw any) (Value, bool, error) {
	text, ok := raw.(string)
	if !ok {
		return Null, false, nil
	}
	if value, ok := schemaDisplayTypeStaticValue("Schema.DisplayType." + text); ok &&
		(strings.EqualFold(typeName, "Schema.DisplayType") || strings.EqualFold(typeName, "DisplayType")) {
		return value, true, nil
	}
	if value, ok := schemaSOAPTypeStaticValue("Schema.SOAPType." + text); ok &&
		(strings.EqualFold(typeName, "Schema.SOAPType") || strings.EqualFold(typeName, "SOAPType")) {
		return value, true, nil
	}
	if class, ok := vm.resolveEnumClass(typeName); ok {
		for i, candidate := range class.EnumValues {
			if strings.EqualFold(candidate, text) {
				value := Value{Kind: ValueObject, Type: class.Name, Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}
				return value, true, nil
			}
		}
		return Null, true, newExceptionError("System.NoSuchElementException", fmt.Sprintf("No enum value found called %s", text))
	}
	return Null, false, nil
}

func jsonSObjectSystemFieldType(fieldName string) (string, bool) {
	switch {
	case strings.EqualFold(fieldName, "CreatedDate"),
		strings.EqualFold(fieldName, "LastModifiedDate"),
		strings.EqualFold(fieldName, "SystemModstamp"):
		return "Datetime", true
	case strings.EqualFold(fieldName, "CreatedById"),
		strings.EqualFold(fieldName, "LastModifiedById"),
		strings.EqualFold(fieldName, "OwnerId"):
		return "String", true
	case strings.EqualFold(fieldName, "IsDeleted"):
		return "Boolean", true
	default:
		return "", false
	}
}

func (vm *VM) jsonSObjectParentRelationshipType(typeName, relationshipName string) (string, bool) {
	if vm.Org == nil {
		return "", false
	}
	_, definition, ok := vm.sObjectDefinitionForRead(typeName)
	if !ok {
		return "", false
	}
	for _, relation := range definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		if len(relation.ParentObjects) == 0 {
			continue
		}
		return relation.ParentObjects[0], true
	}
	if relation, ok := vm.syntheticParentRelationship(definition, relationshipName); ok {
		if len(relation.ParentObjects) == 0 {
			return "", false
		}
		return relation.ParentObjects[0], true
	}
	return "", false
}

func (vm *VM) jsonSObjectChildRelationshipType(typeName, relationshipName string) (string, bool) {
	vm.ensureMetadataCacheGeneration()
	if vm.Org == nil {
		return "", false
	}
	parentObject, _, ok := vm.sObjectDefinitionForRead(typeName)
	if !ok {
		return "", false
	}
	// The completed parent plan includes every alias and answers negative
	// lookups too. Retaining another entry for each query duplicates its values
	// and lets arbitrary misses grow the generation's live working set.
	if vm.jsonChildRelTypeCache == nil {
		vm.jsonChildRelTypeCache = newJSONChildRelTypeLookupCache()
	}
	parentKey := strings.ToLower(strings.TrimSpace(parentObject))
	relationshipKey := strings.ToLower(strings.TrimSpace(relationshipName))
	if index, ok := vm.jsonChildRelTypeCache.loadParent(parentKey); ok {
		if cached, ok := index[relationshipKey]; ok {
			return cached.Type, cached.OK
		}
		return "", false
	}
	index := vm.buildJSONSObjectChildRelationshipTypeIndex(parentObject)
	vm.jsonChildRelTypeCache.storeParent(parentKey, index)
	if cached, ok := index[relationshipKey]; ok {
		return cached.Type, cached.OK
	}
	return "", false
}

func (vm *VM) buildJSONSObjectChildRelationshipTypeIndex(parentObject string) map[string]jsonRelationshipTypeLookup {
	jsonChildRelationshipIndexBuilds.Add(1)
	index := make(map[string]jsonRelationshipTypeLookup)
	// Most aliases have exactly one child. Keep their final value directly;
	// only ambiguous aliases need the ordered, case-folded candidate list.
	var ambiguous map[string][]string
	addMatch := func(name, childName, childType string) {
		for _, alias := range jsonRelationshipNameLookupKeys(vm.Org.Namespace, name) {
			previous, exists := index[alias]
			if !exists {
				index[alias] = jsonRelationshipTypeLookup{Type: childType, OK: true}
				continue
			}
			if previous.Type == childType {
				continue
			}
			if ambiguous == nil {
				ambiguous = make(map[string][]string)
			}
			matches := ambiguous[alias]
			if len(matches) == 0 {
				matches = []string{previous.Type[len("List<") : len(previous.Type)-1]}
			}
			ambiguous[alias] = appendUniqueStringFold(matches, childName)
		}
	}
	names := vm.childRelationshipCandidateNames(parentObject)
	// The parent can itself be a child (for example a standard object's parent
	// lookup). A catalog-only parent participates without being installed in Org.
	var catalogDefinition storage.ObjectDefinition
	catalogOnly := false
	if _, installed := vm.Org.Objects[parentObject]; !installed {
		_, catalogDefinition, catalogOnly = vm.sObjectDefinitionForRead(parentObject)
	}
	// Preserve the original global relation-before-field order: folded duplicate
	// object keys keep the first matching canonical spelling. Read definitions
	// directly in both passes without constructing temporary ObjectState maps.
	for _, relations := range [2]bool{true, false} {
		for _, childName := range names {
			state, ok := vm.Org.Objects[childName]
			if !ok || storage.IsChangeEventObject(state.Definition) {
				continue
			}
			vm.addJSONChildRelationshipDefinition(parentObject, childName, state.Definition, relations, addMatch)
		}
		if catalogOnly {
			vm.addJSONChildRelationshipDefinition(parentObject, parentObject, catalogDefinition, relations, addMatch)
		}
	}
	for relationshipKey, matches := range ambiguous {
		if childName := vm.bestChildRelationshipObject(matches); childName != "" {
			index[relationshipKey] = jsonRelationshipTypeLookup{Type: "List<" + childName + ">", OK: true}
		}
	}
	return index
}

func (vm *VM) addJSONChildRelationshipDefinition(parentObject, childName string, definition storage.ObjectDefinition, relations bool, addMatch func(string, string, string)) {
	if strings.TrimSpace(childName) == "" {
		return
	}
	// Relations and fields often repeat the same child name. Expand each exact
	// schema spelling once per child, without per-reference slices or strings.
	seen := make(map[string]bool)
	childType := "List<" + childName + ">"
	add := func(name string) {
		if strings.TrimSpace(name) == "" || seen[name] {
			return
		}
		seen[name] = true
		addMatch(name, childName, childType)
	}
	derived := derivedVMChildRelationshipName(definition)
	if relations {
		for _, relation := range definition.Relations {
			if !relationshipTargetsObject(relation, parentObject) {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" {
				childRelationshipName = derived
			}
			add(childRelationshipName)
		}
		return
	}
	for _, field := range definition.Fields {
		if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		if !relationshipTargetsObject(storage.Relationship{ParentObjects: field.ReferenceTo}, parentObject) {
			continue
		}
		// Match vmFieldChildRelationshipNames: coalesce aliases within this
		// field using EqualFold before expanding their lowercase lookup keys.
		// EqualFold-equivalent Unicode spellings can have distinct lowercase
		// keys. The exact-name set above only removes repeated work across
		// fields; it must not change which aliases each field contributes.
		explicit := field.ChildRelationshipName
		stored := storage.ChildRelationshipName(field)
		add(explicit)
		if !strings.EqualFold(stored, explicit) {
			add(stored)
		}
		if !strings.EqualFold(derived, explicit) && !strings.EqualFold(derived, stored) {
			add(derived)
		}
	}
}

// childRelationshipScanVisits counts the child objects a relationship lookup
// scanned; perf guards compare it with the number of possible children.
var childRelationshipScanVisits atomic.Uint64

// childRelationshipScanCandidates preserves a name-set snapshot and ordinary
// map visitation without allocating one ObjectState per candidate. Its sole
// caller reads definitions synchronously and performs no schema writes during
// traversal, so current definitions can be read when each name is yielded.
func (vm *VM) childRelationshipScanCandidates(parentObject string) iter.Seq2[string, storage.ObjectState] {
	names := vm.childRelationshipCandidateNames(parentObject)
	var org *storage.OrgState
	if vm != nil {
		org = vm.Org
	}
	children := make(map[string]struct{}, len(names))
	for _, name := range names {
		if org != nil {
			if _, ok := org.Objects[name]; ok {
				children[name] = struct{}{}
			}
		}
	}
	return func(yield func(string, storage.ObjectState) bool) {
		if org == nil {
			return
		}
		for name := range children {
			state, ok := org.Objects[name]
			if ok && !yield(name, state) {
				return
			}
		}
	}
}

// Template orgs promise frozen definitions, so their candidate index is shared
// by every runtime clone. A mutable host org indexes per VM and rebuilds when
// its schema generation or object count changes: every in-place definition
// change clears the schema stamp (and so bumps the generation), and installing
// an object changes the count, so an uncached parent still sees added schema.
func (vm *VM) childRelationshipCandidateTable() map[string][]string {
	if vm == nil || vm.Org == nil {
		return nil
	}
	vm.ensureMetadataCacheGeneration()
	if vm.Org.RuntimeSchemaStamp == "" {
		return vm.mutableOrgChildRelationshipCandidates()
	}
	if vm.jsonChildRelTypeCache == nil {
		vm.jsonChildRelTypeCache = newJSONChildRelTypeLookupCache()
	}
	cache := vm.jsonChildRelTypeCache
	cache.candidatesOnce.Do(func() {
		cache.childCandidates = buildChildRelationshipCandidates(vm.Org)
	})
	return cache.childCandidates
}

func (vm *VM) childRelationshipCandidateNames(parentObject string) []string {
	if vm == nil || vm.Org == nil {
		return nil
	}
	table := vm.childRelationshipCandidateTable()
	keys := jsonRelationshipParentCandidateKeys(parentObject)
	return childRelationshipNamesForKeys(table, keys)
}

// JSON, DML, and lazy lookup already used lowercase candidate narrowing on the
// baseline. Loaded lookup instead scanned every object and used EqualFold, so
// only that route needs its original scan for exceptional Unicode fold pairs.
const loadedChildRelationshipFullScanKey = "\x00loaded-relationship-full-scan"

func (vm *VM) loadedChildRelationshipCandidateNames(parentObject string) []string {
	if vm == nil || vm.Org == nil {
		return nil
	}
	table := vm.childRelationshipCandidateTable()
	keys := jsonRelationshipParentCandidateKeys(parentObject)
	_, fullScan := table[loadedChildRelationshipFullScanKey]
	if !fullScan {
		for _, key := range keys {
			if loadedChildRelationshipParentFoldKey(key) != key {
				fullScan = true
				break
			}
		}
	}
	if fullScan {
		names := make([]string, 0, len(vm.Org.Objects))
		for name := range vm.Org.Objects {
			names = append(names, name)
		}
		return names
	}
	return childRelationshipNamesForKeys(table, keys)
}

func childRelationshipNamesForKeys(table map[string][]string, keys []string) []string {
	if len(keys) == 1 {
		return table[keys[0]]
	}
	seen := make(map[string]struct{})
	var names []string
	for _, key := range keys {
		for _, name := range table[key] {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

// A mutable host may edit fields and relation slices in place. Candidate
// content belongs to exactly one org installation/schema generation; container
// identity never permits carrying parent keys across that boundary. Frozen
// templates instead share buildChildRelationshipCandidates through their
// generation-owned jsonChildRelTypeLookupCache.
type childRelationshipCandidateIndex struct {
	org         *storage.OrgState
	generation  uint64
	objectCount int
	candidates  map[string][]string
}

func (vm *VM) mutableOrgChildRelationshipCandidates() map[string][]string {
	index := vm.childRelCandidates
	if index == nil || index.org != vm.Org || index.generation != vm.Org.SchemaGeneration() || index.objectCount != len(vm.Org.Objects) {
		index = &childRelationshipCandidateIndex{
			org: vm.Org, generation: vm.Org.SchemaGeneration(), objectCount: len(vm.Org.Objects),
			candidates: buildChildRelationshipCandidates(vm.Org),
		}
		vm.childRelCandidates = index
	}
	return index.candidates
}

// Build only from this generation's content. Even unchanged container
// addresses may contain entirely different reference targets after a host edit.
// Each child name is recorded once per parent key. Callers read the live
// definition; this index does not retain relations, fields, or records.
func buildChildRelationshipCandidates(org *storage.OrgState) map[string][]string {
	if org == nil {
		return nil
	}
	childCandidates := make(map[string][]string)
	keysByParent := make(map[string][]string)
	parents := make(map[string]bool, 8)
	for childName, child := range org.Objects {
		if storage.IsChangeEventObject(child.Definition) {
			continue
		}
		clear(parents)
		add := func(parent string) {
			keys, ok := keysByParent[parent]
			if !ok {
				keys = jsonRelationshipParentCandidateKeys(parent)
				keysByParent[parent] = keys
				for _, key := range keys {
					if loadedChildRelationshipParentFoldKey(key) != key {
						childCandidates[loadedChildRelationshipFullScanKey] = nil
						break
					}
				}
			}
			for _, key := range keys {
				parents[key] = true
			}
		}
		for _, relation := range child.Definition.Relations {
			for _, parent := range relation.ParentObjects {
				add(parent)
			}
		}
		for _, field := range child.Definition.Fields {
			if field.Type == storage.FieldReference {
				for _, parent := range field.ReferenceTo {
					add(parent)
				}
			}
		}
		for parent := range parents {
			childCandidates[parent] = append(childCandidates[parent], childName)
		}
	}
	return childCandidates
}

// Include both namespace reductions used by relationshipTargetsObject. The
// original predicate still filters candidates, preserving polymorphic aliases.
func jsonRelationshipParentCandidateKeys(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	keys := []string{strings.ToLower(name)}
	for _, alias := range []string{stripAnyNamespaceToken(name), stripStandardObjectNamespaceToken(name)} {
		key := strings.ToLower(alias)
		found := false
		for _, existing := range keys {
			if existing == key {
				found = true
				break
			}
		}
		if !found {
			keys = append(keys, key)
		}
	}
	return keys
}

// Lowercase separates long-s/S and final-sigma/sigma. Detect exceptional fold
// keys for loaded lookup's full-org fallback without widening other routes.
func loadedChildRelationshipParentFoldKey(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] < 0x80 {
			continue
		}
		var key strings.Builder
		key.Grow(len(name))
		for _, r := range name {
			canonical := r
			for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
				if folded < canonical {
					canonical = folded
				}
			}
			key.WriteRune(unicode.ToLower(canonical))
		}
		return key.String()
	}
	return strings.ToLower(name)
}

func jsonRelationshipNameLookupKeys(namespace, name string) []string {
	seen := make(map[string]bool, 6)
	keys := make([]string, 0, 6)
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		keys = append(keys, value)
	}
	addVariant := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		add(value)
		if hasSuffixFold(value, "__r") {
			add(value[:len(value)-3])
		} else {
			add(value + "__r")
		}
	}
	addVariant(name)
	addVariant(stripAnyNamespaceToken(name))
	if strings.TrimSpace(namespace) != "" {
		addVariant(storage.StripNamespaceToken(namespace, name))
		addVariant(storage.NamespaceTokenName(namespace, name))
		addVariant(jsonNamespacedRelationshipLookupName(namespace, name))
	}
	return keys
}

func jsonNamespacedRelationshipLookupName(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" || strings.Contains(name, "__") {
		return name
	}
	return namespace + "__" + name
}

func (vm *VM) bestChildRelationshipObject(matches []string) string {
	if len(matches) == 0 {
		return ""
	}
	best := matches[0]
	for _, candidate := range matches[1:] {
		if vm.childRelationshipObjectLess(candidate, best) {
			best = candidate
		}
	}
	return best
}

func (vm *VM) childRelationshipObjectLess(left, right string) bool {
	leftPriority := vm.childRelationshipObjectPriority(left)
	rightPriority := vm.childRelationshipObjectPriority(right)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if vm != nil && vm.Org != nil {
		leftState, leftOK := vm.Org.Objects[left]
		rightState, rightOK := vm.Org.Objects[right]
		if leftOK && rightOK {
			leftScore := len(leftState.Definition.Fields) + len(leftState.Definition.Relations)
			rightScore := len(rightState.Definition.Fields) + len(rightState.Definition.Relations)
			if leftScore != rightScore {
				return leftScore > rightScore
			}
		}
	}
	return left < right
}

func (vm *VM) childRelationshipObjectPriority(objectName string) int {
	if vm != nil && vm.Org != nil && vm.Org.Namespace != "" {
		if hasPrefixFold(objectName, strings.ToLower(vm.Org.Namespace)+"__") && isCustomObjectLikeName(objectName) {
			return 0
		}
	}
	if isCustomObjectLikeName(objectName) {
		return 1
	}
	if isCommonSObjectTypeName(objectName) {
		return 2
	}
	return 3
}

func vmFieldChildRelationshipNames(definition storage.ObjectDefinition, field storage.Field) []string {
	names := []string(nil)
	if field.ChildRelationshipName != "" {
		names = appendUniqueStringFold(names, field.ChildRelationshipName)
	}
	if childRelationshipName := storage.ChildRelationshipName(field); childRelationshipName != "" {
		names = appendUniqueStringFold(names, childRelationshipName)
	}
	if derived := derivedVMChildRelationshipName(definition); derived != "" {
		names = appendUniqueStringFold(names, derived)
	}
	return names
}

func (vm *VM) isJSONTypedObjectTarget(typeName string) bool {
	typeName = vm.resolveJSONTypeName(typeName)
	if _, ok := vm.lookupClass(typeName); ok {
		return true
	}
	if vm.isSObjectLikeType(typeName) {
		return true
	}
	if strings.EqualFold(typeName, "Schema.FieldSetMember") {
		return true
	}
	if _, ok := platformJSONDTOFields(typeName); ok {
		return true
	}
	if vm.Org != nil {
		if _, ok := vm.Org.Objects[typeName]; ok {
			return true
		}
	}
	return false
}

func (vm *VM) resolveJSONTypeName(typeName string) string {
	if resolved, ok := vm.resolveClassName(typeName); ok {
		return resolved
	}
	if alias, ok := platformShortTypeAlias(typeName); ok {
		return alias
	}
	if canonical := canonicalRuntimeTypeName(typeName); !strings.EqualFold(canonical, typeName) {
		return canonical
	}
	return typeName
}

func platformJSONDTOFields(typeName string) (map[string]string, bool) {
	resultFields := map[string]string{
		"success":           "Boolean",
		"id":                "Id",
		"errors":            "List<Database.Error>",
		"created":           "Boolean",
		"mergedRecordIds":   "List<Id>",
		"updatedRelatedIds": "List<Id>",
	}
	switch {
	case strings.EqualFold(typeName, "Invocable.Action.Result"):
		// RDTO01/02: a JSON-created Result retains its empty output map,
		// success flag and typed errors. The captured rows still reject
		// populated Object-valued maps before this DTO field mapping.
		return map[string]string{
			"success":              "Boolean",
			"outputParameters":     "Map<String,Object>",
			"invocationParameters": "Map<String,Object>",
			"errors":               "List<Invocable.Action.Error>",
			"action":               "Invocable.Action",
		}, true
	case strings.EqualFold(typeName, "Invocable.Action.Error"):
		// RDTO02 covers standalone and Result-contained Error hydration.
		return map[string]string{"code": "String", "message": "String"}, true
	case strings.EqualFold(typeName, "EventBus.ChangeEventHeader"):
		return map[string]string{
			"entityName": "String", "changeType": "String", "changeOrigin": "String", "transactionKey": "String",
			"sequenceNumber": "Integer", "commitTimestamp": "Long", "commitUser": "String", "commitNumber": "Long",
			"recordIds": "List<String>", "changedFields": "List<String>", "nulledFields": "List<String>", "diffFields": "List<String>",
		}, true
	case strings.EqualFold(typeName, "Database.SaveResult"),
		strings.EqualFold(typeName, "Database.DeleteResult"),
		strings.EqualFold(typeName, "Database.UndeleteResult"),
		strings.EqualFold(typeName, "Database.EmptyRecycleBinResult"),
		strings.EqualFold(typeName, "Database.LockResult"),
		strings.EqualFold(typeName, "Database.UnlockResult"),
		strings.EqualFold(typeName, "Approval.LockResult"),
		strings.EqualFold(typeName, "Approval.UnlockResult"),
		strings.EqualFold(typeName, "Database.UpsertResult"),
		strings.EqualFold(typeName, "Database.MergeResult"):
		return resultFields, true
	case strings.EqualFold(typeName, "Approval.ProcessResult"):
		return map[string]string{
			"success":        "Boolean",
			"entityId":       "String",
			"instanceId":     "String",
			"instanceStatus": "String",
			"actorIds":       "List<Id>",
			"newWorkitemIds": "List<Id>",
			"errors":         "List<Database.Error>",
		}, true
	case strings.EqualFold(typeName, "Database.LeadConvertResult"):
		return map[string]string{
			"success":                "Boolean",
			"leadId":                 "Id",
			"accountId":              "Id",
			"contactId":              "Id",
			"opportunityId":          "Id",
			"relatedPersonAccountId": "Id",
			"errors":                 "List<Database.Error>",
		}, true
	case strings.EqualFold(typeName, "Database.Error"):
		return map[string]string{
			"message":              "String",
			"statusCode":           "String",
			"fields":               "List<String>",
			"extendedErrorDetails": "List<Object>",
		}, true
	default:
		return nil, false
	}
}

func platformJSONDTOFieldType(typeName, field string) (string, bool) {
	fields, ok := platformJSONDTOFields(typeName)
	if !ok {
		return "", false
	}
	for candidate, fieldType := range fields {
		if strings.EqualFold(candidate, field) {
			return fieldType, true
		}
	}
	return "", false
}

func platformJSONDTOFieldName(field string) string {
	switch {
	case strings.EqualFold(field, "statusCode"):
		return "statusCode"
	case strings.EqualFold(field, "mergedRecordIds"):
		return "mergedRecordIds"
	case strings.EqualFold(field, "updatedRelatedIds"):
		return "updatedRelatedIds"
	case field == "":
		return field
	default:
		return strings.ToLower(field[:1]) + field[1:]
	}
}

func typedScalarFromJSON(typeName string, raw any) (Value, bool, error) {
	canonical := canonicalJSONScalarType(typeName)
	if raw == nil {
		return Null, true, nil
	}
	switch canonical {
	case "String":
		switch value := raw.(type) {
		case string:
			return String(value), true, nil
		case json.Number:
			return String(value.String()), true, nil
		case float64:
			return String(strconv.FormatFloat(value, 'f', -1, 64)), true, nil
		case bool:
			if value {
				return String("true"), true, nil
			}
			return String("false"), true, nil
		default:
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
	case "Boolean":
		value, ok := raw.(bool)
		if !ok {
			if text, textOK := raw.(string); textOK {
				parsed, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(text)))
				if err == nil {
					return Bool(parsed), true, nil
				}
			}
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		return Bool(value), true, nil
	case "Integer", "Long":
		value, ok := jsonTruncatedIntegralNumber(raw)
		if !ok {
			if text, textOK := raw.(string); textOK {
				parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
				if err == nil {
					if canonical == "Long" {
						return longIntValue(parsed), true, nil
					}
					return Int(parsed), true, nil
				}
			}
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		if canonical == "Long" {
			return longIntValue(value), true, nil
		}
		if value < -2147483648 || value > 2147483647 {
			return Null, true, jsonParserException("Numeric value (%v) out of range of int", raw)
		}
		return Int(value), true, nil
	case "Decimal", "Double":
		if number, ok := raw.(json.Number); ok {
			if canonical == "Decimal" {
				decimal, err := decimalFromText(number.String())
				if err != nil {
					return Null, true, jsonTypeMappingError(canonical, raw)
				}
				return decimal, true, nil
			}
			parsed, err := strconv.ParseFloat(number.String(), 64)
			if err != nil {
				return Null, true, jsonTypeMappingError(canonical, raw)
			}
			decimal := decimalAsDouble(Decimal(parsed))
			return decimal, true, nil
		}
		value, ok := jsonDecimalNumber(raw)
		if !ok {
			if text, textOK := raw.(string); textOK {
				trimmed := strings.TrimSpace(text)
				if trimmed == "" {
					return Null, true, nil
				}
				if canonical == "Decimal" {
					decimal, err := decimalFromText(trimmed)
					if err == nil {
						return decimal, true, nil
					}
				} else if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
					return decimalAsDouble(Decimal(parsed)), true, nil
				}
			}
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		if canonical == "Double" {
			return decimalAsDouble(Decimal(value)), true, nil
		}
		return Decimal(value), true, nil
	case "Date":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		value, err := parseJSONDateText(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", jsonDateDeserializeErrorMessage(text))
		}
		return platformScalar("Date", value.Format("2006-01-02")), true, nil
	case "Datetime":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		value, err := parseDatetimeTextAllowDateOnly(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Datetime", value.UTC().Format(time.RFC3339Nano)), true, nil
	case "Time":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		value, err := parseTimeText(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Time", value), true, nil
	case "Id":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		if err := validateApexID(text); err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return platformScalar("Id", text), true, nil
	case "Blob":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return Null, true, jsonDeserializeException("JSON.deserialize cannot decode Blob base64: %v", err)
		}
		return platformScalar("Blob", string(decoded)), true, nil
	case "UUID":
		text, ok := raw.(string)
		if !ok {
			return Null, true, jsonTypeMappingError(canonical, raw)
		}
		parsed, err := parseUUIDText(text)
		if err != nil {
			return Null, true, jsonDeserializeException("%s", err.Error())
		}
		return uuidValue(parsed), true, nil
	}
	return Null, false, nil
}

// jsonChildRelTypeLookupCache memoizes child types and parent indexes for one
// org schema generation, shared by runtime clones with that frozen schema.
type jsonChildRelTypeLookupCache struct {
	mu              sync.RWMutex
	entries         map[string]jsonRelationshipTypeLookup
	parentIndexes   map[string]map[string]jsonRelationshipTypeLookup
	candidatesOnce  sync.Once
	childCandidates map[string][]string
}

func newJSONChildRelTypeLookupCache() *jsonChildRelTypeLookupCache {
	return &jsonChildRelTypeLookupCache{
		entries:       make(map[string]jsonRelationshipTypeLookup),
		parentIndexes: make(map[string]map[string]jsonRelationshipTypeLookup),
	}
}

func (c *jsonChildRelTypeLookupCache) load(key string) (jsonRelationshipTypeLookup, bool) {
	if c == nil {
		return jsonRelationshipTypeLookup{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *jsonChildRelTypeLookupCache) store(key string, value jsonRelationshipTypeLookup) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries[key] = value
	c.mu.Unlock()
}

func (c *jsonChildRelTypeLookupCache) loadParent(key string) (map[string]jsonRelationshipTypeLookup, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.parentIndexes[key]
	return value, ok
}

func (c *jsonChildRelTypeLookupCache) storeParent(key string, value map[string]jsonRelationshipTypeLookup) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.parentIndexes[key] = value
	c.mu.Unlock()
}

func canonicalJSONScalarType(typeName string) string {
	switch {
	case strings.EqualFold(typeName, "String"):
		return "String"
	case strings.EqualFold(typeName, "Boolean"):
		return "Boolean"
	case strings.EqualFold(typeName, "Integer"):
		return "Integer"
	case strings.EqualFold(typeName, "Long"):
		return "Long"
	case strings.EqualFold(typeName, "Decimal"):
		return "Decimal"
	case strings.EqualFold(typeName, "Double"):
		return "Double"
	case strings.EqualFold(typeName, "Date"):
		return "Date"
	case strings.EqualFold(typeName, "Datetime") || strings.EqualFold(typeName, "DateTime"):
		return "Datetime"
	case strings.EqualFold(typeName, "Time"):
		return "Time"
	case strings.EqualFold(typeName, "Id"):
		return "Id"
	case strings.EqualFold(typeName, "Blob"):
		return "Blob"
	case strings.EqualFold(typeName, "UUID"):
		return "UUID"
	default:
		return typeName
	}
}
func jsonIntegralNumber(raw any) (int64, bool) {
	switch value := raw.(type) {
	case int:
		return int64(value), true
	case int64:
		return value, true
	case json.Number:
		text := value.String()
		if strings.ContainsAny(text, ".eE") {
			decimal, err := strconv.ParseFloat(text, 64)
			if err != nil || math.Trunc(decimal) != decimal {
				return 0, false
			}
			converted, err := int64FromFloat("JSON number", decimal)
			return converted, err == nil
		}
		converted, err := strconv.ParseInt(text, 10, 64)
		return converted, err == nil
	case float64:
		if math.Trunc(value) != value {
			return 0, false
		}
		converted, err := int64FromFloat("JSON number", value)
		return converted, err == nil
	default:
		return 0, false
	}
}
func jsonDecimalNumber(raw any) (float64, bool) {
	switch value := raw.(type) {
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		converted, err := strconv.ParseFloat(value.String(), 64)
		return converted, err == nil
	case float64:
		return value, true
	default:
		return 0, false
	}
}
func jsonTypeMappingError(typeName string, raw any) error {
	return jsonDeserializeException("JSON.deserialize cannot map JSON %s to %s", jsonRawKind(raw), typeName)
}
func jsonDeserializeException(format string, args ...any) error {
	if format == "JSON.deserializeUntyped invalid JSON input: %v" && len(args) == 1 {
		switch err := args[0].(type) {
		case *jsonNumberInputError:
			return newExceptionError("JSONException", err.Error())
		case *jsonContainerInputError:
			return newExceptionError("JSONException", err.Error())
		case *jsonStringInputError:
			return newExceptionError("JSONException", err.Error())
		case *jsonCharacterInputError:
			return newExceptionError("JSONException", err.Error())
		}
	}
	return newExceptionError("JSONException", fmt.Sprintf(format, args...))
}
func jsonRawKind(raw any) string {
	switch raw.(type) {
	case nil:
		return "null"
	case bool:
		return "Boolean"
	case json.Number, float64:
		return "number"
	case string:
		return "String"
	case []any:
		return "array"
	case map[string]any, orderedJSONObject:
		return "object"
	default:
		return fmt.Sprintf("%T", raw)
	}
}
func (vm *VM) jsonAllowedFields(typeName string) map[string]struct{} {
	allowed := map[string]struct{}{
		"Id":               {},
		"CreatedDate":      {},
		"CreatedById":      {},
		"LastModifiedDate": {},
		"LastModifiedById": {},
		"SystemModstamp":   {},
		"OwnerId":          {},
		"IsDeleted":        {},
	}
	if vm.isPlatformJSONHeader(typeName) {
		allowed = map[string]struct{}{}
		fields, _ := platformJSONDTOFields(typeName)
		for name := range fields {
			allowed[name] = struct{}{}
		}
		return allowed
	}
	if vm.Org != nil {
		if objectName, definition, ok := vm.sObjectDefinitionForRead(typeName); ok {
			definition = vm.describePreparedDefinition(objectName, definition)
			for name := range definition.Fields {
				allowed[name] = struct{}{}
				if vm.Org.Namespace != "" {
					allowed[storage.StripNamespaceToken(vm.Org.Namespace, name)] = struct{}{}
					allowed[storage.NamespaceTokenName(vm.Org.Namespace, name)] = struct{}{}
				}
			}
			for _, relation := range definition.Relations {
				for _, name := range []string{relation.ParentRelationship, relation.ChildRelationship} {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					allowed[name] = struct{}{}
					if vm.Org.Namespace != "" {
						allowed[storage.StripNamespaceToken(vm.Org.Namespace, name)] = struct{}{}
						allowed[storage.NamespaceTokenName(vm.Org.Namespace, name)] = struct{}{}
					}
				}
			}
		}
	}
	for className := typeName; className != ""; {
		class, ok := vm.classEntry(className)
		if !ok {
			break
		}
		for name := range class.Fields {
			allowed[name] = struct{}{}
		}
		className = class.SuperClass
	}
	return allowed
}
func jsonAllowedFieldContains(allowed map[string]struct{}, key string) bool {
	if _, ok := allowed[key]; ok {
		return true
	}
	for candidate := range allowed {
		if strings.EqualFold(candidate, key) {
			return true
		}
	}
	return false
}
func (vm *VM) jsonStrictObjectFieldsError(typeName string, raw any) error {
	if vm.allowOpenSObjectJSONFields(typeName) {
		return nil
	}
	allowed := vm.jsonAllowedFields(typeName)
	fields, _ := jsonObjectFields(raw)
	for _, field := range fields {
		if field.name == "attributes" || jsonAllowedFieldContains(allowed, field.name) || vm.jsonStrictAllowsRelationshipPayload(typeName, field.name) {
			continue
		}
		if vm.isSObjectLikeType(typeName) {
			return jsonDeserializeException("No such column '%s' on sobject of type %s", field.name, typeName)
		}
		if vm.isPlatformJSONHeader(typeName) {
			line, column := jsonInputPosition(field.source, field.end)
			return jsonDeserializeException("Unrecognized field %q at [line:%d, column:%d]", field.name, line, column)
		}
		return jsonDeserializeException("Unknown field: %s.%s", typeName, field.name)
	}
	return nil
}

func (vm *VM) jsonStrictAllowsRelationshipPayload(typeName, key string) bool {
	if !vm.isSObjectLikeType(typeName) {
		return false
	}
	if _, known := vm.jsonSObjectParentRelationshipType(typeName, key); known {
		return true
	}
	_, known := vm.jsonSObjectChildRelationshipType(typeName, key)
	return known
}
func (vm *VM) allowOpenSObjectJSONFields(typeName string) bool {
	if !vm.isSObjectLikeType(typeName) {
		return false
	}
	if _, ok := vm.classEntry(typeName); ok {
		return false
	}
	if vm.Org == nil {
		return true
	}
	_, _, ok := vm.sObjectDefinitionForRead(typeName)
	return !ok
}

func jsonTruncatedIntegralNumber(raw any) (int64, bool) {
	if number, ok := raw.(json.Number); ok {
		rat, valid := new(big.Rat).SetString(number.String())
		if !valid {
			return 0, false
		}
		integer := new(big.Int).Quo(rat.Num(), rat.Denom())
		return integer.Int64(), integer.IsInt64()
	}
	return jsonIntegralNumber(raw)
}

// JSON's scanner reports the cursor after a number. At EOF, its refill advances
// the column by the consumed buffer length; preserve that observable location.
func jsonNumericInputPosition(source string, offset int) (int, int) {
	prefix := source[:offset]
	line := strings.Count(prefix, "\n") + 1
	column := offset - strings.LastIndex(prefix, "\n")
	if offset == len(source) {
		column += len(source)
	}
	return line, column
}

func jsonDeserializeScalarLocation(source string, decoded any, err error) error {
	number, numeric := decoded.(json.Number)
	thrown, exception := err.(*apexThrowError)
	if _, quoted := decoded.(string); quoted && exception && thrown.value.Fields["message"].Text == "JSON.deserialize cannot map JSON String to Integer" {
		line, column := jsonInputPosition(source, jsonTokenInputStart(source, 0))
		return jsonDeserializeException("Value does not match expected type at [line:%d, column:%d]", line, column)
	}
	if !numeric || !exception || thrown.value.Fields["message"].Text != fmt.Sprintf("Numeric value (%s) out of range of int", number) {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(source))
	decoder.UseNumber()
	if _, tokenErr := decoder.Token(); tokenErr != nil {
		return err
	}
	line, column := jsonNumericInputPosition(source, int(decoder.InputOffset()))
	return jsonDeserializeException("Numeric value (%s) out of range of int at [line:%d, column:%d]", number, line, column)
}

func (vm *VM) hydrateJSONParentLookupFields(value Value) {
	if vm != nil && vm.Org != nil && strings.EqualFold(value.Type, "User") && !value.classInstance {
		if object, ok := vm.Org.Objects["User"]; ok {
			loadedParent := false
			for _, relation := range object.Definition.Relations {
				for _, alias := range vm.parentRelationshipValueAliases(value.Type, relation.ParentRelationship) {
					if _, exists := value.Fields[alias]; exists {
						loadedParent = true
					}
				}
			}
			if !loadedParent {
				return
			}
		}
	}
	// A JSON payload is self-contained; its Id does not authorize loading
	// omitted foreign keys from a stored record (customQuery round trips).
	vm.hydrateParentLookupFieldsWithStoredValues(value, false)
}

// Native owner-side Account formula rows retain the qualified alias regardless
// of the two aliases' text order. Ordinary fields and duplicate identical keys
// continue through the existing textual-order mapper.
func (vm *VM) qualifiedTextFormulaJSONShadowsLocal(typeName, key string, fields map[string]any) bool {
	if vm.Org == nil || vm.Org.Namespace == "" || !strings.EqualFold(vm.currentCallerNamespace(), vm.Org.Namespace) || isCustomObjectLikeName(typeName) {
		return false
	}
	_, field, ok := vm.sObjectFieldDefinition(typeName, key)
	if !ok || !isTextFormulaField(field) {
		return false
	}
	prefix, qualified := namespaceTokenPrefix(field.APIName)
	if !qualified || !strings.EqualFold(prefix, vm.Org.Namespace) || !strings.EqualFold(key, storage.StripNamespaceToken(vm.Org.Namespace, field.APIName)) {
		return false
	}
	for inputKey := range fields {
		if strings.EqualFold(inputKey, field.APIName) {
			return true
		}
	}
	return false
}

// fillDuplicateFieldSlots copies a deserialized field into the other slots a
// merged duplicate class keeps for it. When two source units declare the same
// field with the same type, reads from either unit resolve to their own slot,
// so every matching slot must hold the value. Slots with a different type keep
// their own definition.
func (vm *VM) fillDuplicateFieldSlots(obj Value, owner string, field Field, fieldName string, value Value) {
	class, ok := vm.lookupClass(owner)
	if !ok || fieldName == "" {
		return
	}
	prefix := strings.ToLower(fieldName) + "\x00"
	for slot, candidate := range class.Fields {
		if !strings.HasPrefix(strings.ToLower(slot), prefix) || !strings.EqualFold(candidate.Type, field.Type) {
			continue
		}
		obj.Fields[slot] = value
	}
}
