package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const recordTypeNullRoutesFixture = "testdata/conformance/record_type_null_routes"

// recordTypeNullOutcome is what a route's answer means: the first error and
// the record type the record ends up with.
type recordTypeNullOutcome struct {
	Code, Message, Fields, RecordType string
}

func recordTypeNullNativeOutcome(t *testing.T, probe, observation string) recordTypeNullOutcome {
	t.Helper()
	var row struct {
		Body       json.RawMessage `json:"body"`
		Error      json.RawMessage `json:"error"`
		RecordType *string         `json:"recordType"`
	}
	if err := json.Unmarshal([]byte(observation), &row); err != nil {
		t.Fatalf("%s: %v", probe, err)
	}
	out := recordTypeNullOutcome{}
	if row.RecordType != nil {
		out.RecordType = *row.RecordType
	}
	raw := row.Body
	if strings.HasPrefix(probe, "ui.") {
		raw = row.Error
		var enhanced struct {
			Output struct {
				FieldErrors map[string][]struct {
					ErrorCode string `json:"errorCode"`
					Field     string `json:"field"`
					Message   string `json:"message"`
				} `json:"fieldErrors"`
			} `json:"output"`
		}
		if json.Unmarshal(raw, &enhanced) == nil {
			for _, items := range enhanced.Output.FieldErrors {
				for _, item := range items {
					out.Code, out.Fields, out.Message = item.ErrorCode, item.Field, html.UnescapeString(item.Message)
				}
			}
			if out.Code != "" {
				return out
			}
		}
	}
	code, message, fields := recordTypeNullFirstError(t, raw)
	out.Code, out.Message, out.Fields = code, message, fields
	return out
}

// recordTypeNullFirstError reads the REST error list, a collection result or a
// composite subresponse.
func recordTypeNullFirstError(t *testing.T, raw json.RawMessage) (string, string, string) {
	t.Helper()
	var composite struct {
		CompositeResponse []struct {
			Body json.RawMessage `json:"body"`
		} `json:"compositeResponse"`
	}
	if json.Unmarshal(raw, &composite) == nil && len(composite.CompositeResponse) == 1 {
		raw = composite.CompositeResponse[0].Body
	}
	var items []struct {
		ErrorCode string   `json:"errorCode"`
		Message   string   `json:"message"`
		Fields    []string `json:"fields"`
		Errors    []struct {
			StatusCode string   `json:"statusCode"`
			Message    string   `json:"message"`
			Fields     []string `json:"fields"`
		} `json:"errors"`
	}
	if json.Unmarshal(raw, &items) != nil || len(items) == 0 {
		return "", "", ""
	}
	first := items[0]
	if len(first.Errors) != 0 {
		return first.Errors[0].StatusCode, first.Errors[0].Message, strings.Join(first.Errors[0].Fields, ",")
	}
	return first.ErrorCode, first.Message, strings.Join(first.Fields, ",")
}

func recordTypeNullOrg(t *testing.T, api, phase string) storage.OrgState {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(recordTypeNullRoutesFixture, "project")
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		if phase == "noProfileDefault" && strings.Contains(rel, "profiles") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(rel, "sfdx-project.json") {
			raw = []byte(strings.NewReplacer("62.0", api, "67.0", api, "68.0", api).Replace(string(raw)))
		}
		writeServerTestFile(t, filepath.Join(root, rel), string(raw))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := apextest.OrgFromIndex(typesys.Build(p, schema))
	org.APIVersion = api
	return org
}

func recordTypeNullName(org *storage.OrgState, object string, id storage.ID) string {
	if id == "" {
		return ""
	}
	objectState := org.Objects[object]
	_, record, ok := storage.LookupRecordByID(objectState.Records, id)
	if !ok {
		return ""
	}
	value, ok := record.GetField("RecordTypeId")
	if !ok || value.Kind == storage.ValueNull {
		return "null"
	}
	for _, recordType := range objectState.Definition.RecordTypes {
		if storage.IDsEqual(recordType.ID, storage.ID(fmt.Sprint(storageValueJSON(value)))) {
			return recordType.DeveloperName
		}
	}
	return "unknown"
}

func recordTypeNullAlpha(t *testing.T, org *storage.OrgState) string {
	t.Helper()
	for _, recordType := range org.Objects["V35Rt__c"].Definition.RecordTypes {
		if recordType.DeveloperName == "Alpha" {
			return string(recordType.ID)
		}
	}
	t.Fatal("fixture has no Alpha record type")
	return ""
}

func recordTypeNullRequest(t *testing.T, org *storage.OrgState, method, path, body string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	New(org).ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// recordTypeNullRun plays one captured probe on a fresh org.
func recordTypeNullRun(t *testing.T, api, phase, probe string) recordTypeNullOutcome {
	t.Helper()
	org := recordTypeNullOrg(t, api, phase)
	alpha := recordTypeNullAlpha(t, &org)
	base := "/services/data/v" + api
	object := "V35Rt__c"
	seed := func() string {
		status, body := recordTypeNullRequest(t, &org, http.MethodPost, base+"/sobjects/"+object, `{"Name":"seed"}`)
		var created struct {
			ID string `json:"id"`
		}
		if status != http.StatusCreated || json.Unmarshal(body, &created) != nil || created.ID == "" {
			t.Fatalf("seed: %d %s", status, body)
		}
		return created.ID
	}
	fields := map[string]string{
		"unset": `"Name":"r"`, "null": `"Name":"r","RecordTypeId":null`, "alpha": `"Name":"r","RecordTypeId":"` + alpha + `"`,
	}
	idOf := func(raw []byte) storage.ID {
		var one struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &one) == nil && one.ID != "" {
			return storage.ID(one.ID)
		}
		var many []struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &many) == nil && len(many) == 1 {
			return storage.ID(many[0].ID)
		}
		return ""
	}
	finish := func(raw []byte, id storage.ID) recordTypeNullOutcome {
		code, message, errorFields := recordTypeNullFirstError(t, raw)
		return recordTypeNullOutcome{Code: code, Message: message, Fields: errorFields, RecordType: recordTypeNullName(&org, object, id)}
	}
	wire := func(result map[string]any, wireErr *lwcbrowser.WireError) recordTypeNullOutcome {
		if wireErr != nil {
			return recordTypeNullOutcome{Code: wireErr.Type, Message: wireErr.Message}
		}
		id, _ := result["id"].(string)
		return recordTypeNullOutcome{RecordType: recordTypeNullName(&org, object, storage.ID(id))}
	}
	switch {
	case strings.HasPrefix(probe, "rest.post.null."):
		object = strings.TrimPrefix(probe, "rest.post.null.")
		_, body := recordTypeNullRequest(t, &org, http.MethodPost, base+"/sobjects/"+object, `{"Name":"n","RecordTypeId":null}`)
		return finish(body, idOf(body))
	case strings.HasPrefix(probe, "ui.post.null."):
		object = strings.TrimPrefix(probe, "ui.post.null.")
		return wire(createRecordWireData(&org, lwcbrowser.WireCreateRecordRequest{APIName: object, Fields: map[string]any{"Name": "n", "RecordTypeId": nil}}))
	case strings.HasPrefix(probe, "rest.post."):
		_, body := recordTypeNullRequest(t, &org, http.MethodPost, base+"/sobjects/"+object, "{"+fields[strings.TrimPrefix(probe, "rest.post.")]+"}")
		return finish(body, idOf(body))
	case strings.HasPrefix(probe, "ui.post."):
		var input map[string]any
		if err := json.Unmarshal([]byte("{"+fields[strings.TrimPrefix(probe, "ui.post.")]+"}"), &input); err != nil {
			t.Fatal(err)
		}
		return wire(createRecordWireData(&org, lwcbrowser.WireCreateRecordRequest{APIName: object, Fields: input}))
	case probe == "rest.patch.null", probe == "rest.patch.other":
		id := seed()
		patch := `{"RecordTypeId":null}`
		if probe == "rest.patch.other" {
			patch = `{"Name":"x"}`
		}
		_, body := recordTypeNullRequest(t, &org, http.MethodPatch, base+"/sobjects/"+object+"/"+id, patch)
		return finish(body, storage.ID(id))
	case probe == "ui.patch.null":
		id := seed()
		return wire(updateRecordWireData(&org, lwcbrowser.WireUpdateRecordRequest{Fields: map[string]any{"Id": id, "RecordTypeId": nil}}))
	case strings.HasPrefix(probe, "collections.post.null."):
		allOrNone := strings.TrimPrefix(probe, "collections.post.null.allOrNone=") == "True"
		payload, _ := json.Marshal(map[string]any{"allOrNone": allOrNone, "records": []map[string]any{{"attributes": map[string]any{"type": object}, "Name": "c", "RecordTypeId": nil}}})
		_, body := recordTypeNullRequest(t, &org, http.MethodPost, base+"/composite/sobjects", string(payload))
		return finish(body, idOf(body))
	case probe == "collections.patch.null":
		id := seed()
		payload, _ := json.Marshal(map[string]any{"allOrNone": false, "records": []map[string]any{{"attributes": map[string]any{"type": object}, "Id": id, "RecordTypeId": nil}}})
		_, body := recordTypeNullRequest(t, &org, http.MethodPatch, base+"/composite/sobjects", string(payload))
		return finish(body, storage.ID(id))
	case probe == "composite.post.null":
		payload, _ := json.Marshal(map[string]any{"allOrNone": false, "compositeRequest": []map[string]any{{"method": "POST", "url": base + "/sobjects/" + object, "referenceId": "a", "body": map[string]any{"Name": "c", "RecordTypeId": nil}}}})
		_, body := recordTypeNullRequest(t, &org, http.MethodPost, base+"/composite", string(payload))
		return finish(body, "")
	}
	t.Fatalf("unknown probe %s", probe)
	return recordTypeNullOutcome{}
}

// recordTypeNullKnownGaps must still differ; a matching row fails as stale. A
// fresh org rejects a deployed record type until a profile grants it, but a
// retrieved project rarely carries the org's profile visibility, so Glade does
// not infer the absence (applyProjectProfileRecordTypeAvailability).
var recordTypeNullKnownGaps = map[string]bool{
	"noProfileDefault/rest.post.alpha": true,
	"noProfileDefault/ui.post.alpha":   true,
}

// TestRecordTypeNullRoutesNativeConformance replays the REST and UI rows
// (record_type_null_routes ORG-RUN.txt). The 62.0 rows equal the 67.0 rows, and
// the local REST window rejects v62.0 by design (rest_api_version_routes
// windowExclusions), so the replay runs at 67.0.
func TestRecordTypeNullRoutesNativeConformance(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(recordTypeNullRoutesFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 69 || lines[0] != "api\tphase\tprobe\tobservation" {
		t.Fatalf("incomplete rows: %d", len(lines))
	}
	byAPI := map[string]map[string]string{"62.0": {}, "67.0": {}}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 4)
		byAPI[cells[0]][cells[1]+"/"+cells[2]] = strings.ReplaceAll(cells[3], "v"+cells[0], "vAPI")
	}
	if !reflect.DeepEqual(byAPI["62.0"], byAPI["67.0"]) {
		t.Fatal("the 62.0 and 67.0 rows differ; replay both")
	}
	for key, observation := range byAPI["67.0"] {
		t.Run(key, func(t *testing.T) {
			phase, probe, _ := strings.Cut(key, "/")
			want := recordTypeNullNativeOutcome(t, probe, observation)
			want.Message = strings.ReplaceAll(want.Message, "<ALPHA>", "")
			got := recordTypeNullRun(t, "67.0", phase, probe)
			match := got.Code == want.Code && got.RecordType == want.RecordType && strings.Contains(got.Message, want.Message)
			if recordTypeNullKnownGaps[key] {
				if match {
					t.Errorf("stale known gap: %+v", got)
				}
				return
			}
			if !match {
				t.Errorf("native %+v; Glade %+v", want, got)
			}
		})
	}
}

// TestRESTUnknownFieldNativeConformance replays unknown_field.tsv at 67.0: the
// 62.0 rows equal them, and the local REST window rejects v62.0 by design.
func TestRESTUnknownFieldNativeConformance(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(recordTypeNullRoutesFixture, "unknown_field.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 11 || lines[0] != "api\tprobe\tobservation" {
		t.Fatalf("incomplete rows: %d", len(lines))
	}
	byAPI := map[string]map[string]string{"62.0": {}, "67.0": {}}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		byAPI[cells[0]][cells[1]] = cells[2]
	}
	if !reflect.DeepEqual(byAPI["62.0"], byAPI["67.0"]) {
		t.Fatal("the 62.0 and 67.0 rows differ; replay both")
	}
	for probe, observation := range byAPI["67.0"] {
		t.Run(probe, func(t *testing.T) {
			var want struct {
				Status int             `json:"status"`
				Body   json.RawMessage `json:"body"`
			}
			if err := json.Unmarshal([]byte(observation), &want); err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			addAccountForTest(&org, "001000000000001", "Acme")
			base, record := serverTestDataPath, "001000000000001"
			field := `"Nope__c":"x"`
			if probe == "rest.patch.recordTypeId" {
				field = `"RecordTypeId":null`
			}
			var method, path, body string
			switch probe {
			case "rest.post.unknown":
				method, path, body = http.MethodPost, base+"/sobjects/Account", `{"Name":"n",`+field+`}`
			case "rest.patch.unknown", "rest.patch.recordTypeId":
				method, path, body = http.MethodPatch, base+"/sobjects/Account/"+record, `{`+field+`}`
			case "collections.post.unknown":
				method, path, body = http.MethodPost, base+"/composite/sobjects", `{"allOrNone":false,"records":[{"attributes":{"type":"Account"},"Name":"n",`+field+`}]}`
			case "collections.patch.unknown":
				method, path, body = http.MethodPatch, base+"/composite/sobjects", `{"allOrNone":false,"records":[{"attributes":{"type":"Account"},"Id":"`+record+`",`+field+`}]}`
			default:
				t.Fatalf("unknown probe %s", probe)
			}
			status, got := recordTypeNullRequest(t, &org, method, path, body)
			wantCode, wantMessage, wantFields := recordTypeNullFirstError(t, want.Body)
			gotCode, gotMessage, gotFields := recordTypeNullFirstError(t, got)
			if status != want.Status || gotCode != wantCode || gotMessage != wantMessage || gotFields != wantFields {
				t.Errorf("native %d %s %q %q; Glade %d %s", want.Status, wantCode, wantMessage, wantFields, status, got)
			}
		})
	}
}
