package sema

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAsyncWebserviceCalloutQualifiedAnonymousSignature(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/async_webservice_callout/"
	source, err := os.ReadFile(fixture + "anonymous/rejected_signature.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			f, err := os.Open(fixture + "native/api" + api + "/org.tsv")
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
			want := ""
			for _, row := range rows[1:] {
				if row[0] == "anonymous-reject" && row[1] == "R029" {
					want = strings.TrimPrefix(row[2], "REJECT|")
				}
			}
			if want == "" {
				t.Fatal("missing native R029")
			}
			result := AnalyzeAnonymous(typesys.Index{}, string(source), api+".0")
			count := 0
			for _, got := range result.Diagnostics {
				if got.Severity == diagnostic.Error {
					count++
					if got.Message != want {
						t.Errorf("native=%q, Glade=%q", want, got.Message)
					}
				}
			}
			if count == 0 {
				t.Fatalf("native rejects qualified call with %q; Glade accepted", want)
			}
		})
	}
}
