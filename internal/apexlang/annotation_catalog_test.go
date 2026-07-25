package apexlang

import "testing"

func TestAnnotationCatalogRecognizesSupportedAndPreviewEntries(t *testing.T) {
	if spec, ok := LookupAnnotation("AuraEnabled"); !ok || spec.Preview {
		t.Fatalf("AuraEnabled = %#v, %v", spec, ok)
	}
	if spec, ok := LookupAnnotation("IntegrationTest"); !ok || !spec.Preview {
		t.Fatalf("IntegrationTest = %#v, %v", spec, ok)
	}
	if _, ok := LookupAnnotation("DoesNotExist"); ok {
		t.Fatal("unknown annotation unexpectedly recognized")
	}
}
