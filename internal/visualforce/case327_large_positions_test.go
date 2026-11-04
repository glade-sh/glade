package visualforce

import (
	"strings"
	"testing"
)

func TestCase327LargeSourceProjection(t *testing.T) {
	const count = 3000
	source := strings.Repeat("é\n<apex:outputText value='ok'/>", count)
	for i := 0; i < 3; i++ {
		tags := scanSourceTags(source)
		if len(tags) != count {
			t.Fatalf("tags = %d, want %d", len(tags), count)
		}
		for j, tag := range tags {
			if tag.name != "apex:outputText" || tag.line != j+2 || tag.column != 1 || tag.nameEndLine != j+2 || tag.nameEndColumn != 17 {
				t.Fatalf("tag %d: %#v", j, tag)
			}
		}
	}
}
