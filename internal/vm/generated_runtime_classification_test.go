package vm

import "testing"

func TestGeneratedRuntimeDispositionRejectsNonDTOFallback(t *testing.T) {
	method := Method{
		Name:      "Ideas.findSimilar",
		ClassName: "Ideas",
		IsStatic:  true,
		Modifiers: []string{"passive-generated"},
	}

	if got := (New(nil)).generatedRuntimeDisposition(method, Null); got != generatedRuntimeUnsupported {
		t.Fatalf("Ideas.findSimilar disposition = %d, want unsupported", got)
	}
}
