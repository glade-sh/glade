package lwc

import (
	"errors"
	"testing"
)

func TestParseComponentMetaUnknownElementPreservesMetadataDiagnostic(t *testing.T) {
	path := writeComponentMeta(t, `<LightningComponentBundle><apiVersion>67.0</apiVersion><anotherUnknown>true</anotherUnknown></LightningComponentBundle>`)
	_, err := ParseComponentMeta(path)
	var unknown *UnknownMetadataElementError
	if !errors.As(err, &unknown) || unknown.Element != "anotherUnknown" {
		t.Fatalf("ParseComponentMeta error = %v; want typed unknown element", err)
	}
	if want := "Error parsing file: LWC Metadata Xml Parser: Unhandled XML element: anotherUnknown"; err.Error() != want {
		t.Fatalf("metadata diagnostic = %q; want %q", err, want)
	}
}
