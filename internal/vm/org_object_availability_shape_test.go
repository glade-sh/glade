package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func TestOrgObjectAvailabilityShapeCertaintyRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, row := range []struct {
			name     string
			features []string
		}{
			{"unspecified", []string{project.UnspecifiedStandardFieldShape}},
			{"commerce-enabled", []string{"B2BCommerce", "Communities", "CommerceEnabled"}},
		} {
			t.Run(api+"/"+row.name, func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureDeterministicPlatformData(&org)
				storage.ApplyProjectOrgShape(&org, row.features)
				objects := []string{"WebStore"}
				if row.name == "unspecified" {
					objects = append(objects, "Quote", "QuoteLineItem")
				}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.currentMethod = Method{APIVersion: api}
				machine.SetCurrentUser(org.Objects["User"].Records[storage.ID(storage.DefaultUserID)])
				for _, object := range objects {
					storage.EnsureStandardObject(&org, object)
					for i := 0; i < 2; i++ {
						if _, ok := machine.schemaGlobalDescribe().Map[mapKey(String(strings.ToLower(object)))]; !ok {
							t.Errorf("%s missing from describe", object)
						}
						if got := machine.typeForName("", "Schema."+object, false); got.Kind == ValueNull {
							t.Errorf("%s missing from reflection", object)
						}
						if err := machine.enforceOrgShapeObjectAvailability(soql.Query{Object: object}); err != nil {
							t.Errorf("%s query: %v", object, err)
						}
						if _, err := machine.AuthorizeVisualforceRecordFields(object, []string{"Id"}); err != nil {
							t.Errorf("%s record authorization: %v", object, err)
						}
						if _, err := machine.ReadVisualforceRecords(object); err != nil {
							t.Errorf("%s record host: %v", object, err)
						}
					}
				}
			})
		}
	}
}
