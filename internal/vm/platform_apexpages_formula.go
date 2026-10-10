package vm

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// StandardControllerPageError is the opaque page-boundary failure observed for
// a null addFields element at API59/67. The capture does not expose an Apex
// exception type, so this Go error must not become a catchable Apex exception.
type StandardControllerPageError struct{}

func (*StandardControllerPageError) Error() string {
	return "An internal server error has occurred"
}

func (vm *VM) callStandardControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	record, ok := receiver.Fields["record"]
	if !ok || record.Kind != ValueObject {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController has no SObject record")
	}
	receiver = ensureStandardControllerOriginalRecord(receiver, record)
	switch method {
	case "getId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.getId expects 0 arguments")
		}
		return vm.standardControllerID(record), receiver, false, true, nil
	case "getRecord":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.getRecord expects 0 arguments")
		}
		loaded := receiver.Fields["__glade_data_loaded"].Bool
		if pending, ok := receiver.Fields["__glade_add_fields_pending"]; ok && pending.Kind == ValueBool && pending.Bool {
			// API59/67 r_reset_* retain the page projection after reset;
			// a subsequent addFields is accepted without reloading it.
			if !receiver.Fields["__glade_reset_page_projection"].Bool {
				projection, err := vm.standardControllerFieldProjection(record, receiver.Fields["fields"])
				if err != nil {
					return Null, receiver, false, true, err
				}
				original := cloneValue(receiver.Fields["originalRecord"])
				vm.mergeStandardControllerProjection(&original, projection, receiver.Fields["fields"], false)
				vm.mergeStandardControllerProjection(&record, projection, receiver.Fields["fields"], true)
				receiver.Fields["originalRecord"] = original
			}
			receiver.Fields["record"] = record
			receiver.Fields["__glade_add_fields_pending"] = Bool(false)
			receiver.Fields["__glade_data_loaded"] = Bool(true)
			return record, receiver, true, true, nil
		}
		receiver.Fields["__glade_data_loaded"] = Bool(true)
		return record, receiver, !loaded, true, nil
	case "save", "quickSave":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.%s expects 0 arguments", method)
		}
		op := "insert"
		if _, id, ok := objectFieldValue(record, "Id"); ok {
			if idText, ok := idValueText(id); ok && idText != "" {
				op = "update"
			}
		}
		appendStandardControllerActionTrace(result, "start", method, record, map[string]any{"dmlOperation": op})
		results, err := vm.applyDML(op, record, true, "", dml.Options{}, result)
		if err != nil {
			appendStandardControllerErrorTrace(result, method, record, op, err)
			return Null, receiver, false, true, err
		}
		if method == "save" && hasDMLFailures(results) {
			// API59/67 r_action_save_missing_name: save stays on the page
			// and exposes validation failures through ApexPages messages.
			failure := databaseDMLException(op, results, vm.dmlExceptionObjectTypes(record))
			messages, err := vm.apexPagesMessagesFromValue(failure.(*apexThrowError).value, result)
			if err != nil {
				return Null, receiver, false, true, err
			}
			vm.addApexPageMessages(messages)
			appendStandardControllerErrorTrace(result, method, record, op, failure)
			return Null, receiver, false, true, nil
		}
		if len(results) > 0 && results[0].ID != "" {
			record.Fields["Id"] = String(string(results[0].ID))
			receiver.Fields["record"] = record
		}
		receiver.Fields["originalRecord"] = cloneValue(record)
		page := standardControllerPage(record)
		if method == "save" {
			page = vm.standardControllerNavigationPage(record)
		}
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"dmlOperation":  op,
			"pageReference": tracePageReference(page),
		})
		return page, receiver, true, true, nil
	case "delete":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.delete expects 0 arguments")
		}
		page := standardControllerPage(record)
		if navigation, handled, err := vm.standardControllerDeletePage(); err != nil {
			return Null, receiver, false, true, err
		} else if handled {
			page = navigation
		}
		appendStandardControllerActionTrace(result, "start", method, record, map[string]any{"dmlOperation": "delete"})
		if _, err := vm.applyDML("delete", record, true, "", dml.Options{}, result); err != nil {
			appendStandardControllerErrorTrace(result, method, record, "delete", err)
			return Null, receiver, false, true, err
		}
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"dmlOperation":  "delete",
			"pageReference": tracePageReference(page),
		})
		return page, receiver, false, true, nil
	case "view", "edit", "cancel":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.%s expects 0 arguments", method)
		}
		page := vm.standardControllerNavigationPage(record)
		if method == "edit" {
			page = vm.standardControllerEditPage(record)
		}
		appendStandardControllerActionTrace(result, "start", method, record, nil)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"pageReference": tracePageReference(page),
		})
		return page, receiver, false, true, nil
	case "reset":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.reset expects 0 arguments")
		}
		page := standardControllerPage(record)
		pageProjection := false
		if vm.vfStandardControllerReset != nil {
			projection, err := vm.vfStandardControllerReset(record.Type)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if projection.Kind == ValueObject {
				record = projection
				receiver.Fields["originalRecord"] = cloneValue(record)
				receiver.Fields["__glade_caller_provided"] = Bool(false)
				pageProjection = true
			}
		}
		if original, ok := receiver.Fields["originalRecord"]; ok && !pageProjection {
			record = cloneValue(original)
		}
		receiver.Fields["record"] = record
		receiver.Fields["__glade_reset_page_projection"] = Bool(pageProjection)
		receiver.Fields["__glade_add_fields_pending"] = Bool(false)
		receiver.Fields["__glade_data_loaded"] = Bool(false)
		appendStandardControllerActionTrace(result, "start", method, record, nil)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"pageReference": tracePageReference(page),
		})
		return page, receiver, true, true, nil
	case "addFields":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.addFields expects List")
		}
		if callerProvided, ok := receiver.Fields["__glade_caller_provided"]; ok && callerProvided.Kind == ValueBool && callerProvided.Bool {
			return Null, receiver, false, true, newExceptionError("SObjectException", "You cannot call addFields when the data is being passed into the controller by the caller.")
		}
		// API59/67 r_add_after_get_*: acquisition timing wins over null,
		// empty and invalid lists. reset starts a fresh acquisition cycle.
		if receiver.Fields["__glade_data_loaded"].Bool {
			return Null, receiver, false, true, newExceptionError("SObjectException", "You cannot call addFields after you've already loaded the data.  This must be the first thing in your constructor")
		}
		if args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
		}
		if args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.addFields expects List")
		}
		if vm.vfStandardControllerReset != nil {
			for _, field := range args[0].List {
				if field.Kind == ValueNull {
					return Null, receiver, false, true, &StandardControllerPageError{}
				}
			}
		}
		fields, err := apexPagesControllerFieldList(args[0], "ApexPages.StandardController.addFields")
		if err != nil {
			return Null, receiver, false, true, err
		}
		_, definition, known := vm.describeObjectDefinition(record.Type)
		for _, field := range fields.List {
			if known && !vm.standardControllerFieldPathExists(definition, strings.Split(strings.TrimSpace(field.Text), ".")) {
				return Null, receiver, false, true, newExceptionError("SObjectException", field.Text+" does not belong to SObject type "+record.Type)
			}
		}
		if existing, ok := receiver.Fields["fields"]; ok && existing.Kind == ValueList {
			for _, field := range fields.List {
				found := false
				for _, prior := range existing.List {
					if strings.EqualFold(strings.TrimSpace(prior.Text), strings.TrimSpace(field.Text)) {
						found = true
						break
					}
				}
				if !found {
					existing.List = append(existing.List, field)
				}
			}
			fields = existing
		}
		receiver.Fields["fields"] = fields
		receiver.Fields["__glade_add_fields_pending"] = Bool(true)
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) standardControllerFieldPathExists(definition storage.ObjectDefinition, parts []string) bool {
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if len(parts) == 1 {
		_, ok := storage.ResolveFieldName(definition, namespace, parts[0])
		return ok
	}
	fieldName, ok := resolveRelationshipFieldName(definition, namespace, parts[0])
	if !ok {
		return false
	}
	for _, target := range definition.Fields[fieldName].ReferenceTo {
		if _, parent, known := vm.describeObjectDefinition(target); known && vm.standardControllerFieldPathExists(parent, parts[1:]) {
			return true
		}
	}
	return false
}

func (vm *VM) standardControllerID(record Value) Value {
	if _, id, ok := objectFieldValue(record, "Id"); ok {
		if text, ok := idValueText(id); ok && text != "" {
			return id
		}
	}
	// API59/67 new_record, visibility_no_id and action_*_new retain the
	// request ID for controller identity without assigning it to the record.
	params := vm.CurrentPage().Fields["parameters"]
	if params.Kind == ValueMap {
		if id, ok := params.Map[mapKey(String("id"))]; ok {
			if text, ok := idValueText(id); ok && text != "" {
				return platformScalar("Id", text)
			}
		}
	}
	return Null
}

func (vm *VM) standardControllerNavigationPage(record Value) Value {
	page := standardControllerPage(record)
	if page.Fields["url"].Text == "" {
		if id, ok := idValueText(vm.standardControllerID(record)); ok && id != "" {
			page = newPageReference("/" + id)
		}
	}
	// API59/67 action_save/cancel/view_* return redirecting references,
	// including navigation to the current request ID for unsaved records.
	page.Fields["redirect"] = Bool(true)
	return page
}

func (vm *VM) standardControllerEditPage(record Value) Value {
	page := vm.standardControllerNavigationPage(record)
	currentURL := pageReferenceURL(vm.CurrentPage())
	if currentURL.Kind != ValueString || currentURL.Text == "" || page.Fields["url"].Text == "" {
		return page
	}
	// API59/67 r_action_edit_* return an edit reference with the current
	// Visualforce request as its form-encoded return target.
	page = newPageReference(page.Fields["url"].Text + "/e?retURL=" + url.QueryEscape(currentURL.Text))
	page.Fields["redirect"] = Bool(true)
	return page
}

func (vm *VM) standardControllerDeletePage() (Value, bool, error) {
	// The captured delete references use the request ID and its object-list
	// return target even when the supplied record is new or of another type.
	// Keep standalone controllers on their existing path without a page bridge.
	if vm.vfStandardControllerReset == nil {
		return Null, false, nil
	}
	parameters := vm.CurrentPage().Fields["parameters"]
	id, ok := parameters.Map[mapKey(String("id"))]
	idText, valid := idValueText(id)
	if !ok || !valid || len(idText) < 3 {
		return Null, false, nil
	}
	// Request security owns token issuance. An unbound local request exposes
	// the missing token rather than inventing a hosted confirmation credential.
	token := ""
	if vm.vfDeleteConfirmationToken != nil {
		var err error
		token, err = vm.vfDeleteConfirmationToken(idText)
		if err != nil {
			return Null, true, err
		}
	}
	page := newPageReference("/setup/own/deleteredirect.jsp?_CONFIRMATIONTOKEN=" + url.QueryEscape(token) +
		"&delID=" + url.QueryEscape(idText) + "&retURL=" + url.QueryEscape("/"+idText[:3]+"/o"))
	page.Fields["redirect"] = Bool(true)
	return page, true, nil
}

func (vm *VM) standardControllerFieldProjection(record Value, fields Value) (Value, error) {
	if vm == nil || vm.Org == nil || record.Kind != ValueObject || fields.Kind != ValueList {
		return Null, nil
	}
	id := sObjectIDFromFields(record.Fields)
	if id == "" || len(fields.List) == 0 {
		return Null, nil
	}
	objectName, ok := vm.resolveObjectName(record.Type)
	if !ok {
		objectName = record.Type
	}
	selected := []string{"Id"}
	for _, field := range fields.List {
		name := strings.TrimSpace(field.Text)
		if name != "" {
			selected = append(selected, name)
			parts := splitSOQLRelationshipFieldPath(name)
			for index := 1; index < len(parts); index++ {
				selected = append(selected, strings.Join(parts[:index], ".")+".Id")
			}
		}
	}
	if len(selected) == 1 {
		return Null, nil
	}
	query := soql.Query{
		Object: objectName, Fields: selected,
		Where: &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue(id)},
		Limit: 1, HasLimit: true, SecurityMode: "USER_MODE",
	}
	if err := vm.enforceSOQLSecurity(query, ""); err != nil {
		return Null, err
	}
	if err := vm.enforceOrgShapeObjectAvailability(query); err != nil {
		return Null, err
	}
	executionQuery := query
	executionQuery.SecurityMode = ""
	rows, err := soql.Execute(*vm.Org, executionQuery)
	if err != nil {
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			return Null, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return Null, newExceptionError("QueryException", err.Error())
	}
	rows = vm.applySOQLSharing(query, rows)
	if len(rows.Records) == 0 {
		return Null, newExceptionError("QueryException", "List has no rows for assignment to SObject")
	}
	projection := vm.vmValueFromRecord(rows.Records[0])
	for _, field := range fields.List {
		name := strings.TrimSpace(field.Text)
		if name == "" {
			continue
		}
		if _, present := standardControllerFieldPathValue(projection, name); present {
			continue
		}
		if _, _, nullParent := standardControllerNullParentPathValue(projection, name); nullParent {
			continue
		}
		parts := splitSOQLRelationshipFieldPath(name)
		if len(parts) == 0 {
			continue
		}
		if len(parts) > 1 {
			parent, present := standardControllerFieldPathValue(projection, strings.Join(parts[:len(parts)-1], "."))
			if !present || parent.Kind != ValueObject {
				continue
			}
		}
		vm.putVMRecordFieldPath(projection, objectName, name, Null)
	}
	return projection, nil
}

// ReadVisualforceRecord returns the MVP Name/Id projection exposed to the
// Visualforce HTML standard-controller bridge. It requires an explicit VM
// execution user present in Org.User and reuses the StandardController USER_MODE
// read path; the fresh storage record cannot expose query-side system or
// relationship data.
func (vm *VM) ReadVisualforceRecord(objectName string, id storage.ID) (storage.Record, bool, error) {
	if vm == nil || vm.Org == nil {
		return storage.Record{}, false, fmt.Errorf("Visualforce record read requires an org")
	}
	user, err := vm.visualforceExecutionUserRecord()
	if err != nil {
		return storage.Record{}, false, err
	}
	vm.SetCurrentUser(user)
	objectName = strings.TrimSpace(objectName)
	if objectName == "" || id == "" {
		return storage.Record{}, false, nil
	}
	if canonical, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonical
	}
	partial := Object(objectName)
	partial.Fields["Id"] = platformScalar("Id", string(id))
	projection, err := vm.standardControllerFieldProjection(partial, List(String("Name")))
	if err != nil {
		var thrown *apexThrowError
		if errors.As(err, &thrown) && strings.EqualFold(thrown.value.Type, "QueryException") &&
			stringField(thrown.value, "message") == "List has no rows for assignment to SObject" {
			return storage.Record{}, false, nil
		}
		return storage.Record{}, false, err
	}
	selected := storage.Record{
		ID:     id,
		Object: objectName,
		Fields: map[string]storage.Value{"Name": storage.NullValue()},
	}
	if name, ok := standardControllerFieldPathValue(projection, "Name"); ok {
		value, err := storageValueFromVM(name)
		if err != nil {
			return storage.Record{}, false, err
		}
		selected.Fields["Name"] = value
	}
	return selected, true, nil
}

func (vm *VM) visualforceExecutionUserRecord() (storage.Record, error) {
	user := vm.executionUser
	if user.Kind != ValueObject || !strings.EqualFold(user.Type, "User") {
		return storage.Record{}, fmt.Errorf("Visualforce record read requires an explicit execution user")
	}
	userID := storage.ID(stringField(user, "Id"))
	if userID == "" {
		return storage.Record{}, fmt.Errorf("Visualforce record read requires an execution user ID")
	}
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		return storage.Record{}, fmt.Errorf("Visualforce record read does not accept an active test user context")
	}
	users, ok := vm.Org.Objects["User"]
	if !ok {
		return storage.Record{}, fmt.Errorf("Visualforce execution user is absent from Org.User")
	}
	stored, ok := users.Records[userID]
	if !ok || stored.ID != userID {
		return storage.Record{}, fmt.Errorf("Visualforce execution user does not exactly match Org.User")
	}
	return stored, nil
}

// VisualforceExecutionUserRecord returns the stored user explicitly bound to
// this VM. Visualforce globals must not choose a different user from Org.User.
func (vm *VM) VisualforceExecutionUserRecord() (storage.Record, error) {
	if vm == nil || vm.Org == nil {
		return storage.Record{}, fmt.Errorf("Visualforce execution user requires an org")
	}
	return vm.visualforceExecutionUserRecord()
}

func (vm *VM) mergeStandardControllerProjection(record *Value, projection Value, fields Value, preserveEdits bool) {
	if record == nil || record.Kind != ValueObject || projection.Kind != ValueObject {
		return
	}
	visiblePaths := make([]string, 0, len(fields.List))
	for _, field := range fields.List {
		name := strings.TrimSpace(field.Text)
		if name == "" {
			continue
		}
		if preserveEdits {
			if parentPath, conflict := standardControllerConflictingAncestor(*record, projection, name); conflict {
				visiblePaths = append(visiblePaths, parentPath)
				continue
			}
			if parentPath, parentValue, found := standardControllerNullParentPathValue(*record, name); found && vm.standardControllerFieldIsLoadedOrEdited(*record, parentPath, parentValue) {
				visiblePaths = append(visiblePaths, parentPath)
				continue
			}
		}
		value, present := standardControllerFieldPathValue(projection, name)
		if !present {
			var parentPath string
			parentPath, value, present = standardControllerNullParentPathValue(projection, name)
			if !present {
				continue
			}
			name = parentPath
		}
		if previous, exists := standardControllerFieldPathValue(*record, name); exists && preserveEdits {
			if vm.standardControllerFieldIsLoadedOrEdited(*record, name, previous) {
				visiblePaths = append(visiblePaths, name)
				continue
			}
		}
		vm.mergeStandardControllerAncestorIDs(record, projection, name)
		vm.putVMRecordFieldPath(*record, record.Type, name, cloneValue(value))
		visiblePaths = append(visiblePaths, name)
	}
	for _, path := range visiblePaths {
		vm.markStandardControllerFieldVisible(record, path)
	}
}

func standardControllerConflictingAncestor(record Value, projection Value, path string) (string, bool) {
	parts := splitSOQLRelationshipFieldPath(path)
	if len(parts) < 2 {
		return "", false
	}
	current, projected := record, projection
	for index, relationship := range parts[:len(parts)-1] {
		_, existing, present := objectFieldValue(current, relationship)
		_, source, selected := objectFieldValue(projected, relationship)
		if !present || !selected {
			break
		}
		parentPath := strings.Join(parts[:index+1], ".")
		if isExplicitSObjectField(current, relationship) || isUserSetSObjectFieldAlias(current, relationship) {
			return parentPath, true
		}
		if existing.Kind != ValueObject || source.Kind != ValueObject {
			break
		}
		liveID, storedID := sObjectIDFromFields(existing.Fields), sObjectIDFromFields(source.Fields)
		if liveID != "" && storedID != "" && !storage.IDsEqual(liveID, storedID) {
			return parentPath, true
		}
		current, projected = existing, source
	}
	return "", false
}

func (vm *VM) mergeStandardControllerAncestorIDs(record *Value, projection Value, path string) {
	parts := splitSOQLRelationshipFieldPath(path)
	for index := 1; index < len(parts); index++ {
		parentPath := strings.Join(parts[:index], ".")
		projectedID, projected := standardControllerFieldPathValue(projection, parentPath+".Id")
		if !projected || projectedID.Kind == ValueNull {
			continue
		}
		if currentID, present := standardControllerFieldPathValue(*record, parentPath+".Id"); present && currentID.Kind != ValueNull {
			continue
		}
		vm.putVMRecordFieldPath(*record, record.Type, parentPath+".Id", cloneValue(projectedID))
	}
}

func (vm *VM) standardControllerFieldIsLoadedOrEdited(record Value, path string, previous Value) bool {
	if previous.Kind != ValueNull {
		return true
	}
	parts := splitSOQLRelationshipFieldPath(path)
	if len(parts) == 0 {
		return false
	}
	parent := record
	for _, relationship := range parts[:len(parts)-1] {
		_, nested, ok := objectFieldValue(parent, relationship)
		if !ok || nested.Kind != ValueObject {
			return false
		}
		parent = nested
	}
	leaf := parts[len(parts)-1]
	return vm.queriedSObjectFieldsIncludes(parent, leaf) || isExplicitSObjectField(parent, leaf) || isUserSetSObjectFieldAlias(parent, leaf)
}

func (vm *VM) markStandardControllerFieldVisible(record *Value, path string) {
	if record == nil || record.Kind != ValueObject {
		return
	}
	vm.seedStandardControllerQueriedMarkers(record, splitSOQLRelationshipFieldPath(path))
	parts := splitSOQLRelationshipFieldPath(path)
	if len(parts) == 0 {
		return
	}
	markQueriedSObjectField(record, parts[0])
	if len(parts) > 1 {
		vm.markQueriedParentRelationshipPath(record, record.Type, parts)
	}
}

func (vm *VM) seedStandardControllerQueriedMarkers(record *Value, parts []string) {
	if record == nil || record.Kind != ValueObject {
		return
	}
	if _, ok := record.Fields[sobjectQueriedFieldsField]; !ok {
		present := make(map[string]bool)
		for field, value := range record.Fields {
			if !isInternalSObjectField(field) && (value.Kind != ValueNull || isExplicitSObjectField(*record, field)) {
				present[strings.ToLower(field)] = true
			}
		}
		record.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue(record.Type, present)
	}
	if len(parts) < 2 {
		return
	}
	actual, parent, present := objectFieldValue(*record, parts[0])
	if present && parent.Kind == ValueObject {
		vm.seedStandardControllerQueriedMarkers(&parent, parts[1:])
		record.Fields[actual] = parent
	}
}

func standardControllerFieldPathValue(record Value, path string) (Value, bool) {
	current := record
	for _, segment := range strings.Split(path, ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return Null, false
		}
		_, next, exists := objectFieldValue(current, segment)
		if !exists {
			return Null, false
		}
		current = next
	}
	return current, true
}

func standardControllerNullParentPathValue(record Value, path string) (string, Value, bool) {
	current := record
	parts := splitSOQLRelationshipFieldPath(path)
	if len(parts) < 2 {
		return "", Null, false
	}
	for index, segment := range parts[:len(parts)-1] {
		_, next, exists := objectFieldValue(current, segment)
		if !exists {
			return "", Null, false
		}
		if next.Kind == ValueNull {
			return strings.Join(parts[:index+1], "."), next, true
		}
		if next.Kind != ValueObject {
			return "", Null, false
		}
		current = next
	}
	return "", Null, false
}

func ensureStandardControllerOriginalRecord(receiver Value, record Value) Value {
	if _, ok := receiver.Fields["originalRecord"]; !ok {
		receiver.Fields["originalRecord"] = cloneValue(record)
	}
	return receiver
}

func apexPagesControllerFieldList(value Value, surface string) (Value, error) {
	fields := List()
	fields.Type = value.Type
	for _, field := range value.List {
		if field.Kind != ValueString {
			return Null, fmt.Errorf("%s expects field name Strings", surface)
		}
		fields.List = append(fields.List, field)
	}
	return fields, nil
}

func callApexStackMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "empty", "peek", "pop", "push")
	values := receiver.Fields["values"]
	if values.Kind != ValueList {
		values = List()
	}
	switch method {
	case "empty":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.empty expects 0 arguments")
		}
		return Bool(len(values.List) == 0), receiver, false, true, nil
	case "peek":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.peek expects 0 arguments")
		}
		if len(values.List) == 0 {
			return Null, receiver, false, true, newExceptionError("Apex.EmptyStackException", "Stack is empty")
		}
		return values.List[len(values.List)-1], receiver, false, true, nil
	case "pop":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.pop expects 0 arguments")
		}
		if len(values.List) == 0 {
			return Null, receiver, false, true, newExceptionError("Apex.EmptyStackException", "Stack is empty")
		}
		value := values.List[len(values.List)-1]
		values.List = values.List[:len(values.List)-1]
		receiver.Fields["values"] = values
		return value, receiver, true, true, nil
	case "push":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.push expects 1 argument")
		}
		values.List = append(values.List, args[0])
		receiver.Fields["values"] = values
		return args[0], receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callApexPagesActionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getExpression", "invoke")
	switch method {
	case "getExpression":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.Action.getExpression expects 0 arguments")
		}
		if value, ok := receiver.Fields["expression"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "invoke":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.Action.invoke expects 0 arguments")
		}
		expression := ""
		if value, ok := receiver.Fields["expression"]; ok && value.Kind == ValueString {
			expression = strings.TrimSpace(value.Text)
		}
		if expression == "" || strings.EqualFold(expression, "null") || strings.EqualFold(expression, "{!null}") || strings.EqualFold(expression, "{!}") {
			return Null, receiver, false, true, nil
		}
		if strings.EqualFold(expression, "list") || strings.EqualFold(expression, "{!list}") {
			return newPageReference("/list"), receiver, false, true, nil
		}
		if vm != nil && vm.vfActionInvoker != nil {
			value, err := vm.vfActionInvoker(expression, pageReferenceURL(vm.CurrentPage()).Text)
			return value, receiver, false, true, err
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callFormulaBuilderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "withFormula", "withReturnType", "withType", "withGlobalVariables", "treatNumericNullAsZero", "parseAsTemplate", "build")
	switch method {
	case "withFormula":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withFormula expects formula String")
		}
		if args[0].Kind == ValueNull || (args[0].Kind == ValueString && args[0].Text == "") {
			return Null, receiver, false, true, newExceptionError("IllegalArgumentException", "Formula text cannot be null or empty")
		}
		if args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withFormula expects formula String")
		}
		receiver.Fields["formula"] = args[0]
		return receiver, receiver, true, true, nil
	case "withReturnType":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withReturnType expects return type")
		}
		if args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("IllegalArgumentException", "Formula return type cannot be null")
		}
		receiver.Fields["returnType"] = args[0]
		return receiver, receiver, true, true, nil
	case "withType":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withType expects context type")
		}
		receiver.Fields["contextType"] = args[0]
		return receiver, receiver, true, true, nil
	case "withGlobalVariables":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withGlobalVariables expects FormulaGlobal list")
		}
		if len(args[0].List) == 0 {
			return Null, receiver, false, true, newExceptionError("IllegalArgumentException", "Formula globals list cannot be null or empty")
		}
		receiver.Fields["globalVariables"] = args[0]
		return receiver, receiver, true, true, nil
	case "treatNumericNullAsZero":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.treatNumericNullAsZero expects Boolean")
		}
		receiver.Fields["treatNumericNullAsZero"] = args[0]
		return receiver, receiver, true, true, nil
	case "parseAsTemplate":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.parseAsTemplate expects Boolean")
		}
		receiver.Fields["templateMode"] = args[0]
		return receiver, receiver, true, true, nil
	case "build":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.build expects 0 arguments")
		}
		formula, _ := formulaInstanceText(receiver)
		if _, ok := receiver.Fields["treatNumericNullAsZero"]; !ok {
			receiver.Fields["treatNumericNullAsZero"] = Bool(false)
		}
		for _, required := range []string{"formula", "returnType", "contextType"} {
			if _, ok := receiver.Fields[required]; !ok {
				return Null, receiver, false, true, newExceptionError("IllegalArgumentException", "build() can be called only after all three required methods: withFormula(), withReturnType() and withType() have been called")
			}
		}
		templateMode := false
		if value, ok := receiver.Fields["templateMode"]; ok && value.Kind == ValueBool {
			templateMode = value.Bool
		}
		returnType := receiver.Fields["returnType"].Text
		if templateMode && !strings.EqualFold(returnType, "STRING") {
			return Null, receiver, false, true, newExceptionError("IllegalArgumentException", "The only supported return type for template mode is STRING.")
		}
		definition := vm.formulaContextDefinition(formulaContextTypeName(receiver.Fields["contextType"]), make(map[string]bool))
		if message := dml.ValidateFormulaInstance(formula, vm.Org, definition, returnType, templateMode); message != "" {
			return Null, receiver, false, true, newExceptionError("FormulaValidationException", message)
		}
		if contextType, ok := receiver.Fields["contextType"]; ok {
			typeName := typeValueName(contextType)
			if class, found := vm.lookupClass(typeName); found && !strings.EqualFold(class.Access, "global") && vm.typeMatches(typeName, "TriggerRecord", make(map[string]bool)) {
				return Null, receiver, false, true, newExceptionError("FormulaValidationException", typeName+" must be global to be used as a formula context")
			}
		}
		instance := Object("formulaeval.FormulaInstance")
		for field, value := range receiver.Fields {
			instance.Fields[field] = value
		}
		return instance, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callFormulaInstanceMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "evaluate", "getReferencedFields")
	switch method {
	case "evaluate":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaInstance.evaluate expects context object")
		}
		if args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("IllegalArgumentException", "evaluate method parameter cannot be null")
		}
		formula, _ := formulaInstanceText(receiver)
		if formula == "" {
			return Null, receiver, false, true, newExceptionError("FormulaEvaluationException", "formula text is required")
		}
		value, ok, err := vm.evaluateFormulaInstanceValue(receiver, args[0], formula)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if !ok {
			return Null, receiver, false, true, unsupportedCallError("formulaeval.FormulaInstance.evaluate unsupported local formula expression")
		}
		return value, receiver, false, true, nil
	case "getReferencedFields":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaInstance.getReferencedFields expects 0 arguments")
		}
		formula, _ := formulaInstanceText(receiver)
		out := Set()
		out.Type = "Set<String>"
		for _, field := range dml.FormulaReferencedFields(formula, receiver.Fields["templateMode"].Bool) {
			out.Set = append(out.Set, String(field))
		}
		return out, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callFormulaRecalcResultMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "isSuccess", "getSObject", "getErrors")
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "isSuccess":
		if value, ok := receiver.Fields["success"]; ok {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case "getSObject":
		if value, ok := receiver.Fields["sobject"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getErrors":
		if value, ok := receiver.Fields["errors"]; ok {
			return value, receiver, false, true, nil
		}
		return typedList("List<FormulaRecalcFieldError>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callFormulaRecalcFieldErrorMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getFieldName", "getFieldError")
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "getFieldName":
		if value, ok := receiver.Fields["fieldName"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getFieldError":
		if value, ok := receiver.Fields["fieldError"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) recalculateFormulaList(args []Value) (Value, error) {
	if len(args) == 1 && args[0].Kind == ValueNull {
		return Null, newExceptionError("NullPointerException", "Expected a non-null list of SObjects")
	}
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("Formula.recalculateFormulas expects SObject list")
	}
	out := typedList("List<FormulaRecalcResult>")
	for _, item := range args[0].List {
		if item.Kind == ValueNull {
			return Null, newExceptionError("IllegalArgumentException", "Cannot recalculate formulas with null in set of entities")
		}
		if item.Kind != ValueObject || !vm.isSObjectLikeType(item.Type) {
			return Null, fmt.Errorf("Formula.recalculateFormulas expects SObject list")
		}
		// Native returns results only for entities with formula fields (R270).
		if objectName, ok := vm.resolveObjectName(item.Type); ok {
			hasFormula := false
			for _, field := range vm.Org.Objects[objectName].Definition.Fields {
				if field.Type == storage.FieldCalculated && strings.TrimSpace(field.Formula) != "" {
					hasFormula = true
					break
				}
			}
			if !hasFormula {
				continue
			}
		}
		out.List = append(out.List, vm.recalculateFormulaSObject(item))
	}
	return out, nil
}

func (vm *VM) recalculateFormulaSObject(item Value) Value {
	result := Object("FormulaRecalcResult")
	result.Fields["sobject"] = item
	result.Fields["success"] = Bool(true)
	errors := typedList("List<FormulaRecalcFieldError>")
	if vm.Org == nil {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "org metadata is required"))
		result.Fields["errors"] = errors
		return result
	}
	objectName, ok := vm.resolveObjectName(item.Type)
	if !ok {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "object metadata is required"))
		result.Fields["errors"] = errors
		return result
	}
	definition := vm.Org.Objects[objectName].Definition
	record, ok := vm.formulaRecordFromSObject(item)
	if !ok {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "SObject value cannot be converted to a formula context"))
		result.Fields["errors"] = errors
		return result
	}
	fieldNames := make([]string, 0, len(definition.Fields))
	for fieldName := range definition.Fields {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)
	for _, fieldName := range fieldNames {
		field := definition.Fields[fieldName]
		if field.Type != storage.FieldCalculated || strings.TrimSpace(field.Formula) == "" {
			continue
		}
		failure := dml.FormulaError{}
		options := dml.FormulaEvaluationOptions{FormulaSurface: true, PreserveNumericNull: strings.EqualFold(field.FormulaTreatBlanksAs, "BlankAsBlank"), Error: &failure}
		value, explicitNull, ok := dml.EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, vm.Org, definition, record, options)
		if !ok {
			result.Fields["success"] = Bool(false)
			message := "unsupported local formula expression"
			errorField := fieldName
			if failure.Type != "" {
				message = failure.Message
				setExplicitSObjectField(&item, fieldName, Null)
				if field.Label != "" {
					errorField = field.Label
				}
			}
			errors.List = append(errors.List, formulaRecalcFieldError(errorField, message))
			continue
		}
		if explicitNull {
			setExplicitSObjectField(&item, fieldName, Null)
			continue
		}
		setExplicitSObjectField(&item, fieldName, vmValueFromStorage(value))
	}
	result.Fields["sobject"] = item
	result.Fields["errors"] = errors
	return result
}

func formulaRecalcFieldError(fieldName, message string) Value {
	err := Object("FormulaRecalcFieldError")
	err.Fields["fieldName"] = String(fieldName)
	err.Fields["fieldError"] = String(message)
	return err
}

func (vm *VM) evaluateFormulaInstanceValue(instance Value, context Value, formula string) (Value, bool, error) {
	definition := vm.formulaContextDefinition(context.Type, make(map[string]bool))
	referenced := dml.FormulaReferencedFields(formula, instance.Fields["templateMode"].Bool)
	record, ok := vm.formulaInstanceRecord(context, definition, referenced)
	if !ok {
		return Null, false, nil
	}
	field := storage.Field{APIName: "__formula", Type: formulaReturnFieldType(instance), Formula: formula}
	failure := dml.FormulaError{}
	options := dml.FormulaEvaluationOptions{FormulaSurface: true, PreserveNumericNull: true, Error: &failure}
	if value, ok := instance.Fields["treatNumericNullAsZero"]; ok && value.Kind == ValueBool {
		options.PreserveNumericNull = !value.Bool
	}
	if value, ok := instance.Fields["templateMode"]; ok && value.Kind == ValueBool {
		options.Template = value.Bool
	}
	value, explicitNull, ok := dml.EvaluateRecordFormulaValueInOrgWithOptions(formula, field, vm.Org, definition, record, options)
	if failure.Type != "" {
		return Null, false, newExceptionError(failure.Type, failure.Message)
	}
	if !ok {
		return Null, false, nil
	}
	if explicitNull {
		return Null, true, nil
	}
	out := vmValueFromStorage(value)
	switch strings.ToUpper(instance.Fields["returnType"].Text) {
	case "INTEGER":
		if out.Kind == ValueInt {
			out = Int(int64(int32(out.Int)))
		}
	case "DOUBLE":
		n, err := strconv.ParseFloat(value.Decimal, 64)
		if err == nil {
			out = decimalAsDouble(Decimal(n))
		}
	case "TIME":
		out = platformScalar("Time", value.String)
	}
	return out, true, nil
}

func (vm *VM) formulaSObjectContext(context Value, formula string) (Value, string, bool) {
	if context.Kind == ValueObject && vm.isSObjectLikeType(context.Type) {
		return context, formula, true
	}
	if context.Kind != ValueObject {
		return Null, formula, false
	}
	for _, prefix := range []string{"record.", "recordPrior."} {
		if !strings.Contains(formula, prefix) {
			continue
		}
		fieldName := strings.TrimSuffix(prefix, ".")
		field, owner, found := vm.lookupReceiverField(context.Type, fieldName)
		if !found {
			continue
		}
		var value Value
		var err error
		if field.Getter != nil {
			value, err = vm.callGetter(owner, field, context)
		} else if _, candidate, ok := objectFieldValue(context, fieldName); ok {
			value = candidate
		}
		if err != nil || value.Kind != ValueObject || !vm.isSObjectLikeType(value.Type) {
			continue
		}
		return value, strings.ReplaceAll(formula, prefix, ""), true
	}
	return Null, formula, false
}

func formulaInstanceText(instance Value) (string, bool) {
	if value, ok := instance.Fields["formula"]; ok && value.Kind == ValueString {
		return value.Text, true
	}
	if value, ok := instance.Fields["formulaText"]; ok && value.Kind == ValueString {
		return value.Text, true
	}
	return "", false
}

func formulaReturnFieldType(instance Value) storage.FieldType {
	if value, ok := instance.Fields["returnType"]; ok {
		name := strings.ToUpper(strings.TrimSpace(value.Text))
		if name == "" && value.Kind == ValueObject {
			name = strings.ToUpper(strings.TrimSpace(value.Text))
		}
		switch name {
		case "BOOLEAN":
			return storage.FieldBoolean
		case "INTEGER", "LONG":
			return storage.FieldInteger
		case "DECIMAL", "DOUBLE":
			return storage.FieldDecimal
		case "DATE":
			return storage.FieldDate
		case "DATETIME":
			return storage.FieldDateTime
		case "TIME":
			return storage.FieldTime
		case "ID":
			return storage.FieldID
		}
	}
	return storage.FieldString
}

func formulaContextTypeName(value Value) string {
	if isSObjectTypeToken(value) {
		if name, ok := sObjectTypeTokenObjectName(value); ok {
			return name
		}
	}
	return typeValueName(value)
}

// Apex contexts use their declared field types, flattened into the same record
// evaluator as SObjects. No synthetic type is installed in the org or schema.
func (vm *VM) formulaContextDefinition(typeName string, active map[string]bool) storage.ObjectDefinition {
	if vm.Org != nil {
		if name, ok := vm.resolveObjectName(typeName); ok {
			return vm.Org.Objects[name].Definition
		}
	}
	definition := storage.ObjectDefinition{APIName: typeName, Fields: map[string]storage.Field{}}
	if active[typeName] {
		return definition
	}
	active[typeName] = true
	defer delete(active, typeName)
	class, ok := vm.lookupClass(typeName)
	if !ok {
		return definition
	}
	for name, field := range class.Fields {
		if field.Static {
			continue
		}
		typ := storage.FieldString
		switch strings.ToLower(field.Type) {
		case "decimal", "double":
			typ = storage.FieldDecimal
		case "integer", "long":
			typ = storage.FieldInteger
		case "boolean":
			typ = storage.FieldBoolean
		case "date":
			typ = storage.FieldDate
		case "datetime":
			typ = storage.FieldDateTime
		case "time":
			typ = storage.FieldTime
		case "id":
			typ = storage.FieldID
		}
		definition.Fields[name] = storage.Field{APIName: name, Type: typ}
		for nestedName, nested := range vm.formulaContextDefinition(field.Type, active).Fields {
			nested.APIName = name + "." + nestedName
			definition.Fields[nested.APIName] = nested
		}
	}
	return definition
}

func (vm *VM) formulaInstanceRecord(context Value, definition storage.ObjectDefinition, referenced []string) (storage.Record, bool) {
	if context.Kind != ValueObject {
		return storage.Record{}, false
	}
	record := storage.Record{Object: context.Type, Fields: map[string]storage.Value{}, ExplicitNulls: map[string]bool{}}
	if vm.isSObjectLikeType(context.Type) {
		var ok bool
		record, ok = vm.formulaRecordFromSObject(context)
		if !ok {
			return record, false
		}
	}
	for _, name := range referenced {
		if resolved, ok := storage.ResolveFieldName(definition, "", name); ok {
			name = resolved
		}
		current := context
		for _, part := range strings.Split(name, ".") {
			if current.Kind != ValueObject {
				current = Null
				break
			}
			if _, value, ok := objectFieldValue(current, part); ok {
				current = value
				continue
			}
			field, owner, ok := vm.lookupReceiverField(current.Type, part)
			if ok && field.Getter != nil {
				value, err := vm.callGetter(owner, field, current)
				if err != nil {
					return record, false
				}
				current = value
			} else {
				current = Null
			}
		}
		if current.Kind == ValueNull {
			continue
		}
		value, err := storageValueFromVM(current)
		if err != nil {
			continue
		}
		// Keep explicit empty/space strings and decimal scale at the instance
		// boundary; DML's storage normalization is a different operation.
		if current.Kind == ValueString {
			value = storage.StringValue(current.Text)
		}
		if current.Kind == ValueDecimal {
			value = storage.DecimalValue(current.Text)
		}
		record.Fields[name] = value
		delete(record.ExplicitNulls, name)
	}
	return record, true
}

func formulaReferencedFields(formula string) []string {
	return dml.FormulaReferencedFields(formula, false)
}

func callContinuationMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "addHttpRequest", "getRequests")
	switch method {
	case "addHttpRequest":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument cannot be null.")
		}
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "HttpRequest") {
			return Null, receiver, false, true, fmt.Errorf("Continuation.addHttpRequest expects HttpRequest")
		}
		requests, ok := receiver.Fields["requests"]
		if !ok || requests.Kind != ValueMap {
			requests = typedMap("Map<String,HttpRequest>")
		}
		label := fmt.Sprintf("request-%d", len(requests.Map)+1)
		key := mapKey(String(label))
		requests.Map[key] = args[0]
		requests.MapKeys[key] = String(label)
		receiver.Fields["requests"] = requests
		return String(label), receiver, true, true, nil
	case "getRequests":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Continuation.getRequests expects 0 arguments")
		}
		if requests, ok := receiver.Fields["requests"]; ok && requests.Kind == ValueMap {
			return requests, receiver, false, true, nil
		}
		return typedMap("Map<String,HttpRequest>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexPagesComponentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getComponentById")
	if method != "getComponentById" {
		return Null, receiver, false, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, receiver, false, true, fmt.Errorf("%s.getComponentById expects id String", receiver.Type)
	}
	return Null, receiver, false, true, nil
}

func (vm *VM) callApexPagesIdeaStandardControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	receiver = ensureIdeaStandardControllerState(receiver)
	if method == "getCommentList" {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardController.getCommentList expects 0 arguments")
		}
		return typedList("List<IdeaComment>"), receiver, false, true, nil
	}
	return vm.callStandardControllerMember(receiver, method, args, result)
}

func (vm *VM) callApexPagesIdeaStandardSetControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	receiver = ensureIdeaStandardSetControllerState(receiver)
	switch method {
	case "getIdeaList":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardSetController.getIdeaList expects 0 arguments")
		}
		return typedList("List<Idea>"), receiver, false, true, nil
	case "getListViewOptions":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardSetController.getListViewOptions expects 0 arguments")
		}
		return typedList("List<SelectOption>"), receiver, false, true, nil
	default:
		return vm.callStandardSetControllerMember(receiver, method, args, result)
	}
}

func ensureIdeaStandardControllerState(receiver Value) Value {
	if _, ok := receiver.Fields["record"]; !ok {
		receiver.Fields["record"] = Object("Idea")
	}
	return receiver
}

func ensureIdeaStandardSetControllerState(receiver Value) Value {
	if _, ok := receiver.Fields["records"]; !ok {
		records := List()
		records.Type = "List<Idea>"
		receiver.Fields["records"] = records
	}
	if _, ok := receiver.Fields["selected"]; !ok {
		receiver.Fields["selected"] = List()
	}
	if _, ok := receiver.Fields["pageSize"]; !ok {
		receiver.Fields["pageSize"] = Int(20)
	}
	if _, ok := receiver.Fields["pageNumber"]; !ok {
		receiver.Fields["pageNumber"] = Int(1)
	}
	return receiver
}

func callApexPagesKnowledgeArticleVersionStandardControllerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	record, hasRecord := receiver.Fields["record"]
	switch method {
	case "addFields":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.addFields expects List")
		}
		fields := typedList("List<String>")
		for _, field := range args[0].List {
			if field.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.addFields expects field name Strings")
			}
			fields.List = append(fields.List, field)
			if hasRecord && record.Kind == ValueObject {
				if _, _, ok := objectFieldValue(record, field.Text); !ok {
					record.Fields[field.Text] = Null
				}
			}
		}
		receiver.Fields["requestedFields"] = fields
		if hasRecord && record.Kind == ValueObject {
			receiver.Fields["record"] = record
		}
		return Null, receiver, true, true, nil
	case "cancel", "view":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.%s expects 0 arguments", method)
		}
		if hasRecord && record.Kind == ValueObject {
			return standardControllerPage(record), receiver, false, true, nil
		}
		return newPageReference(""), receiver, false, true, nil
	case "getId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.getId expects 0 arguments")
		}
		if hasRecord && record.Kind == ValueObject {
			if _, id, ok := objectFieldValue(record, "Id"); ok {
				return id, receiver, false, true, nil
			}
		}
		return Null, receiver, false, true, nil
	case "getRecord":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.getRecord expects 0 arguments")
		}
		if hasRecord && record.Kind == ValueObject {
			return record, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getSourceId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.getSourceId expects 0 arguments")
		}
		return Null, receiver, false, true, nil
	case "selectDataCategory", "setDataCategory":
		if method == "selectDataCategory" && len(args) == 0 {
			return Null, receiver, false, true, nil
		}
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.%s expects group and category Strings", method)
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func appendStandardControllerActionTrace(result *Result, phase, method string, record Value, extra map[string]any) {
	args := standardControllerTraceArgs(method, record)
	for key, value := range extra {
		args[key] = value
	}
	appendTrace(result, "apex.visualforce.standard_controller.action."+phase, "apex.visualforce.standard_controller", args)
}

func appendStandardControllerErrorTrace(result *Result, method string, record Value, dmlOperation string, err error) {
	actionErr := uiInvocationError(err)
	appendStandardControllerActionTrace(result, "error", method, record, map[string]any{
		"dmlOperation": dmlOperation,
		"error":        actionErr.Message,
		"errorType":    actionErr.Type,
	})
}

func standardControllerTraceArgs(method string, record Value) map[string]any {
	args := map[string]any{"method": method}
	if record.Kind == ValueObject {
		args["objectType"] = record.Type
		if _, id, ok := objectFieldValue(record, "Id"); ok && id.Kind == ValueString && id.Text != "" {
			args["recordId"] = id.Text
		}
	}
	return args
}

func (vm *VM) callStandardSetControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	standardSet := strings.EqualFold(receiver.Type, "ApexPages.StandardSetController")
	records := receiver.Fields["records"]
	switch method {
	case "getRecords":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getRecords expects 0 arguments")
		}
		page := standardSetCurrentPage(receiver, records)
		if standardSet {
			page.Fields = map[string]Value{"__collection_readonly": Bool(true)}
		}
		return page, receiver, false, true, nil
	case "getRecord":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getRecord expects 0 arguments")
		}
		page := standardSetCurrentPage(receiver, records)
		if len(page.List) == 0 {
			if objectType := receiver.Fields["__glade_record_type"].Text; objectType != "" {
				return Object(objectType), receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
		if page.List[0].Kind == ValueObject {
			record := Object(page.List[0].Type)
			record.Fields["Id"] = platformScalar("Id", "000000000000000AAA")
			return record, receiver, false, true, nil
		}
		return page.List[0], receiver, false, true, nil
	case "getResultSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getResultSize expects 0 arguments")
		}
		if records.Kind != ValueList {
			return Int(0), receiver, false, true, nil
		}
		return Int(int64(len(records.List))), receiver, false, true, nil
	case "getSelected":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getSelected expects 0 arguments")
		}
		return receiver.Fields["selected"], receiver, false, true, nil
	case "setSelected":
		if standardSet && len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
		}
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setSelected expects List")
		}
		selected := args[0]
		if standardSet {
			selected = standardSetCopyList(selected)
		}
		receiver.Fields["selected"] = selected
		return Null, receiver, true, true, nil
	case "getPageSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getPageSize expects 0 arguments")
		}
		return receiver.Fields["pageSize"], receiver, false, true, nil
	case "setPageSize":
		if standardSet && len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
		}
		if len(args) != 1 || args[0].Kind != ValueInt || (!standardSet && args[0].Int <= 0) {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setPageSize expects positive Integer")
		}
		if standardSet && args[0].Int == receiver.Fields["pageSize"].Int {
			return Null, receiver, false, true, nil
		}
		if callerProvided := receiver.Fields["__glade_caller_provided"]; callerProvided.Kind == ValueBool && callerProvided.Bool && (!standardSet || len(records.List) != 0) {
			return Null, receiver, false, true, newExceptionError("VisualforceException", "Modified rows exist in the records collection!")
		}
		if args[0].Int <= 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setPageSize expects positive Integer")
		}
		receiver.Fields["pageSize"] = args[0]
		receiver.Fields["pageNumber"] = Int(1)
		return Null, receiver, true, true, nil
	case "getPageNumber":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getPageNumber expects 0 arguments")
		}
		return receiver.Fields["pageNumber"], receiver, false, true, nil
	case "setPageNumber":
		if len(args) != 1 || args[0].Kind != ValueInt || args[0].Int <= 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setPageNumber expects positive Integer")
		}
		page := int(args[0].Int)
		pageCount := standardSetPageCount(receiver, records)
		if page > pageCount {
			page = pageCount
		}
		receiver.Fields["pageNumber"] = Int(int64(page))
		return Null, receiver, true, true, nil
	case "getListViewOptions":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getListViewOptions expects 0 arguments")
		}
		if standardSet {
			return vm.standardSetListViewOptions(receiver), receiver, false, true, nil
		}
		options := typedList("List<SelectOption>")
		options.List = append(options.List, newSelectOption(
			platformScalar("Id", "000000000000000AAA"),
			String("All"),
			Bool(false),
			Bool(true),
		))
		return options, receiver, false, true, nil
	case "addFields":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.addFields expects List")
		}
		if callerProvided, ok := receiver.Fields["__glade_caller_provided"]; ok && callerProvided.Kind == ValueBool && callerProvided.Bool {
			return Null, receiver, false, true, newExceptionError("SObjectException", "You cannot call addFields when the data is being passed into the controller by the caller.")
		}
		fields, err := apexPagesControllerFieldList(args[0], "ApexPages.StandardSetController.addFields")
		if err != nil {
			return Null, receiver, false, true, err
		}
		receiver.Fields["fields"] = fields
		return Null, receiver, true, true, nil
	case "setFilterId":
		allowNull := standardSet && !receiver.Fields["__glade_caller_provided"].Bool
		if len(args) != 1 || (args[0].Kind != ValueString && (!allowNull || args[0].Kind != ValueNull)) {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setFilterId expects String")
		}
		if standardSet {
			return vm.setStandardSetFilter(receiver, args[0], result)
		}
		receiver.Fields["filterId"] = args[0]
		return Null, receiver, true, true, nil
	case "getFilterId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getFilterId expects 0 arguments")
		}
		if value, ok := receiver.Fields["filterId"]; ok {
			return value, receiver, false, true, nil
		}
		if standardSet && !receiver.Fields["__glade_caller_provided"].Bool {
			value := vm.standardSetDefaultFilter(receiver)
			receiver.Fields["filterId"] = value
			return value, receiver, true, true, nil
		}
		return Null, receiver, false, true, nil
	case "first":
		receiver.Fields["pageNumber"] = Int(1)
		return Null, receiver, true, true, nil
	case "last":
		receiver.Fields["pageNumber"] = Int(int64(standardSetPageCount(receiver, records)))
		return Null, receiver, true, true, nil
	case "next":
		page := int(receiver.Fields["pageNumber"].Int)
		if page < standardSetPageCount(receiver, records) {
			receiver.Fields["pageNumber"] = Int(int64(page + 1))
		}
		return Null, receiver, true, true, nil
	case "previous":
		page := int(receiver.Fields["pageNumber"].Int)
		if page > 1 {
			receiver.Fields["pageNumber"] = Int(int64(page - 1))
		}
		return Null, receiver, true, true, nil
	case "getHasNext":
		return Bool(int(receiver.Fields["pageNumber"].Int) < standardSetPageCount(receiver, records)), receiver, false, true, nil
	case "getHasPrevious":
		return Bool(receiver.Fields["pageNumber"].Int > 1), receiver, false, true, nil
	case "getCompleteResult":
		if complete, ok := receiver.Fields["completeResult"]; standardSet && ok {
			return complete, receiver, false, true, nil
		}
		return Bool(true), receiver, false, true, nil
	case "save":
		if standardSet {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.save expects 0 arguments")
			}
			return vm.saveStandardSetController(receiver, result)
		}
		page, updated, changed, handled, err := vm.standardSetDML(receiver, "update", result)
		return page, updated, changed, handled, err
	case "delete":
		return vm.standardSetDML(receiver, "delete", result)
	case "cancel":
		page := newPageReference("/home/home.jsp")
		if standardSet {
			page.Fields["redirect"] = Bool(true)
		}
		return page, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
