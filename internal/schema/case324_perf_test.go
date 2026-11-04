package schema

import (
	"fmt"
	"testing"
)

func TestCase324WideMembershipWorkload(t *testing.T) {
	var src Object
	for i := 0; i < 3000; i++ {
		n := fmt.Sprintf("Field%04d", i)
		src.Fields = append(src.Fields, Field{Name: n})
		src.RecordTypes = append(src.RecordTypes, RecordType{DeveloperName: n})
		src.ValidationRules = append(src.ValidationRules, ValidationRule{Name: n})
	}
	for round := 0; round < 8; round++ {
		var dst Object
		mergeObjectMetadata(&dst, src)
		mergeObjectMetadata(&dst, src)
		if len(dst.Fields) != 3000 || len(dst.RecordTypes) != 3000 || len(dst.ValidationRules) != 3000 || dst.Fields[2999].Name != "Field2999" {
			t.Fatal("membership result differs")
		}
	}
}
