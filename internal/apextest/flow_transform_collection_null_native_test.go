package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowTransformCollectionNullUpdateNative(t *testing.T) {
	const root = "testdata/native-capture/flow_transform_collection_null"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := filepath.Join(root, "api"+strings.TrimSuffix(api, ".0"), "TransformCollection.flow-meta.xml")
			metadata, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{"force-app/main/default/flows/TransformCollection.flow-meta.xml": string(metadata)}
			methods := map[string]string{"collectionNullUpdate": `
				Contact original = new Contact(LastName='Update sample', Title='Prior title'); insert original;
				Flow.Interview f = Flow.Interview.createInterview('TransformCollection', new Map<String,Object>{'InputText'=>'Update sample','InputId'=>String.valueOf(original.Id)}); f.start();
				List<Contact> items = (List<Contact>)f.getVariableValue('OutputRecords');
				System.assertEquals(1, items.size());
				System.assertEquals(true, items[0].getPopulatedFieldsAsMap().containsKey('Title'));
				update items; Contact after = [SELECT Title FROM Contact WHERE Id = :original.Id];
				System.assertEquals(null, after.Title);
			`}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}
