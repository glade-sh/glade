package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowMergeNativeServerRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			dir := t.TempDir()
			var paths []string
			for _, name := range []string{"Format_constant.flow-meta.xml", "Format_datetime.flow-meta.xml"} {
				b, err := os.ReadFile(filepath.Join("../automation/testdata/flow_merge_format", name))
				if err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(dir, name)
				if err = os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "62.0", api)), 0600); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, p)
			}
			idx, err := automation.LoadProject(project.Project{FlowFiles: paths})
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Diagnostics) > 0 {
				t.Fatalf("flow diagnostics: %v", idx.Diagnostics)
			}
			org := testOrg()
			org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{storage.DefaultUserID: {Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("en_US"), "TimeZoneSidKey": storage.StringValue("America/Los_Angeles")}}}}
			automation.ApplyToOrg(&org, idx)
			handler := New(&org)
			for _, tc := range []struct{ name, want string }{{"constant", `constant=1,234.5|negative=-1,234.5|boundary=1,000|below=999.5`}, {"datetime", `datetime=`}} {
				t.Run(tc.name, func(t *testing.T) {
					body := fmt.Sprintf(`{"Name":"FORMAT_%s"}`, strings.ToUpper(tc.name))
					req := httptest.NewRequest(http.MethodPost, "/services/data/v67.0/sobjects/Account", strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
						t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
					}
					if tc.name == "datetime" && !regexp.MustCompile(`datetime=[0-9]{1,2}/[0-9]{1,2}/[0-9]{4}, [0-9]{1,2}:[0-9]{2} [AP]M`).MatchString(rec.Body.String()) {
						t.Fatalf("datetime: %s", rec.Body.String())
					}
				})
			}
		})
	}
}
