package storage

import "testing"

// The standard-parent route must obey the same field-resolution invariant as
// the custom-parent route, for every edge of each supported parent.
func TestStandardParentPlatformChildEdgesResolveOnChildDefinitions(t *testing.T) {
	for _, parent := range []string{"Account", "Case", "Contact", "Opportunity", "User", "Lead", "Product2"} {
		t.Run(parent, func(t *testing.T) {
			checkPlatformChildEdges(t, ObjectDefinition{APIName: parent})
		})
	}
}

func TestStandardParentPlatformChildFieldsRepairPersistedDefinition(t *testing.T) {
	for child, fields := range map[string][]string{
		"FinanceBalanceSnapshot": {"AccountId"},
		"ExtlRecShrStagedRecord": {"AccountId", "ParentRecordId"},
		"FinanceTransaction":     {"AccountId"},
		"MessagingEndUser":       {"AccountId", "ContactId", "AuthenticatedEndUserId"},
		"MessagingSession":       {"CaseId", "OpportunityId"},
		"Order":                  {"OpportunityId"},
	} {
		t.Run(child, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, child)
			definition := org.Objects[child].Definition.Clone()
			for _, field := range fields {
				delete(definition.Fields, field)
			}
			if !StandardObjectFieldsNeedWrite(definition) {
				t.Fatal("persisted definition missing standard-parent edge fields is not reported for repair")
			}
			EnsureStandardObjectFields(&definition)
			for _, field := range fields {
				if _, ok := ResolveFieldName(definition, "", field); !ok {
					t.Errorf("persisted %s.%s was not repaired", child, field)
				}
			}
			if StandardObjectFieldsNeedWrite(definition) {
				t.Fatal("persisted repair is not idempotent")
			}
		})
	}
}
