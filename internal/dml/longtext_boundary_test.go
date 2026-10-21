package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStoredLongTextBoundaryNormalizationIsScoped(t *testing.T) {
	long := storage.Field{Type: storage.FieldString, DisplayType: "TEXTAREA", LongTextArea: true, Length: 32768}
	area := storage.Field{Type: storage.FieldString, DisplayType: "TEXTAREA", Length: 32768}
	// Native named LongTextArea rows establish edge LF/CRLF removal and interior
	// preservation. The unmarked area is a scope control, not a native trim oracle.
	for _, tc := range []struct{ source, want string }{
		{"a\n", "a"}, {"a\nb", "a\nb"}, {"\na", "a"}, {"a\r\n", "a"}, {"a\n\n", "a"}, {"a", "a"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			caller := storage.StringValue(tc.source)
			if got := normalizeStoredFieldValue(long, caller); got.String != tc.want {
				t.Fatalf("stored=%q want=%q", got.String, tc.want)
			}
			if caller.String != tc.source {
				t.Fatal("caller was rewritten")
			}
			if got := normalizeStoredFieldValue(area, caller); got.String != tc.source {
				t.Fatalf("unmarked textarea changed: %q", got.String)
			}
		})
	}
	for _, source := range []string{"\ra", "a\r", "\r\na", "\n", "\r\n", "\n \n"} {
		if got := normalizeStoredFieldValue(long, storage.StringValue(source)); got.String != source {
			t.Fatalf("unobserved boundary changed: source=%q stored=%q", source, got.String)
		}
	}

}
