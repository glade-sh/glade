package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowTransformRecordMapNative(t *testing.T) {
	const root = "testdata/conformance/flow_transform_record_map"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := filepath.Join(root, "api"+strings.TrimSuffix(api, ".0"), "TransformProbe.flow-meta.xml")
			metadata, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{"force-app/main/default/flows/TransformProbe.flow-meta.xml": string(metadata)}
			methods := map[string]string{"mapAndNull": `
                Flow.Interview f = Flow.Interview.createInterview('TransformProbe', new Map<String,Object>{'InputText' => 'Map sample'});
                f.start();
                Contact c = (Contact)f.getVariableValue('OutputRecord');
                System.assertEquals('Map sample', c.LastName);
                System.assertEquals(null, c.Title);
                System.assertEquals(true, c.getPopulatedFieldsAsMap().containsKey('Title'));
            `}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
