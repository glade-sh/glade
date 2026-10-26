package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Native duplicate enforcement rows (testdata/conformance/duplicate_enforcement)
// pin the request boundaries below at API 62.0 and 67.0.
const (
	findDuplicatesEmptyMessage     = "The list of records to check is null or empty."
	findDuplicatesBatchMessage     = "The number of records to check is greater than the permitted batch size."
	findDuplicatesNullRowMessage   = "Cannot invoke \"common.apex.runtime.SObjectRow.getEntityInfo()\" because \"sObject\" is null"
	findDuplicatesMixedTypeMessage = "The list of records to check contains more than one type of record."
	findDuplicatesBatchSize        = 50
)

// Populated match-record fields observed natively: a save error's record
// (ST07/ST08) and a find result's record (ST02-ST05, ST11) differ.
var duplicateMatchRecordFields = map[string]map[bool][]string{
	"account": {
		false: {"BillingCity", "BillingCountry", "BillingPostalCode", "BillingState", "BillingStreet", "Id", "LastModifiedDate", "Name", "OwnerId", "Phone", "Website"},
		true:  {"BillingAddress", "Id", "Name", "Owner", "Phone", "Website"},
	},
	"contact": {
		false: {"Email", "Id", "LastModifiedDate", "MailingCity", "MailingPostalCode", "MailingStreet", "Name", "OwnerId", "Phone"},
		true:  {"Email", "Id", "Name", "Owner", "Phone"},
	},
	"lead": {
		false: {"City", "Company", "Email", "Id", "LastModifiedDate", "Name", "OwnerId", "Phone", "PostalCode", "Street"},
		true:  {"Company", "Email", "Id", "Name", "Phone"},
	},
}

// databaseDuplicateRuleHeaderAllowSave reads the explicit options' header.
// Per-record setOptions ignore it (native ST07).
func databaseDuplicateRuleHeaderAllowSave(value Value) bool {
	for key, header := range value.Fields {
		if !strings.EqualFold(key, "DuplicateRuleHeader") || header.Kind != ValueObject {
			continue
		}
		for field, setting := range header.Fields {
			if strings.EqualFold(field, "allowSave") && setting.Kind == ValueBool && setting.Bool {
				return true
			}
		}
	}
	return false
}

func duplicateRecordName(record storage.Record) Value {
	if name, ok := record.GetField("Name"); ok && name.Kind != storage.ValueNull && workflowStorageText(name) != "" {
		return vmValueFromStorage(name)
	}
	var parts []string
	for _, field := range []string{"FirstName", "LastName"} {
		if part, ok := record.GetField(field); ok && workflowStorageText(part) != "" {
			parts = append(parts, workflowStorageText(part))
		}
	}
	if len(parts) == 0 {
		return Null
	}
	return String(strings.Join(parts, " "))
}

func workflowStorageText(value storage.Value) string {
	if value.Kind == storage.ValueNull {
		return ""
	}
	return strings.TrimSpace(vmValueFromStorage(value).String())
}

func duplicateMatchRecordValue(record storage.Record, find bool) Value {
	full := vmValueFromRecord(record)
	out := Object(record.Object)
	fields, ok := duplicateMatchRecordFields[strings.ToLower(record.Object)][find]
	if !ok {
		fields = []string{"Id", "Name"}
	}
	put := func(field string, value Value) {
		if value.Kind == ValueNull {
			return
		}
		out.Fields[field] = value
		markExplicitSObjectField(&out, field)
	}
	for _, field := range fields {
		switch field {
		case "Name":
			put(field, duplicateRecordName(record))
		case "Owner":
			if owner := record.System.OwnerID; owner != "" {
				user := Object("User")
				user.Fields["Id"] = platformScalar("Id", string(owner))
				put(field, user)
			}
		case "BillingAddress":
			address := Object("Address")
			for component, source := range map[string]string{"street": "BillingStreet", "city": "BillingCity", "state": "BillingState", "postalCode": "BillingPostalCode", "country": "BillingCountry"} {
				if _, value, ok := objectFieldValue(full, source); ok && value.Kind != ValueNull {
					address.Fields[component] = value
				}
			}
			if len(address.Fields) > 0 {
				put(field, address)
			}
		default:
			if _, value, ok := objectFieldValue(full, field); ok {
				put(field, value)
			}
		}
	}
	return out
}

func datacloudDuplicateResultValue(result dml.DuplicateResult, find bool) Value {
	value := Object("Datacloud.DuplicateResult")
	value.Fields["duplicateRule"] = String(result.Rule)
	value.Fields["duplicateRuleEntityType"] = String(result.Object)
	value.Fields["allowSave"] = Bool(result.AllowSave)
	value.Fields["errorMessage"] = Null
	if result.Message != "" {
		value.Fields["errorMessage"] = String(result.Message)
	}
	matches := typedList("List<Datacloud.MatchResult>")
	for _, match := range result.Matches {
		item := Object("Datacloud.MatchResult")
		item.Fields["entityType"] = String(match.Object)
		item.Fields["rule"] = String(match.Rule)
		item.Fields["matchEngine"] = String(match.Engine)
		item.Fields["size"] = Int(int64(len(match.Records)))
		item.Fields["success"] = Bool(true)
		item.Fields["errors"] = typedList("List<Database.Error>")
		records := typedList("List<Datacloud.MatchRecord>")
		for _, record := range match.Records {
			matchRecord := Object("Datacloud.MatchRecord")
			matchRecord.Fields["record"] = duplicateMatchRecordValue(record, find)
			matchRecord.Fields["matchConfidence"] = decimalAsDouble(Decimal(100))
			matchRecord.Fields["additionalInformation"] = typedList("List<Datacloud.AdditionalInformationMap>")
			matchRecord.Fields["fieldDiffs"] = typedList("List<Datacloud.FieldDiff>")
			records.List = append(records.List, matchRecord)
		}
		item.Fields["matchRecords"] = records
		matches.List = append(matches.List, item)
	}
	value.Fields["matchResults"] = matches
	return value
}

func databaseDuplicateErrorValue(err dml.Error) Value {
	value := Object("Database.DuplicateError")
	value.Fields["message"] = String(err.Message)
	value.Fields["statusCode"] = String(err.StatusCode)
	value.Fields["fields"] = List()
	value.Fields["duplicateresult"] = datacloudDuplicateResultValue(*err.Duplicate, false)
	return value
}

func callDatacloudDuplicateMember(_ *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
	members := map[string]map[string]string{
		"datacloud.duplicateresult": {"getduplicaterule": "duplicateRule", "getduplicateruleentitytype": "duplicateRuleEntityType", "geterrormessage": "errorMessage", "getmatchresults": "matchResults", "isallowsave": "allowSave"},
		"datacloud.matchresult":     {"getentitytype": "entityType", "geterrors": "errors", "getmatchengine": "matchEngine", "getmatchrecords": "matchRecords", "getrule": "rule", "getsize": "size", "issuccess": "success"},
		"datacloud.matchrecord":     {"getadditionalinformation": "additionalInformation", "getfielddiffs": "fieldDiffs", "getmatchconfidence": "matchConfidence", "getrecord": "record"},
	}
	fields, ok := members[strings.ToLower(receiver.Type)]
	if !ok {
		return Null, receiver, false, false, nil
	}
	field, ok := fields[strings.ToLower(method)]
	if !ok {
		return Null, receiver, false, false, nil
	}
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	value, ok := receiver.Fields[field]
	if !ok {
		return Null, receiver, false, true, nil
	}
	return value, receiver, false, true, nil
}

func (vm *VM) duplicateResultsForRecord(record storage.Record, object string) []Value {
	var out []Value
	for _, rule := range dml.ActiveDuplicateRules(vm.Org, object) {
		applies, supported := dml.DuplicateRuleApplies(vm.Org, rule, record)
		if !supported || !applies {
			continue
		}
		found, supported := dml.FindDuplicateMatches(vm.Org, rule, record, nil)
		if !supported {
			continue
		}
		// ST02/DR14: a match carries the rule's save decision and alert text;
		// no match reports allowSave false and a null message.
		if found.HasRecords() {
			found.AllowSave = strings.EqualFold(rule.ActionOnInsert, "Allow")
			found.Message = rule.AlertText
		}
		out = append(out, datacloudDuplicateResultValue(found, true))
	}
	return out
}

func newDatacloudFindDuplicatesResultWith(duplicates []Value) Value {
	result := newDatacloudFindDuplicatesResult()
	list := result.Fields["duplicateResults"]
	list.List = append(list.List, duplicates...)
	result.Fields["duplicateResults"] = list
	return result
}

// findDuplicates evaluates every row with the first row's object rules
// (native ST05 evaluates a mixed list against the leading Lead rules).
func (vm *VM) findDuplicates(args []Value) (Value, error) {
	if len(args) == 1 && (args[0].Kind == ValueNull || args[0].Kind == ValueList && len(args[0].List) == 0) {
		return Null, newExceptionError("System.HandledException", findDuplicatesEmptyMessage)
	}
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("Datacloud.FindDuplicates.findDuplicates expects List<SObject>")
	}
	if len(args[0].List) > findDuplicatesBatchSize {
		return Null, newExceptionError("System.HandledException", findDuplicatesBatchMessage)
	}
	object := ""
	records := make([]storage.Record, 0, len(args[0].List))
	for _, row := range args[0].List {
		if row.Kind == ValueNull {
			return Null, newExceptionError("System.HandledException", findDuplicatesNullRowMessage)
		}
		if row.Kind != ValueObject || !vm.isSObjectLikeType(row.Type) {
			return Null, fmt.Errorf("Datacloud.FindDuplicates.findDuplicates expects List<SObject>")
		}
		record := storage.Record{Object: row.Type}
		if vm.Org != nil {
			copied := row
			converted, err := vm.recordFromValue(&copied)
			if err != nil {
				return Null, err
			}
			record = converted
		}
		if object == "" {
			object = record.Object
		}
		records = append(records, record)
	}
	results := make([]Value, 0, len(records))
	for _, record := range records {
		results = append(results, newDatacloudFindDuplicatesResultWith(vm.duplicateResultsForRecord(record, object)))
	}
	return List(results...), nil
}

func (vm *VM) findDuplicatesByIDs(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("Datacloud.FindDuplicatesByIds.findDuplicatesByIds expects List<Id>")
	}
	type stored struct {
		record storage.Record
		found  bool
	}
	rows := make([]stored, 0, len(args[0].List))
	object := ""
	for _, raw := range args[0].List {
		text, ok := idValueText(raw)
		if !ok {
			return Null, fmt.Errorf("Datacloud.FindDuplicatesByIds.findDuplicatesByIds expects List<Id>")
		}
		var row stored
		if vm.Org != nil {
			if name, ok := vm.sObjectNameForID(text); ok {
				if record, ok := vm.findOrgRecord(name, storage.ID(text)); ok && !record.System.IsDeleted {
					row = stored{record: record, found: true}
					// ST05: byIds rejects a list spanning record types.
					if object != "" && !strings.EqualFold(object, name) {
						return Null, newExceptionError("System.HandledException", findDuplicatesMixedTypeMessage)
					}
					object = name
				}
			}
		}
		rows = append(rows, row)
	}
	results := make([]Value, 0, len(rows))
	for _, row := range rows {
		var duplicates []Value
		if row.found {
			duplicates = vm.duplicateResultsForRecord(row.record, object)
		}
		results = append(results, newDatacloudFindDuplicatesResultWith(duplicates))
	}
	return List(results...), nil
}

// queueDuplicateReports keeps Allow+Report saves until the transaction commits.
func (vm *VM) queueDuplicateReports(reports []dml.DuplicateReport) {
	if vm != nil && len(reports) > 0 {
		vm.duplicateReports = append(vm.duplicateReports, reports...)
	}
}

// beginDuplicateReportTransaction scopes pending reports to one execution.
// Native CM01: report rows are absent inside the saving transaction and
// appear after it commits; a test context never commits them (ST09).
func (vm *VM) beginDuplicateReportTransaction() func(committed bool) {
	previous := vm.duplicateReports
	vm.duplicateReports = nil
	return func(committed bool) {
		reports := vm.duplicateReports
		vm.duplicateReports = previous
		if committed && vm.testContext == nil {
			vm.commitDuplicateReports(reports)
		}
	}
}

func (vm *VM) commitDuplicateReports(reports []dml.DuplicateReport) {
	if vm.Org == nil || len(reports) == 0 {
		return
	}
	live := func(object string, id storage.ID) bool {
		name, ok := vm.resolveObjectName(object)
		if !ok {
			return false
		}
		record, ok := vm.findOrgRecord(name, id)
		return ok && !record.System.IsDeleted
	}
	for _, name := range []string{"DuplicateRecordSet", "DuplicateRecordItem"} {
		storage.EnsureStandardObject(vm.Org, name)
	}
	engine := vm.newDMLEngine(nil)
	for _, report := range reports {
		if !live(report.Object, report.RecordID) {
			continue
		}
		ruleID := report.Rule.ID
		if ruleID == "" {
			for id, row := range vm.Org.Objects["DuplicateRule"].Records {
				if !row.System.IsDeleted && strings.EqualFold(row.Fields["DeveloperName"].String, report.Rule.Name) && strings.EqualFold(row.Fields["SobjectType"].String, report.Rule.Object) {
					ruleID = id
				}
			}
		}
		items := []storage.Record{{Object: "DuplicateRecordItem", Fields: map[string]storage.Value{"RecordId": storage.IDValue(report.RecordID)}}}
		for _, match := range report.Matches {
			if live(match.Object, match.ID) {
				items = append(items, storage.Record{Object: "DuplicateRecordItem", Fields: map[string]storage.Value{"RecordId": storage.IDValue(match.ID)}})
			}
		}
		if len(items) < 2 {
			continue
		}
		set := storage.Record{Object: "DuplicateRecordSet", Fields: map[string]storage.Value{}}
		if ruleID != "" {
			set.Fields["DuplicateRuleId"] = storage.IDValue(ruleID)
		}
		sets := engine.Insert([]storage.Record{set})
		if len(sets) != 1 || !sets[0].Success {
			continue
		}
		for i := range items {
			items[i].Fields["DuplicateRecordSetId"] = storage.IDValue(sets[0].ID)
		}
		engine.Insert(items)
	}
	vm.rebuildDMLObjectIndexes([]storage.Record{{Object: "DuplicateRecordSet"}, {Object: "DuplicateRecordItem"}}, nil)
}
