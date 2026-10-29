package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The projection must preserve the existing grouped-field behavior when SELECT
// and GROUP BY use different spellings of the same field. Native rows for
// selected and hidden groups are in testdata/conformance/soql_count_order.
func TestAggregateProjectionEquivalentFieldNames(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Entry__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Entry__c", Fields: map[string]storage.Field{
			"Id":        {APIName: "Id", Type: storage.FieldID},
			"First__c":  {APIName: "First__c", Type: storage.FieldString},
			"Second__c": {APIName: "Second__c", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{"a01000000000001": {
			ID: "a01000000000001", Object: "Entry__c",
			Fields: map[string]storage.Value{"First__c": storage.StringValue("one"), "Second__c": storage.StringValue("two")},
		}},
	}
	for _, tc := range []struct {
		name, query, key, value string
		wantPresent             bool
	}{
		{"qualified-select", "SELECT pkg__First__c, COUNT(Id) total FROM Entry__c GROUP BY First__c", "First__c", "one", true},
		{"qualified-group", "SELECT First__c, COUNT(Id) total FROM Entry__c GROUP BY pkg__First__c", "pkg__First__c", "one", false},
		{"second-field-case", "SELECT PKG__SECOND__C, COUNT(Id) total FROM Entry__c GROUP BY Second__c", "Second__c", "two", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseAndExecute(org, tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Records) != 1 {
				t.Fatalf("records: %#v", result.Records)
			}
			value, ok := result.Records[0].Fields[tc.key]
			if ok != tc.wantPresent || ok && (value.Kind != storage.ValueString || value.String != tc.value) {
				t.Fatalf("group %s = %#v, present=%v; fields=%#v", tc.key, value, ok, result.Records[0].Fields)
			}
		})
	}
}

// Only the qualified-SELECT/unqualified-GROUP spelling is restored. These
// controls retain the train's projection until separate native rows exist.
func TestAggregateProjectionNamespaceScope(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	for _, tc := range []struct{ name, selected, group string }{
		{"reverse", "First__c", "pkg__First__c"},
		{"object-qualified", "Entry__c.First__c", "pkg__First__c"},
		{"other-namespace", "other__First__c", "First__c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]storage.Value{tc.group: storage.StringValue("one")}
			removeUnselectedGroupFields(fields, Query{Fields: []string{tc.selected}, GroupBy: []string{tc.group}}, org)
			if _, ok := fields[tc.group]; ok {
				t.Fatalf("uncaptured spelling retained %q", tc.group)
			}
		})
	}
}

// A project that declares no namespace can still spell a field with its package
// prefix. The qualified-SELECT/unqualified-GROUP shape keeps the group value;
// the scope controls keep the train's projection.
func TestAggregateProjectionUndeclaredNamespace(t *testing.T) {
	org := storage.NewOrgState()
	for _, tc := range []struct {
		name, selected, group string
		keep                  bool
	}{
		{"qualified-select", "pkg__First__c", "First__c", true},
		{"qualified-select-case", "PKG__first__c", "First__c", true},
		{"reverse", "First__c", "pkg__First__c", false},
		{"object-qualified", "Entry__c.pkg__First__c", "First__c", false},
		{"standard-field", "pkg__Name", "Name", false},
		{"two-prefixes", "a__b__First__c", "First__c", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]storage.Value{tc.group: storage.StringValue("one")}
			removeUnselectedGroupFields(fields, Query{Fields: []string{tc.selected}, GroupBy: []string{tc.group}}, org)
			if _, ok := fields[tc.group]; ok != tc.keep {
				t.Fatalf("group %q present=%v, want %v", tc.group, ok, tc.keep)
			}
		})
	}
}
