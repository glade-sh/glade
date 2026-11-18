package automation

import "testing"

// Salesforce rejects duplicate output fields in the captured Transform metadata.
func TestFlowTransformDuplicateTargetUnmodeled(t *testing.T) {
	for _, fields := range [][]string{{"Title", "Title"}, {"LastName", "LastName"}} {
		actions := make([]flowTransformValueActionXML, 0, len(fields))
		for _, field := range fields {
			actions = append(actions, flowTransformValueActionXML{
				OutputFieldAPIName: field, TransformType: "Map", Value: flowValueXML{ElementReference: "InputText"},
			})
		}
		transform := flowTransformXML{Name: "Mapped", DataType: "SObject", ObjectType: "Contact", TransformValues: []flowTransformValuesXML{{Actions: actions}}}
		if _, ok := modeledFlowTransform(transform, nil, nil); ok {
			t.Fatalf("duplicate field %s was modeled", fields[0])
		}
	}
}
