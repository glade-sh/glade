package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseCheckUsesOneAuthoritativeLocalGoRelease(t *testing.T) {
	data, err := os.ReadFile("release-check.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		`export GOMAXPROCS="${GOMAXPROCS:-2}"`,
		"scripts/ci-go-test.sh local-release",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("release check missing resource bound %q:\n%s", want, script)
		}
	}
	for _, forbidden := range []string{
		"go test ./...",
		"go test -count=1 -p=1 ./...",
		"go test ./internal/repoguard",
		"go test ./internal/gladecli ./internal/cliui",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("release check retains duplicate Go gate %q:\n%s", forbidden, script)
		}
	}
	for _, want := range []string{"git diff --check", "npm run release:check --prefix site", "scripts/smoke.sh"} {
		if !strings.Contains(script, want) {
			t.Fatalf("release check dropped existing gate %q:\n%s", want, script)
		}
	}
	for _, forbidden := range []string{"npm test --prefix site", "npm run build --prefix site"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("release check retains duplicate site gate %q:\n%s", forbidden, script)
		}
	}
}
