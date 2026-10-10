package gladecli

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestFlattenTestCasesPreservesSourceFile(t *testing.T) {
	run := testreport.Run{Suites: []testreport.Suite{{Name: "SourceTest", Cases: []testreport.Case{
		{ClassName: "SourceTest", MethodName: "pass", Status: testreport.StatusPass, SelectedSourceFile: "/selected/SourceTest.cls", SourceFile: "/actual/SourceTest.cls"},
		{ClassName: "SourceTest", MethodName: "fail", Status: testreport.StatusFail, SelectedSourceFile: "/selected/SourceTest.cls", SourceFile: "/actual/SourceTest.cls"},
		{ClassName: "SourceTest", MethodName: "unknown", Status: testreport.StatusUnsupported, SelectedSourceFile: "/selected/SourceTest.cls"},
	}}}}
	flat := flattenTestCases(run)
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	var nested struct {
		Suites []struct {
			Cases []map[string]any `json:"cases"`
		} `json:"suites"`
	}
	if err := json.Unmarshal(data, &nested); err != nil {
		t.Fatal(err)
	}
	for i, row := range flat {
		if row["selectedSourceFile"] != "/selected/SourceTest.cls" || row["selectedSourceFile"] != nested.Suites[0].Cases[i]["selectedSourceFile"] {
			t.Fatalf("lost selected source: %#v", row)
		}
		got, exists := row["sourceFile"]
		nestedGot, nestedExists := nested.Suites[0].Cases[i]["sourceFile"]
		if got != nestedGot || exists != nestedExists {
			t.Fatalf("flat/nested source mismatch: %#v / %#v", row, nested.Suites[0].Cases[i])
		}
		if i < 2 && got != "/actual/SourceTest.cls" {
			t.Fatalf("lost actual source: %#v", row)
		}
		if i == 2 && exists {
			t.Fatalf("unknown source must be omitted: %#v", row)
		}
	}
}
