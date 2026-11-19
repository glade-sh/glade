package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
)

// standardDescribeNarrow holds the metadata and field facets needed by stub
// overlays and their check plan. It excludes unrelated field attributes,
// picklist values, child relationships and record types.
type standardDescribeNarrow struct {
	// references are the enabled-candidate reference fields in catalog order,
	// built exactly as the full-decode path builds them. The slice is shared
	// and immutable.
	references []Field
	// currency lists the names of fields whose describe type is currency, in
	// catalog order.
	currency  []string
	mergeable *bool
	fields    []stubDescribeField
}

type stubDescribeField struct {
	Name, Type, RelationshipName         string
	Precision, Scale                     int
	Calculated                           bool
	Createable, Updateable, Aggregatable *bool
	ReferenceTo                          []string
}

func stubDescribeFieldFromFull(f standardDescribeField) stubDescribeField {
	return stubDescribeField{f.Name, f.Type, f.RelationshipName, f.Precision, f.Scale, f.Calculated, f.Createable, f.Updateable, f.Aggregatable, f.ReferenceTo}
}

// standardDescribeFullDecodes and standardDescribeNarrowDecodes count member
// decodes of the production pack. Tests read them to prove which path ran.
var (
	standardDescribeFullDecodes   atomic.Uint64
	standardDescribeNarrowDecodes atomic.Uint64
)

// standardDescribeNarrowFromDescribe derives the narrow result from a fully
// decoded member. It is the reference the narrow decode must equal.
func standardDescribeNarrowFromDescribe(describe standardDescribeObject) standardDescribeNarrow {
	out := standardDescribeNarrow{mergeable: describe.Mergeable}
	for _, captured := range describe.Fields {
		out.fields = append(out.fields, stubDescribeFieldFromFull(captured))
		if strings.EqualFold(captured.Type, "currency") {
			out.currency = append(out.currency, captured.Name)
		}
		if field, ok := standardDescribeNarrowReference(captured); ok {
			out.references = append(out.references, field)
		}
	}
	return out
}

func standardDescribeNarrowReference(captured standardDescribeField) (Field, bool) {
	if !strings.EqualFold(captured.Type, "reference") || (captured.DeprecatedAndHidden != nil && *captured.DeprecatedAndHidden) {
		return Field{}, false
	}
	if !standardStubReferenceTargetsAvailable(captured.ReferenceTo) {
		return Field{}, false
	}
	field := describeField(captured)
	// Non-nillability alone does not establish an omitted-field DML error.
	// Existing explicit requirements remain intact when this field merges.
	field.Required = false
	return field, true
}

// decodeStandardDescribeCatalogV2Narrow reads the narrow result of one member
// of the pack. The fast scan declines on any input outside the plain shape the
// full fast decoder accepts; encoding/json then decodes the member fully.
func decodeStandardDescribeCatalogV2Narrow(pack []byte, entry standardDescribeCatalogV2IndexEntry) (standardDescribeNarrow, error) {
	var out standardDescribeNarrow
	err := withStandardDescribeCatalogV2MemberBytes(pack, entry, standardDescribeCatalogV2Magic, len(standardDescribeCatalogV2Index), func(data []byte) error {
		if narrow, ok := fastDecodeStandardDescribeNarrow(data); ok {
			out = narrow
			return nil
		}
		var describe standardDescribeObject
		if err := json.Unmarshal(data, &describe); err != nil {
			return fmt.Errorf("decode standard describe member %q: %w", entry.Name, err)
		}
		out = standardDescribeNarrowFromDescribe(describe)
		return nil
	})
	if err != nil {
		return standardDescribeNarrow{}, err
	}
	standardDescribeNarrowDecodes.Add(1)
	return out, nil
}

func fastDecodeStandardDescribeNarrow(data []byte) (standardDescribeNarrow, bool) {
	scratch := getDescribeDecodeScratch()
	defer putDescribeDecodeScratch(scratch)
	decoder := describeJSONDecoder{data: data, scratch: scratch}
	var out standardDescribeNarrow
	if !decoder.narrowObject(&out) {
		return standardDescribeNarrow{}, false
	}
	decoder.space()
	if decoder.pos != len(decoder.data) {
		return standardDescribeNarrow{}, false
	}
	return out, true
}

// narrowObject walks the member object and reads mergeable and the "fields" array.
// Every other value is validated and skipped, never materialized. A skipped
// value is checked as JSON, not against the Go type the full decode would use.
func (decoder *describeJSONDecoder) narrowObject(out *standardDescribeNarrow) bool {
	var seenFields, seenMergeable bool
	for first := true; ; first = false {
		key, more, ok := decoder.nextKey(first)
		if !ok {
			return false
		}
		if !more {
			return true
		}
		switch string(key) {
		case "fields":
			if seenFields {
				return false
			}
			seenFields = true
			ok = decoder.narrowFields(out)
		case "mergeable":
			if seenMergeable {
				return false
			}
			seenMergeable = true
			ok = decoder.boolPointerInto(&out.mergeable)
		case "name", "label", "labelPlural", "keyPrefix", "triggerable", "childRelationships", "recordTypeInfos":
			ok = decoder.skipValue(1)
		default:
			ok = !describeObjectKeyFolds(key) && decoder.skipValue(1)
		}
		if !ok {
			return false
		}
	}
}

func (decoder *describeJSONDecoder) narrowFields(out *standardDescribeNarrow) bool {
	if decoder.null() {
		return true
	}
	for first := true; ; first = false {
		more, ok := decoder.nextElement(first)
		if !ok {
			return false
		}
		if !more {
			return true
		}
		start := decoder.pos
		var facet stubDescribeField
		if !decoder.stubField(&facet) {
			return false
		}
		end := decoder.pos
		out.fields = append(out.fields, facet)
		if strings.EqualFold(facet.Type, "currency") {
			out.currency = append(out.currency, facet.Name)
		} else if strings.EqualFold(facet.Type, "reference") {
			decoder.pos = start
			var captured standardDescribeField
			if !decoder.describeField(&captured) || decoder.pos != end {
				return false
			}
			if field, ok := standardDescribeNarrowReference(captured); ok {
				out.references = append(out.references, field)
			}
		}
	}
}

// stubField retains only facets read while applying stub overlays.
func (decoder *describeJSONDecoder) stubField(f *stubDescribeField) bool {
	var seen uint16
	for first := true; ; first = false {
		key, more, ok := decoder.nextKey(first)
		if !ok {
			return false
		}
		if !more {
			return true
		}
		var bit uint16
		switch string(key) {
		case "name":
			bit, ok = 1<<0, decoder.stubPlainString(&f.Name)
		case "type":
			bit, ok = 1<<1, decoder.stubPlainString(&f.Type)
		case "relationshipName":
			bit, ok = 1<<2, decoder.stringInto(&f.RelationshipName)
		case "precision":
			bit, ok = 1<<3, decoder.intInto(&f.Precision)
		case "scale":
			bit, ok = 1<<4, decoder.intInto(&f.Scale)
		case "calculated":
			bit, ok = 1<<5, decoder.boolInto(&f.Calculated)
		case "createable":
			bit, ok = 1<<6, decoder.boolPointerInto(&f.Createable)
		case "updateable":
			bit, ok = 1<<7, decoder.boolPointerInto(&f.Updateable)
		case "aggregatable":
			bit, ok = 1<<8, decoder.boolPointerInto(&f.Aggregatable)
		case "referenceTo":
			bit, ok = 1<<9, decoder.stringsInto(&f.ReferenceTo)
		case "label", "length", "defaultValue", "defaultValueFormula", "compoundFieldName", "nillable", "defaultedOnCreate", "filterable", "groupable", "sortable", "permissionable", "deprecatedAndHidden", "externalId", "unique", "encrypted", "caseSensitive", "idLookup", "polymorphicForeignKey", "picklistValues", "nameField", "extraTypeInfo", "restrictedPicklist":
			ok = decoder.skipValue(1)
		default:
			ok = !describeFieldKeyFolds(key) && decoder.skipValue(1)
		}
		if !ok || seen&bit != 0 {
			return false
		}
		seen |= bit
	}
}

func (decoder *describeJSONDecoder) stubPlainString(out *string) bool {
	if decoder.null() {
		return true
	}
	start := decoder.pos
	end, plain, ok := decoder.scanString()
	if !ok || !plain {
		return false
	}
	*out = string(decoder.data[start+1 : end-1])
	return true
}

// decodeStandardDescribeCatalogV2Fields skips object-level relationships and
// record types while preserving every field facet consumed by enrichment.
func decodeStandardDescribeCatalogV2Fields(pack []byte, entry standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error) {
	var out standardDescribeObject
	err := withStandardDescribeCatalogV2MemberBytes(pack, entry, standardDescribeCatalogV2Magic, len(standardDescribeCatalogV2Index), func(data []byte) error {
		scratch := getDescribeDecodeScratch()
		decoder := describeJSONDecoder{data: data, scratch: scratch}
		var seen uint8
		valid := true
		for first := true; ; first = false {
			key, more, ok := decoder.nextKey(first)
			if !ok {
				valid = false
				break
			}
			if !more {
				break
			}
			var bit uint8
			switch string(key) {
			case "fields":
				bit, ok = 1, decoder.describeFields(&out.Fields)
			case "mergeable":
				bit, ok = 2, decoder.boolPointerInto(&out.Mergeable)
			case "name", "label", "labelPlural", "keyPrefix", "triggerable", "childRelationships", "recordTypeInfos":
				ok = decoder.skipValue(1)
			default:
				ok = !describeObjectKeyFolds(key) && decoder.skipValue(1)
			}
			if !ok || seen&bit != 0 {
				valid = false
				break
			}
			seen |= bit
		}
		decoder.space()
		valid = valid && decoder.pos == len(data)
		putDescribeDecodeScratch(scratch)
		if valid {
			return nil
		}
		var full standardDescribeObject
		if err := json.Unmarshal(data, &full); err != nil {
			return fmt.Errorf("decode standard describe member %q: %w", entry.Name, err)
		}
		out = standardDescribeObject{Fields: full.Fields, Mergeable: full.Mergeable}
		return nil
	})
	if err != nil {
		return standardDescribeObject{}, err
	}
	return out, nil
}
