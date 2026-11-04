package storage

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Old persisted Flow rules have no ownership marker; new records retain it.
func TestFlowProjectSourceOwnershipJSONCompatibility(t *testing.T) {
	var old FlowRule
	if err := json.Unmarshal([]byte(`{"name":"Old","file":"/synthetic/Old.flow"}`), &old); err != nil {
		t.Fatal(err)
	}
	if field := reflect.ValueOf(old).FieldByName("ProjectSourceOwned"); field.IsValid() && field.Bool() {
		t.Fatal("legacy rule unexpectedly marked")
	}
	newRule := FlowRule{Name: "New"}
	field := reflect.ValueOf(&newRule).Elem().FieldByName("ProjectSourceOwned")
	if !field.IsValid() {
		t.Fatal("ownership field missing")
	}
	field.SetBool(true)
	encoded, err := json.Marshal(newRule)
	if err != nil {
		t.Fatal(err)
	}
	var current FlowRule
	if err := json.Unmarshal(encoded, &current); err != nil {
		t.Fatal(err)
	}
	if !reflect.ValueOf(current).FieldByName("ProjectSourceOwned").Bool() {
		t.Fatal("ownership lost in JSON round trip")
	}
}
