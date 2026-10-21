package apextest

import (
	"encoding/json"
	"os"
	"testing"
)

// Salesforce's Code Commenting documentation says both single-line and
// multiline comment contents are ignored. Punctuation inside a comment must
// not terminate an enum's members. This is docs-backed local API coverage,
// not a claim of new native API-specific captures.
func TestEnumCommentMembersConformance(t *testing.T) {
	var fixture struct {
		Basis, TestClass, TestSource string
		Sources, APIs                []string
		MethodCount                  int
		Variants                     []struct{ ID, Source string }
	}
	raw, err := os.ReadFile("testdata/conformance/enum_comment_members.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Basis == "" || len(fixture.Sources) != 2 || len(fixture.APIs) != 3 || len(fixture.Variants) != 6 || fixture.MethodCount != 3 {
		t.Fatal("incomplete documented comment matrix")
	}
	for _, api := range fixture.APIs {
		for _, variant := range fixture.Variants {
			t.Run(api+"/"+variant.ID, func(t *testing.T) {
				index := nativeBoundaryIndex(t, api, map[string]string{
					"OwnedEnumCarrier": variant.Source,
					fixture.TestClass:  fixture.TestSource,
				}, nil)
				run := Run(index, Options{SelectedClasses: []string{fixture.TestClass}, NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != fixture.MethodCount || summary.Passed != fixture.MethodCount {
					data, _ := json.Marshal(run)
					t.Fatalf("documented enum comments: %s", data)
				}
			})
		}
	}
}
