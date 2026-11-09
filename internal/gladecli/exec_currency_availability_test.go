package gladecli

import (
	"context"
	"fmt"
	"io"
	"testing"
)

func TestRunExecCurrencyAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		available    bool
	}{
		{"plain", "org:\n  features: []\n", false},
		{"communities", "org:\n  features: [Communities]\n", false},
		{"multi", "org:\n  features: [MultiCurrency]\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := execStandardOrgProject(t, tc.config)
			body := fmt.Sprintf(`for (String n : new List<String>{'CurrencyType','DatedConversionRate'}) { System.assertEquals(%t, Schema.getGlobalDescribe().containsKey(n)); Boolean ok=true; try { Database.countQuery('SELECT COUNT() FROM '+n); } catch (System.QueryException e) { ok=false; } System.assertEquals(%t,ok); }`, tc.available, tc.available)
			if err := runExec(context.Background(), []string{"--project", root, "--json", body}, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}
