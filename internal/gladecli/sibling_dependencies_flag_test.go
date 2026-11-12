package gladecli

import (
	"os"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestApplySiblingDependenciesFlag(t *testing.T) {
	t.Setenv(project.SiblingDependencyScanEnvVar, "")
	got := applySiblingDependenciesFlag([]string{"check", "--json"})
	if !reflect.DeepEqual(got, []string{"check", "--json"}) || os.Getenv(project.SiblingDependencyScanEnvVar) != "" {
		t.Fatalf("without the flag: args=%v env=%q", got, os.Getenv(project.SiblingDependencyScanEnvVar))
	}
	got = applySiblingDependenciesFlag([]string{"check", "--sibling-dependencies", "--json", "--", "--sibling-dependencies"})
	if !reflect.DeepEqual(got, []string{"check", "--json", "--", "--sibling-dependencies"}) {
		t.Fatalf("args=%v", got)
	}
	if os.Getenv(project.SiblingDependencyScanEnvVar) != "1" {
		t.Fatalf("env=%q, want 1", os.Getenv(project.SiblingDependencyScanEnvVar))
	}
}
