package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// BenchmarkPhoneDuplicateScan measures a custom phone rule over numeric records.
func BenchmarkPhoneDuplicateScan(b *testing.B) {
	org := duplicateScanOrg(5000, false)
	st := org.Objects["Account"]
	for id, record := range st.Records {
		record.Fields["Phone"] = storage.StringValue(fmt.Sprintf("1-800-%07d", int64(idNumber(id))%10000000))
		st.Records[id] = record
	}
	org.Objects["Account"] = st
	rule := fuzzyRule(fuzzyNativeRow{Object: "Account", Field: "Phone", Method: "Phone", Blank: "NullNotAllowed"})
	candidate := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Phone": storage.StringValue("1-800-999-9999")}}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, supported := FindDuplicateMatches(org, rule, candidate, nil)
		if !supported || result.HasRecords() {
			b.Fatal("unexpected scan result")
		}
	}
}

func idNumber(id storage.ID) int {
	var n int
	fmt.Sscanf(string(id), "001%d", &n)
	return n
}
