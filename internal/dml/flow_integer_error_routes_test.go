package dml

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowIntegerErrorStatusAndActionResultRouteTwins(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		body, err := os.ReadFile(filepath.Join("testdata/conformance/flow_value_boundaries/integer_errors", "api"+api, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
			cells := strings.SplitN(line, "\t", 3)
			if cells[0] != "anonymous" {
				continue
			}
			var native struct {
				Status *string `json:"status"`
				Code   *string `json:"actionErrorCode"`
			}
			if err := json.Unmarshal([]byte(cells[2]), &native); err != nil {
				t.Fatal(err)
			}
			if native.Status == nil && native.Code == nil {
				continue
			}
			t.Run(api+"/"+cells[1], func(t *testing.T) {
				failure := flowActionFailure("Observe", &FlowActionInputError{Cause: errors.New("Decimal cannot convert to Integer")})
				if native.Status != nil {
					got := resultFromError("", failure)
					if got.Success || got.StatusCode != *native.Status {
						t.Fatalf("native status=%s; Glade=%+v", *native.Status, got)
					}
				} else {
					engine := &Engine{}
					got, ok := engine.InvocableFlowActionError(failure)
					if !ok || got.StatusCode != *native.Code {
						t.Fatalf("native action code=%s; Glade=%+v recognized=%v", *native.Code, got, ok)
					}
				}
			})
		}
	}
}

func TestFlowIntegerErrorUnrelatedActionFailureRetainsStatus(t *testing.T) {
	failure := flowActionFailure("Observe", fmt.Errorf("unrelated action failure"))
	if got := resultFromError("", failure); got.StatusCode != "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY" {
		t.Fatalf("unrelated failure changed: %+v", got)
	}
}

func TestFlowIntegerErrorInterpreterCallbackRouteTwins(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		body, err := os.ReadFile(filepath.Join("testdata/conformance/flow_value_boundaries/integer_errors", "api"+api, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		var native struct {
			Status string `json:"status"`
		}
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
			cells := strings.SplitN(line, "\t", 3)
			if cells[0] == "anonymous" && cells[1] == "triggerFraction" {
				if err := json.Unmarshal([]byte(cells[2]), &native); err != nil {
					t.Fatal(err)
				}
			}
		}
		if native.Status == "" {
			t.Fatal("missing native trigger status")
		}
		for _, route := range []string{"steps/context_callback", "steps/legacy_callback", "flat/legacy_callback"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testOrg()
				engine := NewEngine(&org)
				failure := func() error { return &FlowActionInputError{Cause: errors.New("Decimal cannot convert to Integer")} }
				engine.FlowActionInvoker = func(storage.FlowAction, storage.Record) error { return failure() }
				if route == "steps/context_callback" {
					engine.FlowActionInvokerWithContext = func(storage.FlowAction, storage.Record, FlowActionContext) error { return failure() }
				}
				action := storage.FlowAction{Name: "Observe", ActionType: "apex", ClassName: "IntegerBoundary"}
				steps := []storage.FlowStep{{Kind: "action", Action: action}}
				var actions []storage.FlowAction
				if strings.HasPrefix(route, "flat/") {
					steps = nil
					actions = []storage.FlowAction{action}
				}
				record := storage.Record{Object: "Account"}
				_, err := engine.applyFlowEffects("IntegerBoundary", "Account", &record, org.Objects["Account"].Definition, steps, nil, actions, nil, nil, nil, nil)
				if err == nil {
					t.Fatal("native rejected fractional input")
				}
				if got := resultFromError("", err); got.StatusCode != native.Status {
					t.Fatalf("native status=%s; Glade=%+v", native.Status, got)
				}
			})
		}
	}
}
