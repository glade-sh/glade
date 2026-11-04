package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestConformancePairingDeferredTopLevelSkip(t *testing.T) {
	const name = "TestVerifyEngineRequiresExactSourceOffers"
	const pkg = "github.com/glade-sh/glade/internal/dataweave"
	for _, tc := range []struct {
		name, value string
		pass        bool
	}{
		{"prerequisite missing", "", true},
		{"prerequisite present", "/configured", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GLADE_DATAWEAVE_TEST_INSTALLED_ENGINE", tc.value)
			script, dir, fake, carries := pairingFixture(t, "internal/dataweave", "native_test.go", "package dataweave\nimport \"testing\"\nfunc "+name+"(t *testing.T) {}\n", "", "[]")
			events := []map[string]string{
				{"Action": "output", "Package": pkg, "Test": name, "Output": "    native_test.go:25: explicit provisioned engine required\n"},
				{"Action": "skip", "Package": pkg, "Test": name},
				{"Action": "pass", "Package": pkg},
			}
			data := "#!/bin/sh\ncat <<'EVENTS'\n"
			for _, event := range events {
				line, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				data += string(line) + "\n"
			}
			data += "EVENTS\n"
			if err := os.WriteFile(fake, []byte(data), 0755); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v: %v %s", tc.pass, err, out)
			}
		})
	}
}
