package storage

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"
)

// The DML-only provenance must not change the persisted Record encoding.
func TestDMLRecordTypeProvenanceDoesNotPersist(t *testing.T) {
	base := Record{Object: "Account", Fields: map[string]Value{"RecordTypeId": IDValue("012000000000001")}}
	marked := base.Clone()
	marker, ok := any(&marked).(interface{ MarkDMLRecordTypeDefaulted() })
	if !ok {
		t.Fatal("DML record type provenance marker missing")
	}
	marker.MarkDMLRecordTypeDefaulted()
	if got, ok := any(marked).(interface{ DMLRecordTypeDefaulted() bool }); !ok || !got.DMLRecordTypeDefaulted() {
		t.Fatal("DML provenance was not retained in memory")
	}
	changed := marked.Clone()
	changed.Fields["RecordTypeId"] = IDValue("012000000000002")
	if got, ok := any(changed).(interface{ DMLRecordTypeDefaulted() bool }); !ok || got.DMLRecordTypeDefaulted() {
		t.Fatal("changed record type was mistaken for the original default")
	}
	plainJSON, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	markedJSON, err := json.Marshal(marked)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plainJSON, markedJSON) {
		t.Fatalf("transient provenance changed JSON: %s != %s", plainJSON, markedJSON)
	}
	encode := func(r Record) []byte {
		var b bytes.Buffer
		if err := gob.NewEncoder(&b).Encode(r); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	plainGob, markedGob := encode(base), encode(marked)
	if !bytes.Equal(plainGob, markedGob) {
		t.Fatal("transient provenance changed gob encoding")
	}
	var restored Record
	if err := gob.NewDecoder(bytes.NewReader(markedGob)).Decode(&restored); err != nil {
		t.Fatal(err)
	}
	if got, ok := any(restored).(interface{ DMLRecordTypeDefaulted() bool }); !ok || got.DMLRecordTypeDefaulted() {
		t.Fatal("DML provenance persisted in gob")
	}
}
