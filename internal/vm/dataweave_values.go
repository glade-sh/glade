package vm

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/dataweave"
)

// dataWeaveInputValue preserves Apex scalar and record identity for the owned
// data format. Raw String and Blob inputs keep the source-declared reader path.
func (vm *VM) dataWeaveInputValue(value Value, seen map[uint64]bool, depth int) (dataweave.TypedValue, error) {
	if depth > 128 {
		return dataweave.TypedValue{}, &dataweave.HostError{Kind: "input", Detail: "DataWeave typed input exceeds depth limit"}
	}
	if value.Ref != 0 {
		if seen[value.Ref] {
			return dataweave.TypedValue{}, unsupportedCallError("DataWeave cyclic typed input")
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	out := dataweave.TypedValue{Type: value.Type}
	switch value.Kind {
	case ValueNull:
		out.Kind = "null"
	case ValueString:
		out.Kind = "string"
		out.Text = value.Text
	case ValueInt:
		out.Kind = "integer"
		if strings.EqualFold(value.Type, "Long") || strings.EqualFold(value.Runtime, "Long") {
			out.Kind = "long"
		}
		out.Text = strconv.FormatInt(value.Int, 10)
	case ValueDecimal:
		if isFloatBackedDecimal(value) {
			return out, unsupportedCallError("DataWeave Double input")
		}
		out.Kind = "decimal"
		out.Text = decimalDisplayText(value)
	case ValueBool:
		out.Kind = "boolean"
		out.Boolean = value.Bool
	case ValueList, ValueSet:
		out.Kind = "list"
		children := value.List
		if value.Kind == ValueSet {
			children = value.Set
		}
		for _, child := range children {
			typed, err := vm.dataWeaveInputValue(child, seen, depth+1)
			if err != nil {
				return out, err
			}
			out.Elements = append(out.Elements, typed)
		}
	case ValueMap, ValueObject:
		fields := value
		out.Kind = "map"
		if value.Kind == ValueObject {
			switch strings.ToLower(strings.TrimPrefix(value.Type, "System.")) {
			case "date":
				v, err := parsePlatformDate(value)
				out.Kind = "date"
				out.Text = v.Format("2006-01-02")
				return out, err
			case "datetime":
				v, err := parsePlatformDatetime(value)
				out.Kind = "datetime"
				out.Text = v.UTC().Format(time.RFC3339Nano)
				return out, err
			case "time":
				v, err := parsePlatformTime(value)
				out.Kind = "time"
				out.Text = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Add(v).Format("15:04:05.000")
				return out, err
			case "blob":
				out.Kind = "blob"
				out.Text = base64.StdEncoding.EncodeToString([]byte(blobText(value)))
				return out, nil
			case "id":
				v, err := platformScalarText(value, "Id")
				out.Kind = "string"
				out.Text = v
				return out, err
			}
			out.Kind = "object"
			if vm.isSObjectType(value.Type) && !vm.userClassShadowsSObjectType(value.Type) {
				populated, handled, err := vm.callSObjectMember(value, "getPopulatedFieldsAsMap", nil)
				if err != nil {
					return out, err
				}
				if !handled {
					return out, unsupportedCallError("DataWeave SObject populated fields")
				}
				fields = populated
			} else {
				if _, ok := vm.lookupClass(value.Type); !ok {
					return out, unsupportedCallError("DataWeave typed input " + value.Type)
				}
				for _, name := range vm.jsonSerializableFieldNames(value.Type) {
					field, owner, ok := vm.lookupField(value.Type, name)
					if !ok || field.Static {
						continue
					}
					_, child, exists := objectFieldValue(value, field.Name)
					if field.Getter != nil {
						var err error
						child, err = vm.callGetter(vm.getterOwner(owner, field), field, value)
						if err != nil {
							return out, err
						}
						exists = true
					}
					if !exists {
						continue
					}
					typed, err := vm.dataWeaveInputValue(child, seen, depth+1)
					if err != nil {
						return out, err
					}
					out.Fields = append(out.Fields, dataweave.TypedField{Name: field.Name, Value: typed})
				}
				return out, nil
			}
		}
		for _, raw := range orderedValueMapKeys(fields) {
			key := mapStoredKey(fields, raw)
			if key.Kind != ValueString {
				return out, unsupportedCallError("DataWeave non-String Map key")
			}
			typed, err := vm.dataWeaveInputValue(fields.Map[raw], seen, depth+1)
			if err != nil {
				return out, err
			}
			if value.Kind == ValueObject && typed.Kind == "list" {
				if _, queried := value.Fields[sobjectQueriedFieldsField]; queried {
					if _, relationship := vm.jsonSObjectChildRelationshipType(value.Type, key.Text); relationship {
						typed.Kind = "query-list"
					}
				}
			}
			out.Fields = append(out.Fields, dataweave.TypedField{Name: key.Text, Value: typed})
		}
	default:
		return out, unsupportedCallError("DataWeave typed input " + valueShape(value))
	}
	return out, nil
}

func (vm *VM) dataWeaveOutputValue(value dataweave.TypedValue, depth int) (Value, error) {
	if depth > 128 {
		return Null, &dataweave.HostError{Kind: "output", Detail: "DataWeave typed output exceeds depth limit"}
	}
	switch value.Kind {
	case "null":
		return Value{Kind: ValueNull, Type: value.Type}, nil
	case "string":
		return String(value.Text), nil
	case "boolean":
		return Bool(value.Boolean), nil
	case "integer", "long":
		n, err := strconv.ParseInt(value.Text, 10, 64)
		if err != nil {
			return Null, fmt.Errorf("DataWeave Integer output: %w", err)
		}
		return integerValueForType(Int(n), value.Type), nil
	case "date":
		parsed, err := time.Parse("2006-01-02", value.Text)
		return platformScalar("Date", formatPlatformDate(parsed)), err
	case "datetime":
		parsed, err := time.Parse(time.RFC3339Nano, value.Text)
		return platformScalar("Datetime", formatPlatformDatetime(parsed)), err
	case "time":
		parsed, err := parseTimeText(value.Text)
		return platformScalar("Time", parsed), err
	case "blob":
		decoded, err := base64.StdEncoding.Strict().DecodeString(value.Text)
		return NewBlobValue(string(decoded)), err
	case "decimal":
		return decimalFromText(value.Text)
	case "list":
		out := List()
		out.Type = value.Type
		if out.Type == "" {
			out.Type = "List<Object>"
		}
		for _, child := range value.Elements {
			v, err := vm.dataWeaveOutputValue(child, depth+1)
			if err != nil {
				return Null, err
			}
			out.List = append(out.List, v)
		}
		return out, nil
	case "map", "object":
		fields := make(map[string]Value, len(value.Fields))
		out := Map()
		out.Type = value.Type
		for _, field := range value.Fields {
			if _, exists := fields[field.Name]; exists {
				return Null, fmt.Errorf("duplicate DataWeave output field %s", field.Name)
			}
			v, err := vm.dataWeaveOutputValue(field.Value, depth+1)
			if err != nil {
				return Null, err
			}
			fields[field.Name] = v
			key := String(field.Name)
			raw := mapKey(key)
			out.Map[raw] = v
			out.MapKeys[raw] = key
			out.MapOrder = append(out.MapOrder, raw)
		}
		if value.Kind == "map" {
			return out, nil
		}
		if strings.TrimSpace(value.Type) == "" {
			return Null, unsupportedCallError("DataWeave output object without class")
		}
		if vm.isSObjectType(value.Type) && !vm.userClassShadowsSObjectType(value.Type) {
			return vm.constructValueWithLiteral(value.Type, nil, fields, &Result{}, false)
		}
		if _, ok := vm.lookupClass(value.Type); !ok {
			return Null, unsupportedCallError("DataWeave output object " + value.Type)
		}
		object, err := vm.jsonObjectBaseValue(value.Type)
		if err != nil {
			return Null, err
		}
		for _, entry := range value.Fields {
			field, owner, ok := vm.lookupField(value.Type, entry.Name)
			if !ok || field.Static {
				return Null, unsupportedCallError("DataWeave output field " + value.Type + "." + entry.Name)
			}
			typed, err := vm.coerceAssignable(vm.resolveTypeNameInClass(owner, field.Type), fields[entry.Name])
			if err != nil {
				return Null, err
			}
			object.Fields[field.Name] = typed
		}
		return object, nil
	default:
		return Null, unsupportedCallError("DataWeave typed output " + value.Kind)
	}
}
