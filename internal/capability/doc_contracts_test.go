package capability

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexdocs"
)

func TestBuildDocContractsPinsSymbolFromPath(t *testing.T) {
	inv := apexdocs.Inventory{
		Documents: []apexdocs.Document{
			{
				SourcePath: "apex_System_PageReference_getContentAsPDF.md",
				Name:       "getContentAsPDF()",
				Namespace:  "System",
				Behaviors: []apexdocs.DocBehavior{
					{Kind: apexdocs.BehaviorCalloutInTest, Evidence: "treated as a callout"},
				},
			},
			{
				SourcePath: "apex_class_System_Limits.md",
				Name:       "Limits",
				Namespace:  "System",
				Behaviors:  nil,
			},
		},
	}

	report := BuildDocContracts(inv)
	if report.TotalDocuments != 2 {
		t.Fatalf("total docs = %d", report.TotalDocuments)
	}
	if report.TotalContracts != 1 || report.DocsWithContracts != 1 {
		t.Fatalf("contracts = %d docs = %d", report.TotalContracts, report.DocsWithContracts)
	}
	c := report.Contracts[0]
	if c.Symbol != "PageReference.getContentAsPDF()" {
		t.Fatalf("symbol = %q", c.Symbol)
	}
	if c.Type != "PageReference" || c.Member != "getContentAsPDF()" {
		t.Fatalf("type/member = %q/%q", c.Type, c.Member)
	}
	if c.Behavior != apexdocs.BehaviorCalloutInTest {
		t.Fatalf("behavior = %q", c.Behavior)
	}
}

func TestBuildDocContractsTypeLevelSymbol(t *testing.T) {
	inv := apexdocs.Inventory{
		Documents: []apexdocs.Document{
			{
				SourcePath: "apex_class_Messaging_SingleEmailMessage.md",
				Name:       "SingleEmailMessage",
				Namespace:  "Messaging",
				Behaviors: []apexdocs.DocBehavior{
					{Kind: apexdocs.BehaviorDeprecated, Evidence: "deprecated"},
				},
			},
		},
	}
	report := BuildDocContracts(inv)
	if got := report.Contracts[0].Symbol; got != "Messaging.SingleEmailMessage" {
		t.Fatalf("symbol = %q", got)
	}
	if report.Contracts[0].Member != "" {
		t.Fatalf("member should be empty, got %q", report.Contracts[0].Member)
	}
}

func TestDocContractsFilterByBehavior(t *testing.T) {
	inv := apexdocs.Inventory{
		Documents: []apexdocs.Document{
			{
				SourcePath: "apex_System_A_m.md", Name: "m()", Namespace: "System",
				Behaviors: []apexdocs.DocBehavior{
					{Kind: apexdocs.BehaviorThrows, Evidence: "throws X"},
					{Kind: apexdocs.BehaviorDeprecated, Evidence: "deprecated"},
				},
			},
		},
	}
	report := BuildDocContracts(inv).FilterByBehavior(apexdocs.BehaviorThrows)
	if report.TotalContracts != 1 || report.Contracts[0].Behavior != apexdocs.BehaviorThrows {
		t.Fatalf("filtered = %#v", report.Contracts)
	}
}

func TestWriteDocContractsMarkdownDeterministic(t *testing.T) {
	inv := apexdocs.Inventory{
		Documents: []apexdocs.Document{
			{
				SourcePath: "apex_System_A_m.md", Name: "m()", Namespace: "System",
				Behaviors: []apexdocs.DocBehavior{{Kind: apexdocs.BehaviorThrows, Evidence: "throws X"}},
			},
		},
	}
	report := BuildDocContracts(inv)
	var a, b strings.Builder
	if err := WriteDocContractsMarkdown(&a, report); err != nil {
		t.Fatal(err)
	}
	if err := WriteDocContractsMarkdown(&b, report); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("markdown output not deterministic")
	}
	if !strings.Contains(a.String(), "A.m()") {
		t.Fatalf("missing symbol in markdown:\n%s", a.String())
	}
}
