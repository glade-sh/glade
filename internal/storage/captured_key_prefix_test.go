package storage

import (
	"strings"
	"testing"
)

// The shipped describe catalog supplies these public standard-object prefixes.
// Storage must use the same identity even when field shape comes from stubs.
func TestCapturedStandardKeyPrefixPropagation(t *testing.T) {
	for _, name := range []string{"MessagingSession", "MessagingEndUser", "Conversation"} {
		t.Run(name, func(t *testing.T) {
			captured, known := StandardObjectDescribeKeyPrefix(name)
			if !known || captured == nil || len(*captured) != 3 {
				t.Fatal("missing captured stable prefix")
			}
			t.Run("lookup", func(t *testing.T) {
				if got := StandardKeyPrefix(name); got != *captured {
					t.Fatalf("prefix = %q, captured %q", got, *captured)
				}
			})
			t.Run("allocation", func(t *testing.T) {
				if got := AssignDeterministicPrefixes([]string{name, "PrefixWidget__c"}, nil)[name]; got != *captured {
					t.Fatalf("allocated prefix = %q, captured %q", got, *captured)
				}
			})
			t.Run("org", func(t *testing.T) {
				org := NewOrgState()
				EnsureStandardObject(&org, name)
				EnsureUniqueKeyPrefixes(&org)
				if got := org.Objects[name].Definition.KeyPrefix; got != *captured {
					t.Fatalf("org prefix = %q, captured %q", got, *captured)
				}
			})
			t.Run("generator", func(t *testing.T) {
				generator := NewStandardIDGenerator()
				id, err := generator.Next(name)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(string(id), *captured) {
					t.Fatalf("generated ID %q does not use captured prefix %q", id, *captured)
				}
			})
		})
	}
}

func TestCapturedStandardKeyPrefixReservesIdentity(t *testing.T) {
	const name = "MessagingSession"
	captured, known := StandardObjectDescribeKeyPrefix(name)
	if !known || captured == nil {
		t.Fatal("missing captured prefix")
	}
	org := NewOrgState()
	org.Objects["Alpha__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Alpha__c", KeyPrefix: *captured}}
	org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name, KeyPrefix: "a12"}}
	EnsureUniqueKeyPrefixes(&org)
	if got := org.Objects[name].Definition.KeyPrefix; got != *captured {
		t.Fatalf("standard prefix = %q, captured %q", got, *captured)
	}
	if got := org.Objects["Alpha__c"].Definition.KeyPrefix; got == *captured {
		t.Fatal("custom object retained a standard object's captured prefix")
	}
	EnsureUniqueKeyPrefixes(&org)
	if got := org.Objects[name].Definition.KeyPrefix; got != *captured {
		t.Fatalf("repeat normalization changed captured prefix: %q", got)
	}
}

func TestCapturedStandardKeyPrefixPreservesBoundaries(t *testing.T) {
	if got := StandardKeyPrefix("Account"); got != "001" {
		t.Fatalf("Account prefix = %q", got)
	}
	if got := StandardKeyPrefix("PrefixMissing__c"); got != "" {
		t.Fatalf("unknown custom prefix = %q", got)
	}
	if got := AssignDeterministicPrefixes([]string{"PrefixWidget__c"}, map[string]string{"PrefixWidget__c": "a42"})["PrefixWidget__c"]; got != "a42" {
		t.Fatalf("explicit custom prefix = %q", got)
	}
	for _, name := range []string{"UserRecordAccess", "AINaturalLangProcessRsltShare"} {
		captured, known := StandardObjectDescribeKeyPrefix(name)
		if !known || captured != nil && *captured != "" {
			t.Fatalf("%s missing null/empty captured control", name)
		}
		org := NewOrgState()
		EnsureStandardObject(&org, name)
		if got := org.Objects[name].Definition.KeyPrefix; len(got) != 3 || !strings.HasPrefix(got, "a") {
			t.Fatalf("%s local fallback prefix = %q", name, got)
		}
	}
}

func TestCapturedStandardKeyPrefixPreservesLegacySharedOwner(t *testing.T) {
	for _, pair := range []struct{ owner, alias, prefix string }{
		{"ContentDocument", "ContentNote", "069"},
		{"CaseComment", "IdeaComment", "00a"},
	} {
		t.Run(pair.owner, func(t *testing.T) {
			org := NewOrgState()
			org.Objects[pair.owner] = ObjectState{Definition: ObjectDefinition{APIName: pair.owner, KeyPrefix: pair.prefix}}
			org.Objects[pair.alias] = ObjectState{Definition: ObjectDefinition{APIName: pair.alias, KeyPrefix: "a12"}}
			EnsureUniqueKeyPrefixes(&org)
			if got := org.Objects[pair.owner].Definition.KeyPrefix; got != pair.prefix {
				t.Fatalf("legacy owner %s prefix = %q, want %q", pair.owner, got, pair.prefix)
			}
			aliasPrefix := org.Objects[pair.alias].Definition.KeyPrefix
			if aliasPrefix == pair.prefix {
				t.Fatal("shared captured prefix bypassed the existing unique-storage boundary")
			}
			// Change the object set so the validated snapshot cannot skip rebuild.
			org.Objects["UnrelatedPrefix__c"] = ObjectState{Definition: ObjectDefinition{APIName: "UnrelatedPrefix__c", KeyPrefix: "a99"}}
			EnsureUniqueKeyPrefixes(&org)
			if org.Objects[pair.owner].Definition.KeyPrefix != pair.prefix || org.Objects[pair.alias].Definition.KeyPrefix != aliasPrefix {
				t.Fatal("repeated normalization changed the established identities")
			}
		})
	}
}
