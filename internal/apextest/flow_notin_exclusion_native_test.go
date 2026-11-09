package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowNotInExcludesMatchingRecord(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := filepath.Join("testdata/native-capture/pr163_flow_notin_exclusion", "api"+strings.TrimSuffix(api, ".0"), "Capture_Filter_NotIn.flow-meta.xml")
			metadata, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{"force-app/main/default/flows/Capture_Filter_NotIn.flow-meta.xml": string(metadata)}
			methods := map[string]string{"excluded": `Account b = new Account(Name='Capture Collection Seed B'); insert b;` +
				`Flow.Interview f = Flow.Interview.createInterview('Capture_Filter_NotIn', new Map<String,Object>{'ids'=>new List<String>{String.valueOf(b.Id)}});` +
				`f.start(); System.assertEquals(null, f.getVariableValue('outcome'));`}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
