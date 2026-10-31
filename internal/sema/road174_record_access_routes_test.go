package sema

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Exercise member binding on the relationship selected by the native query rows.
func TestRoad174RecordAccessMemberBinding(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Lead", "Campaign", "Asset"} {
			t.Run(api+"/"+object, func(t *testing.T) {
				src := fmt.Sprintf(`%s row = [SELECT UserRecordAccess.HasReadAccess FROM %s LIMIT 1]; Boolean visible = row.UserRecordAccess.HasReadAccess;`, object, object)
				if result := AnalyzeAnonymous(typesys.Index{}, src, api); result.HasErrors() {
					t.Fatalf("member binding: %#v", result.Diagnostics)
				}
			})
		}
	}
}
