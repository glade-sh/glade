package storage

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"strings"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/project"
)

// ApplyEmailServices seeds setup records atomically. Generated domains preserve
// the native shape and stable local identity; email delivery is a hosted service.
func ApplyEmailServices(org *OrgState, services []project.EmailService) error {
	if org == nil || len(services) == 0 {
		return nil
	}
	staged := *org
	staged.Objects = maps.Clone(org.Objects)
	staged.IDSequences = maps.Clone(org.IDSequences)
	if staged.Objects == nil {
		staged.Objects = make(map[string]ObjectState)
	}
	if staged.IDSequences == nil {
		staged.IDSequences = make(map[string]uint64)
	}
	for _, name := range []string{"ApexClass", "EmailServicesFunction", "EmailServicesAddress"} {
		if state, ok := staged.Objects[name]; ok {
			staged.Objects[name] = state.Clone()
		}
		EnsureStandardObject(&staged, name)
	}
	generator := NewStandardIDGenerator()
	generator.Sequences = staged.IDSequences
	for _, service := range services {
		classID := emailServiceReference(&staged, "ApexClass", "Name", service.ApexClass)
		if classID == "" && service.HandlerDeclared {
			var err error
			classID, err = upsertEmailServiceRecord(&staged, &generator, "ApexClass", "Name", map[string]Value{"Name": StringValue(service.ApexClass)})
			if err != nil {
				return err
			}
		}
		if classID == "" {
			return fmt.Errorf("email service %s: an Apex Class must be specified", service.FunctionName)
		}
		fields := map[string]Value{
			"FunctionName": StringValue(service.FunctionName), "ApexClassId": IDValue(classID),
			"AttachmentOption":            StringValue(service.AttachmentOption),
			"AuthenticationFailureAction": StringValue(service.AuthenticationFailureAction),
			"AuthorizationFailureAction":  StringValue(service.AuthorizationFailureAction),
			"AddressInactiveAction":       StringValue("Discard"),
			"FunctionInactiveAction":      StringValue(service.FunctionInactiveAction),
			"OverLimitAction":             StringValue(service.OverLimitAction),
			"AuthorizedSenders":           emailServiceOptionalString(service.AuthorizedSenders),
			"ErrorRoutingAddress":         emailServiceOptionalString(service.ErrorRoutingAddress),
			"IsActive":                    BooleanValue(service.IsActive),
			"IsAuthenticationRequired":    BooleanValue(service.IsAuthenticationRequired),
			"IsErrorRoutingEnabled":       BooleanValue(service.IsErrorRoutingEnabled),
			"IsTextAttachmentsAsBinary":   BooleanValue(service.IsTextAttachmentsAsBinary),
			"IsTlsRequired":               BooleanValue(service.IsTlsRequired),
		}
		functionID, err := upsertEmailServiceRecord(&staged, &generator, "EmailServicesFunction", "FunctionName", fields)
		if err != nil {
			return err
		}
		for _, address := range service.Addresses {
			if utf8.RuneCountInString(address.LocalPart) > 64 {
				return fmt.Errorf("email service %s: LocalPart maximum length is:64", service.FunctionName)
			}
			userID := emailServiceReference(&staged, "User", "Username", address.RunAsUser)
			if userID == "" {
				return fmt.Errorf("email service %s: no User named %s found", service.FunctionName, address.RunAsUser)
			}
			digest := sha256.Sum256([]byte(string(functionID) + "/" + address.DeveloperName))
			domain := fmt.Sprintf("local%x.local000000.local00.apex.local00.salesforce.com", digest[:24])
			fields := map[string]Value{
				"DeveloperName": StringValue(address.DeveloperName), "LocalPart": StringValue(address.LocalPart),
				"EmailDomainName": StringValue(domain), "FunctionId": IDValue(functionID), "RunAsUserId": IDValue(userID),
				"IsActive": BooleanValue(address.IsActive), "AuthorizedSenders": emailServiceOptionalString(address.AuthorizedSenders),
			}
			if _, err := upsertEmailServiceRecord(&staged, &generator, "EmailServicesAddress", "DeveloperName", fields); err != nil {
				return err
			}
		}
	}
	if org.Objects == nil {
		org.Objects = make(map[string]ObjectState)
	}
	for _, name := range []string{"ApexClass", "EmailServicesFunction", "EmailServicesAddress"} {
		org.Objects[name] = staged.Objects[name]
		RebuildObjectIndexes(org, name)
	}
	org.IDSequences = generator.Sequences
	org.ClearRuntimeSchemaStamp()
	return nil
}

func emailServiceReference(org *OrgState, object, field, value string) ID {
	var selected ID
	for id, record := range org.Objects[object].Records {
		if !record.System.IsDeleted && strings.EqualFold(record.Fields[field].String, value) && (selected == "" || id < selected) {
			selected = id
		}
	}
	return selected
}

func emailServiceOptionalString(value string) Value {
	if value == "" {
		return NullValue()
	}
	return StringValue(value)
}

func upsertEmailServiceRecord(org *OrgState, generator *IDGenerator, object, key string, fields map[string]Value) (ID, error) {
	state := org.Objects[object]
	id := ID("")
	for candidate, record := range state.Records {
		if record.System.IsDeleted || !strings.EqualFold(record.Fields[key].String, fields[key].String) {
			continue
		}
		if object == "EmailServicesAddress" && !IDsEqual(record.Fields["FunctionId"].ID, fields["FunctionId"].ID) {
			continue
		}
		if id == "" || candidate < id {
			id = candidate
		}
	}
	if id == "" {
		for {
			var err error
			id, err = generator.Next(object)
			if err != nil {
				return "", err
			}
			if _, _, exists := LookupRecordByID(state.Records, id); !exists {
				break
			}
		}
	}
	record := state.Records[id].Clone()
	record.ID = id
	record.Object = object
	if record.Fields == nil {
		record.Fields = make(map[string]Value)
	}
	for name, value := range fields {
		record.Fields[name] = value
	}
	record.Fields["Id"] = IDValue(id)
	stampSeedSystemFields(&record)
	if state.Records == nil {
		state.Records = make(map[ID]Record)
	}
	state.Records[id] = record
	org.Objects[object] = state
	return id, nil
}
