package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowTransformStringLiteralNative(t *testing.T) {
	const root = "testdata/native-capture/flow_transform_literal"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, tc := range []struct{ name, want string }{{"TransformLiteral", "Mapped"}, {"TransformEmpty", ""}} {
				t.Run(tc.name, func(t *testing.T) {
					path := filepath.Join(root, "api"+strings.TrimSuffix(api, ".0"), tc.name+".flow-meta.xml")
					flow, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					files := map[string]string{"force-app/main/default/flows/" + tc.name + ".flow-meta.xml": string(flow)}
					methods := map[string]string{"literal": `
						Flow.Interview f = Flow.Interview.createInterview('` + tc.name + `', new Map<String,Object>{'InputText'=>'Sample'}); f.start();
						Contact c=(Contact)f.getVariableValue('OutputRecord');
						System.assertEquals('Sample',c.LastName);
						System.assertEquals('` + tc.want + `',c.Title);
						System.assertEquals(true,c.getPopulatedFieldsAsMap().containsKey('Title'));
					`}
					runFlowPreservationProjectRoutes(t, api, files, methods)
				})
			}
		})
	}
}
