package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Captured acceptance, populated and empty-list outcomes are in
// testdata/native-capture/flow_type_matrix/org.tsv at both API versions.
func TestFlowLookupCapturedOperandTypes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := strings.TrimSuffix(api, ".0")
			files := map[string]string{}
			methods := map[string]string{}
			for _, tc := range []struct{ name, body string }{
				{"InTextDate", `Boolean caught=false; try { Flow.Interview f=Flow.Interview.createInterview('FLOWNAME',new Map<String,Object>{'values'=>new List<Date>{Date.newInstance(2001,2,3)}}); f.start(); } catch (Exception e) { caught=true; System.assertEquals('System.FlowException',e.getTypeName()); } System.assertEquals(true,caught);`},
				{"NotInTextDate", `Boolean caught=false; try { Flow.Interview f=Flow.Interview.createInterview('FLOWNAME',new Map<String,Object>{'values'=>new List<Date>{Date.newInstance(2001,2,3)}}); f.start(); } catch (Exception e) { caught=true; System.assertEquals('System.FlowException',e.getTypeName()); } System.assertEquals(true,caught);`},
				{"InReferenceNumber", `Boolean caught=false; try { Flow.Interview f=Flow.Interview.createInterview('FLOWNAME',new Map<String,Object>{'values'=>new List<Decimal>{42}}); f.start(); } catch (Exception e) { caught=true; System.assertEquals('System.FlowException',e.getTypeName()); } System.assertEquals(true,caught);`},
				{"NotInReferenceNumber", `Boolean caught=false; try { Flow.Interview f=Flow.Interview.createInterview('FLOWNAME',new Map<String,Object>{'values'=>new List<Decimal>{42}}); f.start(); } catch (Exception e) { caught=true; System.assertEquals('System.FlowException',e.getTypeName()); } System.assertEquals(true,caught);`},
				{"InNumberNumber", `Flow.Interview f=Flow.Interview.createInterview('FLOWNAME',new Map<String,Object>{'values'=>new List<Decimal>{42}}); f.start(); System.assertEquals('Flow Type Probe B',f.getVariableValue('outcome'));`},
				{"NotInNumberNumber", `Flow.Interview f=Flow.Interview.createInterview('FLOWNAME',new Map<String,Object>{'values'=>new List<Decimal>{42}}); f.start(); System.assertEquals('Flow Type Probe A',f.getVariableValue('outcome'));`},
				{"InReferenceString", `Account b=[SELECT OwnerId FROM Account WHERE Name='Flow Type Probe B' LIMIT 1]; Flow.Interview f=Flow.Interview.createInterview('FLOWNAME',new Map<String,Object>{'values'=>new List<String>{String.valueOf(b.OwnerId)}}); f.start(); System.assertEquals('Flow Type Probe A',f.getVariableValue('outcome'));`},
			} {
				name := "F" + version + tc.name
				path := filepath.Join("testdata", "native-capture", "flow_type_matrix", "api"+version, name+".flow-meta.xml")
				metadata, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = string(metadata)
				seed := `insert new List<Account>{new Account(Name='Flow Type Probe A',NumberOfEmployees=41),new Account(Name='Flow Type Probe B',NumberOfEmployees=42)};`
				methods[tc.name] = seed + strings.ReplaceAll(tc.body, "FLOWNAME", name)
			}
			methods["EmptyTextDate"] = `insert new Account(Name='Flow Type Probe A'); Flow.Interview f=Flow.Interview.createInterview('F` + version + `InTextDate',new Map<String,Object>{'values'=>new List<Date>()}); f.start(); System.assertEquals(null,f.getVariableValue('outcome'));`
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
