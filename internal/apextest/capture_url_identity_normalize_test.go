package apextest

import (
	"reflect"
	"testing"
)

func TestCaptureURLAndShortIdentityNormalization(t *testing.T) {
	a := map[string]any{"job": "707000000000001AAA", "row": map[string]any{"Id": "08e000000000001AAA", "url": "/services/data/v68.0/sobjects/CronTrigger/08e000000000001AAA"}}
	b := map[string]any{"job": "707000000000002", "row": map[string]any{"Id": "08e000000000002", "url": "/services/data/v68.0/sobjects/CronTrigger/08e000000000002"}}
	if !reflect.DeepEqual(normalizeCaptureIdentities(a), normalizeCaptureIdentities(b)) {
		t.Fatal("short and long forms did not preserve identity")
	}
	b["row"].(map[string]any)["url"] = "/services/data/v65.0/sobjects/CronTrigger/08e000000000002"
	if reflect.DeepEqual(normalizeCaptureIdentities(a), normalizeCaptureIdentities(b)) {
		t.Fatal("URL API version was lost")
	}
}
