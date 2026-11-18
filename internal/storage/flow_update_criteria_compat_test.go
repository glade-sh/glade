package storage

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"
)

func TestFlowRecordUpdateCriteriaLogicCompatibility(t *testing.T) {
	const legacy = `{"name":"Update","objectName":"Cedar__c","criteria":[{"field":"Id","operation":"equals","sourceField":"recId"}]}`
	var old FlowRecordUpdate
	if err := json.Unmarshal([]byte(legacy), &old); err != nil {
		t.Fatal(err)
	}
	if old.CriteriaLogic != "" || len(old.Criteria) != 1 {
		t.Fatalf("legacy update changed: %+v", old)
	}
	old.CriteriaLogic = "1 AND (2 OR 3 OR 4 OR 5)"
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var fromJSON FlowRecordUpdate
	if err := json.Unmarshal(data, &fromJSON); err != nil {
		t.Fatal(err)
	}
	if fromJSON.CriteriaLogic != old.CriteriaLogic {
		t.Fatalf("JSON lost criteria logic: %+v", fromJSON)
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(old); err != nil {
		t.Fatal(err)
	}
	var fromGob FlowRecordUpdate
	if err := gob.NewDecoder(&buf).Decode(&fromGob); err != nil {
		t.Fatal(err)
	}
	if fromGob.CriteriaLogic != old.CriteriaLogic {
		t.Fatalf("gob lost criteria logic: %+v", fromGob)
	}
}

func TestProcessStringConstantsCloneAndSerialization(t *testing.T) {
	def := ObjectDefinition{FlowRules: []FlowRule{{Name: "Sub", ProcessStringConstants: map[string]string{"fixed": "value"}}}}
	clone := def.Clone()
	clone.FlowRules[0].ProcessStringConstants["fixed"] = "changed"
	if def.FlowRules[0].ProcessStringConstants["fixed"] != "value" {
		t.Fatal("clone shares process constants")
	}
	encoded, err := json.Marshal(def.FlowRules[0])
	if err != nil {
		t.Fatal(err)
	}
	var decoded FlowRule
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ProcessStringConstants["fixed"] != "value" {
		t.Fatalf("JSON lost constant: %+v", decoded.ProcessStringConstants)
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(def.FlowRules[0]); err != nil {
		t.Fatal(err)
	}
	decoded = FlowRule{}
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ProcessStringConstants["fixed"] != "value" {
		t.Fatalf("gob lost constant: %+v", decoded.ProcessStringConstants)
	}
}
