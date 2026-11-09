package gladecli

import (
	"strings"
	"testing"
)

func TestV035PublicBindWarningDescribesAuthority(t *testing.T) {
	t.Setenv("GLADE_SERVER_PUBLIC", "")
	err := validateServerBindAllowed("0.0.0.0:1789")
	if err == nil || !strings.Contains(err.Error(), "capability") || strings.Contains(err.Error(), "unauthenticated local server routes") {
		t.Fatalf("incorrect bind guidance: %v", err)
	}
}
