package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Exercise the real project loader and anonymous execution mode for every
// native locale and API, comparing the result with both native capture routes.
func TestLocaleOrder466NativeConformanceExec(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	const fixture = "../apextest/testdata/conformance/locale_order_466"
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	lines := strings.Split(strings.TrimSpace(read("org.tsv")), "\n")
	if len(lines) != 25 || lines[0] != "locale\tapi\troute\torder" {
		t.Fatal("incomplete native capture")
	}
	rows := make(map[string]string)
	for _, line := range lines[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 4 {
			t.Fatal("malformed native row")
		}
		key := c[0] + "/" + c[1] + "/" + c[2]
		if _, ok := rows[key]; ok {
			t.Fatalf("duplicate row %s", key)
		}
		rows[key] = c[3]
	}
	source := read("seed.apex") + "\n" + read("query.apex")
	payload := regexp.MustCompile(`ROAD466ROW:(de_DE|sv_SE|ja_JP|fr_FR|es_ES|en_US):([0-9,]+)`)
	for _, api := range []string{"62.0", "67.0"} {
		for _, locale := range []string{"de_DE", "sv_SE", "ja_JP", "fr_FR", "es_ES", "en_US"} {
			t.Run(api+"/"+locale, func(t *testing.T) {
				root := t.TempDir()
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				db := filepath.Join(root, "org.sqlite")
				setup := "User u = [SELECT Id, LocaleSidKey FROM User WHERE Id = :UserInfo.getUserId()]; u.LocaleSidKey = '" + locale + "'; update u;"
				var stdout, stderr bytes.Buffer
				code := Run(context.Background(), []string{"exec", "--project", root, "--db", db, "--json", setup}, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("exec locale setup failed: exit=%d", code)
				}
				stdout.Reset()
				stderr.Reset()
				code = Run(context.Background(), []string{"exec", "--project", root, "--db", db, "--json", source}, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("exec probe failed: exit=%d", code)
				}
				var result struct {
					Debug []string `json:"debug"`
				}
				envelope := decodeCLIEnvelopeData(t, stdout.Bytes(), "exec", &result)
				if envelope.Status != "passed" || envelope.ExitCode != 0 {
					t.Fatalf("unexpected exec envelope: %+v", envelope)
				}
				if len(result.Debug) != 1 {
					t.Fatalf("got %d debug rows, want 1", len(result.Debug))
				}
				got := payload.FindStringSubmatch(result.Debug[0])
				if len(got) != 3 || got[1] != locale {
					t.Fatalf("missing or wrong running-user locale: %q", result.Debug[0])
				}
				for _, route := range []string{"anonymous", "istest"} {
					key := locale + "/" + api + "/" + route
					want, ok := rows[key]
					if !ok {
						t.Fatalf("missing native row %s", key)
					}
					if got[2] != want {
						t.Errorf("native %s / glade exec: got %s want %s", key, got[2], want)
					}
				}
			})
		}
	}
}
