package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

func writeLDSRecordResponse(w http.ResponseWriter, data any, wireErr *lwcbrowser.WireError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var responseError any
	if wireErr != nil {
		status := wireErr.Status
		if status == 0 {
			status = http.StatusBadRequest
		}
		body := wireErr.RecordBody
		if body == nil {
			body = map[string]any{
				"errorCode":  wireErr.Type,
				"message":    wireErr.Message,
				"statusCode": status,
			}
		}
		if wireErr.RecordErrorID != "" {
			body["id"] = wireErr.RecordErrorID
		}
		statusText := http.StatusText(status)
		if status == http.StatusInternalServerError {
			statusText = "Server Error"
		}
		responseError = map[string]any{
			"body":       body,
			"errorType":  "fetchResponse",
			"headers":    map[string]any{},
			"ok":         false,
			"status":     status,
			"statusText": statusText,
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "error": responseError})
}

func recordDMLWireError(definition storage.ObjectDefinition, result dml.Result, operation string, inputs ...map[string]any) *lwcbrowser.WireError {
	errors := result.Errors
	if len(errors) == 0 {
		errors = []dml.Error{{StatusCode: result.StatusCode, Message: result.Error, Fields: result.Fields}}
	}
	general := []map[string]any{}
	fieldErrors := map[string][]map[string]any{}
	for _, err := range errors {
		if len(err.Fields) == 0 {
			general = append(general, recordErrorDetail(err.StatusCode, err.Message, "", ""))
			continue
		}
		for _, name := range err.Fields {
			field := definition.Fields[name]
			message := err.Message
			if err.StatusCode == "INVALID_EMAIL_ADDRESS" && len(inputs) != 0 {
				if value, ok := inputs[0][name].(string); ok {
					message = fmt.Sprintf("%s: invalid email address: %s", name, value)
				}
			}
			fieldErrors[name] = append(fieldErrors[name], recordErrorDetail(err.StatusCode, message, name, labelOrFallback(field.Label, name)))
		}
	}
	return &lwcbrowser.WireError{
		Type:          result.StatusCode,
		Message:       result.Error,
		Status:        http.StatusBadRequest,
		RecordErrorID: recordMutationErrorID(operation, "record"),
		RecordBody: map[string]any{
			"enhancedErrorType": "RecordError",
			"message":           "An error occurred while trying to update the record. Please try again.",
			"output":            map[string]any{"errors": general, "fieldErrors": fieldErrors},
			"statusCode":        http.StatusBadRequest,
		},
	}
}

func recordErrorDetail(code, message, field, label string) map[string]any {
	detail := map[string]any{
		"constituentField":     nil,
		"duplicateRecordError": nil,
		"errorCode":            code,
		"field":                nil,
		"fieldLabel":           nil,
		"message":              message,
	}
	if field != "" {
		if code != "INVALID_EMAIL_ADDRESS" {
			detail["constituentField"] = field
		}
		detail["field"] = field
		detail["fieldLabel"] = label
	}
	return detail
}

func recordCollisionWireError(org *storage.OrgState, record storage.Record, since string) *lwcbrowser.WireError {
	if since == "" {
		return nil
	}
	requested, err := time.Parse(time.RFC3339Nano, since)
	if err != nil {
		return nil
	}
	modified, err := time.Parse(time.RFC3339Nano, recordLastModifiedDate(record))
	if err != nil || !requested.Before(modified) {
		return nil
	}
	userName := ""
	if userObject, ok := org.Objects["User"]; ok {
		if _, user, ok := storage.LookupRecordByID(userObject.Records, storage.ID(recordLastModifiedByID(record))); ok {
			if name, ok := user.GetField("Name"); ok {
				userName = strings.TrimSpace(fmt.Sprint(storageValueJSON(name)))
			}
		}
	}
	message := fmt.Sprintf("This record was modified by %s during your edit session. Make a note of the data you entered, then reload the record and enter your updates again.", userName)
	return &lwcbrowser.WireError{
		Type:          "CollisionDetectedException",
		Message:       message,
		Status:        http.StatusBadRequest,
		RecordErrorID: recordMutationErrorID("update", "record"),
		RecordBody: map[string]any{
			"enhancedErrorType": "RecordError",
			"output": map[string]any{
				"errors":      []map[string]any{recordErrorDetail("CollisionDetectedException", message, "", "")},
				"fieldErrors": map[string]any{},
			},
			"statusCode": http.StatusBadRequest,
		},
	}
}

// Native API 59/67 controls vary invalid values and field
// names, and required/length/collision errors. IDs are stable per LDS operation
// and error category, independent of record ID and diagnostic message.
func recordMutationErrorID(operation, category string) string {
	ids := map[string]map[string]string{
		"create": {"record": "1387527938", "number": "38510245", "field": "-437357844"},
		"update": {"record": "418879747", "number": "-1447809457", "field": "-1017175928"},
		"delete": {"record": "1161085053"},
	}
	return ids[operation][category]
}

func recordParseWireError(operation, category, message string) *lwcbrowser.WireError {
	return &lwcbrowser.WireError{Type: "POST_BODY_PARSE_ERROR", Status: http.StatusBadRequest,
		Message: message, RecordErrorID: recordMutationErrorID(operation, category)}
}

// The captured batch validation query retains the audit/identity fields around
// the invalid selection, including when a valid Name selection accompanies it.
// Single-record layout projections follow a different, org-dependent query.
func recordBatchFieldDiagnostic(objectName, fieldName string) string {
	return "INVALID_FIELD: \nSELECT LastModifiedDate, " + fieldName + ", Id, LastModifiedById\n" +
		"                         ^\nERROR at Row:1:Column:26\n" +
		fmt.Sprintf("No such column '%s' on entity '%s'. If you are attempting to use a custom field, be sure to append the '__c' after the custom field name. Please reference your WSDL or the describe call for the appropriate names.", fieldName, objectName)
}

// getLDSRecordWireData handles record reads at the LDS transport boundary.
// Layout selection replaces required fields, while optional fields are unioned
// into the selected layout (native review controls at API 59 and 67).
func getLDSRecordWireData(org *storage.OrgState, req lwcbrowser.WireGetRecordRequest, source SourceMetadata) (map[string]any, *lwcbrowser.WireError) {
	fields := req.Fields
	if len(req.LayoutTypes) > 0 {
		for _, value := range req.LayoutTypes {
			if value != "Compact" && value != "Full" {
				normalized := strings.ToLower(value)
				if normalized != "" {
					normalized = strings.ToUpper(normalized[:1]) + normalized[1:]
				}
				return nil, recordSelectionEnumError(normalized, "LayoutType")
			}
		}
		for _, value := range req.Modes {
			if value != "View" && value != "Edit" && value != "Create" {
				return nil, recordSelectionEnumError(value, "Mode")
			}
		}
		if objectName, _, ok := findOrgRecord(org, req.RecordID); ok {
			fields = recordLayoutSelection(org, objectName, req.LayoutTypes, req.Modes, source)
		}
	}
	data, wireErr := getRecordWireData(org, req.RecordID, fields, req.OptionalFields)
	if wireErr != nil && wireErr.Type == "INVALID_FIELD" {
		if objectName, _, ok := findOrgRecord(org, req.RecordID); ok {
			name := strings.TrimPrefix(wireErr.Message, "field not found: ")
			wireErr.Message = recordRetainedFieldDiagnostic(org.Objects[objectName].Definition, objectName, name, req.RetainedFields)
		}
	}
	return data, wireErr
}

func recordSelectionEnumError(value, kind string) *lwcbrowser.WireError {
	message := fmt.Sprintf("Value: %s could not be parsed into a valid enum value of type: %s", value, kind)
	return &lwcbrowser.WireError{Status: http.StatusInternalServerError, Message: message, RecordBody: map[string]any{"error": message}}
}

func recordLayoutSelection(org *storage.OrgState, objectName string, types, modes []string, source SourceMetadata) []string {
	def := org.Objects[objectName].Definition
	if len(modes) == 0 {
		modes = []string{"View"}
	}
	names := []string{}
	add := func(name string, mode string) {
		canonical, ok := storage.ResolveFieldName(def, org.Namespace, name)
		if !ok {
			return
		}
		field := def.Fields[canonical]
		if mode == "Create" && !fieldCreateable(field) {
			return
		}
		names = appendUniqueFieldNames(names, objectName+"."+canonical)
		if field.RelationshipName != "" && field.Type == storage.FieldReference {
			// The captured Account/User projection exposes Name. Do not
			// synthesize that selector for targets whose schema lacks it.
			hasName := len(field.ReferenceTo) > 0
			for _, target := range field.ReferenceTo {
				_, object, found := findOrgObject(org, target)
				if !found {
					hasName = false
					break
				}
				if _, found := storage.ResolveFieldName(object.Definition, org.Namespace, "Name"); !found {
					hasName = false
					break
				}
			}
			if hasName {
				names = appendUniqueFieldNames(names, objectName+"."+field.RelationshipName+".Name")
			}
			names = appendUniqueFieldNames(names, objectName+"."+field.RelationshipName+".Id")
		}
	}
	for _, typ := range types {
		for _, mode := range modes {
			if typ == "Compact" {
				if layouts := source.Compact[objectName]; len(layouts) > 0 {
					for _, name := range layouts[0].Fields {
						add(name, "View")
					}
				}
				continue
			}
			if layout, ok := sourceCreateLayout(source, objectName); ok {
				for _, section := range layout.Sections {
					for _, column := range section.Columns {
						for _, item := range column.Items {
							add(item.Field, mode)
						}
					}
				}
			}
		}
	}
	return names
}

// Native invalid-field controls retain the validation selection's hash-bucket
// order. Sorting names would move Id ahead of the failed selector and alter
// both the query text and its column. The field/error strings remain generic.
func recordValidationFieldDiagnostic(objectName, fieldName string) string {
	names := []string{"LastModifiedDate", "Id", "LastModifiedById", fieldName}
	bucket := func(value string) uint32 {
		var hash uint32
		for _, ch := range value {
			hash = 31*hash + uint32(ch)
		}
		return (hash ^ (hash >> 16)) & 15
	}
	sort.SliceStable(names, func(i, j int) bool { return bucket(names[i]) < bucket(names[j]) })
	column := 8
	for _, name := range names {
		if name == fieldName {
			break
		}
		column += len(name) + 2
	}
	return "INVALID_FIELD: \nSELECT " + strings.Join(names, ", ") + "\n" + strings.Repeat(" ", column-1) + "^\n" +
		fmt.Sprintf("ERROR at Row:1:Column:%d\nNo such column '%s' on entity '%s'. If you are attempting to use a custom field, be sure to append the '__c' after the custom field name. Please reference your WSDL or the describe call for the appropriate names.", column, fieldName, objectName)
}

// The native query-seed and query-trace controls distinguish cold validation
// from validation after createRecord has retained its layout projection. Build
// that projection from the fields actually held by LDS, including picklist
// labels, formatted values and reference audit fields. Only the validation
// diagnostic uses this selection; it does not expand a successful field read.
func recordRetainedFieldDiagnostic(def storage.ObjectDefinition, objectName, fieldName string, retained []string) string {
	if len(retained) == 0 {
		return recordValidationFieldDiagnostic(objectName, fieldName)
	}
	names := []string{"LastModifiedDate", "Id", "LastModifiedById", "SystemModstamp"}
	for _, name := range retained {
		if _, ok := def.Fields[name]; !ok {
			for key, field := range def.Fields {
				if field.Type == storage.FieldReference && field.RelationshipName == name {
					name = key
					break
				}
			}
		}
		if _, ok := def.Fields[name]; ok {
			names = appendUniqueFieldNames(names, name)
		}
	}
	termsFor := func(name string) []string {
		terms := []string{name}
		field := def.Fields[name]
		if field.Type == storage.FieldPicklist {
			terms = append(terms, "toLabel("+name+") "+name+"__l")
		}
		if name != "SystemModstamp" && (field.Type == storage.FieldDateTime || strings.EqualFold(field.DisplayType, "CURRENCY")) {
			terms = append(terms, "format("+name+") "+name+"__f")
		}
		return terms
	}
	referenceTerms := func(name string) []string {
		field := def.Fields[name]
		if field.Type != storage.FieldReference || field.RelationshipName == "" {
			return nil
		}
		terms := []string{}
		for _, suffix := range []string{"Id", "Name", "LastModifiedDate", "LastModifiedById", "SystemModstamp"} {
			terms = append(terms, field.RelationshipName+"."+suffix)
		}
		return terms
	}
	termCount := 1 // Failed selector.
	for _, name := range names {
		termCount += len(termsFor(name)) + len(referenceTerms(name))
	}
	capacity := uint32(16)
	for int(capacity)*3 < termCount*4 {
		capacity *= 2
	}
	bucket := func(value string) uint32 {
		var hash uint32
		for _, ch := range value {
			hash = 31*hash + uint32(ch)
		}
		return (hash ^ (hash >> 16)) & (capacity - 1)
	}
	// Scalar slots precede expanded references; the captured colliding scalar
	// selectors retain descending name order within their hash bucket.
	sort.Slice(names, func(i, j int) bool {
		if bucket(names[i]) == bucket(names[j]) {
			return names[i] > names[j]
		}
		return bucket(names[i]) < bucket(names[j])
	})
	terms := []string{"LastModifiedDate", "Id", "LastModifiedById", "SystemModstamp"}
	for _, name := range names {
		terms = appendUniqueFieldNames(terms, termsFor(name)...)
	}
	for _, name := range names {
		terms = appendUniqueFieldNames(terms, referenceTerms(name)...)
	}
	terms = appendUniqueFieldNames(terms, fieldName)
	sort.SliceStable(terms, func(i, j int) bool { return bucket(terms[i]) < bucket(terms[j]) })
	column := 8
	for _, term := range terms {
		if term == fieldName {
			break
		}
		column += len(term) + 2
	}
	query := "SELECT " + strings.Join(terms, ", ") + " FROM " + objectName
	position := column - 1
	// Include the boundary character before extending to the token's end;
	// ctrl_querySeed_freshField places that character on a separator space.
	start, end := max(0, position-30), min(len(query), position+31)
	for start > 0 && query[start-1] != ' ' {
		start--
	}
	for end < len(query) && query[end] != ' ' {
		end++
	}
	excerpt := strings.TrimSuffix(query[start:end], ",")
	return "INVALID_FIELD: \n" + excerpt + "\n" + strings.Repeat(" ", position-start) + "^\n" +
		fmt.Sprintf("ERROR at Row:1:Column:%d\nNo such column '%s' on entity '%s'. If you are attempting to use a custom field, be sure to append the '__c' after the custom field name. Please reference your WSDL or the describe call for the appropriate names.", column, fieldName, objectName)
}
