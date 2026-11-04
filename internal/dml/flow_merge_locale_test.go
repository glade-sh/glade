package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestFlowMergeNumericConstantsUseExecutingLocale(t *testing.T) {
	for _, tc := range []struct{ locale, want string }{
		{"en_US", "v=1,234.5|neg=-1,234.5|edge=1,000"},
		{"fr_FR", "v={!large}|neg={!negative}|edge={!boundary}"},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			org := testOrg()
			org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{storage.DefaultUserID: {Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(tc.locale)}}}}
			engine := NewEngine(&org)
			got := engine.renderFlowCustomErrorTextWithConstants("v={!large}|neg={!negative}|edge={!boundary}", &flowFrame{}, map[string]string{"large": "1234.50", "negative": "-1234.50", "boundary": "1000.00"})
			if got != tc.want {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}
