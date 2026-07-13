package sema

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestLayeredModelCutoverPrecedence(t *testing.T) {
	projectMath := typesys.TypeSymbol{
		Kind:    apexast.DeclarationClass,
		Name:    "Math",
		Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationField, Name: "ProjectOnly", Type: "String"}},
	}
	dependencyDatabase := typesys.TypeSymbol{
		Kind:       apexast.DeclarationClass,
		Name:       "Database",
		Dependency: true,
		Artifact:   true,
		Members:    []typesys.MemberSymbol{{Kind: apexast.DeclarationField, Name: "DependencyOnly", Type: "String"}},
	}
	namespacedDependency := typesys.TypeSymbol{
		Kind:       apexast.DeclarationClass,
		Name:       "Gateway",
		Namespace:  "pkg",
		Dependency: true,
		Artifact:   true,
		Members:    []typesys.MemberSymbol{{Kind: apexast.DeclarationField, Name: "NamespacedOnly", Type: "String"}},
	}
	account := schema.Object{Name: "Account", Fields: []schema.Field{
		{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"pkg__Queue__c"}, RelationshipName: "pkg__Owner"},
	}}
	state := buildSemaTypeMemberState(typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Types:   []typesys.TypeSymbol{projectMath, dependencyDatabase, namespacedDependency},
		Objects: []schema.Object{account},
	}, nil)
	view := state.view()

	for _, test := range []struct {
		name  string
		field string
	}{
		{name: "Math", field: "ProjectOnly"},
		{name: "Database", field: "DependencyOnly"},
		{name: "pkg.Gateway", field: "NamespacedOnly"},
	} {
		members, _, ok := semaLookupTypeMembers(view, test.name)
		if !ok {
			t.Fatalf("layered lookup omitted %s", test.name)
		}
		if _, ok := members.fields[normalizeName(test.field)]; !ok {
			t.Fatalf("%s resolved without %s: %#v", test.name, test.field, members.fields)
		}
	}
	if _, _, ok := semaLookupTypeMembers(view, "Gateway"); ok {
		t.Fatal("artifact dependency resolved without its namespace")
	}
	accountMembers, _, ok := semaLookupTypeMembers(view, "Account")
	if !ok || accountMembers.fields[normalizeName("pkg__Owner")].Type != "pkg__Queue__c" {
		t.Fatalf("project relationship did not shadow standard Account.Owner: %#v, %v", accountMembers.fields, ok)
	}
	if _, leaked := accountMembers.fields[normalizeName("Owner")]; leaked {
		t.Fatalf("standard Account.Owner leaked beside the project relationship: %#v", accountMembers.fields)
	}

	duplicate := typesys.TypeSymbol{
		Kind:    apexast.DeclarationClass,
		Name:    "Math",
		Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationField, Name: "CurrentOnly", Type: "Boolean"}},
	}
	current := semaModelWithCurrentType(view, duplicate)
	currentMath, _, ok := semaLookupTypeMembers(current, "Math")
	if !ok || currentMath.fields[normalizeName("CurrentOnly")].Type != "Boolean" {
		t.Fatalf("duplicate current type did not shadow project/base/platform layers: %#v, %v", currentMath, ok)
	}
	baseMath, _, _ := semaLookupTypeMembers(view, "Math")
	if _, leaked := baseMath.fields[normalizeName("CurrentOnly")]; leaked {
		t.Fatal("duplicate current type leaked into the base layer")
	}
}

func TestLayeredModelCutoverRemapsDependencySource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dependencyRoot := filepath.Join(root, "dependency")
	consumerRoot := filepath.Join(root, "consumer")
	dependencyFile := filepath.Join(dependencyRoot, "Helper.cls")
	consumerFile := filepath.Join(consumerRoot, "Consumer.cls")
	for _, dir := range []string{dependencyRoot, consumerRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeSemaFile(t, dependencyFile, `
global class Helper {
  global static BasePkg.Helper identity(BasePkg.Helper value) { return value; }
}
`)
	writeSemaFile(t, consumerFile, `
public class Consumer {
  public stagepkg.Helper run(stagepkg.Helper value) {
    return stagepkg.Helper.identity(value);
  }
}
`)
	remaps := []namespaceremap.Rule{{From: "BasePkg", To: "stagepkg"}}
	dependencyProject := project.Project{
		Root:            dependencyRoot,
		Namespace:       "stagepkg",
		NamespaceRemaps: remaps,
		ApexFiles:       []string{dependencyFile},
	}
	index := typesys.Build(project.Project{
		Root:      consumerRoot,
		ApexFiles: []string{consumerFile},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "stagepkg",
			SourceRoot: dependencyRoot,
			Project:    &dependencyProject,
			Status:     "loaded",
		}},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("remapped dependency source did not resolve through the layered model: %#v", result.Diagnostics)
	}
}

func TestLayeredModelCutoverDiagnosticOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "CutoverDiagnostics.cls")
	writeSemaFile(t, file, `
public class CutoverDiagnostics {
  public void run() {
    missingHelper();
    List<Account> rows = [SELECT MissingCutover__c FROM Account];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	type identity struct {
		code    string
		message string
		range_  diagnostic.Range
	}
	selectDiagnostics := func(result Result) []identity {
		var out []identity
		for _, item := range result.Diagnostics {
			if item.Code != "GLADESEMA008" && item.Code != "GLADESEMA_QUERY_FIELD" {
				continue
			}
			entry := identity{code: item.Code, message: item.Message}
			if item.Range != nil {
				entry.range_ = *item.Range
			}
			out = append(out, entry)
		}
		return out
	}
	first := selectDiagnostics(Analyze(index))
	second := selectDiagnostics(Analyze(index))
	if len(first) != 2 {
		t.Fatalf("cutover diagnostic fixture produced %#v, want two sentinels", first)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("diagnostic identity/order changed between analyses\nfirst: %#v\nsecond: %#v", first, second)
	}
	if first[0].code != "GLADESEMA008" || !strings.Contains(first[0].message, "missingHelper") ||
		first[1].code != "GLADESEMA_QUERY_FIELD" || !strings.Contains(first[1].message, "MissingCutover__c") {
		t.Fatalf("diagnostic precedence/order = %#v", first)
	}
}

func TestLayeredModelCutoverSharedMutationIsolation(t *testing.T) {
	first := buildSemaTypeMemberState(typesys.Index{}, nil)
	second := buildSemaTypeMemberState(typesys.Index{}, nil)
	if first.platform == nil || first.platform != second.platform {
		t.Fatal("analysis states did not retain the shared platform model")
	}
	firstMath, _, ok := semaLookupTypeMembers(first.view(), "Math")
	if !ok {
		t.Fatal("first analysis omitted Math")
	}
	firstMath.fields[normalizeName("CutoverMutation")] = typesys.MemberSymbol{Name: "CutoverMutation"}
	secondMath, _, ok := semaLookupTypeMembers(second.view(), "Math")
	if !ok {
		t.Fatal("second analysis omitted Math")
	}
	if _, leaked := secondMath.fields[normalizeName("CutoverMutation")]; leaked {
		t.Fatal("caller mutation leaked through the shared platform model")
	}

	firstView := first.view()
	secondView := second.view()
	if _, _, ok := semaLookupTypeMembers(firstView, "Account"); !ok {
		t.Fatal("first analysis could not hydrate Account")
	}
	key := normalizeName("Account")
	if len(firstView.hydrated[key].fields) == 0 {
		t.Fatal("first analysis did not retain local Account hydration")
	}
	if _, leaked := secondView.hydrated[key]; leaked {
		t.Fatal("standard SObject hydration leaked between analysis states")
	}
}
