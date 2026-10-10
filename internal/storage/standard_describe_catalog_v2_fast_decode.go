package storage

import (
	"encoding/json"
	"strconv"
	"unicode/utf8"
)

// decodeStandardDescribeObjectJSON decodes one describe catalog member into
// standardDescribeObject with the same result as json.Unmarshal.
//
// Catalog members are full Salesforce describe responses, and the struct reads
// a small subset of their keys. The fast path decodes those keys directly and
// validates everything it skips. Any input outside the plain shape it knows
// (escaped, non-ASCII or case-folded keys, duplicate keys, type mismatches,
// nulls inside arrays, deep nesting, invalid JSON) makes it decline, and the
// member is then decoded by encoding/json, so results and errors match by
// construction. TestStandardDescribeCatalogV2FastDecodeMatchesJSON checks every
// generated member against encoding/json.
func decodeStandardDescribeObjectJSON(data []byte) (standardDescribeObject, error) {
	if describe, ok := fastDecodeStandardDescribeObject(data); ok {
		return describe, nil
	}
	var describe standardDescribeObject
	err := json.Unmarshal(data, &describe)
	return describe, err
}

func fastDecodeStandardDescribeObject(data []byte) (standardDescribeObject, bool) {
	decoder := describeJSONDecoder{data: data}
	var describe standardDescribeObject
	if !decoder.describeObject(&describe) {
		return standardDescribeObject{}, false
	}
	decoder.space()
	if decoder.pos != len(decoder.data) {
		return standardDescribeObject{}, false
	}
	return describe, true
}

// describeJSONDecoder is a strict cursor over one JSON document. Methods
// return false when the input is outside the shape the fast path accepts.
type describeJSONDecoder struct {
	data []byte
	pos  int
}

// describeJSONMaxDepth bounds recursion in skipped values. Deeper input falls
// back to encoding/json, which applies its own limit.
const describeJSONMaxDepth = 256

func (decoder *describeJSONDecoder) describeObject(describe *standardDescribeObject) bool {
	var seen uint32
	for first := true; ; first = false {
		key, more, ok := decoder.nextKey(first)
		if !ok {
			return false
		}
		if !more {
			return true
		}
		var bit uint32
		switch string(key) {
		case "name":
			bit, ok = 1<<0, decoder.stringInto(&describe.Name)
		case "label":
			bit, ok = 1<<1, decoder.stringInto(&describe.Label)
		case "labelPlural":
			bit, ok = 1<<2, decoder.stringInto(&describe.LabelPlural)
		case "keyPrefix":
			bit, ok = 1<<3, decoder.stringInto(&describe.KeyPrefix)
		case "mergeable":
			bit, ok = 1<<4, decoder.boolPointerInto(&describe.Mergeable)
		case "triggerable":
			bit, ok = 1<<5, decoder.boolPointerInto(&describe.Triggerable)
		case "fields":
			bit, ok = 1<<6, decoder.describeFields(&describe.Fields)
		case "childRelationships":
			bit, ok = 1<<7, decoder.describeChildRelationships(&describe.ChildRelationships)
		case "recordTypeInfos":
			bit, ok = 1<<8, decoder.describeRecordTypeInfos(&describe.RecordTypeInfos)
		default:
			ok = !describeObjectKeyFolds(key) && decoder.skipValue(1)
		}
		// A duplicate key changes how encoding/json merges values; decline it.
		if !ok || seen&bit != 0 {
			return false
		}
		seen |= bit
	}
}

func (decoder *describeJSONDecoder) describeFields(out *[]standardDescribeField) bool {
	if decoder.null() {
		*out = nil
		return true
	}
	fields := make([]standardDescribeField, 0)
	for first := true; ; first = false {
		more, ok := decoder.nextElement(first)
		if !ok {
			return false
		}
		if !more {
			*out = fields
			return true
		}
		fields = append(fields, standardDescribeField{})
		if !decoder.describeField(&fields[len(fields)-1]) {
			return false
		}
	}
}

func (decoder *describeJSONDecoder) describeField(field *standardDescribeField) bool {
	var seen uint32
	for first := true; ; first = false {
		key, more, ok := decoder.nextKey(first)
		if !ok {
			return false
		}
		if !more {
			return true
		}
		var bit uint32
		switch string(key) {
		case "name":
			bit, ok = 1<<0, decoder.stringInto(&field.Name)
		case "label":
			bit, ok = 1<<1, decoder.stringInto(&field.Label)
		case "type":
			bit, ok = 1<<2, decoder.stringInto(&field.Type)
		case "length":
			bit, ok = 1<<3, decoder.intInto(&field.Length)
		case "precision":
			bit, ok = 1<<4, decoder.intInto(&field.Precision)
		case "scale":
			bit, ok = 1<<5, decoder.intInto(&field.Scale)
		case "calculated":
			bit, ok = 1<<6, decoder.boolInto(&field.Calculated)
		case "defaultValue":
			bit, ok = 1<<7, decoder.anyInto(&field.DefaultValue)
		case "defaultValueFormula":
			bit, ok = 1<<8, decoder.stringPointerInto(&field.DefaultValueFormula)
		case "compoundFieldName":
			bit, ok = 1<<9, decoder.stringInto(&field.CompoundFieldName)
		case "nillable":
			bit, ok = 1<<10, decoder.boolPointerInto(&field.Nillable)
		case "defaultedOnCreate":
			bit, ok = 1<<11, decoder.boolPointerInto(&field.DefaultedOnCreate)
		case "createable":
			bit, ok = 1<<12, decoder.boolPointerInto(&field.Createable)
		case "updateable":
			bit, ok = 1<<13, decoder.boolPointerInto(&field.Updateable)
		case "filterable":
			bit, ok = 1<<14, decoder.boolPointerInto(&field.Filterable)
		case "groupable":
			bit, ok = 1<<15, decoder.boolPointerInto(&field.Groupable)
		case "sortable":
			bit, ok = 1<<16, decoder.boolPointerInto(&field.Sortable)
		case "aggregatable":
			bit, ok = 1<<17, decoder.boolPointerInto(&field.Aggregatable)
		case "permissionable":
			bit, ok = 1<<18, decoder.boolPointerInto(&field.Permissionable)
		case "deprecatedAndHidden":
			bit, ok = 1<<19, decoder.boolPointerInto(&field.DeprecatedAndHidden)
		case "externalId":
			bit, ok = 1<<20, decoder.boolInto(&field.ExternalID)
		case "unique":
			bit, ok = 1<<21, decoder.boolInto(&field.Unique)
		case "encrypted":
			bit, ok = 1<<22, decoder.boolInto(&field.Encrypted)
		case "caseSensitive":
			bit, ok = 1<<23, decoder.boolInto(&field.CaseSensitive)
		case "idLookup":
			bit, ok = 1<<24, decoder.boolInto(&field.IDLookup)
		case "referenceTo":
			bit, ok = 1<<25, decoder.stringsInto(&field.ReferenceTo)
		case "relationshipName":
			bit, ok = 1<<26, decoder.stringInto(&field.RelationshipName)
		case "polymorphicForeignKey":
			bit, ok = 1<<27, decoder.boolInto(&field.Polymorphic)
		case "picklistValues":
			bit, ok = 1<<28, decoder.describePicklistValues(&field.PicklistValues)
		default:
			ok = !describeFieldKeyFolds(key) && decoder.skipValue(1)
		}
		if !ok || seen&bit != 0 {
			return false
		}
		seen |= bit
	}
}

func (decoder *describeJSONDecoder) describePicklistValues(out *[]standardDescribePicklistValue) bool {
	if decoder.null() {
		*out = nil
		return true
	}
	values := make([]standardDescribePicklistValue, 0)
	for first := true; ; first = false {
		more, ok := decoder.nextElement(first)
		if !ok {
			return false
		}
		if !more {
			*out = values
			return true
		}
		var value standardDescribePicklistValue
		var seen uint8
		for firstKey := true; ; firstKey = false {
			key, moreKeys, ok := decoder.nextKey(firstKey)
			if !ok {
				return false
			}
			if !moreKeys {
				break
			}
			var bit uint8
			switch string(key) {
			case "value":
				bit, ok = 1<<0, decoder.stringInto(&value.Value)
			case "label":
				bit, ok = 1<<1, decoder.stringInto(&value.Label)
			case "active":
				bit, ok = 1<<2, decoder.boolInto(&value.Active)
			case "defaultValue":
				bit, ok = 1<<3, decoder.boolInto(&value.DefaultValue)
			default:
				ok = !describePicklistValueKeyFolds(key) && decoder.skipValue(1)
			}
			if !ok || seen&bit != 0 {
				return false
			}
			seen |= bit
		}
		values = append(values, value)
	}
}

func (decoder *describeJSONDecoder) describeChildRelationships(out *[]standardDescribeChildRelationship) bool {
	if decoder.null() {
		*out = nil
		return true
	}
	relationships := make([]standardDescribeChildRelationship, 0)
	for first := true; ; first = false {
		more, ok := decoder.nextElement(first)
		if !ok {
			return false
		}
		if !more {
			*out = relationships
			return true
		}
		var relationship standardDescribeChildRelationship
		var seen uint8
		for firstKey := true; ; firstKey = false {
			key, moreKeys, ok := decoder.nextKey(firstKey)
			if !ok {
				return false
			}
			if !moreKeys {
				break
			}
			var bit uint8
			switch string(key) {
			case "childSObject":
				bit, ok = 1<<0, decoder.stringInto(&relationship.ChildSObject)
			case "field":
				bit, ok = 1<<1, decoder.stringInto(&relationship.Field)
			case "relationshipName":
				bit, ok = 1<<2, decoder.stringInto(&relationship.RelationshipName)
			case "cascadeDelete":
				bit, ok = 1<<3, decoder.boolInto(&relationship.CascadeDelete)
			case "restrictedDelete":
				bit, ok = 1<<4, decoder.boolInto(&relationship.RestrictedDelete)
			case "deprecatedAndHidden":
				bit, ok = 1<<5, decoder.boolInto(&relationship.DeprecatedAndHidden)
			default:
				ok = !describeChildRelationshipKeyFolds(key) && decoder.skipValue(1)
			}
			if !ok || seen&bit != 0 {
				return false
			}
			seen |= bit
		}
		relationships = append(relationships, relationship)
	}
}

func (decoder *describeJSONDecoder) describeRecordTypeInfos(out *[]standardDescribeRecordTypeInfo) bool {
	if decoder.null() {
		*out = nil
		return true
	}
	infos := make([]standardDescribeRecordTypeInfo, 0)
	for first := true; ; first = false {
		more, ok := decoder.nextElement(first)
		if !ok {
			return false
		}
		if !more {
			*out = infos
			return true
		}
		var info standardDescribeRecordTypeInfo
		var seen uint8
		for firstKey := true; ; firstKey = false {
			key, moreKeys, ok := decoder.nextKey(firstKey)
			if !ok {
				return false
			}
			if !moreKeys {
				break
			}
			var bit uint8
			switch string(key) {
			case "recordTypeId":
				bit, ok = 1<<0, decoder.stringInto(&info.RecordTypeID)
			case "developerName":
				bit, ok = 1<<1, decoder.stringInto(&info.DeveloperName)
			case "name":
				bit, ok = 1<<2, decoder.stringInto(&info.Name)
			case "active":
				bit, ok = 1<<3, decoder.boolInto(&info.Active)
			case "available":
				bit, ok = 1<<4, decoder.boolInto(&info.Available)
			case "defaultRecordTypeMapping":
				bit, ok = 1<<5, decoder.boolInto(&info.DefaultRecordTypeMapping)
			default:
				ok = !describeRecordTypeInfoKeyFolds(key) && decoder.skipValue(1)
			}
			if !ok || seen&bit != 0 {
				return false
			}
			seen |= bit
		}
		infos = append(infos, info)
	}
}

// The *KeyFolds functions report whether an unmatched key would still match a
// struct field under encoding/json's case-insensitive rule. Keys are plain
// ASCII here (nextKey declines anything else), so ASCII folding is exact.
func describeObjectKeyFolds(key []byte) bool {
	switch describeJSONLowerKey(key) {
	case "name", "label", "labelplural", "keyprefix", "mergeable", "triggerable", "fields", "childrelationships", "recordtypeinfos":
		return true
	}
	return false
}

func describeFieldKeyFolds(key []byte) bool {
	switch describeJSONLowerKey(key) {
	case "name", "label", "type", "length", "precision", "scale", "calculated", "defaultvalue", "defaultvalueformula",
		"compoundfieldname", "nillable", "defaultedoncreate", "createable", "updateable", "filterable", "groupable",
		"sortable", "aggregatable", "permissionable", "deprecatedandhidden", "externalid", "unique", "encrypted",
		"casesensitive", "idlookup", "referenceto", "relationshipname", "polymorphicforeignkey", "picklistvalues":
		return true
	}
	return false
}

func describePicklistValueKeyFolds(key []byte) bool {
	switch describeJSONLowerKey(key) {
	case "value", "label", "active", "defaultvalue":
		return true
	}
	return false
}

func describeChildRelationshipKeyFolds(key []byte) bool {
	switch describeJSONLowerKey(key) {
	case "childsobject", "field", "relationshipname", "cascadedelete", "restricteddelete", "deprecatedandhidden":
		return true
	}
	return false
}

func describeRecordTypeInfoKeyFolds(key []byte) bool {
	switch describeJSONLowerKey(key) {
	case "recordtypeid", "developername", "name", "active", "available", "defaultrecordtypemapping":
		return true
	}
	return false
}

// describeJSONLowerKey lowercases an ASCII key. Keys longer than every struct
// field name cannot fold onto one, so they map to "".
func describeJSONLowerKey(key []byte) string {
	var buffer [32]byte
	if len(key) > len(buffer) {
		return ""
	}
	for index, char := range key {
		if 'A' <= char && char <= 'Z' {
			char += 'a' - 'A'
		}
		buffer[index] = char
	}
	return string(buffer[:len(key)])
}

func (decoder *describeJSONDecoder) space() {
	for decoder.pos < len(decoder.data) {
		switch decoder.data[decoder.pos] {
		case ' ', '\t', '\n', '\r':
			decoder.pos++
		default:
			return
		}
	}
}

// nextKey reads the next object key and its colon. On the first call it also
// reads the opening brace. more is false at the closing brace.
func (decoder *describeJSONDecoder) nextKey(first bool) (key []byte, more, ok bool) {
	decoder.space()
	if first {
		if !decoder.consume('{') {
			return nil, false, false
		}
		decoder.space()
		if decoder.consume('}') {
			return nil, false, true
		}
	} else {
		if decoder.consume('}') {
			return nil, false, true
		}
		if !decoder.consume(',') {
			return nil, false, false
		}
		decoder.space()
	}
	key, ok = decoder.plainKey()
	if !ok {
		return nil, false, false
	}
	decoder.space()
	if !decoder.consume(':') {
		return nil, false, false
	}
	decoder.space()
	return key, true, true
}

// nextElement positions the cursor at the next array element. On the first
// call it also reads the opening bracket. more is false at the closing bracket.
func (decoder *describeJSONDecoder) nextElement(first bool) (more, ok bool) {
	decoder.space()
	if first {
		if !decoder.consume('[') {
			return false, false
		}
		decoder.space()
		if decoder.consume(']') {
			return false, true
		}
	} else {
		if decoder.consume(']') {
			return false, true
		}
		if !decoder.consume(',') {
			return false, false
		}
		decoder.space()
	}
	// encoding/json leaves a struct or string element untouched for null, which
	// the fast path does not model.
	if decoder.peekNull() {
		return false, false
	}
	return true, true
}

func (decoder *describeJSONDecoder) consume(char byte) bool {
	if decoder.pos < len(decoder.data) && decoder.data[decoder.pos] == char {
		decoder.pos++
		return true
	}
	return false
}

// plainKey reads a quoted key of printable ASCII without escapes.
func (decoder *describeJSONDecoder) plainKey() ([]byte, bool) {
	if !decoder.consume('"') {
		return nil, false
	}
	start := decoder.pos
	for decoder.pos < len(decoder.data) {
		char := decoder.data[decoder.pos]
		if char == '"' {
			key := decoder.data[start:decoder.pos]
			decoder.pos++
			return key, true
		}
		if char < 0x20 || char >= utf8.RuneSelf || char == '\\' {
			return nil, false
		}
		decoder.pos++
	}
	return nil, false
}

func (decoder *describeJSONDecoder) peekNull() bool {
	data := decoder.data[decoder.pos:]
	return len(data) >= 4 && data[0] == 'n' && data[1] == 'u' && data[2] == 'l' && data[3] == 'l'
}

// null consumes a null literal. encoding/json leaves non-pointer scalars
// unchanged for null and sets pointers, slices and interfaces to nil.
func (decoder *describeJSONDecoder) null() bool {
	if decoder.peekNull() {
		decoder.pos += 4
		return true
	}
	return false
}

func (decoder *describeJSONDecoder) literal(word string) bool {
	if len(decoder.data)-decoder.pos >= len(word) && string(decoder.data[decoder.pos:decoder.pos+len(word)]) == word {
		decoder.pos += len(word)
		return true
	}
	return false
}

func (decoder *describeJSONDecoder) boolValue() (value, ok bool) {
	if decoder.literal("true") {
		return true, true
	}
	if decoder.literal("false") {
		return false, true
	}
	return false, false
}

func (decoder *describeJSONDecoder) boolInto(out *bool) bool {
	if decoder.null() {
		return true
	}
	value, ok := decoder.boolValue()
	if ok {
		*out = value
	}
	return ok
}

func (decoder *describeJSONDecoder) boolPointerInto(out **bool) bool {
	if decoder.null() {
		*out = nil
		return true
	}
	value, ok := decoder.boolValue()
	if ok {
		*out = &value
	}
	return ok
}

func (decoder *describeJSONDecoder) stringInto(out *string) bool {
	if decoder.null() {
		return true
	}
	value, ok := decoder.stringValue()
	if ok {
		*out = value
	}
	return ok
}

func (decoder *describeJSONDecoder) stringPointerInto(out **string) bool {
	if decoder.null() {
		*out = nil
		return true
	}
	value, ok := decoder.stringValue()
	if ok {
		*out = &value
	}
	return ok
}

func (decoder *describeJSONDecoder) stringsInto(out *[]string) bool {
	if decoder.null() {
		*out = nil
		return true
	}
	values := make([]string, 0)
	for first := true; ; first = false {
		more, ok := decoder.nextElement(first)
		if !ok {
			return false
		}
		if !more {
			*out = values
			return true
		}
		value, ok := decoder.stringValue()
		if !ok {
			return false
		}
		values = append(values, value)
	}
}

// stringValue reads a JSON string. Strings without escapes and with valid
// UTF-8 are copied as-is, which is what encoding/json does for them. Any other
// string is unquoted by encoding/json itself.
func (decoder *describeJSONDecoder) stringValue() (string, bool) {
	start := decoder.pos
	end, plain, ok := decoder.scanString()
	if !ok {
		return "", false
	}
	raw := decoder.data[start:end]
	body := raw[1 : len(raw)-1]
	if plain || utf8.Valid(body) && !describeJSONHasBackslash(body) {
		return string(body), true
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func describeJSONHasBackslash(data []byte) bool {
	for _, char := range data {
		if char == '\\' {
			return true
		}
	}
	return false
}

// scanString validates the string at the cursor and moves past it. plain
// reports a string of printable ASCII without escapes.
func (decoder *describeJSONDecoder) scanString() (end int, plain, ok bool) {
	if !decoder.consume('"') {
		return 0, false, false
	}
	plain = true
	data := decoder.data
	for position := decoder.pos; position < len(data); position++ {
		char := data[position]
		switch {
		case char == '"':
			decoder.pos = position + 1
			return decoder.pos, plain, true
		case char < 0x20:
			return 0, false, false
		case char == '\\':
			plain = false
			position++
			if position >= len(data) {
				return 0, false, false
			}
			switch data[position] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			case 'u':
				if position+4 >= len(data) {
					return 0, false, false
				}
				for _, hex := range data[position+1 : position+5] {
					if !('0' <= hex && hex <= '9' || 'a' <= hex && hex <= 'f' || 'A' <= hex && hex <= 'F') {
						return 0, false, false
					}
				}
				position += 4
			default:
				return 0, false, false
			}
		case char >= utf8.RuneSelf:
			plain = false
		}
	}
	return 0, false, false
}

// scanNumber validates a JSON number at the cursor and returns its text.
func (decoder *describeJSONDecoder) scanNumber() ([]byte, bool) {
	data := decoder.data
	start := decoder.pos
	position := start
	digits := func() bool {
		begin := position
		for position < len(data) && '0' <= data[position] && data[position] <= '9' {
			position++
		}
		return position > begin
	}
	if position < len(data) && data[position] == '-' {
		position++
	}
	if position < len(data) && data[position] == '0' {
		position++
	} else if position >= len(data) || data[position] < '1' || data[position] > '9' || !digits() {
		return nil, false
	}
	if position < len(data) && data[position] == '.' {
		position++
		if !digits() {
			return nil, false
		}
	}
	if position < len(data) && (data[position] == 'e' || data[position] == 'E') {
		position++
		if position < len(data) && (data[position] == '+' || data[position] == '-') {
			position++
		}
		if !digits() {
			return nil, false
		}
	}
	decoder.pos = position
	return data[start:position], true
}

// intInto decodes an int field. encoding/json uses strconv.ParseInt on the
// number text and rejects fractions and exponents, as this does.
func (decoder *describeJSONDecoder) intInto(out *int) bool {
	if decoder.null() {
		return true
	}
	text, ok := decoder.scanNumber()
	if !ok {
		return false
	}
	value, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil || int64(int(value)) != value {
		return false
	}
	*out = int(value)
	return true
}

// anyInto decodes an interface{} field. Strings take the string path; other
// non-null values are decoded by encoding/json from their validated text.
func (decoder *describeJSONDecoder) anyInto(out *any) bool {
	if decoder.null() {
		*out = nil
		return true
	}
	if decoder.pos < len(decoder.data) && decoder.data[decoder.pos] == '"' {
		value, ok := decoder.stringValue()
		if ok {
			*out = value
		}
		return ok
	}
	start := decoder.pos
	if !decoder.skipValue(1) {
		return false
	}
	var value any
	if err := json.Unmarshal(decoder.data[start:decoder.pos], &value); err != nil {
		return false
	}
	*out = value
	return true
}

// skipValue validates and skips one JSON value of any type.
func (decoder *describeJSONDecoder) skipValue(depth int) bool {
	if depth > describeJSONMaxDepth || decoder.pos >= len(decoder.data) {
		return false
	}
	switch char := decoder.data[decoder.pos]; {
	case char == '"':
		_, _, ok := decoder.scanString()
		return ok
	case char == '{':
		for first := true; ; first = false {
			decoder.space()
			if first {
				decoder.pos++
				decoder.space()
				if decoder.consume('}') {
					return true
				}
			} else {
				if decoder.consume('}') {
					return true
				}
				if !decoder.consume(',') {
					return false
				}
				decoder.space()
			}
			if _, _, ok := decoder.scanString(); !ok {
				return false
			}
			decoder.space()
			if !decoder.consume(':') {
				return false
			}
			decoder.space()
			if !decoder.skipValue(depth + 1) {
				return false
			}
		}
	case char == '[':
		for first := true; ; first = false {
			decoder.space()
			if first {
				decoder.pos++
				decoder.space()
				if decoder.consume(']') {
					return true
				}
			} else {
				if decoder.consume(']') {
					return true
				}
				if !decoder.consume(',') {
					return false
				}
				decoder.space()
			}
			if !decoder.skipValue(depth + 1) {
				return false
			}
		}
	case char == 't':
		return decoder.literal("true")
	case char == 'f':
		return decoder.literal("false")
	case char == 'n':
		return decoder.literal("null")
	case char == '-' || '0' <= char && char <= '9':
		_, ok := decoder.scanNumber()
		return ok
	default:
		return false
	}
}
