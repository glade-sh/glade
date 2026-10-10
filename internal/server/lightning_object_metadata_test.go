package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
)

func TestObjectMetadataErrorEnvelopeCapturedScope(t *testing.T) {
	for _, tc := range []struct {
		field, code string
		want        bool
	}{
		{"Account.Name", "INVALID_FIELD", true},
		{"Account.Phone", "INVALID_FIELD", false},
		{"Contact.Name", "INVALID_FIELD", false},
		{"Name", "INVALID_FIELD", false},
		{"Account.Name", "NOT_FOUND", false},
	} {
		if got := capturedPicklistErrorEnvelope(lwcbrowser.WireGetPicklistValuesRequest{FieldAPIName: tc.field}, tc.code); got != tc.want {
			t.Errorf("picklist %s / %s: envelope %v, want %v", tc.field, tc.code, got, tc.want)
		}
	}
	for _, tc := range []struct {
		object, code string
		fields       []string
		want         bool
	}{
		{"Account", "ILLEGAL_QUERY_PARAMETER_VALUE", []string{"Name"}, true},
		{"Account", "ILLEGAL_QUERY_PARAMETER_VALUE", []string{"Phone"}, false},
		{"Contact", "ILLEGAL_QUERY_PARAMETER_VALUE", []string{"Name"}, false},
		{"Account", "ILLEGAL_QUERY_PARAMETER_VALUE", []string{"Phone", "Name"}, false},
		{"Account", "NOT_FOUND", []string{"Name"}, false},
	} {
		req := lwcbrowser.WireGetRecordCreateDefaultsRequest{ObjectAPIName: tc.object, OptionalFields: tc.fields}
		if got := capturedCreateDefaultsErrorEnvelope(req, tc.code); got != tc.want {
			t.Errorf("defaults %s / %v / %s: envelope %v, want %v", tc.object, tc.fields, tc.code, got, tc.want)
		}
	}
}

func TestObjectMetadataErrorEnvelopeIndependentOfOpaqueID(t *testing.T) {
	// These two exact bodies come from the API 59/67 native rows
	// r_pick_not_picklist and r_defaults_optional_unqualified.
	for _, tc := range []struct {
		code, message, body string
	}{
		{"INVALID_FIELD", "Field Name is not a picklist.", `{"errorCode":"INVALID_FIELD","id":"-1797869752","message":"Field Name is not a picklist.","statusCode":400}`},
		{"ILLEGAL_QUERY_PARAMETER_VALUE", "Expected '.' in all qualified names: Name is invalid", `{"errorCode":"ILLEGAL_QUERY_PARAMETER_VALUE","id":"-116886043","message":"Expected '.' in all qualified names: Name is invalid","statusCode":400}`},
	} {
		rec := httptest.NewRecorder()
		if !writeObjectMetadataDataError(rec, true, tc.code, tc.message) {
			t.Fatal("captured condition did not select envelope")
		}
		want := "{\"error\":{\"body\":" + tc.body + ",\"errorType\":\"fetchResponse\",\"headers\":{},\"ok\":false,\"status\":400,\"statusText\":\"Bad Request\"}}\n"
		if rec.Code != http.StatusBadRequest || rec.Body.String() != want {
			t.Errorf("native envelope: status %d body %q, want %q", rec.Code, rec.Body.String(), want)
		}
		rec = httptest.NewRecorder()
		if writeObjectMetadataDataError(rec, false, tc.code, tc.message) || rec.Body.Len() != 0 || rec.Code != http.StatusOK {
			t.Fatal("captured message or opaque ID selected an uncaptured request envelope")
		}
	}
	// A local structural control: an unknown ID cannot change an already
	// selected envelope, and no opaque ID is invented for different text.
	rec := httptest.NewRecorder()
	if !writeObjectMetadataDataError(rec, true, "INVALID_FIELD", "different text") {
		t.Fatal("missing opaque ID suppressed selected envelope")
	}
	want := "{\"error\":{\"body\":{\"errorCode\":\"INVALID_FIELD\",\"message\":\"different text\",\"statusCode\":400},\"errorType\":\"fetchResponse\",\"headers\":{},\"ok\":false,\"status\":400,\"statusText\":\"Bad Request\"}}\n"
	if rec.Code != http.StatusBadRequest || rec.Body.String() != want {
		t.Fatalf("unknown opaque ID: status %d body %q, want %q", rec.Code, rec.Body.String(), want)
	}
}
