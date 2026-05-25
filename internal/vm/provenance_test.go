package vm

import "testing"

func TestSymbolOriginPreferenceRank(t *testing.T) {
	if got := symbolOriginFromDependency(true); got != symbolOriginDependency {
		t.Fatalf("dependency origin = %q", got)
	}
	if got := symbolOriginFromDependency(false); got != symbolOriginProject {
		t.Fatalf("project origin = %q", got)
	}
	if dependencyPreferenceRank(symbolOriginDependency, true) >= dependencyPreferenceRank(symbolOriginProject, true) {
		t.Fatal("dependency lookup should prefer dependency origin")
	}
	if dependencyPreferenceRank(symbolOriginProject, false) >= dependencyPreferenceRank(symbolOriginDependency, false) {
		t.Fatal("project lookup should prefer project origin")
	}
}

func TestRegisteredMethodCandidateKeyKeepsDuplicateOrigins(t *testing.T) {
	left := Method{Name: "Service.run", ClassName: "Service", File: "project/Service.cls"}
	right := left
	right.Dependency = true
	right.File = "dependency/Service.cls"
	if gotLeft, gotRight := registeredMethodCandidateKey(left), registeredMethodCandidateKey(right); gotLeft == gotRight {
		t.Fatalf("candidate keys collapsed duplicate origins: %q", gotLeft)
	}
}
