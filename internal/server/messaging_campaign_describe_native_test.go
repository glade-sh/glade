package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestMessagingCampaignRESTDescribeNativeRows(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/messaging_campaign_presence/routes.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "MessagingSession")
	handler := New(&org)
	checked := 0
	for _, row := range rows[1:] {
		if row[1] != "rest-describe" {
			continue
		}
		checked++
		t.Run(row[0], func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v"+row[0]+"/sobjects/MessagingSession/describe", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d", rec.Code)
			}
			var payload struct {
				Fields []struct {
					Name string `json:"name"`
				} `json:"fields"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, field := range payload.Fields {
				if field.Name == "CampaignId" {
					found = true
				}
			}
			if found != (row[2] == "present") {
				t.Fatalf("present=%t native=%s", found, row[2])
			}
		})
	}
	if checked != 4 {
		t.Fatalf("native routes=%d", checked)
	}
}
