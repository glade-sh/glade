package flagparse

import (
	"strings"
	"testing"
)

func TestParserHandlesAliasesPositionalsAndDoubleDash(t *testing.T) {
	parser := New("glade sample").
		String("project", "p").
		Bool("json", "j").
		AllowPositionals(true)

	result, err := parser.Parse([]string{"-p", "force-app", "-j", "--", "--not-a-flag", "tail"})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String("project"); got != "force-app" {
		t.Fatalf("project = %q", got)
	}
	if !result.Bool("json") {
		t.Fatalf("json = false, want true")
	}
	if got := strings.Join(result.Positionals, "\x00"); got != "--not-a-flag\x00tail" {
		t.Fatalf("positionals = %#v", result.Positionals)
	}
}

func TestParserCollectsRepeatedStringValues(t *testing.T) {
	parser := New("glade config init").
		String("package-dir", "").
		String("feature", "")

	result, err := parser.Parse([]string{"--package-dir", "force-app", "--package-dir=packages/core", "--feature", "PersonAccounts", "--feature", "Communities"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(result.Strings("package-dir"), ","); got != "force-app,packages/core" {
		t.Fatalf("package-dir values = %q", got)
	}
	if got := result.String("feature"); got != "Communities" {
		t.Fatalf("feature = %q", got)
	}
	if got := strings.Join(result.Strings("feature"), ","); got != "PersonAccounts,Communities" {
		t.Fatalf("feature values = %q", got)
	}
}

func TestParserSuggestsClosestFlag(t *testing.T) {
	parser := New("glade test").
		String("filter", "").
		String("project", "p")

	_, err := parser.Parse([]string{"--filteer", "AccountTest"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `unknown flag "--filteer"`) {
		t.Fatalf("missing unknown flag text: %v", err)
	}
	if !strings.Contains(err.Error(), `did you mean "--filter"?`) {
		t.Fatalf("missing suggestion: %v", err)
	}
}

func TestParserRejectsMissingValues(t *testing.T) {
	parser := New("glade doctor").String("project", "p")
	_, err := parser.Parse([]string{"-p"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--project requires a value") {
		t.Fatalf("error = %v", err)
	}
}

func TestParserRejectsFlagTokenAsValue(t *testing.T) {
	parser := New("glade check").
		String("project", "p").
		Bool("json", "j")

	_, err := parser.Parse([]string{"--project", "--json"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--project requires a value") {
		t.Fatalf("error = %v", err)
	}
}

func TestParserRejectsSingleDashLongFlagTypo(t *testing.T) {
	parser := New("glade check").
		String("project", "p").
		AllowPositionals(true)

	_, err := parser.Parse([]string{"-project", "force-app"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `unknown flag "-project"`) {
		t.Fatalf("error = %v", err)
	}
}
