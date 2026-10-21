package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type objectDMLPolicy struct {
	beforeDMLDerivedFields    func(*VM, *storage.Record)
	storedDMLDerivedFields    func(*VM, *storage.Record)
	storedInsertDerivedFields func(*VM, *storage.Record) error
	transientDMLDerivedFields []string
	defaultRecordTypeID       func(storage.ObjectDefinition, storage.Record) storage.ID
}

func dmlPolicyForObject(objectName string) *objectDMLPolicy {
	switch {
	case strings.EqualFold(objectName, "Task"):
		return &taskDMLPolicy
	case strings.EqualFold(objectName, "Opportunity"):
		return &opportunityDMLPolicy
	case strings.EqualFold(objectName, "Account"):
		return &accountDMLPolicy
	case strings.EqualFold(objectName, "Knowledge__kav"):
		return &knowledgeArticleDMLPolicy
	default:
		return nil
	}
}

func hasStoredInsertDerivedFields(records []storage.Record) bool {
	for _, record := range records {
		if policy := dmlPolicyForObject(record.Object); policy != nil && policy.storedInsertDerivedFields != nil {
			return true
		}
	}
	return false
}
