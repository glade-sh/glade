package sema

import "strings"

// The captured rows close the captured direct field-token forms against actual
// object metadata, before source-reference enrichment can invent the field.
// Partial, external-package, value-bound and chained token forms retain their
// existing policy. Ordinary objects are covered only for SetupOwnerId here.
func (a *Analyzer) customSettingTokenMessage(name string, scope irSemaScope, model *semaTypeMemberView) string {
	parts := strings.Split(name, ".")
	if len(parts) != 2 || strings.EqualFold(parts[1], "class") || !semaLooksLikeSObjectFieldTokenInModel(name, model) {
		return ""
	}
	if _, bound := scope.lookup(parts[0]); bound {
		return ""
	}
	members, _, ok := semaLookupTypeMembers(model, parts[0])
	if !ok || !members.sobject || members.dependency || members.partialSObject || members.externalPackageSObject {
		return ""
	}
	for _, object := range a.queryDeclaredObjects {
		if !strings.EqualFold(object.Name, parts[0]) || object.Partial || semaNamespaceFromAPIName(object.Name) != "" {
			continue
		}
		listCustomField := strings.EqualFold(object.CustomSettingsType, "List") && semaHasAPISuffixFold(parts[1], "__c")
		ordinaryOwnerField := object.CustomSettingsType == "" && strings.EqualFold(parts[1], "SetupOwnerId")
		if !listCustomField && !ordinaryOwnerField {
			return ""
		}
		if _, found := newSemaSObjectFieldProvider(a.namespace, object).lookup(parts[1]); found {
			return ""
		}
		return "Variable does not exist: " + parts[1]
	}
	return ""
}

func (a *Analyzer) listSettingOwnerAssignment(target string, scope irSemaScope, model *semaTypeMemberView, currentType string) bool {
	root, field, qualified := strings.Cut(target, ".")
	if !qualified || !strings.EqualFold(field, "SetupOwnerId") || !scope.hasNonFieldBinding(root) {
		return false
	}
	receiver := semaIRReceiverType(root, scope, model, currentType)
	for _, object := range a.queryDeclaredObjects {
		if strings.EqualFold(object.Name, receiver) && !object.Partial && semaNamespaceFromAPIName(object.Name) == "" && strings.EqualFold(object.CustomSettingsType, "List") {
			return true
		}
	}
	return false
}
