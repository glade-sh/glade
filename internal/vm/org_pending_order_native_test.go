package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native uncatchable count results: org_commerce_exceptions/org.tsv.
func TestPendingOrderSummaryCountNativeFailure(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.ApplyProjectOrgShape(&org, []string{"OrderManagement"})
			storage.EnsureStandardObject(&org, "PendingOrderSummary")
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{APIVersion: api}
			machine.dynamicSOQLBindRoute = "Database.countQuery"
			_, err := machine.executeSOQL("SELECT COUNT() FROM PendingOrderSummary", &Result{})
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.UnexpectedException" || runtimeErr.Message != "COUNT() query could not be processed" {
				t.Fatalf("unexpected count result: %v", err)
			}
		})
	}
}

// org_commerce_b2b/raw-exceptions.jsonl: both APIs and both routes terminate.
func TestPendingOrderSummaryB2BCountNativeFailure(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		org := storage.NewOrgState()
		storage.ApplyProjectOrgShape(&org, []string{"B2BCommerce", "Communities", "NetworksEnabled", "CommerceEnabled"})
		storage.EnsureStandardObject(&org, "PendingOrderSummary")
		machine := New(nil)
		machine.SetOrg(&org)
		machine.currentMethod = Method{APIVersion: api}
		machine.dynamicSOQLBindRoute = "Database.countQuery"
		_, err := machine.executeSOQL("SELECT COUNT() FROM PendingOrderSummary", &Result{})
		var native *RuntimeError
		if !errors.As(err, &native) || native.Type != "System.UnexpectedException" {
			t.Errorf("API %s: %v", api, err)
		}
	}
}
