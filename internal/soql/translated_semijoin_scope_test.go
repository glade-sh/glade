package soql

import (
	"errors"
	"testing"
)

func TestTranslatedSemiJoinPreservesScopeUnsupported(t *testing.T) {
	org := usingScopeMineOrg("Account")
	inner := Query{Object: "Account", Fields: []string{"Id"}, UsingScope: "Mine"}
	for name, condition := range map[string]Condition{
		"direct": {Subquery: &inner},
		"nested": {And: []Condition{{Subquery: &inner}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := resolveSubqueries(org, condition, ExecutionContext{UserID: "005000000000001", Language: "fr"})
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) {
				t.Fatalf("semi-join scope should remain unsupported, got %v", err)
			}
		})
	}
}
