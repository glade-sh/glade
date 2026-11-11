package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The native capture observes null write-back when a mapped record is updated.
func TestFlowTransformRecordNullUpdateNative(t *testing.T) {
	const root = "testdata/native-capture/flow_transform_record_update"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := filepath.Join(root, "api"+strings.TrimSuffix(api, ".0"), "TransformUpdate.flow-meta.xml")
			metadata, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{"force-app/main/default/flows/TransformUpdate.flow-meta.xml": string(metadata)}
			methods := map[string]string{"nullUpdate": `
				Contact before = new Contact(LastName='Update sample', Title='Prior title'); insert before;
				Flow.Interview f = Flow.Interview.createInterview('TransformUpdate', new Map<String,Object>{'InputText'=>'Update sample','InputId'=>String.valueOf(before.Id)});
				f.start(); Contact result = (Contact)f.getVariableValue('OutputRecord');
				System.assertEquals(true, result.getPopulatedFieldsAsMap().containsKey('Title'));
				update result; Contact after = [SELECT Title FROM Contact WHERE Id = :before.Id];
				System.assertEquals(null, after.Title);
			`}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
