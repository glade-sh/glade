package storage_test

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Original and repeated org captures disagree on the generated label length.
// Bound it by the native observations instead of picking one org's exact value.
// email_services_metadata/org.tsv:2,3 and carried_conflict_isolation/org.tsv:2,254.
func TestEmailDomainGeneratedLabelNativeBounds(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance")
	min, max := 0, 0
	suffix := ""
	observations := 0
	add := func(lengths []int, nativeSuffix string) {
		t.Helper()
		if len(lengths) != 7 || lengths[0] <= 0 || nativeSuffix == "" {
			t.Fatalf("invalid native domain shape: %v %q", lengths, nativeSuffix)
		}
		if suffix != "" && suffix != nativeSuffix {
			t.Fatalf("native suffix differs: %q versus %q", suffix, nativeSuffix)
		}
		suffix = nativeSuffix
		if min == 0 || lengths[0] < min {
			min = lengths[0]
		}
		if lengths[0] > max {
			max = lengths[0]
		}
		observations++
	}
	type nativeAddress struct {
		Lengths []int  `json:"domainLabelLengths"`
		Suffix  string `json:"domainSuffix"`
	}
	read := func(family string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join(root, family, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma, r.LazyQuotes = '\t', true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	for _, row := range read("email_services_metadata") {
		if row[3] == "missing" {
			continue
		}
		var addresses []nativeAddress
		if err := json.Unmarshal([]byte(row[4]), &addresses); err != nil || len(addresses) != 1 {
			t.Fatalf("invalid original native row: %v", err)
		}
		add(addresses[0].Lengths, addresses[0].Suffix)
	}
	for _, row := range read("carried_conflict_isolation") {
		if (row[0] != "plain" && row[0] != "plain2") || row[1] != "baseline" {
			continue
		}
		// Profile observations have a different type, so decode only relevant keys.
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row[7]), &fields); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"active", "inactive", "limited"} {
			var addresses []nativeAddress
			if err := json.Unmarshal(fields[key], &addresses); err != nil {
				t.Fatal(err)
			}
			if len(addresses) != 1 {
				t.Fatalf("invalid native isolation %s", key)
			}
			add(addresses[0].Lengths, addresses[0].Suffix)
		}
	}
	if observations < 16 || min >= max {
		t.Fatalf("insufficient variable native captures: %d [%d,%d]", observations, min, max)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	services := []project.EmailService{{HandlerDeclared: true, ApexClass: "SyntheticHandler", FunctionName: "SyntheticFunction", Addresses: []project.EmailServiceAddress{
		{DeveloperName: "First", LocalPart: "x", RunAsUser: "system@example.invalid"},
		{DeveloperName: "Second", LocalPart: "longer_local", RunAsUser: "system@example.invalid"},
	}}}
	if err := storage.ApplyEmailServices(&org, services); err != nil {
		t.Fatal(err)
	}
	addresses := org.Objects["EmailServicesAddress"].Records
	if len(addresses) != 2 {
		t.Fatalf("got %d addresses", len(addresses))
	}
	for _, row := range addresses {
		domain := row.Fields["EmailDomainName"].String
		if !strings.HasSuffix(domain, suffix) {
			t.Errorf("domain %q does not have native suffix %q", domain, suffix)
		}
		first, _, found := strings.Cut(domain, ".")
		if !found || len(first) < min || len(first) > max {
			t.Errorf("first label length %d outside native observed range [%d,%d]", len(first), min, max)
		}
	}
}
