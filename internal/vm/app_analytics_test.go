package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func newAppAnalyticsTestVM(t *testing.T) *VM {
	t.Helper()
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	if err := machine.RegisterClass(Class{Name: "ResidualLens.Gesture", EnumValues: []string{"Opened"}}); err != nil {
		t.Fatal(err)
	}
	return machine
}

// Salesforce documents that running Apex tests produce no package usage data:
// https://developer.salesforce.com/docs/platform/pkg2-dev/guide/app-analytics-custom-interactions.html
// Owned captures cover anonymous calls, test calls and the Id/UUID overloads.
func TestExecAppAnalyticsEnumInTestContext(t *testing.T) {
	program, err := CompileAnonymous(`
IsvPartners.AppAnalytics.logCustomInteraction(ResidualLens.Gesture.Opened);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := newAppAnalyticsTestVM(t)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAppAnalyticsInvalidLabelsKeepNativeValidation(t *testing.T) {
	// Owned metadata conformance observations reject null and String labels
	// at API 62 and 67.
	for _, testContext := range []bool{false, true} {
		for _, label := range []string{"null", "'Opened'", "1", "1L", "1.0", "Double.valueOf('1')"} {
			t.Run(fmt.Sprintf("%s/test=%t", label, testContext), func(t *testing.T) {
				program, err := CompileAnonymous(`
Boolean caught = false;
try { IsvPartners.AppAnalytics.logCustomInteraction(` + label + `); }
catch (System.InvalidParameterValueException e) {
    caught = true;
    System.assert(e.getMessage().endsWith('must be an Apex enum.'));
}
System.assert(caught);
`)
				if err != nil {
					t.Fatal(err)
				}
				machine := newAppAnalyticsTestVM(t)
				if testContext {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestExecAppAnalyticsPreservesUncapturedHostedBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, arguments string
		testContext     bool
	}{
		{"testUncapturedOverload", "ResidualLens.Gesture.Opened, true", true},
		{"testNumericOverload", "1, (Id)null", true},
		{"testPlatformOverload", "System.LoggingLevel.INFO, (Id)null", true},
		{"testObjectLabel", "new Account()", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous("IsvPartners.AppAnalytics.logCustomInteraction(" + tc.arguments + ");")
			if err != nil {
				t.Fatal(err)
			}
			machine := newAppAnalyticsTestVM(t)
			if tc.testContext {
				machine.EnableTestContext()
			}
			if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "hosted telemetry") {
				t.Fatalf("expected existing hosted boundary, got %v", err)
			}
		})
	}
}

func TestExecAppAnalyticsCapturedReturnsAndPlatformLabel(t *testing.T) {
	for _, testContext := range []bool{false, true} {
		t.Run(fmt.Sprintf("test=%t", testContext), func(t *testing.T) {
			program, err := CompileAnonymous(`
IsvPartners.AppAnalytics.logCustomInteraction(ResidualLens.Gesture.Opened);
IsvPartners.AppAnalytics.logCustomInteraction(ResidualLens.Gesture.Opened,UserInfo.getUserId());
IsvPartners.AppAnalytics.logCustomInteraction(ResidualLens.Gesture.Opened,(Id)null);
IsvPartners.AppAnalytics.logCustomInteraction(ResidualLens.Gesture.Opened,UUID.randomUUID());
IsvPartners.AppAnalytics.logCustomInteraction(ResidualLens.Gesture.Opened,(UUID)null);
Boolean rejected=false;
try{IsvPartners.AppAnalytics.logCustomInteraction(System.LoggingLevel.INFO);}
catch(System.InvalidParameterValueException e){
 rejected=true;
 System.assertEquals('We couldn’t log the custom interaction with App Analytics. The interaction label provided to IsvPartners.AppAnalytics.logCustomInteraction must have the same namespace as the Apex code that called the method.',e.getMessage());
}
System.assert(rejected);
`)
			if err != nil {
				t.Fatal(err)
			}
			machine := newAppAnalyticsTestVM(t)
			if testContext {
				machine.EnableTestContext()
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAppAnalyticsUncapturedNamespaceContextsKeepHostedBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, caller            string
		testContext, dependency bool
	}{
		{"namespacedAnonymous", "sample", false, false},
		{"foreignNamespace", "other", true, false},
		{"dependencyEnum", "sample", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			machine.currentNamespace = tc.caller
			if err := machine.RegisterClass(Class{Name: "Gesture", Namespace: "sample", EnumValues: []string{"Opened"}, Dependency: tc.dependency}); err != nil {
				t.Fatal(err)
			}
			if tc.testContext {
				machine.EnableTestContext()
			}
			label := Object("sample.Gesture")
			label.Text = "Opened"
			if _, err := machine.appAnalyticsLogCustomInteraction([]Value{label}); err == nil || !strings.Contains(err.Error(), "hosted telemetry") {
				t.Fatalf("uncaptured context left hosted boundary: %v", err)
			}
		})
	}
}
