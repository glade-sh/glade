package apextest

import (
	"reflect"
	"strings"
	"testing"
)

func TestSecuredProjectionBoundaryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "secured_projection_boundary", "SecuredProjectionProbeTest", func(t *testing.T, got, want any) {
		// These controls qualify rejection; the retained rows also keep native wording.
		for _, observation := range []any{got, want} {
			fields := observation.(map[string]any)
			if message, ok := fields["hiddenQueryError"].(string); ok {
				fields["hiddenQueryError"], _, _ = strings.Cut(message, ":")
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("native=%v; Glade=%v", want, got)
		}
	}, "62.0", "67.0")
}
