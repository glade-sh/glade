package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native deployment and anonymous execution rows cover the same metadata at
// API 62 and 67 in testdata/native-capture/flow_type_matrix/org.tsv.
func TestFlowTransformCapturedOutputTypes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := strings.TrimSuffix(api, ".0")
			files := map[string]string{}
			methods := map[string]string{}
			for _, tc := range []struct{ name, input, assertion string }{
				{"TextNumber", "42", "System.assertEquals('42', ((Account)f.getVariableValue('outputRecord')).Name);"},
				{"TextBoolean", "true", "System.assertEquals('true', ((Account)f.getVariableValue('outputRecord')).Name);"},
				{"TextDate", "Date.newInstance(2001,2,3)", "System.assertEquals('February 3, 2001', ((Account)f.getVariableValue('outputRecord')).Name);"},
				{"TextDateTime", "Datetime.newInstanceGMT(2001,2,3,4,5,6)", "System.assertEquals('2/2/2001, 8:05 PM', ((Account)f.getVariableValue('outputRecord')).Name);"},
				{"PicklistNumber", "42", "System.assertEquals('42', ((Account)f.getVariableValue('outputRecord')).Type);"},
				{"PicklistBoolean", "true", "System.assertEquals('true', ((Account)f.getVariableValue('outputRecord')).Type);"},
				{"PicklistDate", "Date.newInstance(2001,2,3)", "System.assertEquals('2001-02-03', ((Account)f.getVariableValue('outputRecord')).Type);"},
				{"PicklistDateTime", "Datetime.newInstanceGMT(2001,2,3,4,5,6)", "System.assertEquals('2001-02-03T04:05:06Z', ((Account)f.getVariableValue('outputRecord')).Type);"},
				{"DateDateTime", "Datetime.newInstanceGMT(2001,2,3,4,5,6)", "System.assertEquals(Date.newInstance(2001,2,3), ((Contact)f.getVariableValue('outputRecord')).Birthdate);"},
				{"DateTimeDate", "Date.newInstance(2001,2,3)", "System.assertEquals(Datetime.newInstanceGMT(2001,2,3,0,0,0), ((Account)f.getVariableValue('outputRecord')).CreatedDate);"},
			} {
				name := "T" + version + tc.name
				path := filepath.Join("testdata", "native-capture", "flow_type_matrix", "api"+version, name+".flow-meta.xml")
				metadata, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = string(metadata)
				methods[tc.name] = "Flow.Interview f=Flow.Interview.createInterview('" + name + "',new Map<String,Object>{'inputValue'=>" + tc.input + "}); f.start(); " + tc.assertion
			}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
