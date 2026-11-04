package apextest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandardFieldSupplementRouteCaptureIntegrity(t *testing.T) {
	const dir = "testdata/conformance/standard_field_supplement_routes"
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"query.apex", "Case251Query62.cls", "Case251Query67.cls", "Case251Page59.page", "Case251Page67.page", "Case251Access.permissionset-meta.xml"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("%s %x\n", name, sha256.Sum256(data))) {
			t.Errorf("native capture source changed: %s", name)
		}
	}
}
