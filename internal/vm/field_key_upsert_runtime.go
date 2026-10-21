package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// FieldKey 066/113: a queried clone without an Id retains its readable
// auto-number value, but that implicitly loaded readonly value is not an
// upsert key. Restrict the DML projection to the captured single-record shape;
// do not change clone contents or explicitly assigned/generated-field inputs.
func (vm *VM) fieldKeyProjectAutoNumberUpsertKey(records []storage.Record, targets []*Value, fieldName string) {
	if vm == nil || vm.Org == nil || len(records) != 1 || len(targets) != 1 || records[0].ID != "" || targets[0] == nil || targets[0].Kind != ValueObject {
		return
	}
	target := *targets[0]
	clone := target.Fields[sobjectCloneMarkerField]
	sourceID := target.Fields[sobjectCloneSourceIDField]
	selected := target.Fields[sobjectQueriedFieldsField]
	if clone.Kind != ValueBool || !clone.Bool || sourceID.Kind == ValueNull || selected.Kind != ValueMap {
		return
	}
	_, definition, ok := vm.describeObjectDefinition(records[0].Object)
	if !ok {
		return
	}
	field, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		return
	}
	metadata := definition.Fields[field]
	queried := selected.Map[mapKey(String(strings.ToLower(field)))]
	if !metadata.ExternalID || !metadata.AutoNumber || metadata.Type != storage.FieldString ||
		!strings.EqualFold(metadata.DisplayType, "STRING") || queried.Kind != ValueBool || !queried.Bool ||
		isExplicitSObjectField(target, field) || isUserSetSObjectFieldAlias(target, field) {
		return
	}
	deleteRecordFieldAliases(&records[0], field)
}

// FieldKey 066/082-085/113 retain the selector's canonical name and an empty
// field list for the missing-key result. This is separate from missing row Id.
func fieldKeyMissingUpsertArgument(field string) dml.Result {
	return dmlFailure("", field+" not specified", "MISSING_ARGUMENT", nil)
}

// Only the captured missing-selector upsert error opts into the statement's
// composed exception format. Other unobserved upsert failures stay unchanged.
func fieldKeyMissingUpsertResult(results []dml.Result) bool {
	for _, result := range results {
		if result.Success || result.Error == "" {
			continue
		}
		return result.StatusCode == "MISSING_ARGUMENT" && len(result.Fields) == 0 &&
			strings.HasSuffix(result.Error, " not specified")
	}
	return false
}

// FieldKey 123/125/127 capture a two-input partial upsert of ordinary Text
// external keys. Keep other cardinalities, key types, seeded IDs and existing
// error precedence on their previous paths until their boundaries are captured.
func (vm *VM) fieldKeyRejectDuplicateInputs(records []storage.Record, fieldName string, allOrNone bool, failures []dml.Result) {
	if allOrNone || len(records) != 2 || len(failures) != 2 || vm.Org == nil {
		return
	}
	var objectName, canonicalField, firstKey string
	for i, record := range records {
		if record.ID != "" || !failures[i].Success {
			return
		}
		owner, definition, ok := vm.describeObjectDefinition(record.Object)
		if !ok {
			return
		}
		field, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
		if !ok {
			return
		}
		metadata := definition.Fields[field]
		if !metadata.ExternalID || metadata.Type != storage.FieldString || !strings.EqualFold(metadata.DisplayType, "STRING") || metadata.Formula != "" || metadata.AutoNumber {
			return
		}
		key, present := record.GetField(field)
		if !present || key.Kind != storage.ValueString || key.String == "" || !fieldKeyASCII(key.String) {
			return
		}
		text := key.String
		if !metadata.CaseSensitive {
			text = strings.ToLower(text)
		}
		if i == 0 {
			objectName, canonicalField, firstKey = owner, field, text
		} else if owner != objectName || field != canonicalField || text != firstKey {
			return
		}
	}
	for i := range failures {
		failures[i] = dmlFailure("", "Duplicate external id specified: "+firstKey, "DUPLICATE_VALUE", []string{canonicalField})
	}
}

func fieldKeyASCII(value string) bool {
	for _, character := range value {
		if character > 127 {
			return false
		}
	}
	return true
}

// FieldKey 130/131 capture the three-argument partial overload, one typed
// record, and a foreign Id or null token. Other overloads and uncaptured source
// versions retain their existing handling rather than extrapolating the drift.
func (vm *VM) fieldKeyUpsertTokenPreflight(args []Value) (Value, bool, error) {
	if vm == nil || vm.Org == nil || len(args) != 3 || args[2].Kind != ValueBool || args[2].Bool || args[0].Kind != ValueList || len(args[0].List) != 1 {
		return Null, false, nil
	}
	api, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || (api != 62 && api != 67 && api != 68) {
		return Null, false, nil
	}
	elementType, ok := collectionElementType(args[0].Static)
	if !ok {
		elementType, ok = collectionElementType(args[0].Type)
	}
	if !ok || strings.EqualFold(elementType, "SObject") || strings.EqualFold(elementType, "System.SObject") {
		return Null, false, nil
	}
	owner, ok := vm.describeObjectTokenName(elementType)
	if !ok || args[0].List[0].Kind != ValueObject {
		return Null, false, nil
	}
	actual, ok := vm.describeObjectTokenName(args[0].List[0].Type)
	if !ok || !strings.EqualFold(owner, actual) {
		return Null, false, nil
	}
	token := args[1]
	if token.Kind == ValueNull {
		if !isSObjectFieldTokenType(token.Type) && !isSObjectFieldTokenType(token.Static) {
			return Null, false, nil
		}
		return Null, true, newExceptionError("System.NullPointerException", "Argument 2 cannot be null")
	}
	if token.Kind != ValueObject || !isSObjectFieldTokenType(token.Type) {
		return Null, false, nil
	}
	field := token.Fields["field"]
	source := token.Fields["object"]
	if field.Kind != ValueString || source.Kind != ValueString {
		return Null, false, nil
	}
	foreign, definition, ok := vm.describeObjectDefinition(source.Text)
	if !ok || strings.EqualFold(foreign, owner) {
		return Null, false, nil
	}
	canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field.Text)
	if !ok || definition.Fields[canonical].Type != storage.FieldID {
		return Null, false, nil
	}
	message := fmt.Sprintf("Expected field from SObject type %s but found %s from %s", owner, canonical, foreign)
	if vm.testContext != nil && (api == 67 || api == 68) {
		failure := dmlFailure("", message, "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil)
		return fieldKeyDMLResultsValue("upsert", args[0], []dml.Result{failure}), true, nil
	}
	return Null, true, newExceptionError("System.TypeException", message)
}

// Share the existing Database result projection with the token preflight.
func fieldKeyDMLResultsValue(op string, input Value, results []dml.Result) Value {
	values := make([]Value, 0, len(results))
	for _, result := range results {
		resultType := databaseDMLResultType(op)
		row := Object(resultType)
		row.Fields["success"] = Bool(result.Success)
		row.Fields["id"] = databaseDMLResultIDValue(op, result)
		row.Fields["error"] = String(result.Error)
		if op == "upsert" {
			row.Fields["created"] = Bool(result.UpsertCreated)
		}
		row.Fields["errors"] = databaseErrorsList(result, resultType)
		values = append(values, row)
	}
	if input.Kind == ValueList {
		return List(values...)
	}
	if len(values) == 0 {
		return Null
	}
	return values[0]
}
