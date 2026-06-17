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

func TestRegisteredMethodSourceAliasKeyCollapsesNamespaceAlias(t *testing.T) {
	left := Method{
		Name:      "fflib_MethodVerifier.throwException",
		ClassName: "fflib_MethodVerifier",
		Params:    []Param{{Name: "qm", Type: "fflib_QualifiedMethod"}},
		File:      "force-app/fflib/classes/fflib_MethodVerifier.cls",
		Line:      55,
	}
	right := left
	right.ClassName = "samplepkg.fflib_MethodVerifier"
	if gotLeft, gotRight := registeredMethodSourceAliasKey(left), registeredMethodSourceAliasKey(right); gotLeft != gotRight {
		t.Fatalf("source alias keys differ: %q != %q", gotLeft, gotRight)
	}
}
