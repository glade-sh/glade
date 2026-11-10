package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native rows in testdata/native-capture/flow_transform_coercion/org.tsv record
// the value and runtime type exposed by Transform Map outputs at API 62 and 67:
// integer fields truncate decimals toward zero, decimal fields keep the source
// digits regardless of the variable scale, and ID and reference targets expose
// the text form of the source.
func TestFlowTransformCapturedNumberAndReferenceOutputs(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := strings.TrimSuffix(api, ".0")
			files := map[string]string{}
			for _, dir := range []string{"flow_transform_coercion", "flow_type_matrix"} {
				entries, err := os.ReadDir(filepath.Join("testdata", "native-capture", dir, "api"+version))
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					name := strings.TrimSuffix(entry.Name(), ".flow-meta.xml")
					if dir == "flow_type_matrix" && !strings.HasPrefix(name, "T"+version+"Reference") {
						continue
					}
					data, err := os.ReadFile(filepath.Join("testdata", "native-capture", dir, "api"+version, entry.Name()))
					if err != nil {
						t.Fatal(err)
					}
					files["force-app/main/default/flows/"+entry.Name()] = string(data)
				}
			}
			methods := map[string]string{}
			run := func(label, flow, input, object, field, expected string) {
				methods[label] = "Flow.Interview f=Flow.Interview.createInterview('" + flow + "',new Map<String,Object>{'inputValue'=>" + input + "}); f.start(); " +
					"SObject o=(SObject)f.getVariableValue('outputRecord'); System.assertEquals(" + expected + ", String.valueOf(o.get('" + field + "')));"
			}
			for _, scale := range []string{"0", "2"} {
				for _, tc := range []struct{ label, input, integer, decimal string }{
					{"I42", "42", "'42'", "'42'"},
					{"D415", "Decimal.valueOf('41.5')", "'41'", "'41.5'"},
					{"D4125", "Decimal.valueOf('41.25')", "'41'", "'41.25'"},
					{"D416", "Decimal.valueOf('41.6')", "'41'", "'41.6'"},
					{"DNeg", "Decimal.valueOf('-41.5')", "'-41'", "'-41.5'"},
					{"D420", "Decimal.valueOf('42.0')", "'42'", "'42.0'"},
				} {
					run("Emp"+scale+tc.label, "C"+version+"Dec"+scale+"Emp", tc.input, "Account", "NumberOfEmployees", tc.integer)
					run("Rev"+scale+tc.label, "C"+version+"Dec"+scale+"Rev", tc.input, "Account", "AnnualRevenue", tc.decimal)
				}
			}
			for _, tc := range []struct{ label, input, expected string }{
				{"String", "'42'", "'42'"},
				{"Number", "42", "'42'"},
				{"Boolean", "true", "'true'"},
				{"Date", "Date.newInstance(2001,2,3)", "'February 3, 2001'"},
				{"DateTime", "Datetime.newInstanceGMT(2001,2,3,4,5,6)", "'2/2/2001, 8:05 PM'"},
			} {
				run("Owner"+tc.label, "T"+version+"Reference"+tc.label, tc.input, "Account", "OwnerId", tc.expected)
			}
			methods["OwnerUser"] = "Flow.Interview f=Flow.Interview.createInterview('C" + version + "OwnerIdString',new Map<String,Object>{'inputValue'=>UserInfo.getUserId()}); f.start(); System.assertEquals(String.valueOf(UserInfo.getUserId()), String.valueOf(((SObject)f.getVariableValue('outputRecord')).get('OwnerId')));"
			methods["AccountId"] = "Account a=new Account(Name='Flow Coerce Probe'); insert a; Flow.Interview f=Flow.Interview.createInterview('C" + version + "AcctIdString',new Map<String,Object>{'inputValue'=>String.valueOf(a.Id)}); f.start(); System.assertEquals(String.valueOf(a.Id), String.valueOf(((SObject)f.getVariableValue('outputRecord')).get('AccountId')));"
			run("AccountId42", "C"+version+"AcctIdString", "'42'", "Contact", "AccountId", "'42'")
			run("AccountIdText", "C"+version+"AcctIdString", "'not an id'", "Contact", "AccountId", "'not an id'")
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
