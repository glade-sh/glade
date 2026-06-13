package server

import (
	"net/http"
	"strings"
)

type salesforceError struct {
	ErrorCode string   `json:"errorCode"`
	Message   string   `json:"message"`
	Fields    []string `json:"fields,omitempty"`
}

type serverErrorKind string

const (
	errUnknownEndpoint      serverErrorKind = "unknown_endpoint"
	errUnknownObject        serverErrorKind = "unknown_object"
	errUnknownRecord        serverErrorKind = "unknown_record"
	errUnknownSObject       serverErrorKind = "unknown_sobject_endpoint"
	errUnknownGLADE         serverErrorKind = "unknown_glade_endpoint"
	errUnknownTooling       serverErrorKind = "unknown_tooling_endpoint"
	errUnknownComposite     serverErrorKind = "unknown_composite_endpoint"
	errMethodNotAllowed     serverErrorKind = "method_not_allowed"
	errMalformedJSON        serverErrorKind = "malformed_json"
	errMalformedQuery       serverErrorKind = "malformed_query"
	errMalformedID          serverErrorKind = "malformed_id"
	errInvalidField         serverErrorKind = "invalid_field"
	errInvalidReset         serverErrorKind = "invalid_reset"
	errInvalidFixture       serverErrorKind = "invalid_fixture"
	errUnsupportedFeature   serverErrorKind = "unsupported_feature"
	errDMLFailure           serverErrorKind = "dml_failure"
	errStoreFailure         serverErrorKind = "store_failure"
	errRequiredFieldMissing serverErrorKind = "required_field_missing"
	errDuplicateValue       serverErrorKind = "duplicate_value"
	errRequestLimitExceeded serverErrorKind = "request_limit_exceeded"
)

type serverErrorSpec struct {
	status  int
	code    string
	message string
}

var serverErrorSpecs = map[serverErrorKind]serverErrorSpec{
	errUnknownEndpoint:      {status: http.StatusNotFound, code: "NOT_FOUND", message: "unknown endpoint"},
	errUnknownObject:        {status: http.StatusNotFound, code: "NOT_FOUND", message: "unknown object"},
	errUnknownRecord:        {status: http.StatusNotFound, code: "NOT_FOUND", message: "record not found"},
	errUnknownSObject:       {status: http.StatusNotFound, code: "NOT_FOUND", message: "unknown sobject endpoint"},
	errUnknownGLADE:         {status: http.StatusNotFound, code: "NOT_FOUND", message: "unknown glade endpoint"},
	errUnknownTooling:       {status: http.StatusNotFound, code: "NOT_FOUND", message: "unknown tooling endpoint"},
	errUnknownComposite:     {status: http.StatusNotFound, code: "NOT_FOUND", message: "unknown composite endpoint"},
	errMethodNotAllowed:     {status: http.StatusMethodNotAllowed, code: "METHOD_NOT_ALLOWED", message: "method not allowed"},
	errMalformedJSON:        {status: http.StatusBadRequest, code: "JSON_PARSER_ERROR", message: "malformed JSON"},
	errMalformedQuery:       {status: http.StatusBadRequest, code: "MALFORMED_QUERY", message: "malformed query"},
	errMalformedID:          {status: http.StatusBadRequest, code: "MALFORMED_ID", message: "malformed id"},
	errInvalidField:         {status: http.StatusBadRequest, code: "INVALID_FIELD", message: "invalid field"},
	errInvalidReset:         {status: http.StatusBadRequest, code: "INVALID_RESET", message: "invalid reset"},
	errInvalidFixture:       {status: http.StatusBadRequest, code: "INVALID_FIXTURE", message: "invalid fixture"},
	errUnsupportedFeature:   {status: http.StatusNotImplemented, code: "UNSUPPORTED_FEATURE", message: "unsupported feature"},
	errDMLFailure:           {status: http.StatusBadRequest, code: "DML_EXCEPTION", message: "DML operation failed"},
	errStoreFailure:         {status: http.StatusInternalServerError, code: "SERVER_ERROR", message: "store failure"},
	errRequiredFieldMissing: {status: http.StatusBadRequest, code: "REQUIRED_FIELD_MISSING", message: "required field missing"},
	errDuplicateValue:       {status: http.StatusBadRequest, code: "DUPLICATE_VALUE", message: "duplicate value"},
	errRequestLimitExceeded: {status: http.StatusBadRequest, code: "REQUEST_LIMIT_EXCEEDED", message: "request limit exceeded"},
}

func writeSalesforceError(w http.ResponseWriter, kind serverErrorKind, message ...string) {
	spec, ok := serverErrorSpecs[kind]
	if !ok {
		spec = serverErrorSpec{status: http.StatusInternalServerError, code: "SERVER_ERROR", message: "server error"}
	}
	msg := spec.message
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	writeJSON(w, spec.status, []salesforceError{{ErrorCode: spec.code, Message: msg}})
}

func salesforceErrorCode(kind serverErrorKind) string {
	spec, ok := serverErrorSpecs[kind]
	if !ok {
		return "SERVER_ERROR"
	}
	return spec.code
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed ...string) {
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
	}
	writeSalesforceError(w, errMethodNotAllowed)
}
