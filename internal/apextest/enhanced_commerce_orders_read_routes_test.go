package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The native named compile rows at both APIs reject these reads with the
// setting off and accept them with it on. Exercise anonymous analysis too.
func TestEnhancedCommerceOrdersAnonymousReadRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, variant := range []string{"feature-only", "setting-only"} {
			t.Run(api+"/"+variant, func(t *testing.T) {
				index := loadTestIndex(t, enhancedOrdersProject(t, api, variant))
				for _, source := range []string{
					"Order row; Object result = row.OrderedDate;",
					"OrderItem row; Object result = row.TotalLineAmount;",
					"Order row; Object result = (row).OrderedDate;",
					"OrderItem row; Object result = (row).TotalLineAmount;",
				} {
					for _, pass := range []string{"cold", "warm"} {
						analysis := sema.AnalyzeAnonymous(index, source, api)
						if got, want := analysis.HasErrors(), variant == "feature-only"; got != want {
							t.Errorf("%s %s: rejected=%t want=%t diagnostics=%v", source, pass, got, want, analysis.Diagnostics)
						}
					}
				}
			})
		}
	}
}

// The same captured field-availability rule applies while compiling test classes.
func TestEnhancedCommerceOrdersIsTestReadRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, variant := range []string{"feature-only", "setting-only"} {
			for _, statement := range []string{"Order row; Object value = row.OrderedDate;", "OrderItem row; Object value = row.TotalLineAmount;"} {
				t.Run(api+"/"+variant+"/"+statement, func(t *testing.T) {
					root := enhancedOrdersProject(t, api, variant)
					writeEnhancedOrdersClass(t, root, api, "OwnedGateTest", "@IsTest private class OwnedGateTest { @IsTest static void check() { "+statement+" } }")
					index := loadTestIndex(t, root)
					analysis := sema.Analyze(index)
					if got, want := analysis.HasErrors(), variant == "feature-only"; got != want {
						t.Fatalf("rejected=%t want=%t diagnostics=%v", got, want, analysis.Diagnostics)
					}
				})
			}
		}
	}
}
