package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

var opportunityDMLPolicy = objectDMLPolicy{
	beforeDMLDerivedFields:    applyOpportunityStageFlagFields,
	storedDMLDerivedFields:    applyOpportunityStageFlagFields,
	transientDMLDerivedFields: []string{"IsClosed", "IsWon"},
}

func applyOpportunityStageFlagFields(vm *VM, record *storage.Record) {
	if vm == nil || vm.Org == nil || record == nil {
		return
	}
	stageName := storageRecordStringField(*record, "StageName")
	if strings.TrimSpace(stageName) == "" {
		return
	}
	closed, won, ok := vm.opportunityStageFlags(stageName)
	if !ok {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["IsClosed"] = storage.BooleanValue(closed)
	record.Fields["IsWon"] = storage.BooleanValue(won)
}

func (vm *VM) opportunityStageFlags(stageName string) (bool, bool, bool) {
	stageName = strings.TrimSpace(stageName)
	if stageName == "" {
		return false, false, false
	}
	if vm != nil && vm.Org != nil {
		if stages, ok := vm.Org.Objects["OpportunityStage"]; ok {
			for _, record := range stages.Records {
				masterLabel := storageRecordStringField(record, "MasterLabel")
				apiName := storageRecordStringField(record, "ApiName")
				if !strings.EqualFold(stageName, masterLabel) && !strings.EqualFold(stageName, apiName) {
					continue
				}
				return storageRecordBoolField(record, "IsClosed"), storageRecordBoolField(record, "IsWon"), true
			}
		}
	}
	if strings.EqualFold(stageName, "Closed Won") {
		return true, true, true
	}
	if strings.EqualFold(stageName, "Closed Lost") {
		return true, false, true
	}
	return false, false, true
}

func storageRecordBoolField(record storage.Record, field string) bool {
	value, ok := record.GetField(field)
	return ok && value.Kind == storage.ValueBoolean && value.Boolean
}
