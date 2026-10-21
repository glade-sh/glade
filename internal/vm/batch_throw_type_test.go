package vm

import (
	"fmt"
	"testing"
)

func TestBatchErrorTypeRetainsApexThrownType(t *testing.T) {
	v := Object("IllegalArgumentException")
	v.Fields["message"] = String("synthetic execute")
	e := fmt.Errorf("wrapped: %w", &apexThrowError{value: v})
	if got := batchErrorExceptionType(e); got != "System.IllegalArgumentException" {
		t.Fatal(got)
	}
	if got := batchErrorMessage(e); got != "synthetic execute" {
		t.Fatal(got)
	}
}
