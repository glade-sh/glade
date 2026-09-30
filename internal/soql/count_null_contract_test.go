package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCountNullContractCountsRowsAndNonNullValues(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	const omittedRevenueID storage.ID = "001000000000004"
	account.Records[omittedRevenueID] = storage.Record{
		ID:     omittedRevenueID,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("No Revenue"),
		},
	}
	org.Objects["Account"] = account

	const fourAccountIDs = "Id IN ('001000000000001','001000000000002','001000000000003','001000000000004')"
	for _, tc := range []struct {
		name  string
		query string
		want  int64
	}{
		{"row468_count_all_matched_rows", "SELECT COUNT() FROM Account WHERE " + fourAccountIDs, 4},
		{"row469_count_non_null_annual_revenue", "SELECT COUNT(AnnualRevenue) FROM Account WHERE " + fourAccountIDs, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseAndExecute(org, tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if result.Rows != 1 || len(result.Records) != 1 {
				t.Fatalf("result = %#v", result)
			}
			assertStorageInt(t, result.Records[0].Fields["expr0"], tc.want)
		})
	}
}
