package storage

import "testing"

func TestStandardFieldRichTextLengthFacet(t *testing.T) {
	for _, tc := range []struct {
		object, field string
		length        int
		known         bool
	}{
		{"LiveChatTranscript", "Body", 131072, true},
		{"livechattranscript", "body", 131072, true},
		{"Account", "Description", 0, false},
		{"Account", "Name", 0, false},
		{"LiveChatTranscript", "Absent__c", 0, false},
		{"RichPanel__c", "Body__c", 0, false},
	} {
		t.Run(tc.object+"/"+tc.field, func(t *testing.T) {
			length, known := StandardFieldRichTextLength(tc.object, tc.field)
			if length != tc.length || known != tc.known {
				t.Fatalf("rich facet=(%d,%t), want (%d,%t)", length, known, tc.length, tc.known)
			}
		})
	}
}
