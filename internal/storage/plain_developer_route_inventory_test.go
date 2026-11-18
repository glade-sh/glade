package storage

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A positive on any captured route must not be generalized into a global
// inventory removal. Singleton aliases are not evidence of a separate object.
func TestPlainDeveloperSpreadRetainsCapturedRouteObjects(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "plain_developer_route_inventory", "org.tsv"))
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
	positive := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 7 {
			t.Fatalf("invalid native row with %d columns", len(row))
		}
		route, name, first, second, third := row[1], row[2], row[3], row[4], row[5]
		switch route {
		case "anonymous", "istest", "visualforce":
			positive[name] = positive[name] || first == "true" || second == "ok"
		case "entity_definition", "global_describe":
			positive[name] = positive[name] || first == "true"
		case "rest":
			positive[name] = positive[name] || first == "200" || third == "200"
		case "lwc_batch":
			positive[name] = positive[name] || first == "200"
		case "lwc_single":
			positive[name] = positive[name] || (first == "true" && strings.EqualFold(name, third))
		}
	}
	names := make([]string, 0, len(positive))
	for name, present := range positive {
		if present {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("native route fixture has no positive objects")
	}
	features := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}
	for _, tc := range []struct {
		name string
		org  func() OrgState
	}{
		{"eager", func() OrgState { return sourceSpreadEagerOrg(features) }},
		{"source", func() OrgState { return sourceSpreadLazyOrg(features) }},
		{"legacy", func() OrgState { return lazySpreadOrg(features) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := tc.org()
			for _, name := range names {
				if _, exists := org.Objects[name]; !exists {
					EnsureLazyStandardObject(&org, name)
				}
				if _, exists := org.Objects[name]; !exists {
					t.Errorf("captured route object %s was not retained", name)
				}
			}
		})
	}
}
