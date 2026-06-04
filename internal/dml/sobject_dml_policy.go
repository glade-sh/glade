package dml

import "github.com/glade-sh/glade/internal/storage"

func sobjectInsertNeedsFullRollback(objectName string) bool {
	switch objectName {
	case "ContentVersion":
		return true
	default:
		return false
	}
}

func (e *Engine) afterInsertSObject(objectName string, record storage.Record) error {
	switch objectName {
	case "ContentVersion":
		return e.afterInsertContentVersion(record)
	case "ContentDistribution":
		e.afterInsertContentDistribution(record.ID)
	case "EmailMessage":
		return e.afterInsertEmailMessage(record)
	case "User":
		e.afterInsertUser(record)
	}
	return nil
}

func syncPersonContactAfterUpdate(objectName string) bool {
	return objectName == "Account"
}
