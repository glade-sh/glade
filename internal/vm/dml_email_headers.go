package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// The captured options overload covers each flag and their combinations.
func (vm *VM) caseLeadEmailHeaderInsert(records, options Value) (dml.Options, bool, error) {
	if vm == nil || vm.Org == nil {
		return dml.Options{}, false, nil
	}
	items := []Value{records}
	if records.Kind == ValueList {
		items = records.List
	}
	if len(items) == 0 {
		return dml.Options{}, false, nil
	}
	for _, item := range items {
		name := vm.canonicalSObjectValueType(item)
		if item.Kind != ValueObject || (!strings.EqualFold(name, "Case") && !strings.EqualFold(name, "Lead")) {
			return dml.Options{}, false, nil
		}
	}
	flags := map[string]bool{}
	for key, header := range options.Fields {
		if !strings.EqualFold(key, "EmailHeader") || header.Kind == ValueNull {
			continue
		}
		if header.Kind != ValueObject {
			return dml.Options{}, false, nil
		}
		for name, v := range header.Fields {
			if v.Kind == ValueNull {
				continue
			}
			key := strings.ToLower(name)
			if v.Kind != ValueBool || (key != "triggeruseremail" && key != "triggerotheremail" && key != "triggerautoresponseemail") {
				return dml.Options{}, false, nil
			}
			if old, ok := flags[key]; ok && old != v.Bool {
				return dml.Options{}, false, nil
			}
			flags[key] = v.Bool
		}
	}
	if len(flags) == 0 {
		return dml.Options{}, false, nil
	}

	clean := cloneDatabaseOptionsObject(options)
	for key := range clean.Fields {
		if strings.EqualFold(key, "EmailHeader") {
			clean.Fields[key] = Null
		}
	}
	result, err := databaseDMLOptions(clean)
	if err != nil {
		return result, true, err
	}
	result.CaptureAutoResponseEmail = flags["triggerautoresponseemail"]
	result.CaseContactEmail = flags["triggerotheremail"]
	return result, true, nil
}

// perRecordEmailOptions only consumes the insert routes observed on the org.
func (vm *VM) perRecordEmailOptions(record, value Value) (dml.Options, bool, error) {
	if vm.accountEmailHeaderInsert(record, value) {
		clean := cloneDatabaseOptionsObject(value)
		for key := range clean.Fields {
			if strings.EqualFold(key, "EmailHeader") {
				clean.Fields[key] = Null
			}
		}
		options, err := databaseDMLOptions(clean)
		return options, true, err
	}
	return vm.caseLeadEmailHeaderInsert(record, value)
}

func (vm *VM) perRecordHeaderOptions(targets []*Value, i int) (dml.Options, bool, error) {
	if i >= len(targets) || targets[i] == nil || targets[i].Kind != ValueObject {
		return dml.Options{}, false, nil
	}
	value, ok := targets[i].Fields[sobjectDMLOptionsField]
	if !ok || !isDatabaseDMLOptionsValue(value) || !databaseDMLOptionConfigured(value, "EmailHeader", true) {
		return dml.Options{}, false, nil
	}
	return vm.perRecordEmailOptions(*targets[i], value)
}

func (vm *VM) caseContactEmailFailures(records []storage.Record, targets []*Value, global bool) []dml.Result {
	var failures []dml.Result
	for i, record := range records {
		if !global {
			options, accepted, _ := vm.perRecordHeaderOptions(targets, i)
			if !accepted || !options.CaseContactEmail {
				continue
			}
		}
		if !strings.EqualFold(record.Object, "Case") {
			continue
		}
		contact := storageStringField(record, "ContactId")
		message := ""
		if contact == "" {
			message = "No selected contact"
		} else if _, c, ok := storage.LookupRecordByID(vm.Org.Objects["Contact"].Records, storage.ID(contact)); ok && storageStringField(c, "Email") == "" {
			message = "Contact has an invalid or missing email address"
		}
		if message == "" {
			continue
		}
		if vm.testContext == nil {
			message += ": Contact ID"
		}
		if failures == nil {
			failures = make([]dml.Result, len(records))
			for j := range failures {
				failures[j].Success = true
			}
		}
		failures[i] = dmlFailure(record.ID, message, "FIELD_INTEGRITY_EXCEPTION", []string{"ContactId"})
	}
	return failures
}

func (vm *VM) applyAutoResponseActivities(engine *dml.Engine, records []storage.Record, targets []*Value, results []dml.Result, global bool, result *Result) error {
	for i, input := range records {
		if !global {
			options, accepted, _ := vm.perRecordHeaderOptions(targets, i)
			if !accepted || !options.CaptureAutoResponseEmail {
				continue
			}
		}
		if i >= len(results) || !results[i].Success {
			continue
		}
		input.ID = results[i].ID
		subject, matched, err := engine.AutoResponseSubject(input)
		if err != nil {
			return err
		}
		if !matched {
			continue
		}
		_, stored, ok := storage.LookupRecordByID(vm.Org.Objects[input.Object].Records, input.ID)
		if !ok {
			return fmt.Errorf("auto-response record unavailable")
		}
		if strings.EqualFold(input.Object, "Lead") && storageStringField(stored, "Email") == "" {
			continue
		}
		if strings.EqualFold(input.Object, "Case") && storageStringField(stored, "SuppliedEmail") == "" {
			contactID := storageStringField(stored, "ContactId")
			_, contact, ok := storage.LookupRecordByID(vm.Org.Objects["Contact"].Records, storage.ID(contactID))
			if !ok || storageStringField(contact, "Email") == "" {
				continue
			}
		}
		storage.EnsureStandardObject(vm.Org, "Task")
		whoID, whatID := Null, Null
		if strings.EqualFold(input.Object, "Lead") {
			whoID = String(string(input.ID))
		} else {
			whatID = String(string(input.ID))
			if contactID := storageStringField(stored, "ContactId"); contactID != "" {
				whoID = String(contactID)
			}
		}
		subject = vm.renderEmailTemplateText(subject, whoID, whatID)
		fields := map[string]storage.Value{"Subject": storage.StringValue("Email: " + subject), "Status": storage.StringValue("Completed"), "Priority": storage.StringValue("Normal"), "TaskSubtype": storage.StringValue("Email"), "ActivityDate": storage.DateValue(vm.fakeNow.Format("2006-01-02"))}
		if whoID.Kind == ValueString {
			fields["WhoId"] = storage.IDValue(storage.ID(whoID.Text))
		}
		if whatID.Kind == ValueString {
			fields["WhatId"] = storage.IDValue(storage.ID(whatID.Text))
		}
		taskEngine := vm.newDMLEngine(result)
		saved := taskEngine.Insert([]storage.Record{{Object: "Task", Fields: fields}})
		if len(saved) != 1 || !saved[0].Success {
			return fmt.Errorf("auto-response activity could not be saved")
		}
		object := vm.Org.Objects["Task"]
		if id, task, ok := storage.LookupRecordByID(object.Records, saved[0].ID); ok {
			task.Fields["IsClosed"] = storage.BooleanValue(true)
			object.PutRecord(id, task)
			vm.Org.Objects["Task"] = object
		}
	}
	return nil
}
