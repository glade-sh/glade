package vm

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func newStandardSetController(input Value) (Value, error) {
	callerProvided := input.Kind == ValueList
	records := input
	if !callerProvided {
		var ok bool
		records, ok = input.Fields["Records"]
		if !ok {
			records = List()
		}
	}
	objectType := ""
	for _, record := range records.List {
		if record.Kind != ValueObject {
			continue
		}
		if objectType == "" {
			objectType = record.Type
		} else if !strings.EqualFold(objectType, record.Type) {
			// A set controller requires records of one SObject type.
			return Null, newExceptionError("VisualforceException", "Record set cannot be empty.")
		}
	}
	if objectType == "" {
		objectType, _ = collectionElementType(records.Type)
	}
	complete := true
	if callerProvided {
		// The controller owns list membership while retaining the supplied
		// SObject identities.
		records = standardSetCopyList(records)
		if len(records.List) > visualforceSetRecordLimit {
			// List construction returns the first 10,000 records and reports
			// an incomplete result.
			records.List = records.List[:visualforceSetRecordLimit]
			complete = false
		}
	}
	return newStandardSetControllerState(records, objectType, callerProvided, complete), nil
}

func newStandardSetControllerState(records Value, objectType string, callerProvided, complete bool) Value {
	controller := Object("ApexPages.StandardSetController")
	controller.Fields["records"] = records
	controller.Fields["selected"] = List()
	controller.Fields["pageSize"] = Int(20)
	controller.Fields["pageNumber"] = Int(1)
	controller.Fields["completeResult"] = Bool(complete)
	controller.Fields["__glade_record_type"] = String(objectType)
	controller.Fields["__glade_record_snapshot"] = String(standardSetRecordSnapshot(records))
	if callerProvided {
		controller.Fields["__glade_caller_provided"] = Bool(true)
	}
	return controller
}

// NewVisualforceStandardSetController initializes the native recordSetVar path
// from its authorized records. The page projection is retained across empty
// results and does not include list-view display columns.
func NewVisualforceStandardSetController(records Value, objectType string, filterFields []string) Value {
	controller := newStandardSetControllerState(records, objectType, false, true)
	controller.Fields["__glade_native_page"] = Bool(true)
	fields := List()
	for _, field := range filterFields {
		fields.List = append(fields.List, String(field))
	}
	controller.Fields["__glade_page_projection"] = fields
	return controller
}

// ObserveVisualforceStandardSetField records the first page read of recordSetVar.
// The captured bound-page GET/postback controls retain the initial selection:
// setting that same filter leaves the materialized rows unchanged, while a
// different filter reloads. Pages which never read their records stay unselected.
func (vm *VM) ObserveVisualforceStandardSetField(controller Value, field string) {
	if !controller.Fields["__glade_native_page"].Bool || !strings.EqualFold(field, controller.Fields["__glade_record_set_var"].Text) {
		return
	}
	if _, exists := controller.Fields["filterId"]; !exists {
		controller.Fields["filterId"] = vm.standardSetDefaultFilter(controller)
	}
}

func standardSetRecordSnapshot(records Value) string {
	encoded, _ := json.Marshal(jsonFromValue(records, false))
	return string(encoded)
}

type standardSetListViewMetadata struct {
	Name    string   `xml:"fullName"`
	Label   string   `xml:"label"`
	Scope   string   `xml:"filterScope"`
	Columns []string `xml:"columns"`
	Filters []struct {
		Field     string `xml:"field"`
		Operation string `xml:"operation"`
		Value     string `xml:"value"`
	} `xml:"filters"`
}

func standardSetListViewColumnField(column string) (string, error) {
	// The captured Account list-view column ACCOUNT.PHONE1 selects and sorts
	// the Phone field. This is a metadata column alias, not a suffix rule.
	switch strings.ToUpper(column) {
	case "ACCOUNT.NAME":
		return "Name", nil
	case "ACCOUNT.PHONE1":
		return "Phone", nil
	default:
		return "", unsupportedCallError("StandardSetController list-view column " + column)
	}
}

// LoadStandardSetListViews connects project metadata to the request's org.
// Existing ListView identities remain stable across fresh GET/postback VMs.
func (vm *VM) LoadStandardSetListViews(paths []string) error {
	if vm.Org == nil || len(paths) == 0 {
		return nil
	}
	storage.EnsureStandardObject(vm.Org, "ListView")
	views := vm.Org.Objects["ListView"]
	generator := storage.NewRuntimeIDGeneratorForOrg(vm.Org)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var metadata standardSetListViewMetadata
		if err := xml.Unmarshal(data, &metadata); err != nil {
			return err
		}
		objectType := filepath.Base(filepath.Dir(filepath.Dir(path)))
		var id storage.ID
		for existing, record := range views.Records {
			name, _ := record.GetField("DeveloperName")
			kind, _ := record.GetField("SobjectType")
			if storageValueText(name) == metadata.Name && strings.EqualFold(storageValueText(kind), objectType) {
				id = existing
				break
			}
		}
		if id == "" {
			for {
				id, err = generator.Next("ListView")
				if err != nil {
					return err
				}
				// Native filter getters expose an 18-character ListView identity.
				id = storage.ID(displayIDText(string(id)))
				if _, exists := views.Records[id]; !exists {
					break
				}
			}
		}
		views.Records[id] = storage.Record{ID: id, Object: "ListView", Fields: map[string]storage.Value{
			"DeveloperName": storage.StringValue(metadata.Name), "Name": storage.StringValue(metadata.Label),
			"SobjectType": storage.StringValue(objectType), "__glade_list_view_metadata": storage.StringValue(string(data)),
		}}
	}
	vm.Org.Objects["ListView"] = views
	return nil
}

func standardSetListViewQuery(view storage.Record, records, pageProjection Value) (string, error) {
	data, _ := view.GetField("__glade_list_view_metadata")
	var metadata standardSetListViewMetadata
	if storageValueText(data) == "" {
		return "", fmt.Errorf("StandardSetController list-view metadata unavailable")
	}
	if err := xml.Unmarshal([]byte(storageValueText(data)), &metadata); err != nil {
		return "", err
	}
	if metadata.Scope != "Everything" {
		return "", unsupportedCallError("StandardSetController list-view scope " + metadata.Scope)
	}
	kind, _ := view.GetField("SobjectType")
	fields := map[string]bool{"Id": true}
	if pageProjection.Kind == ValueList {
		for _, field := range pageProjection.List {
			fields[field.Text] = true
		}
	} else if len(records.List) != 0 {
		for field := range records.List[0].Fields {
			if !strings.HasPrefix(field, "__") {
				fields[field] = true
			}
		}
	}
	for _, column := range metadata.Columns {
		name, err := standardSetListViewColumnField(column)
		if err != nil {
			return "", err
		}
		// A native page loads its own projection, not list-view display
		// columns. Filtering and ordering still use the view metadata.
		if pageProjection.Kind == ValueList {
			continue
		}
		found := false
		for field := range fields {
			if strings.EqualFold(field, name) {
				found = true
				break
			}
		}
		if !found {
			fields[name] = true
		}
	}
	var selected []string
	for field := range fields {
		selected = append(selected, field)
	}
	sort.Strings(selected)
	query := "SELECT " + strings.Join(selected, ",") + " FROM " + storageValueText(kind)
	var criteria []string
	for _, filter := range metadata.Filters {
		if filter.Operation != "startsWith" {
			return "", unsupportedCallError("StandardSetController list-view filter " + filter.Operation)
		}
		if !strings.EqualFold(filter.Field, "ACCOUNT.NAME") {
			return "", unsupportedCallError("StandardSetController list-view filter field " + filter.Field)
		}
		value := strings.NewReplacer("\\", "\\\\", "'", "\\'", "%", "\\%", "_", "\\_").Replace(filter.Value)
		criteria = append(criteria, "Name LIKE '"+value+"%'")
	}
	if len(criteria) != 0 {
		query += " WHERE " + strings.Join(criteria, " AND ")
	}
	if len(metadata.Columns) != 0 {
		field, err := standardSetListViewColumnField(metadata.Columns[0])
		if err != nil {
			return "", err
		}
		query += " ORDER BY " + field
	}
	return query, nil
}

func (vm *VM) standardSetDefaultFilter(receiver Value) Value {
	options := vm.standardSetListViewOptions(receiver)
	if len(options.List) == 0 {
		return Null
	}
	return options.List[0].Fields["value"]
}

func (vm *VM) setStandardSetFilter(receiver Value, filter Value, result *Result) (Value, Value, bool, bool, error) {
	// Caller-provided filter setters have no captured controls yet.
	if receiver.Fields["__glade_caller_provided"].Bool {
		receiver.Fields["filterId"] = filter
		return Null, receiver, true, true, nil
	}
	if filter.Kind != ValueNull {
		if prior, ok := receiver.Fields["filterId"]; ok && prior.Kind == filter.Kind && prior.Text == filter.Text {
			return Null, receiver, false, true, nil
		}
	} else {
		filter = vm.standardSetDefaultFilter(receiver)
	}
	// The new identity is observable even when dirty rows prevent reloading.
	receiver.Fields["filterId"] = filter
	records := standardSetCopyList(receiver.Fields["records"])
	for i, record := range records.List {
		records.List[i] = vm.mergeSObjectAliasFields(record)
	}
	if standardSetRecordSnapshot(records) != receiver.Fields["__glade_record_snapshot"].Text {
		return Null, receiver, true, true, newExceptionError("VisualforceException", "Modified rows exist in the records collection!")
	}
	// Empty and wrong-object filter IDs expose this fatal message at the
	// native page boundary, without an observable Apex exception type.
	if filter.Kind == ValueString {
		objectType := receiver.Fields["__glade_record_type"].Text
		if filter.Text == "" {
			return Null, receiver, true, true, fmt.Errorf("Filter Id value is not valid for the %s standard controller", objectType)
		}
		if storage.ValidateID(storage.ID(filter.Text)) == nil && !strings.HasPrefix(filter.Text, storage.StandardKeyPrefix("ListView")) {
			return Null, receiver, true, true, fmt.Errorf("Filter Id value %s is not valid for the %s standard controller", filter.Text, objectType)
		}
	}
	if vm.Org == nil {
		return Null, receiver, true, true, fmt.Errorf("StandardSetController list-view metadata unavailable")
	}
	view, exists := vm.Org.Objects["ListView"].Records[storage.ID(filter.Text)]
	if !exists {
		return Null, receiver, true, true, fmt.Errorf("StandardSetController list-view metadata unavailable")
	}
	query, err := standardSetListViewQuery(view, records, receiver.Fields["__glade_page_projection"])
	if err != nil {
		return Null, receiver, true, true, err
	}
	var loaded Value
	if receiver.Fields["__glade_native_page"].Bool {
		// Preserve the USER_MODE authorization used for the initial native
		// page read when a list-view filter reloads its records.
		accessLevel := Object("AccessLevel")
		accessLevel.Text = "USER_MODE"
		loaded, err = vm.executeSOQLWithAccessLevel(query, accessLevel, result)
	} else {
		loaded, err = vm.executeSOQL(query, result)
	}
	if err != nil {
		return Null, receiver, true, true, err
	}
	receiver.Fields["records"] = loaded
	receiver.Fields["__glade_record_snapshot"] = String(standardSetRecordSnapshot(loaded))
	receiver.Fields["selected"] = List()
	receiver.Fields["pageNumber"] = Int(1)
	return Null, receiver, true, true, nil
}

func standardSetCopyList(value Value) Value {
	value.Ref = newValueRef()
	value.List = append([]Value{}, value.List...)
	return value
}

func (vm *VM) standardSetListViewOptions(receiver Value) Value {
	options := typedList("List<SelectOption>")
	if vm.Org == nil {
		return options
	}
	objectType := receiver.Fields["__glade_record_type"].Text
	views := vm.Org.Objects["ListView"]
	ids := make([]string, 0, len(views.Records))
	for id, record := range views.Records {
		kind, ok := record.GetField("SobjectType")
		if ok && objectType != "" && strings.EqualFold(storageValueText(kind), objectType) {
			ids = append(ids, string(id))
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		label, _ := views.Records[storage.ID(id)].GetField("Name")
		options.List = append(options.List, newSelectOption(String(id), String(storageValueText(label)), Bool(false), Bool(true)))
	}
	return options
}

func (vm *VM) saveStandardSetController(receiver Value, result *Result) (Value, Value, bool, bool, error) {
	records := receiver.Fields["records"]
	if records.Kind != ValueList {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.save requires records")
	}
	// Native save does not assign insert IDs to the caller's SObjects. Keep
	// their current field values while detaching the DML write-back identities.
	current := standardSetCopyList(records)
	for i, record := range current.List {
		current.List[i] = vm.mergeSObjectAliasFields(record)
	}
	saved := cloneValue(current)
	results, err := vm.applyDML("upsert", saved, true, "", dml.Options{}, result)
	if err != nil {
		return Null, receiver, false, true, err
	}
	if hasDMLFailures(results) {
		for _, failure := range results {
			if failure.Success {
				continue
			}
			message := Object("ApexPages.Message")
			severity, _ := apexPagesSeverityStaticValue("ApexPages.Severity.ERROR")
			message.Fields["severity"] = severity
			message.Fields["summary"] = String(failure.Error)
			message.Fields["detail"] = String(failure.Error)
			vm.addApexPageMessage(message)
		}
		return Null, receiver, false, true, nil
	}
	page := newPageReference("/home/home.jsp")
	if len(records.List) != 0 && records.List[0].Kind == ValueObject && sObjectIDFromFields(records.List[0].Fields) == "" {
		// New-record save navigates to the created record; persisted and empty
		// controllers retain the captured home navigation contract.
		page = standardControllerPage(saved.List[0])
	}
	page.Fields["redirect"] = Bool(true)
	return page, receiver, false, true, nil
}
