package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// semaRecordTypeFieldAbsent reports a static RecordTypeId or RecordType
// reference on an object that has no record type in a declared org shape.
// Native record_type_shape org.tsv c1, c3, c4 and c6 (API 62.0 and 67.0,
// anonymous): Account, Contact and a custom object without a record type
// reject a.RecordTypeId and a.RecordType with "Variable does not exist", and
// c2 rejects SOQL on RecordTypeId; a custom object with record types, and
// Account once it has one, compile. Callers apply it to the anonymous route
// only: no named-class row exists, and record types can live outside the
// loaded source (record_type_field_presence ORG-RUN.txt).
// This is the static twin of vm.recordTypeFieldAbsent: an unspecified shape,
// an objectSettings defaultRecordType and an object whose schema carries the
// field keep it, and a standard object is closed only in the captured plain
// Developer shape. Dependency and external-package objects stay open.
func (a *Analyzer) semaRecordTypeFieldAbsent(object, field string, model *semaTypeMemberView) bool {
	if a == nil || model == nil || model.state == nil || model.state.base == nil ||
		(!strings.EqualFold(field, "RecordTypeId") && !strings.EqualFold(field, "RecordType")) {
		return false
	}
	members, _, known := semaLookupTypeMembers(model, object)
	if !known || !members.sobject || members.dependency || members.externalPackageSObject || members.name == "" {
		return false
	}
	return semaDeclaredShapeLacksRecordTypeField(model.state.base.orgShapeFeatures, members.name, a.queryDeclaredObjects)
}

// semaDeclaredShapeLacksRecordTypeField applies the record type presence rule
// to one canonical object name against the index's declared objects.
func semaDeclaredShapeLacksRecordTypeField(features []string, object string, declared []schema.Object) bool {
	if len(features) == 0 || object == "" {
		return false
	}
	// A standard object's record types can come with the org shape (Person
	// Accounts, industry features), so only the captured plain Developer shape
	// closes them. A custom object has record types only from its metadata or
	// an objectSettings defaultRecordType.
	needsPlainShape := !strings.HasSuffix(strings.ToLower(object), "__c")
	defaultRecordType := project.ObjectDefaultRecordTypeFeatureFor(object)
	for _, feature := range features {
		if feature == project.UnspecifiedStandardFieldShape || strings.EqualFold(feature, defaultRecordType) {
			return false
		}
		if feature == project.PlainDeveloperObjectShape {
			needsPlainShape = false
		}
	}
	if needsPlainShape {
		return false
	}
	for _, candidate := range declared {
		if !strings.EqualFold(candidate.Name, object) {
			continue
		}
		if hasQuerySchemaField(candidate.Fields, "RecordTypeId") {
			return false
		}
		for _, recordType := range candidate.RecordTypes {
			if !strings.EqualFold(recordType.DeveloperName, "Master") {
				return false
			}
		}
	}
	return true
}
