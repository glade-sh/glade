package sema

import (
	"strings"
)

// semaAnonymousMissingCustomFieldTokenPath reports the native message for an
// execute-anonymous field token path whose custom field the object does not
// have in the anonymous view. glade exec removes custom fields the running
// administrator has no grant on (WithoutAnonymousHiddenFields), so a missing
// field here is either absent or hidden. Native qualified_receivers
// compile.tsv (anonymous, Flag__c deployed with no field grant, API 62.0 and
// 67.0):
//
//	Meadow__c.fields.Flag__c.getDescribe()        Variable does not exist: Meadow__c.fields.Flag__c
//	SObjectType.Meadow__c.fields.Flag__c.getName() Variable does not exist: Flag__c
//
// The same expressions compile in classes and test classes (isTest rows), and
// the Schema-qualified and SObjectType-token spellings already resolve the
// field, so only these two anonymous spellings on a custom object are covered.
func semaAnonymousMissingCustomFieldTokenPath(a *Analyzer, typeName, receiverExpr string, scope irSemaScope, model *semaTypeMemberView) (string, bool) {
	if typeName != "__GladeAnonymous" {
		return "", false
	}
	parts := strings.Split(strings.TrimSpace(receiverExpr), ".")
	if _, scoped := scope.lookup(parts[0]); scoped || semaProjectTypeShadowsPlatform(model, parts[0]) {
		return "", false
	}
	object, field, message := "", "", ""
	switch {
	case len(parts) == 3 && strings.EqualFold(parts[1], "fields"):
		object, field, message = parts[0], parts[2], "Variable does not exist: "+receiverExpr
	case len(parts) == 4 && strings.EqualFold(parts[0], "SObjectType") && strings.EqualFold(parts[2], "fields"):
		object, field, message = parts[1], parts[3], "Variable does not exist: "+parts[3]
	default:
		return "", false
	}
	if !semaIsCustomAPIName(object) || !strings.HasSuffix(strings.ToLower(object), "__c") || !semaIsCustomAPIName(field) ||
		semaQualifierNamesKnownNamespace(a, object) || semaProjectTypeShadowsPlatform(model, object) ||
		!semaSObjectExistsInModel(object, model) || !semaSObjectIdentityEstablished(a, object, model) {
		return "", false
	}
	members, _, known := semaLookupTypeMembers(model, object)
	if !known || !members.sobject || members.partialSObject {
		return "", false
	}
	if _, ok := semaResolveField(model, object, field, make(map[string]bool)); ok {
		return "", false
	}
	return message, true
}
