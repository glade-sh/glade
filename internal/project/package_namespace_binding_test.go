package project

import (
	"reflect"
	"testing"
)

func TestPackageNamespaceBinding(t *testing.T) {
	loaded := ManagedPackageDependency{Namespace: "withsrc", Status: "loaded", Project: &Project{}}
	declared := ManagedPackageDependency{Namespace: "NoSrc", Status: DependencyStatusDeclared}
	missing := ManagedPackageDependency{Namespace: "gone", Status: "missing"}
	cases := []struct {
		name string
		p    Project
		want PackageNamespaceBinding
	}{
		{"declares nothing stays open", Project{}, PackageNamespaceBinding{}},
		{"declared and loaded", Project{ManagedPackageDependencies: []ManagedPackageDependency{loaded, declared, missing}},
			PackageNamespaceBinding{Narrow: true, Sourceless: []string{"NoSrc", "gone"}}},
		{"loaded only", Project{ManagedPackageDependencies: []ManagedPackageDependency{loaded}}, PackageNamespaceBinding{Narrow: true}},
		{"alias without namespace binding stays open", Project{
			ManagedPackageDependencies: []ManagedPackageDependency{declared},
			DependencyDiagnostics:      []DependencyDiagnostic{{Namespace: "SomeAlias", Status: "missing", Code: "dependency_missing"}},
		}, PackageNamespaceBinding{}},
		{"diagnostic for a bound namespace keeps narrowing", Project{
			ManagedPackageDependencies: []ManagedPackageDependency{missing},
			DependencyDiagnostics:      []DependencyDiagnostic{{Namespace: "gone", Status: "missing", Code: "dependency_missing"}},
		}, PackageNamespaceBinding{Narrow: true, Sourceless: []string{"gone"}}},
	}
	for _, tc := range cases {
		if got := tc.p.PackageNamespaceBinding(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %#v want %#v", tc.name, got, tc.want)
		}
	}
}
