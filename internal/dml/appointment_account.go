package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) applyAppointmentAccountDefault(record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(record.Object, "ServiceAppointment") {
		return
	}
	if account, present := record.GetField("AccountId"); present && account.Kind != storage.ValueNull {
		return
	}
	parentID := idFromStorageValue(record.Fields["ParentRecordId"])
	_, parent, found := storage.LookupRecordByID(e.Org.Objects["WorkOrder"].Records, parentID)
	if !found {
		return
	}
	if account, present := parent.GetField("AccountId"); present {
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		record.Fields["AccountId"] = account.Clone()
	}
}
