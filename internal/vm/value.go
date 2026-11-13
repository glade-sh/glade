package vm

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

type Value struct {
	// C10: a recovered Number(18,0) Double token read rejects boxed Long casts.
	// Keep this authority local to that captured read, not arbitrary Doubles.
	scaleZeroNumberTokenDouble bool

	// Loaded SOQL reference authority cannot be authored by Apex JSON fields.
	loadedReferenceSnapshot *sobjectLoadedReferenceSnapshot

	// Display ownership is retained when the constructor resolves its symbol;
	// source classes may share a platform DTO's short name.
	platformHTTPDTO bool
	projectClass    bool
	// Resolved class-instance provenance includes dependency classes and stays
	// separate from project-only display ownership. Schema records leave it false.
	classInstance bool
	// Only source literals that establish lexical class identity authorize JSON
	// to use it. Keep this provenance outside serializable token fields.
	lexicalClassLiteral bool
	// Preserve class-literal origin for the legacy dependency construction path.
	dependencyClassLiteral bool
	// Retained only for the captured single-helper exception text boundary.
	exceptionAnonymousCallerLine int
	// Keep the observed String-constructor display separate from runtime faults,
	// cause constructors and default exception construction.
	exceptionMessageConstructor bool
	// Some platform Ids retain their original text for valueOf and concatenation.
	idOriginalDisplay bool
	// A report service dimension can expose an empty getter while retaining a
	// null raw groupings field for JSON serialization.
	reportEmptyDimension bool

	Kind          ValueKind `json:"kind"`
	Int           int64     `json:"int,omitempty"`
	Decimal       float64   `json:"decimal,omitempty"`
	Bool          bool      `json:"bool,omitempty"`
	Text          string    `json:"text,omitempty"`
	Type          string    `json:"type,omitempty"`
	Static        string    `json:"-"`
	Runtime       string    `json:"-"`
	Ref           uint64    `json:"-"`
	ExplicitScale bool      `json:"-"`
	// Native list membership retains backing values until materialized in an ordinary list.
	nativeListMembership bool
	// Explicit List<SObject> allocations are distinct from framework lists
	// whose Runtime is also generic.
	genericSObjectList bool
	// Retain constructor provenance so the captured newSObject(Id) fix does
	// not change unobserved ordinary-constructor twins.
	schemaIDConstructor bool
	// JSON-deserialized PluginBatchable subclasses skip the platform
	// constructor (platform_batch_runtime org.tsv:4-8,13-17).
	pluginBatchableDetached bool
	// Set history belongs to this collection, not to its mutable elements.
	setInsertionHashes []setInsertionHash
	primitiveSetState  *primitiveSetState
	mapKeySetSource    *mapKeySetBacking
	mapKeySetHashes    *mapKeySetHashCache

	// List-based map insertion retains an unsaved record under a null Id key
	// that can be retrieved through keySet, but not through a literal null.
	sobjectListNullMapKey bool

	// mapBackingShared marks a map whose Map, MapKeys and MapOrder storage is
	// shared with another holder (a typed view of the global describe map).
	// Mutations copy the storage first, so holders never see each other's
	// writes while reads share one copy.
	mapBackingShared bool
	// mapEntriesShared marks a Map whose entries other VMs of the same schema
	// generation read too (the generation's global describe maps). It is
	// read-only: a write clones Map first and clears the mark (detachSharedEntries).
	mapEntriesShared bool
	// fieldsShared marks an object whose Fields map is shared with other VMs of
	// the same schema generation (child relationship describes). It is
	// read-only: a write clones Fields first and clears the mark.
	fieldsShared                   bool
	globalDescribeEntriesCanonical bool

	nativeListElement bool
	Fields            map[string]Value `json:"fields,omitempty"`
	List              []Value          `json:"list,omitempty"`
	Set               []Value          `json:"set,omitempty"`
	Map               map[string]Value `json:"map,omitempty"`
	MapKeys           map[string]Value `json:"-"`
	MapOrder          []string         `json:"-"`
}

// detachSharedEntries gives the value a private copy of Map before an in-place
// entry write, when other VMs share the entries.
func (v *Value) detachSharedEntries() {
	if v.mapEntriesShared {
		v.Map = maps.Clone(v.Map)
		v.mapEntriesShared = false
	}
}

// detachSharedFields gives the value a private copy of Fields before an
// in-place field write, when other VMs share the fields.
func (v *Value) detachSharedFields() {
	if v.fieldsShared {
		v.Fields = maps.Clone(v.Fields)
		v.fieldsShared = false
	}
}

type ValueKind string

const (
	ValueNull    ValueKind = "null"
	ValueInt     ValueKind = "integer"
	ValueDecimal ValueKind = "decimal"
	ValueBool    ValueKind = "boolean"
	ValueString  ValueKind = "string"
	ValueList    ValueKind = "list"
	ValueSet     ValueKind = "set"
	ValueMap     ValueKind = "map"
	ValueObject  ValueKind = "object"
)

var Null = Value{Kind: ValueNull}

// JSONBudgetSnapshot exposes the same current collection view used by JSON
// serialization, so a host can budget it before invoking the encoder.
func (v Value) JSONBudgetSnapshot() Value {
	return v.currentMapKeySet()
}

func (v Value) MarshalJSON() ([]byte, error) {
	v = v.currentMapKeySet()
	type jsonValue Value
	if !strings.EqualFold(v.Type, "AccessLevel") {
		return json.Marshal(jsonValue(v))
	}
	out := jsonValue(v)
	out.Fields = make(map[string]Value, len(v.Fields))
	for name, value := range v.Fields {
		if strings.EqualFold(name, "SYSTEM_MODE") || strings.EqualFold(name, "USER_MODE") {
			continue
		}
		out.Fields[name] = value
	}
	return json.Marshal(out)
}

var nextValueRef atomic.Uint64

func newValueRef() uint64 {
	return nextValueRef.Add(1)
}

func Int(v int64) Value {
	return Value{Kind: ValueInt, Int: v}
}

func longIntValue(v int64) Value {
	value := Int(v)
	value.Type = "Long"
	return value
}

func integerValueForType(value Value, typeName string) Value {
	if strings.EqualFold(strings.TrimSpace(typeName), "Long") {
		value.Type = "Long"
	} else if strings.EqualFold(strings.TrimSpace(typeName), "Integer") {
		value.Type = ""
	}
	return value
}

func Decimal(v float64) Value {
	return Value{Kind: ValueDecimal, Decimal: v, Text: strconv.FormatFloat(v, 'f', -1, 64)}
}

func isFloatBackedDecimal(value Value) bool {
	return value.Kind == ValueDecimal && (strings.EqualFold(strings.TrimSpace(value.Runtime), "Double") || strings.EqualFold(strings.TrimSpace(value.Static), "Double"))
}

func decimalAsDouble(value Value) Value {
	out := Decimal(value.Decimal)
	out.Static = "Double"
	out.Runtime = "Double"
	return out
}

func decimalFromText(text string) (Value, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Null, fmt.Errorf("decimal value cannot be empty")
	}
	canonical, err := canonicalDecimalText(text)
	if err != nil {
		return Null, err
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return Null, err
	}
	return Value{Kind: ValueDecimal, Decimal: parsed, Text: canonical}, nil
}

func canonicalDecimalText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "+") {
		text = text[1:]
	}
	rat, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", fmt.Errorf("invalid decimal %q", text)
	}
	mantissa := text
	exponent := 0
	if expIndex := strings.IndexAny(mantissa, "eE"); expIndex >= 0 {
		parsed, err := strconv.Atoi(mantissa[expIndex+1:])
		if err != nil {
			return "", fmt.Errorf("invalid decimal %q", text)
		}
		exponent = parsed
		mantissa = mantissa[:expIndex]
	}
	fractionDigits := 0
	if dotIndex := strings.IndexByte(mantissa, '.'); dotIndex >= 0 {
		fractionDigits = len(mantissa) - dotIndex - 1
	}
	scale := fractionDigits - exponent
	return ratFixedText(rat, int64(scale)), nil
}

func decimalFromRat(value *big.Rat, scale int64) Value {
	if value == nil {
		return Null
	}
	text := ratFixedText(value, scale)
	parsed, _ := value.Float64()
	return Value{Kind: ValueDecimal, Decimal: parsed, Text: text}
}

func Bool(v bool) Value {
	return Value{Kind: ValueBool, Bool: v}
}

func String(v string) Value {
	return Value{Kind: ValueString, Text: v}
}

func List(values ...Value) Value {
	// Apex collection values are never null merely because they contain no
	// elements. Keep the backing slice non-nil so an empty query result remains
	// a usable empty List value (and continues to differ from Null).
	if values == nil {
		values = []Value{}
	}
	return Value{Kind: ValueList, List: values, Ref: newValueRef()}
}

func Set(values ...Value) Value {
	out := Value{Kind: ValueSet, Ref: newValueRef()}
	for _, value := range values {
		found := false
		for _, stored := range out.Set {
			if setPrimitiveValuesEqual(stored, value) {
				found = true
				break
			}
		}
		if !found {
			out.Set = append(out.Set, value)
		}
	}
	out.primitiveSetState = &primitiveSetState{items: out.Set}
	return out
}

func Map() Value {
	return Value{Kind: ValueMap, Map: make(map[string]Value), MapKeys: make(map[string]Value), Ref: newValueRef()}
}

func Object(typeName string) Value {
	return Value{Kind: ValueObject, Type: typeName, Fields: make(map[string]Value), Ref: newValueRef()}
}

func (v Value) writeText(w *textWriter) {
	if w.full {
		return
	}
	switch v.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		w.WriteString(v.String())
	case ValueList:
		w.WriteString("List")
		writeValuesText(w, v.List)
	case ValueSet:
		w.WriteString("Set")
		writeValuesText(w, v.Set)
	case ValueMap:
		if v.mapKeySetSource != nil {
			v = v.currentFieldMapHolder()
		}
		writeMapText(w, v)
	case ValueObject:
		if writeHTTPDTOText(w, v) {
			return
		}
		if strings.EqualFold(v.Runtime, "System.Location") {
			writeLocationText(w, v)
			return
		}
		if strings.EqualFold(v.Type, "AccessLevel") {
			writeAccessLevelText(w, v, make(map[uint64]bool))
			return
		}
		if writeDatabaseDTOText(w, v) {
			return
		}
		if writeAutomationDTOText(w, v, make(map[uint64]bool)) {
			return
		}
		if stubbedType, ok := stubProxyTypeName(v); ok {
			w.WriteString(stubbedType)
			w.WriteString("__sfdc_ApexStub:")
			w.WriteString(strconv.FormatUint(v.Ref, 10))
			return
		}
		// Source exceptions retain the message fallback used by surfaced errors
		// (r_error_custom_imperative and r_repeat_exception).
		if v.projectClass && !isExceptionType(v.Type) {
			writeObjectFieldsText(w, v, make(map[uint64]bool))
			return
		}
		if strings.EqualFold(v.Type, "PageReference") {
			if rawURL, ok := v.Fields["url"]; ok && rawURL.Kind == ValueString {
				w.WriteString(rawURL.Text)
			}
			return
		}
		if strings.EqualFold(v.Type, "Schema.SObjectType") {
			if objectName, ok := v.Fields["object"]; ok && objectName.Kind == ValueString {
				w.WriteString(objectName.Text)
				return
			}
		}
		if strings.EqualFold(v.Type, "Schema.SObjectField") {
			objectName, hasObject := v.Fields["object"]
			fieldName, hasField := v.Fields["field"]
			if hasObject && hasField && objectName.Kind == ValueString && fieldName.Kind == ValueString {
				w.WriteString(fieldName.Text)
				return
			}
		}
		if raw, ok := v.Fields["value"]; ok && raw.Kind == ValueString {
			if strings.EqualFold(v.Type, "Id") {
				w.WriteString(displayIDText(raw.Text))
				return
			}
			w.WriteString(raw.Text)
			return
		}
		if message, ok := v.Fields["message"]; ok && message.Kind == ValueString {
			w.WriteString(message.Text)
			return
		}
		if writeObjectFieldsText(w, v, make(map[uint64]bool)) {
			return
		}
		w.WriteString(v.Type)
		w.WriteString(":{}")
	default:
		w.WriteString("<")
		w.WriteString(string(v.Kind))
		w.WriteString(">")
	}
}

var accessLevelConstantRefs = [2]uint64{newValueRef(), newValueRef()}

func isAccessLevelValue(value Value) bool {
	return value.Kind == ValueObject && !value.classInstance && strings.EqualFold(value.Type, "AccessLevel") &&
		value.Fields["currentAccessPermissions"].Kind == ValueString
}

// accessLevelHasCustomScope reports a withPermissionSetId scope. Native rows
// cover only the System/User constants, so custom scopes keep base behaviour.
func accessLevelHasCustomScope(value Value) bool {
	_, custom := value.Fields["permissionSetId"]
	return custom
}

func accessLevelUsesReferenceEquality(left, right Value) bool {
	if !isAccessLevelValue(left) || !isAccessLevelValue(right) {
		return false
	}
	_, leftCustom := left.Fields["permissionSetId"]
	_, rightCustom := right.Fields["permissionSetId"]
	return !leftCustom && !rightCustom
}

func accessLevelValue(mode string) Value {
	system := Object("AccessLevel")
	system.Ref = accessLevelConstantRefs[0]
	system.Text = "SYSTEM_MODE"
	user := Object("AccessLevel")
	user.Ref = accessLevelConstantRefs[1]
	user.Text = "USER_MODE"
	system.Fields["SYSTEM_MODE"] = system
	system.Fields["USER_MODE"] = user
	system.Fields["currentAccessPermissions"] = String("SYSTEM_MODE")
	system.Fields["permSetId"] = Null
	user.Fields["SYSTEM_MODE"] = system
	user.Fields["USER_MODE"] = user
	user.Fields["currentAccessPermissions"] = String("USER_MODE")
	user.Fields["permSetId"] = Null
	if strings.EqualFold(mode, "USER_MODE") {
		return user
	}
	return system
}

func accessLevelClone(value Value) Value {
	root := Object("AccessLevel")
	root.Text = strings.ToUpper(strings.TrimSpace(value.Text))
	system := Object("AccessLevel")
	system.Text = "SYSTEM_MODE"
	user := Object("AccessLevel")
	user.Text = "USER_MODE"
	root.Fields["SYSTEM_MODE"] = system
	root.Fields["USER_MODE"] = root
	root.Fields["currentAccessPermissions"] = String(root.Text)
	root.Fields["permSetId"] = Null
	system.Fields["SYSTEM_MODE"] = root
	system.Fields["USER_MODE"] = user
	system.Fields["currentAccessPermissions"] = String("SYSTEM_MODE")
	system.Fields["permSetId"] = Null
	user.Fields["SYSTEM_MODE"] = root
	user.Fields["USER_MODE"] = user
	user.Fields["currentAccessPermissions"] = String("USER_MODE")
	user.Fields["permSetId"] = Null
	if permissionSetID, ok := value.Fields["permissionSetId"]; ok {
		root.Fields["permissionSetId"] = cloneValue(permissionSetID)
		if permissionSetID.Kind != ValueNull {
			root.Fields["currentAccessPermissions"] = String("CUSTOM")
		}
	}
	return root
}

func accessLevelString(value Value) string {
	var w textWriter
	writeAccessLevelText(&w, value, make(map[uint64]bool))
	return w.String()
}

func writeAccessLevelText(w *textWriter, value Value, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			w.WriteString("(already output)")
			return
		}
		// Native marks an object "already output" once it has been written
		// anywhere in the rendering, not only while it is being written (org
		// capture access_level_rendering).
		seen[value.Ref] = true
	}
	w.WriteString("AccessLevel:[SYSTEM_MODE=")
	writeAccessLevelFieldText(w, value.Fields["SYSTEM_MODE"], seen)
	w.WriteString(", USER_MODE=")
	writeAccessLevelFieldText(w, value.Fields["USER_MODE"], seen)
	w.WriteString(", currentAccessPermissions=")
	writeAccessLevelFieldText(w, value.Fields["currentAccessPermissions"], seen)
	w.WriteString(", permSetId=")
	if permissionSetID, ok := value.Fields["permissionSetId"]; ok {
		permissionSetID.writeText(w)
	} else {
		w.WriteString("null")
	}
	w.WriteString("]")
}

func writeAccessLevelFieldText(w *textWriter, value Value, seen map[uint64]bool) {
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "AccessLevel") {
		writeAccessLevelText(w, value, seen)
		return
	}
	value.writeText(w)
}

// writeObjectFieldsText writes the field display of an object and reports
// whether that display applies; it writes nothing when it does not.
func writeObjectFieldsText(w *textWriter, v Value, seen map[uint64]bool) bool {
	if v.Kind != ValueObject || (len(v.Fields) == 0 && !v.projectClass) {
		return false
	}
	// A stub proxy never shows the VM's provider bookkeeping fields (NS312B).
	if _, ok := stubProxyTypeName(v); ok {
		v.writeText(w)
		return true
	}
	if writeAutomationDTOText(w, v, seen) {
		return true
	}
	if writeDatabaseDTOText(w, v) {
		return true
	}
	if writeMessagingTemplateResultText(w, v) {
		return true
	}
	if v.Ref != 0 {
		if seen[v.Ref] {
			w.WriteString(v.Type)
			w.WriteString(":{...}")
			return true
		}
		seen[v.Ref] = true
		defer delete(seen, v.Ref)
	}
	keys := make([]string, 0, len(v.Fields))
	isSObject := sObjectValueType(v.Type)
	for key := range v.Fields {
		if isInheritedInstanceFieldStorage(key) || isPicklistJSONValidForField(v, key) {
			continue
		}
		// R243: Object text includes populated fields, without implicit
		// SObject defaults or the VM's field-tracking metadata.
		if isLexicalInstanceFieldStorage(key) || isSObject && (isInternalSObjectField(key) || isDefaultedSObjectField(v, key) || !sObjectEqualityFieldVisible(v, key)) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if selected, queried := v.Fields[sobjectQueriedFieldsField]; isSObject && queried && len(selected.MapOrder) > 0 {
		order := make(map[string]int, len(selected.MapOrder))
		for i, encoded := range selected.MapOrder {
			order[mapStoredKey(selected, encoded).String()] = i
		}
		sort.SliceStable(keys, func(i, j int) bool {
			left, leftSelected := order[strings.ToLower(keys[i])]
			right, rightSelected := order[strings.ToLower(keys[j])]
			if leftSelected != rightSelected {
				return leftSelected
			}
			return leftSelected && left < right
		})
	}
	w.WriteString(v.Type)
	if v.projectClass {
		w.WriteString(":[")
	} else {
		w.WriteString(":{")
	}
	for i, key := range keys {
		if w.full {
			return true
		}
		if i > 0 {
			w.WriteString(", ")
		}
		w.WriteString(key)
		w.WriteString("=")
		field := v.Fields[key]
		if isSObject && field.Kind == ValueObject && strings.EqualFold(field.Type, "Id") {
			field.writeText(w)
		} else {
			writeValueTextWithSeen(w, field, seen)
		}
	}
	if v.projectClass {
		w.WriteString("]")
	} else {
		w.WriteString("}")
	}
	return true
}

// databaseDTOPart is one "label=value" segment of a Database DTO display.
// The label carries its leading separator.
type databaseDTOPart struct {
	label string
	field string
	mode  byte // 'f' field, 'b' boolean, 'o' outer DTO field
}

type databaseDTOFormat struct {
	parts  []databaseDTOPart
	suffix string
}

var databaseDTOFormats = map[string]databaseDTOFormat{
	"assignmentruleheader": {[]databaseDTOPart{
		{"AssignmentRuleHeader:[AssignmentRuleId=", "AssignmentRuleId", 'f'},
		{", UseDefaultRule=", "UseDefaultRule", 'f'},
	}, "]"},
	"duplicateruleheader": {[]databaseDTOPart{
		{"DuplicateRuleHeader:[AllowSave=", "AllowSave", 'f'},
		{", RunAsCurrentUser=", "RunAsCurrentUser", 'f'},
	}, "]"},
	"emailheader": {[]databaseDTOPart{
		{"EmailHeader:[TriggerAutoResponseEmail=", "TriggerAutoResponseEmail", 'f'},
		{", TriggerOtherEmail=", "TriggerOtherEmail", 'f'},
		{", TriggerUserEmail=", "TriggerUserEmail", 'f'},
	}, "]"},
	"dmloptions": {[]databaseDTOPart{
		{"DMLOptions:[AllowFieldTruncation=", "AllowFieldTruncation", 'o'},
		{", AssignmentRuleHeader=", "AssignmentRuleHeader", 'o'},
		{", DuplicateRuleHeader=", "DuplicateRuleHeader", 'o'},
		{", EmailHeader=", "EmailHeader", 'o'},
		{", LocaleOptions=", "LocaleOptions", 'o'},
		{", LocalizeErrors=", "LocalizeErrors", 'o'},
		{", OptAllOrNone=", "OptAllOrNone", 'o'},
	}, "]"},
	"leadconvert": {[]databaseDTOPart{
		{"Database.LeadConvert[getAccountId=", "accountId", 'f'},
		{";getAccountRecord=", "accountRecord", 'f'},
		{";getBypassAccountDedupeCheck=", "bypassAccountDedupeCheck", 'f'},
		{";getBypassContactDedupeCheck=", "bypassContactDedupeCheck", 'f'},
		{";getContactId=", "contactId", 'f'},
		{";getContactRecord=", "contactRecord", 'f'},
		{";getConvertedStatus=", "convertedStatus", 'f'},
		{";getLeadId=", "leadId", 'f'},
		{";getOpportunityId=", "opportunityId", 'f'},
		{";getOpportunityName=", "opportunityName", 'f'},
		{";getOpportunityRecord=", "opportunityRecord", 'f'},
		{";getOwnerId=", "ownerId", 'f'},
		{";getRelatedPersonAccountId=", "relatedPersonAccountId", 'f'},
		{";getRelatedPersonAccountRecord=", "relatedPersonAccountRecord", 'f'},
		{";isDoNotCreateOpportunity=", "doNotCreateOpportunity", 'b'},
		{";isOverwriteLeadSource=", "overwriteLeadSource", 'b'},
		{";isSendNotificationEmail=", "sendNotificationEmail", 'b'},
	}, ";]"},
	"querylocator": {[]databaseDTOPart{
		{"Database.QueryLocator[Query=", "Query", 'f'},
	}, "]"},
	// pagination_cursor_versions: a cursor from any factory, before any
	// member call, prints this at class API 61.0 to 67.0.
	"paginationcursor": {nil, "PaginationCursor:[numRecords=null, queryId=<empty>]"},
	"recordtypeinfo": {[]databaseDTOPart{
		{"Schema.RecordTypeInfo[getDeveloperName=", "developerName", 'f'},
		{";getName=", "name", 'f'},
		{";getRecordTypeId=", "recordTypeId", 'f'},
		{";isActive=", "active", 'b'},
		{";isAvailable=", "available", 'b'},
		{";isDefaultRecordTypeMapping=", "defaultRecordTypeMapping", 'b'},
		{";isMaster=", "master", 'b'},
	}, ";]"},
}

func databaseDTOFormatFor(typeName string) (databaseDTOFormat, bool) {
	switch key := strings.ToLower(typeName); key {
	case "database.assignmentruleheader", "database.duplicateruleheader", "database.emailheader",
		"database.dmloptions", "database.leadconvert", "database.querylocator":
		format, ok := databaseDTOFormats[strings.TrimPrefix(key, "database.")]
		return format, ok
	case "database.paginationcursor":
		return databaseDTOFormats["paginationcursor"], true
	case "schema.recordtypeinfo":
		return databaseDTOFormats["recordtypeinfo"], true
	case "assignmentruleheader", "duplicateruleheader", "emailheader", "dmloptions", "leadconvert",
		"querylocator", "recordtypeinfo":
		format, ok := databaseDTOFormats[key]
		return format, ok
	default:
		return databaseDTOFormat{}, false
	}
}

func writeDatabaseDTOText(w *textWriter, v Value) bool {
	format, ok := databaseDTOFormatFor(v.Type)
	if !ok {
		return false
	}
	for _, part := range format.parts {
		if w.full {
			return true
		}
		w.WriteString(part.label)
		value, found := databaseDTOField(v, part.field)
		switch part.mode {
		case 'b':
			if !found || value.Kind == ValueNull {
				w.WriteString("false")
				continue
			}
		case 'o':
			if !found || value.Kind == ValueNull {
				w.WriteString("null")
				continue
			}
			if value.Kind == ValueObject {
				if databaseDTOObjectIsDefault(value) {
					w.WriteString("null")
					continue
				}
				if writeDatabaseDTOText(w, value) {
					continue
				}
			}
		default:
			if !found {
				w.WriteString("null")
				continue
			}
		}
		value.writeText(w)
	}
	w.WriteString(format.suffix)
	return true
}

func databaseDTOField(v Value, field string) (Value, bool) {
	if value, ok := v.Fields[field]; ok {
		return value, true
	}
	for name, value := range v.Fields {
		if strings.EqualFold(name, field) {
			return value, true
		}
	}
	return Null, false
}

func databaseDTOObjectIsDefault(v Value) bool {
	if v.Kind != ValueObject || len(v.Fields) == 0 {
		return false
	}
	for _, value := range v.Fields {
		if value.Kind != ValueNull {
			return false
		}
	}
	return true
}

func valueStringWithSeen(v Value, seen map[uint64]bool) string {
	var w textWriter
	writeValueTextWithSeen(&w, v, seen)
	return w.String()
}

func writeValueTextWithSeen(w *textWriter, v Value, seen map[uint64]bool) {
	if w.full {
		return
	}
	v = v.currentMapKeySet()
	switch v.Kind {
	case ValueList, ValueSet:
		items := v.List
		if v.Kind == ValueSet {
			items = v.Set
			w.WriteString("Set[")
		} else {
			w.WriteString("List[")
		}
		for i, value := range items {
			if w.full {
				return
			}
			if i > 0 {
				w.WriteString(", ")
			}
			writeValueTextWithSeen(w, value, seen)
		}
		w.WriteString("]")
		return
	case ValueMap:
		if len(v.Map) == 0 {
			w.WriteString("{}")
			return
		}
		w.WriteString("Map{")
		for i, key := range sortedMapKeys(v.Map) {
			if w.full {
				return
			}
			if i > 0 {
				w.WriteString(", ")
			}
			writeValueTextWithSeen(w, mapStoredKey(v, key), seen)
			w.WriteString("=")
			writeValueTextWithSeen(w, v.Map[key], seen)
		}
		w.WriteString("}")
		return
	case ValueObject:
		if writeObjectFieldsText(w, v, seen) {
			return
		}
	}
	v.writeText(w)
}

func decimalDisplayText(value Value) string {
	text := value.Text
	if text == "" {
		text = strconv.FormatFloat(value.Decimal, 'f', -1, 64)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if strings.ContainsAny(text, "eE") {
		if rat, ok := new(big.Rat).SetString(text); ok {
			if rat.IsInt() {
				text = rat.Num().String()
			} else {
				scale := decimalScale(value)
				if scale < 0 {
					scale = 0
				}
				text = rat.FloatString(scale)
			}
		}
	}
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}
	if text == "-0" {
		return "0"
	}
	return text
}

func stubProxyTypeName(value Value) (string, bool) {
	if value.Kind != ValueObject {
		return "", false
	}
	raw, ok := value.Fields["__gladeStubbedType"]
	if !ok || raw.Kind != ValueString || raw.Text == "" {
		return "", false
	}
	return raw.Text, true
}

func (v Value) Equal(other Value) bool {
	return v.equal(other, make(map[[2]uint64]bool))
}

func (v Value) equal(other Value, seen map[[2]uint64]bool) bool {
	v, other = v.currentMapKeySet(), other.currentMapKeySet()
	if v.Ref != 0 && other.Ref != 0 {
		if v.Ref == other.Ref && v.Kind == ValueObject && other.Kind == ValueObject {
			return true
		}
		key := [2]uint64{v.Ref, other.Ref}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	if v.Kind != other.Kind {
		if v.Kind == ValueInt && other.Kind == ValueDecimal {
			return numericDecimalValuesEqual(v, other)
		}
		if v.Kind == ValueDecimal && other.Kind == ValueInt {
			return numericDecimalValuesEqual(v, other)
		}
		if v.Kind == ValueString && other.Kind == ValueObject && strings.EqualFold(other.Type, "Id") {
			if text, ok := platformScalarObjectText(other); ok {
				return apexIDTextEqual(v.Text, text)
			}
		}
		if v.Kind == ValueObject && strings.EqualFold(v.Type, "Id") && other.Kind == ValueString {
			if text, ok := platformScalarObjectText(v); ok {
				return apexIDTextEqual(text, other.Text)
			}
		}
		if v.Kind == ValueString && other.Kind == ValueObject && isStringComparableEnum(other.Type) {
			return v.Text == other.Text
		}
		if v.Kind == ValueObject && isStringComparableEnum(v.Type) && other.Kind == ValueString {
			return v.Text == other.Text
		}
		return false
	}
	switch v.Kind {
	case ValueNull:
		return true
	case ValueInt:
		return v.Int == other.Int
	case ValueDecimal:
		return numericDecimalValuesEqual(v, other)
	case ValueBool:
		return v.Bool == other.Bool
	case ValueString:
		if strings.EqualFold(v.Type, "Id") && strings.EqualFold(other.Type, "Id") {
			return apexIDTextEqual(v.Text, other.Text)
		}
		// Apex String equality is case-sensitive. SOQL text comparison is intentionally
		// case-insensitive in internal/soql to match query semantics.
		return v.Text == other.Text
	case ValueList:
		if len(v.List) != len(other.List) {
			return false
		}
		for i := range v.List {
			if !listElementValuesEqual(v.List[i], other.List[i], seen) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(v.Set) != len(other.Set) {
			return false
		}
		for _, value := range v.Set {
			if !containsValue(other.Set, value) {
				return false
			}
		}
		return true
	case ValueMap:
		if len(v.Map) != len(other.Map) {
			return false
		}
		for key, value := range v.Map {
			otherValue, ok := other.Map[key]
			if !ok || !mapStoredKey(v, key).equal(mapStoredKey(other, key), seen) || !value.equal(otherValue, seen) {
				return false
			}
		}
		return true
	case ValueObject:
		if accessLevelUsesReferenceEquality(v, other) {
			return v.Ref != 0 && v.Ref == other.Ref
		}
		if strings.EqualFold(v.Type, "Matcher") || strings.EqualFold(other.Type, "Matcher") {
			return strings.EqualFold(v.Type, other.Type) && v.Ref != 0 && v.Ref == other.Ref
		}
		if isDescribeSObjectResultType(v.Type) || isDescribeSObjectResultType(other.Type) {
			return isDescribeSObjectResultType(v.Type) && isDescribeSObjectResultType(other.Type) && describeSObjectResultIdentityEqual(v, other)
		}
		if isDescribeFieldResultType(v.Type) || isDescribeFieldResultType(other.Type) {
			return isDescribeFieldResultType(v.Type) && isDescribeFieldResultType(other.Type) && describeFieldResultIdentityEqual(v, other)
		}
		if strings.EqualFold(v.Type, "Type") && strings.EqualFold(other.Type, "Type") {
			leftType := typeValueIdentityName(v)
			rightType := typeValueIdentityName(other)
			if leftType != "" || rightType != "" {
				return canonicalTypeValueIdentity(leftType) == canonicalTypeValueIdentity(rightType)
			}
		}
		if strings.EqualFold(v.Type, "Schema.SObjectType") && strings.EqualFold(other.Type, "Schema.SObjectType") {
			if sObjectTypeTokenEqual(v, other) {
				return true
			}
			return mapKey(v) == mapKey(other)
		}
		if strings.EqualFold(v.Type, "Schema.SObjectField") && strings.EqualFold(other.Type, "Schema.SObjectField") {
			return mapKey(v) == mapKey(other)
		}
		if strings.EqualFold(v.Type, "PageReference") && strings.EqualFold(other.Type, "PageReference") {
			leftURL, leftOK := v.Fields["url"]
			rightURL, rightOK := other.Fields["url"]
			return leftOK && rightOK && leftURL.Equal(rightURL)
		}
		if platformScalarObject(v.Type) {
			value, ok := v.Fields["value"]
			otherValue, otherOK := other.Fields["value"]
			if !ok || !otherOK {
				return false
			}
			if strings.EqualFold(v.Type, "Id") && strings.EqualFold(other.Type, "Id") && value.Kind == ValueString && otherValue.Kind == ValueString {
				return apexIDTextEqual(value.Text, otherValue.Text)
			}
			if strings.EqualFold(v.Type, "Date") && strings.EqualFold(other.Type, "Date") {
				leftDate, leftErr := parsePlatformDate(v)
				rightDate, rightErr := parsePlatformDate(other)
				if leftErr == nil && rightErr == nil {
					return leftDate.Year() == rightDate.Year() && leftDate.Month() == rightDate.Month() && leftDate.Day() == rightDate.Day()
				}
			}
			if sameDateAndMidnightDatetime(v, other) || sameDateAndMidnightDatetime(other, v) {
				return true
			}
			return strings.EqualFold(v.Type, other.Type) && value.equal(otherValue, seen)
		}
		if v.Text != "" || other.Text != "" {
			return (strings.EqualFold(v.Type, other.Type) || namespaceQualifiedTypeEquivalent(v.Type, other.Type)) && v.Text == other.Text
		}
		if sObjectValueType(v.Type) && sObjectValueType(other.Type) {
			return sObjectValuesEqual(v, other, seen)
		}
		if strings.HasPrefix(strings.ToLower(v.Type), "connectapi.") && strings.HasPrefix(strings.ToLower(other.Type), "connectapi.") {
			return strings.EqualFold(v.Type, other.Type) && objectFieldsEqual(v.Fields, other.Fields, seen)
		}
		if v.Ref == 0 && other.Ref == 0 && strings.EqualFold(v.Type, other.Type) {
			return objectFieldsEqual(v.Fields, other.Fields, seen)
		}
		return strings.EqualFold(v.Type, other.Type) && fmt.Sprintf("%p", v.Fields) == fmt.Sprintf("%p", other.Fields)
	default:
		return false
	}
}

func numericDecimalValuesEqual(left, right Value) bool {
	if isFloatBackedDecimal(left) && isFloatBackedDecimal(right) && math.IsNaN(left.Decimal) && math.IsNaN(right.Decimal) {
		return true
	}

	leftRat, leftOK := valueDecimalRat(left)
	rightRat, rightOK := valueDecimalRat(right)
	if leftOK && rightOK {
		return leftRat.Cmp(rightRat) == 0
	}
	switch {
	case left.Kind == ValueInt && right.Kind == ValueDecimal:
		return float64(left.Int) == right.Decimal
	case left.Kind == ValueDecimal && right.Kind == ValueInt:
		return left.Decimal == float64(right.Int)
	case left.Kind == ValueDecimal && right.Kind == ValueDecimal:
		return left.Decimal == right.Decimal
	default:
		return false
	}
}

func valueDecimalRat(value Value) (*big.Rat, bool) {
	if isFloatBackedDecimal(value) {
		return nil, false
	}
	switch value.Kind {
	case ValueInt:
		return new(big.Rat).SetInt64(value.Int), true
	case ValueDecimal:
		text := strings.TrimSpace(value.Text)
		if text == "" {
			return nil, false
		}
		rat, ok := new(big.Rat).SetString(text)
		return rat, ok
	default:
		return nil, false
	}
}

func objectFieldsEqual(left, right map[string]Value, seen map[[2]uint64]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		rightValue, ok := right[key]
		if !ok || !leftValue.equal(rightValue, seen) {
			return false
		}
	}
	return true
}

func sameDateAndMidnightDatetime(dateValue, datetimeValue Value) bool {
	if !strings.EqualFold(dateValue.Type, "Date") ||
		(!strings.EqualFold(datetimeValue.Type, "Datetime") && !strings.EqualFold(datetimeValue.Type, "DateTime")) {
		return false
	}
	date, err := parsePlatformDate(dateValue)
	if err != nil {
		return false
	}
	datetime, err := parsePlatformDatetime(datetimeValue)
	if err != nil {
		return false
	}
	return datetime.Hour() == 0 && datetime.Minute() == 0 && datetime.Second() == 0 && datetime.Nanosecond() == 0 &&
		date.Year() == datetime.Year() && date.Month() == datetime.Month() && date.Day() == datetime.Day()
}

func sObjectValueType(typeName string) bool {
	key := strings.ToLower(typeName)
	return strings.EqualFold(typeName, "sObject") || strings.EqualFold(typeName, "AggregateResult") ||
		isCommonSObjectTypeName(typeName) || strings.HasSuffix(key, "__c") ||
		strings.HasSuffix(key, "__b") || strings.HasSuffix(key, "__e") || strings.HasSuffix(key, "__mdt") ||
		strings.HasSuffix(key, "__r") || strings.HasSuffix(key, "__share")
}

func sObjectValuesEqual(left, right Value, seen map[[2]uint64]bool) bool {
	if !sObjectValueTypesEqual(left.Type, right.Type) {
		return false
	}
	for key, leftValue := range left.Fields {
		if !sObjectEqualityFieldIncluded(left, key, leftValue) {
			continue
		}
		_, rightValue, ok := objectFieldValue(right, key)
		if !ok || !sObjectEqualityFieldIncluded(right, key, rightValue) {
			return false
		}
		if !sObjectFieldValuesEqual(leftValue, rightValue, seen) {
			return false
		}
	}
	for key, rightValue := range right.Fields {
		if !sObjectEqualityFieldIncluded(right, key, rightValue) {
			continue
		}
		if _, leftValue, ok := objectFieldValue(left, key); !ok || !sObjectEqualityFieldIncluded(left, key, leftValue) {
			return false
		}
	}
	return true
}

// Equality and hashing use the same field projection. Captured Schema
// Checkbox presence is separate from assignment; explicit nulls remain present.
func sObjectEqualityFieldIncluded(owner Value, field string, value Value) bool {
	// Captured standard records distinguish an explicitly cleared identity
	// from an absent field. Keep other record shapes on their existing path.
	if strings.EqualFold(field, "Id") && value.Kind == ValueNull &&
		!(capturedStandardHashRecord(owner) && isExplicitSObjectField(owner, field)) {
		return false
	}
	return !isInternalSObjectField(field) && sObjectEqualityFieldVisible(owner, field) &&
		(schemaLoadedCheckboxEqualityField(owner, field, value) || !sObjectMissingFieldEqualsImplicitDefault(owner, field, value))
}

func capturedStandardHashRecord(value Value) bool {
	if value.Kind != ValueObject || value.classInstance {
		return false
	}
	switch strings.ToLower(value.Type) {
	case "account", "contact", "opportunity", "lead", "case":
		return true
	default:
		return false
	}
}

// System fields hydrated for runtime bookkeeping do not widen an SObject's
// field values for equality. Explicit assignments and SOQL selections do.
func sObjectEqualityFieldVisible(owner Value, field string) bool {
	if !isSObjectSystemField(field) || strings.EqualFold(field, "Id") || isExplicitSObjectField(owner, field) {
		return true
	}
	selected, queried := owner.Fields[sobjectQueriedFieldsField]
	if !queried || selected.Kind != ValueMap {
		return true
	}
	if dmlAccessibleSObject(owner) {
		return false
	}
	value, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
	return ok && value.Kind == ValueBool && value.Bool
}

func listElementValuesEqual(left, right Value, seen map[[2]uint64]bool) bool {
	if isNumeric(left) && isNumeric(right) && !strings.EqualFold(runtimeValueTypeName(left), runtimeValueTypeName(right)) {
		return false
	}
	return left.equal(right, seen)
}

func sObjectFieldValuesEqual(left, right Value, seen map[[2]uint64]bool) bool {
	leftID, leftOK := sObjectIDFromValue(left)
	rightID, rightOK := sObjectIDFromValue(right)
	if leftOK && rightOK {
		return apexIDTextEqual(string(leftID), string(rightID))
	}
	if isRelationshipNull(left) && parentRelationshipObjectIsMissingDeep(right) {
		return true
	}
	if isRelationshipNull(right) && parentRelationshipObjectIsMissingDeep(left) {
		return true
	}
	return left.equal(right, seen)
}

func sObjectValueTypesEqual(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	leftBase := stripSObjectNamespacePrefix(canonicalRuntimePlatformType(left))
	rightBase := stripSObjectNamespacePrefix(canonicalRuntimePlatformType(right))
	if !strings.EqualFold(leftBase, rightBase) {
		return false
	}
	leftNamespaced := !strings.EqualFold(leftBase, canonicalRuntimePlatformType(left))
	rightNamespaced := !strings.EqualFold(rightBase, canonicalRuntimePlatformType(right))
	return leftNamespaced != rightNamespaced
}

func sObjectMissingFieldEqualsImplicitDefault(owner Value, field string, value Value) bool {
	if isExplicitSObjectField(owner, field) {
		return false
	}
	// All-null parent projections represent the same absence as a typed null.
	// Explicit fields keep constructed parents from being treated as missing.
	if value.Kind == ValueObject && sObjectValueType(value.Type) && sObjectHashMissingRelationship(value, map[uint64]bool{}) {
		return true
	}
	// Queried non-null values remain present even when they are zero or false.
	// A queried null stays absent until the caller explicitly assigns it.
	if value.Kind != ValueNull {
		if selected, queried := owner.Fields[sobjectQueriedFieldsField]; queried && selected.Kind == ValueMap {
			selectedValue, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
			if ok && selectedValue.Kind == ValueBool && selectedValue.Bool {
				return false
			}
		}
	}
	// User JSON allocations retain only supplied fields. Keep constructor
	// defaults out of the same value/hash projection, without hiding explicit
	// assignments or selected values. Other default boundaries need captures.
	if strings.EqualFold(owner.Type, "User") && isDefaultedSObjectField(owner, field) {
		return true
	}
	switch value.Kind {
	case ValueNull:
		return true
	case ValueBool:
		return !value.Bool
	case ValueInt:
		return value.Int == 0
	case ValueDecimal:
		return value.Decimal == 0
	case ValueString:
		return value.Text == ""
	default:
		return false
	}
}

func isStringComparableEnum(typeName string) bool {
	switch {
	case strings.EqualFold(typeName, "Schema.DisplayType"), strings.EqualFold(typeName, "Schema.SOAPType"),
		strings.EqualFold(typeName, "StatusCode"):
		return true
	default:
		return false
	}
}

func apexIDTextEqual(left, right string) bool {
	if left == right {
		return true
	}
	if len(left) == 15 && len(right) == 18 {
		return validateApexID(right) == nil && left == right[:15]
	}
	if len(left) == 18 && len(right) == 15 {
		return validateApexID(left) == nil && left[:15] == right
	}
	return false
}

func canonicalIDMapKey(value string) string {
	if len(value) == 15 || (len(value) == 18 && validateApexID(value) == nil) {
		return value[:15]
	}
	return value
}

func looksLikeID(value string) bool {
	if len(value) != 15 && len(value) != 18 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		return false
	}
	return true
}

func looksLikeComparableIDText(value string) bool {
	if !looksLikeID(value) || len(value) < 3 {
		return false
	}
	_, ok := standardSObjectPrefixes[value[:3]]
	return ok
}

func shouldCompareTextAsID(left, right string) bool {
	if !looksLikeID(left) || !looksLikeID(right) {
		return false
	}
	if looksLikeComparableIDText(left) || looksLikeComparableIDText(right) {
		return true
	}
	return len(left) == 18 || len(right) == 18
}

func valueIdentityEqual(left, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case ValueList, ValueSet, ValueMap, ValueObject:
		if accessLevelUsesReferenceEquality(left, right) {
			return left.Ref != 0 && left.Ref == right.Ref
		}
		if left.Kind == ValueObject && schemaTokenIdentityEqual(left, right) {
			return true
		}
		if left.Kind == ValueObject && left.Text != "" && right.Text != "" &&
			(strings.EqualFold(left.Type, right.Type) || namespaceQualifiedTypeEquivalent(left.Type, right.Type)) {
			return strings.EqualFold(left.Text, right.Text)
		}
		return left.Ref != 0 && left.Ref == right.Ref
	default:
		return left.Equal(right)
	}
}

func schemaTokenIdentityEqual(left, right Value) bool {
	if !strings.EqualFold(left.Type, right.Type) {
		return false
	}
	switch {
	case strings.EqualFold(left.Type, "Schema.SObjectType"):
		leftObject, leftOK := left.Fields["object"]
		rightObject, rightOK := right.Fields["object"]
		return leftOK && rightOK && leftObject.Kind == ValueString && rightObject.Kind == ValueString && strings.EqualFold(leftObject.Text, rightObject.Text)
	case strings.EqualFold(left.Type, "Schema.SObjectField"):
		leftObject, leftObjectOK := left.Fields["object"]
		rightObject, rightObjectOK := right.Fields["object"]
		leftField, leftFieldOK := left.Fields["field"]
		rightField, rightFieldOK := right.Fields["field"]
		return leftObjectOK && rightObjectOK && leftFieldOK && rightFieldOK &&
			leftObject.Kind == ValueString && rightObject.Kind == ValueString &&
			leftField.Kind == ValueString && rightField.Kind == ValueString &&
			strings.EqualFold(leftObject.Text, rightObject.Text) &&
			strings.EqualFold(leftField.Text, rightField.Text)
	default:
		return false
	}
}

func queryResultRecordsList(value Value) (Value, bool) {
	if value.Kind != ValueMap {
		return Null, false
	}
	records, ok := value.Map[mapKey(String("records"))]
	if !ok {
		return Null, false
	}
	if records.Kind != ValueList {
		return Null, false
	}
	return records, true
}

func platformScalarObjectText(value Value) (string, bool) {
	if value.Kind != ValueObject {
		return "", false
	}
	raw, ok := value.Fields["value"]
	if !ok || raw.Kind != ValueString {
		return "", false
	}
	return raw.Text, true
}

func typeValueIdentityName(value Value) string {
	if identity, ok := value.Fields[reflectionTypeIdentityField]; ok && identity.Kind == ValueString {
		return identity.Text
	}
	return typeValueText(value)
}

func typeValueText(value Value) string {
	if value.Text != "" {
		return value.Text
	}
	if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
		return raw.Text
	}
	return ""
}

// Type identity ignores spelling case; token display retains its original name.
func canonicalTypeValueIdentity(text string) string {
	return strings.ToLower(canonicalTypeValueText(text))
}

func canonicalTypeValueText(text string) string {
	normalized := strings.TrimPrefix(text, "System.")
	switch strings.ToLower(normalized) {
	case "blob":
		return "Blob"
	case "boolean":
		return "Boolean"
	case "date":
		return "Date"
	case "datetime":
		return "Datetime"
	case "decimal":
		return "Decimal"
	case "double":
		return "Double"
	case "id":
		return "Id"
	case "integer":
		return "Integer"
	case "long":
		return "Long"
	case "object":
		return "Object"
	case "string":
		return "String"
	case "time":
		return "Time"
	case "type":
		return "Type"
	case "url":
		return "URL"
	case "void":
		return "Void"
	default:
		return text
	}
}

func platformScalarObject(typeName string) bool {
	switch typeName {
	case "Blob", "Date", "Datetime", "Id", "Time", "Type", "URL", "UUID":
		return true
	default:
		return false
	}
}

func mapKey(v Value) string {
	if v.Kind == ValueNull && v.sobjectListNullMapKey {
		return "null:sobject-list-id"
	}
	if v.Kind == ValueObject && strings.EqualFold(v.Type, "Schema.SObjectType") {
		if objectName, ok := v.Fields["object"]; ok && objectName.Kind == ValueString {
			return string(v.Kind) + ":" + v.Type + ":" + schemaTokenObjectKey(objectName.Text)
		}
	}
	if v.Kind == ValueObject && strings.EqualFold(v.Type, "Schema.SObjectField") {
		objectName, hasObject := v.Fields["object"]
		fieldName, hasField := v.Fields["field"]
		if hasObject && hasField && objectName.Kind == ValueString && fieldName.Kind == ValueString {
			return string(v.Kind) + ":" + v.Type + ":" + schemaTokenObjectKey(objectName.Text) + "." + schemaTokenFieldKey(fieldName.Text)
		}
	}
	if v.Kind == ValueObject && strings.EqualFold(v.Type, "Schema.ChildRelationship") {
		relationshipName, hasRelationship := v.Fields["relationshipName"]
		childSObject, hasChild := v.Fields["childSObject"]
		field, hasField := v.Fields["field"]
		if hasRelationship && hasChild && hasField && relationshipName.Kind == ValueString && childSObject.Kind == ValueObject && strings.EqualFold(childSObject.Type, "Schema.SObjectType") && field.Kind == ValueObject && strings.EqualFold(field.Type, "Schema.SObjectField") {
			childName, childOK := childSObject.Fields["object"]
			fieldName, fieldOK := field.Fields["field"]
			if childOK && fieldOK && childName.Kind == ValueString && fieldName.Kind == ValueString {
				return string(v.Kind) + ":" + v.Type + ":" + relationshipName.Text + "|" + childName.Text + "|" + fieldName.Text
			}
		}
	}
	if v.Kind == ValueObject && strings.EqualFold(v.Type, "Type") {
		if name := typeValueIdentityName(v); name != "" {
			return string(v.Kind) + ":Type:" + canonicalTypeValueIdentity(name)
		}
	}
	if v.Kind == ValueObject && platformScalarObject(v.Type) {
		if raw, ok := v.Fields["value"]; ok && raw.Kind == ValueString {
			if strings.EqualFold(v.Type, "Id") {
				return string(v.Kind) + ":" + v.Type + ":" + canonicalIDMapKey(raw.Text)
			}
			return string(v.Kind) + ":" + v.Type + ":" + raw.Text
		}
	}
	if v.Kind == ValueObject {
		if isStubProxy(v) {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
		if sObjectValueType(v.Type) && !v.classInstance {
			return sObjectProjectedMapKey(v, make(map[uint64]bool))
		}
		if key, ok := stableObjectFieldMapKey(v, make(map[uint64]bool)); ok {
			return key
		}
		if v.Type != "" && !sObjectValueType(v.Type) && v.Ref != 0 {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
	}
	if v.Kind == ValueObject && v.Type != "" {
		if key, ok := objectIDFieldMapKey(v); ok {
			return key
		}
	}
	if v.Kind == ValueString && strings.EqualFold(v.Type, "Id") && looksLikeID(v.Text) {
		return string(ValueObject) + ":Id:" + canonicalIDMapKey(v.Text)
	}
	if v.Kind == ValueObject && v.Type != "" && v.Text != "" {
		return string(v.Kind) + ":" + v.Type + ":" + v.Text
	}
	if v.Kind == ValueInt && strings.EqualFold(v.Type, "Long") {
		return "long:" + v.String()
	}
	if isFloatBackedDecimal(v) {
		return "double:" + v.String()
	}
	switch v.Kind {
	case ValueList:
		// Display text is ambiguous: ['a, b'] and ['a', 'b'] print alike (native listKeys).
		return string(v.Kind) + ":" + collectionElementKeys(v.List, false)
	case ValueSet:
		// Set equality ignores insertion order (native setKeys).
		return string(v.Kind) + ":" + collectionElementKeys(v.Set, true)
	}
	return string(v.Kind) + ":" + v.String()
}

// collectionElementKeys joins length-prefixed element keys, sorted when order does not matter.
func collectionElementKeys(items []Value, sorted bool) string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, mapKey(item))
	}
	if sorted {
		sort.Strings(keys)
	}
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(strconv.Itoa(len(key)))
		b.WriteByte(':')
		b.WriteString(key)
	}
	return b.String()
}

func objectIDFieldMapKey(v Value) (string, bool) {
	if v.Kind != ValueObject || v.Type == "" {
		return "", false
	}
	id, ok := v.Fields["Id"]
	if !ok {
		return "", false
	}
	text, ok := idValueText(id)
	if !ok || text == "" {
		return "", false
	}
	return string(v.Kind) + ":" + v.Type + ":" + canonicalIDMapKey(text), true
}

func stableObjectFieldMapKey(v Value, seen map[uint64]bool) (string, bool) {
	return stableFieldMapKey(v, seen, false)
}

func stableSObjectFieldMapKey(v Value, seen map[uint64]bool) (string, bool) {
	return stableFieldMapKey(v, seen, true)
}

func stableFieldMapKey(v Value, seen map[uint64]bool, allowSObject bool) (string, bool) {
	if v.Type == "" || len(v.Fields) == 0 || (!allowSObject && sObjectValueType(v.Type)) {
		return "", false
	}
	if v.Ref != 0 {
		if seen[v.Ref] {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10), true
		}
		seen[v.Ref] = true
		defer delete(seen, v.Ref)
	}
	names := make([]string, 0, len(v.Fields))
	for name := range v.Fields {
		if strings.HasPrefix(name, "__glade") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "", false
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names)+2)
	parts = append(parts, string(v.Kind), v.Type)
	for _, name := range names {
		if !stableObjectMapKeyField(v.Fields[name]) {
			continue
		}
		parts = append(parts, name+"="+mapKeyWithSeen(v.Fields[name], seen))
	}
	if len(parts) == 2 {
		return "", false
	}
	return strings.Join(parts, ":"), true
}

func stableObjectMapKeyField(value Value) bool {
	switch value.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString, ValueList, ValueSet, ValueMap:
		return true
	case ValueObject:
		return strings.EqualFold(value.Type, "Type") ||
			hasTypePrefixFold(value.Type, "Schema") ||
			platformScalarObject(value.Type) ||
			sObjectValueType(value.Type)
	default:
		return false
	}
}

func mapKeyWithSeen(v Value, seen map[uint64]bool) string {
	if v.Kind == ValueObject {
		if isStubProxy(v) {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
		if sObjectValueType(v.Type) && !v.classInstance {
			return sObjectProjectedMapKey(v, seen)
		}
		if key, ok := stableObjectFieldMapKey(v, seen); ok {
			return key
		}
		if v.Type != "" && !sObjectValueType(v.Type) && v.Ref != 0 {
			return string(v.Kind) + ":" + v.Type + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
	}
	return mapKey(v)
}

func schemaTokenObjectKey(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 && dot < len(name)-1 {
		name = name[dot+1:]
	}
	return strings.ToLower(stripSchemaNamespaceToken(name))
}

func schemaTokenFieldKey(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 && dot < len(name)-1 {
		name = name[dot+1:]
	}
	return strings.ToLower(stripSchemaNamespaceToken(name))
}

func stripSchemaNamespaceToken(name string) string {
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	rest := name[first+2:]
	if strings.Contains(rest, "__") {
		return rest
	}
	return name
}

func sObjectTypeTokenEqual(left, right Value) bool {
	leftObject, leftOK := left.Fields["object"]
	rightObject, rightOK := right.Fields["object"]
	if !leftOK || !rightOK || leftObject.Kind != ValueString || rightObject.Kind != ValueString {
		return false
	}
	return strings.EqualFold(schemaTokenObjectKey(leftObject.Text), schemaTokenObjectKey(rightObject.Text))
}

func describeSObjectResultIdentityEqual(left, right Value) bool {
	leftType, leftOK := left.Fields["sObjectType"]
	rightType, rightOK := right.Fields["sObjectType"]
	return leftOK && rightOK && schemaTokenIdentityEqual(leftType, rightType)
}

func describeFieldResultIdentityEqual(left, right Value) bool {
	leftField, leftOK := left.Fields["sObjectField"]
	rightField, rightOK := right.Fields["sObjectField"]
	if leftOK && rightOK && schemaTokenIdentityEqual(leftField, rightField) {
		return true
	}
	leftObject, leftObjectOK := left.Fields["sObjectName"]
	rightObject, rightObjectOK := right.Fields["sObjectName"]
	leftName, leftNameOK := left.Fields["name"]
	rightName, rightNameOK := right.Fields["name"]
	return leftObjectOK && rightObjectOK && leftNameOK && rightNameOK &&
		leftObject.Kind == ValueString && rightObject.Kind == ValueString &&
		leftName.Kind == ValueString && rightName.Kind == ValueString &&
		strings.EqualFold(leftObject.Text, rightObject.Text) &&
		strings.EqualFold(leftName.Text, rightName.Text)
}

func containsValue(values []Value, needle Value) bool {
	for _, value := range values {
		if value.Equal(needle) {
			return true
		}
	}
	return false
}

func writeValuesText(w *textWriter, values []Value) {
	w.WriteString("[")
	for i, value := range values {
		if w.full {
			return
		}
		if i > 0 {
			w.WriteString(", ")
		}
		value.writeText(w)
	}
	w.WriteString("]")
}

func apexCollectionString(value Value) string {
	return apexCollectionStringLimit(value, 0)
}

// apexMapValueCollectionString renders a List or Set held as a Map value in
// Map text: native shows its first ten elements, then ", ..."
// (record_type_shape list_forms mapValue rows).
func apexMapValueCollectionString(value Value) string {
	return apexCollectionStringLimit(value, collectionDisplayLimit)
}

// apexCollectionStringLimit renders a collection; a positive limit keeps the
// first limit List or Set elements and appends "...".
func apexCollectionStringLimit(value Value, limit int) string {
	var w textWriter
	writeApexCollectionText(&w, value, limit)
	return w.String()
}

func writeApexCollectionText(w *textWriter, value Value, limit int) {
	if w.full {
		return
	}
	value = value.currentMapKeySet()
	switch value.Kind {
	case ValueList, ValueSet:
		items := value.List
		opening, closing := "(", ")"
		var idSet bool
		if value.Kind == ValueSet {
			items = value.Set
			opening, closing = "{", "}"
			elementType, typed := collectionElementType(value.Type)
			idSet = typed && strings.EqualFold(canonicalApexScalarType(elementType), "Id")
		}
		truncated := limit > 0 && len(items) > limit
		if truncated {
			items = items[:limit]
		}
		w.WriteString(opening)
		for i, item := range items {
			if w.full {
				return
			}
			if i > 0 {
				w.WriteString(", ")
			}
			// Preserve stored Id payloads; the captured scalar String view is
			// expanded only in this typed Set projection. Nested text rows are
			// still pending, so ordinary String/Object sets retain their path.
			if idSet && item.Kind == ValueString && strings.EqualFold(item.Type, "Id") {
				w.WriteString(displayIDText(item.Text))
				continue
			}
			writeCollectionElementText(w, item)
		}
		if truncated {
			if len(items) > 0 {
				w.WriteString(", ")
			}
			w.WriteString("...")
		}
		w.WriteString(closing)
	case ValueMap:
		writeMapText(w, value)
	default:
		value.writeText(w)
	}
}

// Match the scalar Id rendering used by VM List/Set display when a collection
// is nested in Map text, without treating ordinary Id-shaped Strings as Ids.
func writeCollectionElementText(w *textWriter, value Value) {
	if text, ok := typedIDValueText(value); ok {
		w.WriteString(displayIDText(text))
		return
	}
	value.writeText(w)
}

func mapString(value Value) string {
	var w textWriter
	writeMapText(&w, value)
	return w.String()
}

func writeMapText(w *textWriter, value Value) {
	values := value.Map
	if len(values) == 0 {
		w.WriteString("{}")
		return
	}
	// Map text has bare braces, with the
	// ordinary Apex representation for nested collection values.
	w.WriteString("{")
	for i, key := range sortedMapKeys(values) {
		if w.full {
			return
		}
		if i > 0 {
			w.WriteString(", ")
		}
		mapStoredKey(value, key).writeText(w)
		w.WriteString("=")
		writeApexCollectionText(w, values[key], 0)
	}
	w.WriteString("}")
}

func sortedMapKeys(values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func orderedValueMapKeys(value Value) []string {
	if len(value.MapOrder) == 0 {
		return sortedMapKeys(value.Map)
	}
	keys := make([]string, 0, len(value.Map))
	seen := make(map[string]bool, len(value.Map))
	for _, key := range value.MapOrder {
		if _, ok := value.Map[key]; !ok || seen[key] {
			continue
		}
		keys = append(keys, key)
		seen[key] = true
	}
	for _, key := range sortedMapKeys(value.Map) {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	return keys
}

// sObjectProjectedMapKey hashes an SObject over the field projection that
// SObject == compares (sObjectEqualityFieldIncluded), with case-folded field
// names and Ids normalized like apexIDTextEqual, so equal records hash equally.
// Native stdlib_unicode_small_c14 sobjectKeys: equal Ids with different field
// values are distinct keys.
func sObjectProjectedMapKey(v Value, seen map[uint64]bool) string {
	if v.Ref != 0 {
		if seen[v.Ref] {
			return string(v.Kind) + ":" + strings.ToLower(v.Type) + ":ref:" + strconv.FormatUint(v.Ref, 10)
		}
		seen[v.Ref] = true
		defer delete(seen, v.Ref)
	}
	parts := make(map[string]string, len(v.Fields))
	names := make([]string, 0, len(v.Fields))
	for name, raw := range v.Fields {
		if !sObjectEqualityFieldIncluded(v, name, raw) {
			continue
		}
		value := raw
		if _, resolved, ok := objectFieldValue(v, name); ok {
			value = resolved
		}
		folded := strings.ToLower(name)
		var text string
		if id, ok := sObjectIDFromValue(value); ok {
			text = "id:" + canonicalIDMapKey(string(id))
		} else {
			text = mapKeyWithSeen(value, seen)
		}
		if _, dup := parts[folded]; !dup {
			names = append(names, folded)
		}
		parts[folded] = text
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString(string(v.Kind))
	b.WriteByte(':')
	b.WriteString(strings.ToLower(v.Type))
	for _, name := range names {
		b.WriteByte(':')
		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(parts[name])
	}
	return b.String()
}
