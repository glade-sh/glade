package dml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowScalarTextInterviewAndInvocablePredicatesNativeRows(t *testing.T) {
	const fixture = "testdata/conformance/flow_text_prefix_suffix"
	body, err := os.ReadFile(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cases := strings.Split(strings.TrimSpace(string(body)), "\n")[1:]
	for _, api := range []string{"62.0", "67.0"} {
		data, err := os.ReadFile(filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
			row := strings.Split(line, "\t")
			want[row[0]+"/"+row[1]] = row[2] == "true"
		}
		for _, context := range []string{"anonymous", "istest"} {
			for _, route := range []string{"interview", "invocable"} {
				for _, line := range cases {
					row := strings.Split(line, "\t")
					t.Run(api+"/"+context+"/"+route+"/"+row[0], func(t *testing.T) {
						value := storage.StringValue(row[3])
						if row[2] == "null" {
							value = storage.NullValue()
						} else if row[2] == "empty" {
							value = storage.StringValue("")
						}
						org := storage.NewOrgState()
						org.APIVersion = api
						engine := NewEngine(&org)
						engine.InvocableFlow = route == "invocable"
						frame := newFlowFrame()
						frame.scalars["inputtext"] = value
						got, supported := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "InputText", Operation: row[1], Value: row[4]}}}, storage.Record{}, storage.ObjectDefinition{}, frame)
						expected, ok := want[context+"/"+row[0]]
						if !ok {
							t.Fatal("missing native row")
						}
						if !supported || got != expected {
							t.Fatalf("matched=%t supported=%t; native matched=%t", got, supported, expected)
						}
					})
				}
			}
		}
	}
}
