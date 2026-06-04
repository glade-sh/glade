package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type objectDMLPolicy struct {
	beforeDMLDerivedFields    func(*VM, *storage.Record)
	storedDMLDerivedFields    func(*VM, *storage.Record)
	transientDMLDerivedFields []string
	defaultRecordTypeID       func(storage.ObjectDefinition, storage.Record) storage.ID
}

func dmlPolicyForObject(objectName string) *objectDMLPolicy {
	switch {
	case strings.EqualFold(objectName, "Opportunity"):
		return &opportunityDMLPolicy
	case strings.EqualFold(objectName, "Account"):
		return &accountDMLPolicy
	default:
		return nil
	}
}
