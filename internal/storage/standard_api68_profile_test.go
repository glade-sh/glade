package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestStandardAPI68ProfileSelection(t *testing.T) {
	for _, row := range []struct {
		name     string
		features []string
		want     string
		ok       bool
	}{
		{"developer", []string{project.StandardFieldEditionDeveloper}, "P0", true},
		{"currency", []string{project.StandardFieldEditionDeveloper, "MultiCurrency"}, "P1", true},
		{"enterprise", []string{project.StandardFieldEditionEnterprise}, "P11", true},
		{"commerce", []string{project.StandardFieldEditionDeveloper, "B2BCommerce", "Communities", "CommerceEnabled"}, "P10", true},
		{"unspecified", []string{project.UnspecifiedStandardFieldShape}, "", false},
		{"unobserved combination", []string{project.StandardFieldEditionDeveloper, "MultiCurrency", "PersonAccounts"}, "", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, ok := StandardAPI68DescribeProfile(row.features)
			if got != row.want || ok != row.ok {
				t.Fatalf("profile=%q ok=%t, want %q %t", got, ok, row.want, row.ok)
			}
		})
	}
}
