package server

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const bulkV1TestPath = "/services/async/65.0"

func TestBulkV1InsertCSVCreatesRecords(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	jobID := createBulkV1TestJob(t, handler, "insert", "Account", "")

	upload := httptest.NewRecorder()
	handler.ServeHTTP(upload, httptest.NewRequest(http.MethodPost, bulkV1TestPath+"/job/"+jobID+"/batch", strings.NewReader("Name,Description\nTrail Cabin,Hand hewn\n")))
	if upload.Code != http.StatusCreated {
		t.Fatalf("batch status = %d body=%s", upload.Code, upload.Body.String())
	}

	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("record count = %d", len(org.Objects["Account"].Records))
	}
	for id, record := range org.Objects["Account"].Records {
		if !strings.HasPrefix(string(id), "001") {
			t.Fatalf("record id = %s", id)
		}
		if got := record.Fields["Name"].String; got != "Trail Cabin" {
			t.Fatalf("Name = %q", got)
		}
		if got := record.Fields["Description"].String; got != "Hand hewn" {
			t.Fatalf("Description = %q", got)
		}
	}
}

func TestBulkV1UpsertCSVUpdatesByExternalID(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Before"),
			"External_Id__c": storage.StringValue("EXT-1"),
		},
	}
	org.Objects["Account"] = object
	handler := New(&org)

	jobID := createBulkV1TestJob(t, handler, "upsert", "Account", "External_Id__c")

	upload := httptest.NewRecorder()
	handler.ServeHTTP(upload, httptest.NewRequest(http.MethodPost, bulkV1TestPath+"/job/"+jobID+"/batch", strings.NewReader("External_Id__c,Name\nEXT-1,After\nEXT-2,New Cabin\n")))
	if upload.Code != http.StatusCreated {
		t.Fatalf("batch status = %d body=%s", upload.Code, upload.Body.String())
	}

	if got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String; got != "After" {
		t.Fatalf("updated Name = %q", got)
	}
	if len(org.Objects["Account"].Records) != 2 {
		t.Fatalf("record count = %d", len(org.Objects["Account"].Records))
	}
}

func TestBulkV1BatchStatusAndResultCSV(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	jobID := createBulkV1TestJob(t, handler, "insert", "Account", "")
	batchID := uploadBulkV1TestBatch(t, handler, jobID, "Name,Description\nBank Vault Door,Slick\n")

	status := httptest.NewRecorder()
	handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, bulkV1TestPath+"/job/"+jobID+"/batch/"+batchID, nil))
	if status.Code != http.StatusOK {
		t.Fatalf("status code = %d body=%s", status.Code, status.Body.String())
	}
	var batch struct {
		ID     string `xml:"id"`
		JobID  string `xml:"jobId"`
		State  string `xml:"state"`
		Number int    `xml:"numberRecordsProcessed"`
	}
	if err := xml.Unmarshal(status.Body.Bytes(), &batch); err != nil {
		t.Fatalf("decode batch XML: %v body=%s", err, status.Body.String())
	}
	if batch.ID != batchID || batch.JobID != jobID || batch.State != "Completed" || batch.Number != 1 {
		t.Fatalf("batch info = %#v", batch)
	}

	result := httptest.NewRecorder()
	handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, bulkV1TestPath+"/job/"+jobID+"/batch/"+batchID+"/result", nil))
	if result.Code != http.StatusOK {
		t.Fatalf("result status = %d body=%s", result.Code, result.Body.String())
	}
	rows, err := csv.NewReader(bytes.NewReader(result.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("decode result CSV: %v body=%s", err, result.Body.String())
	}
	if len(rows) != 2 {
		t.Fatalf("result rows = %#v", rows)
	}
	if strings.Join(rows[0], ",") != "Id,Success,Created,Error" {
		t.Fatalf("result header = %#v", rows[0])
	}
	if rows[1][0] == "" || rows[1][1] != "true" || rows[1][2] != "true" || rows[1][3] != "" {
		t.Fatalf("result row = %#v", rows[1])
	}

	closeJob := httptest.NewRecorder()
	handler.ServeHTTP(closeJob, httptest.NewRequest(http.MethodPost, bulkV1TestPath+"/job/"+jobID, strings.NewReader(`<jobInfo><state>Closed</state></jobInfo>`)))
	if closeJob.Code != http.StatusOK {
		t.Fatalf("close status = %d body=%s", closeJob.Code, closeJob.Body.String())
	}
	var closed struct {
		ID    string `xml:"id"`
		State string `xml:"state"`
	}
	if err := xml.Unmarshal(closeJob.Body.Bytes(), &closed); err != nil {
		t.Fatalf("decode close XML: %v body=%s", err, closeJob.Body.String())
	}
	if closed.ID != jobID || closed.State != "Closed" {
		t.Fatalf("closed job = %#v", closed)
	}
}

func TestBulkV1RejectsUnsupportedOperations(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, operation := range []string{"query", "delete", "hardDelete", "update"} {
		t.Run(operation, func(t *testing.T) {
			rec := httptest.NewRecorder()
			body := bulkV1TestJobInfo(operation, "Account", "")
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, bulkV1TestPath+"/job", strings.NewReader(body)))
			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "Bulk API v1") || !strings.Contains(rec.Body.String(), operation) {
				t.Fatalf("body = %s", rec.Body.String())
			}
		})
	}
}

func createBulkV1TestJob(t *testing.T, handler http.Handler, operation, objectName, externalIDField string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, bulkV1TestPath+"/job", strings.NewReader(bulkV1TestJobInfo(operation, objectName, externalIDField))))
	if rec.Code != http.StatusCreated {
		t.Fatalf("job status = %d body=%s", rec.Code, rec.Body.String())
	}
	var job struct {
		ID string `xml:"id"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job XML: %v body=%s", err, rec.Body.String())
	}
	if job.ID == "" {
		t.Fatalf("job id empty: %s", rec.Body.String())
	}
	return job.ID
}

func uploadBulkV1TestBatch(t *testing.T, handler http.Handler, jobID, body string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, bulkV1TestPath+"/job/"+jobID+"/batch", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("batch status = %d body=%s", rec.Code, rec.Body.String())
	}
	var batch struct {
		ID string `xml:"id"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &batch); err != nil {
		t.Fatalf("decode batch XML: %v body=%s", err, rec.Body.String())
	}
	if batch.ID == "" {
		t.Fatalf("batch id empty: %s", rec.Body.String())
	}
	return batch.ID
}

func bulkV1TestJobInfo(operation, objectName, externalIDField string) string {
	var b strings.Builder
	b.WriteString(`<jobInfo>`)
	b.WriteString(`<operation>` + operation + `</operation>`)
	b.WriteString(`<object>` + objectName + `</object>`)
	b.WriteString(`<contentType>CSV</contentType>`)
	if externalIDField != "" {
		b.WriteString(`<externalIdFieldName>` + externalIDField + `</externalIdFieldName>`)
	}
	b.WriteString(`</jobInfo>`)
	return b.String()
}
