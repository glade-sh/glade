package sema

import (
	"strings"
	"testing"
)

func TestSOQLBindNestedLiteralBoundary(t *testing.T) {
	// BI001/003/019/021: inner indexed operands cannot close the query.
	for _, query := range []string{
		"[SELECT Id FROM Account WHERE Id IN (:a[0].Id,:a[1].Id)]",
		"[SELECT Id FROM Account WHERE (Name>:a[0].Name OR Id>:a[0].Id)]",
		"[SELECT Id FROM Account WHERE Id=:(a[0].Id)]",
		"[SELECT Id FROM Account WHERE Id=: a [ 0 ] . Id]",
	} {
		if end := semaMatchingBracket(query, 0); end != len(query)-1 {
			t.Fatalf("%s ends at %d", query, end)
		}
	}
	// BI030 remains malformed; bare field/literal controls remain intact.
	if end := semaMatchingBracket("[SELECT Id FROM Account WHERE Id=:a[0.Id]", 0); end != -1 {
		t.Fatalf("missing outer bracket accepted: %d", end)
	}
	query := "[SELECT Id FROM Account WHERE Name='Acme']"
	if end := semaMatchingBracket(query, 0); end != strings.LastIndex(query, "]") {
		t.Fatalf("literal control ends at %d", end)
	}
}
