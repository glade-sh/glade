package apextest

import (
	"reflect"
	"testing"
)

func TestCaptureIdentityNormalization(t *testing.T) {
	a := map[string]any{"before": []any{"001000000000001AAA", "001000000000002AAA"}, "ordered": []any{"001000000000002AAA", "001000000000001AAA"}, "userId": "005000000000001AAA", "text": "ordinary"}
	b := map[string]any{"before": []any{"001000000000011AAA", "001000000000012AAA"}, "ordered": []any{"001000000000012AAA", "001000000000011AAA"}, "userId": "005000000000011AAA", "text": "ordinary"}
	if !reflect.DeepEqual(normalizeCaptureIdentities(a), normalizeCaptureIdentities(b)) {
		t.Fatal("equivalent identities did not match")
	}
	b["ordered"] = []any{"001000000000011AAA", "001000000000012AAA"}
	if reflect.DeepEqual(normalizeCaptureIdentities(a), normalizeCaptureIdentities(b)) {
		t.Fatal("different identity order was lost")
	}
}

func TestCaptureAsyncIdentityNormalization(t *testing.T) {
	a := map[string]any{"job": "707000000000001AAA", "cron": []any{"08e000000000001AAA", "08e000000000001AAA"}}
	b := map[string]any{"job": "707000000000002AAA", "cron": []any{"08e000000000002AAA", "08e000000000002AAA"}}
	if !reflect.DeepEqual(normalizeCaptureIdentities(a), normalizeCaptureIdentities(b)) {
		t.Fatal("async identities did not match")
	}
	b["cron"] = []any{"08e000000000002AAA", "08e000000000003AAA"}
	if reflect.DeepEqual(normalizeCaptureIdentities(a), normalizeCaptureIdentities(b)) {
		t.Fatal("distinct async identities were collapsed")
	}
}
