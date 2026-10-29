package dml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// This tests both the literal and variable operand lowering without changing
// formula, record-field, text, or collection comparison semantics.
func TestFlowNullComparisonsNativeRows(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_null_comparisons"
	read := func(path string) []string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
	cases := read("cases.tsv")
	if len(cases) != 61 {
		t.Fatalf("case count: %d", len(cases)-1)
	}
	for _, api := range []string{"62", "67"} {
		for _, context := range []string{"anonymous", "istest"} {
			want := map[string]bool{}
			for _, line := range read("native/api" + api + "/" + context + "/org.tsv")[1:] {
				row := strings.Split(line, "\t")
				if len(row) != 3 {
					t.Fatalf("bad native row: %s", line)
				}
				want[row[1]+"/"+row[0]] = row[2] == "match"
			}
			for _, route := range []string{"auto", "trigger"} {
				for _, encoding := range []string{"SourceField", "MergeValue"} {
					for _, line := range cases[1:] {
						row := strings.Split(line, "\t")
						t.Run(api+"/"+context+"/"+route+"/"+encoding+"/"+row[0], func(t *testing.T) {
							org := storage.NewOrgState()
							org.APIVersion = api + ".0"
							engine := NewEngine(&org)
							frame := newFlowFrame()
							frame.autolaunched = route == "auto"
							frame.beforeSaveDecisions = route == "trigger"
							for _, prefix := range []string{"N", "C"} {
								kind := "Number"
								if prefix == "C" {
									kind = "Currency"
								}
								for _, name := range []string{"Unset", "OtherUnset", "AssignedNull", "Zero", "Positive", "Negative"} {
									frame.variables[flowFrameKey(prefix+name)] = storage.FlowVariable{Name: prefix + name, DataType: kind}
								}
								frame.scalars[flowFrameKey(prefix+"AssignedNull")] = storage.NullValue()
								frame.scalars[flowFrameKey(prefix+"Zero")] = storage.IntegerValue(0)
								frame.scalars[flowFrameKey(prefix+"Positive")] = storage.DecimalValue("1.25")
								frame.scalars[flowFrameKey(prefix+"Negative")] = storage.DecimalValue("-1.25")
							}
							operation := strings.ToLower(row[3])
							if operation == "equalto" {
								operation = "equals"
							}
							if operation == "notequalto" {
								operation = "notequal"
							}
							item := storage.WorkflowCriteriaItem{Field: row[2], Operation: operation, Value: row[5]}
							if row[4] == "ref" {
								item.Value = ""
								if encoding == "SourceField" {
									item.SourceField = row[5]
								} else {
									item.Value = "{!" + row[5] + "}"
								}
							}
							expected, exists := want[route+"/"+row[0]]
							if !exists {
								t.Fatal("missing native row")
							}
							got, supported := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{item}}, storage.Record{}, storage.ObjectDefinition{}, frame)
							if !supported || got != expected {
								t.Fatalf("match=%t supported=%t; native match=%t", got, supported, expected)
							}
						})
					}
				}
			}
		}
	}
}
