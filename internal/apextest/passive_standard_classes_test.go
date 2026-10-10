package apextest

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// freshPassiveStandardRuntimeClasses is the per-call build that the process
// template replaced. It is the oracle for the cached path.
func freshPassiveStandardRuntimeClasses(indexTypes []typesys.TypeSymbol, existing []vm.Class) []vm.Class {
	seen := make(map[string]bool, len(indexTypes)+len(existing))
	for _, typ := range indexTypes {
		seen[strings.ToLower(typeSymbolRuntimeName(typ))] = true
	}
	for _, class := range existing {
		seen[strings.ToLower(class.Name)] = true
	}
	var out []vm.Class
	for _, typ := range typesys.StandardPlatformSymbolView() {
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface && typ.Kind != apexast.DeclarationEnum {
			continue
		}
		name := typeSymbolRuntimeName(typ)
		if name == "" || seen[strings.ToLower(name)] || (typ.Kind != apexast.DeclarationEnum && !isPassiveStandardRuntimeType(name)) {
			continue
		}
		out = append(out, passiveRuntimeClassFromTypeSymbol(typ, name))
		seen[strings.ToLower(name)] = true
	}
	return out
}

func passiveStandardRuntimeClassFilterCases() []struct {
	name       string
	indexTypes []typesys.TypeSymbol
	existing   []vm.Class
} {
	return []struct {
		name       string
		indexTypes []typesys.TypeSymbol
		existing   []vm.Class
	}{
		{name: "empty"},
		{
			name: "project types shadow by runtime name, any case",
			indexTypes: []typesys.TypeSymbol{
				{Kind: apexast.DeclarationClass, Name: "XmlTag"},
				{Kind: apexast.DeclarationClass, Namespace: "Auth", Name: "jwt"},
				{Kind: apexast.DeclarationInterface, Name: "AUTH.AUTHPROVIDERPLUGIN"},
			},
		},
		{
			name:     "existing classes shadow by name, any case",
			existing: []vm.Class{{Name: "loggingLevel"}, {Name: "Apex.Stack"}, {Name: "NotAPlatformType"}},
		},
	}
}

func TestPassiveStandardRuntimeClassesMatchFreshBuild(t *testing.T) {
	for _, tc := range passiveStandardRuntimeClassFilterCases() {
		t.Run(tc.name, func(t *testing.T) {
			want := freshPassiveStandardRuntimeClasses(tc.indexTypes, tc.existing)
			got := passiveStandardRuntimeClasses(tc.indexTypes, tc.existing)
			if len(got) != len(want) {
				t.Fatalf("passive classes = %d, want %d", len(got), len(want))
			}
			for i := range want {
				if !reflect.DeepEqual(got[i], want[i]) {
					t.Fatalf("passive class %d = %s, want %s (or contents differ)", i, got[i].Name, want[i].Name)
				}
			}
		})
	}
	shadowed := passiveStandardRuntimeClasses(passiveStandardRuntimeClassFilterCases()[1].indexTypes, nil)
	for _, class := range shadowed {
		switch strings.ToLower(class.Name) {
		case "xmltag", "auth.jwt", "auth.authproviderplugin":
			t.Fatalf("project type %s did not shadow its passive platform class", class.Name)
		}
	}
}

// TestPassiveStandardRuntimeClassesDoNotAliasTemplate proves each call owns
// every map and slice it returns: RegisterClass stamps them in place.
func TestPassiveStandardRuntimeClassesDoNotAliasTemplate(t *testing.T) {
	first := passiveStandardRuntimeClasses(nil, nil)
	for i := range first {
		class := &first[i]
		class.Name += "X"
		if len(class.Interfaces) > 0 {
			class.Interfaces[0] = "Mutated"
		}
		if len(class.Modifiers) > 0 {
			class.Modifiers[0] = "mutated"
		}
		if len(class.FieldOrder) > 0 {
			class.FieldOrder[0] = "mutated"
		}
		if len(class.StaticFieldOrder) > 0 {
			class.StaticFieldOrder[0] = "mutated"
		}
		if len(class.EnumValues) > 0 {
			class.EnumValues[0] = "MUTATED"
		}
		for name, field := range class.StaticFields {
			if len(field.Modifiers) > 0 {
				field.Modifiers[0] = "mutated"
			}
			field.Value = vm.Value{Kind: vm.ValueString, Text: "mutated"}
			class.StaticFields[name] = field
		}
		for name, field := range class.Fields {
			if len(field.Modifiers) > 0 {
				field.Modifiers[0] = "mutated"
			}
			class.Fields[name] = field
		}
		class.Fields["mutatedField"] = vm.Field{Name: "mutatedField"}
		for name, method := range class.Methods {
			if len(method.Params) > 0 {
				method.Params[0].Type = "Mutated"
			}
			if len(method.Modifiers) > 0 {
				method.Modifiers[0] = "mutated"
			}
			method.ClassName = "Mutated"
			class.Methods[name] = method
		}
		class.Methods["mutated()"] = vm.Method{Name: "mutated"}
		for j := range class.Constructors {
			if len(class.Constructors[j].Params) > 0 {
				class.Constructors[j].Params[0].Type = "Mutated"
			}
			class.Constructors[j].ClassName = "Mutated"
		}
	}
	want := freshPassiveStandardRuntimeClasses(nil, nil)
	second := passiveStandardRuntimeClasses(nil, nil)
	if !reflect.DeepEqual(second, want) {
		t.Fatal("mutating one call's passive classes changed a later call")
	}
	templates := passiveStandardRuntimeClassTemplates()
	if len(templates) != len(want) {
		t.Fatalf("templates = %d, want %d", len(templates), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(templates[i].class, want[i]) {
			t.Fatalf("template %s changed after caller mutation", want[i].Name)
		}
	}
}

// TestPassiveStandardRuntimeClassesBuildOncePerProcess fails if the template
// cache is removed: Runs and direct calls must not rebuild it.
func TestPassiveStandardRuntimeClassesBuildOncePerProcess(t *testing.T) {
	passiveStandardRuntimeClasses(nil, nil)
	passiveStandardRuntimeClasses(nil, []vm.Class{{Name: "XmlTag"}})
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PassiveOnceTest.cls"), `
@isTest
private class PassiveOnceTest {
  @isTest static void runs() {
    System.assertNotEquals(null, Type.forName('Auth.JWT'));
  }
}
`)
	for i := 0; i < 2; i++ {
		if got := Run(loadTestIndex(t, root), Options{}).Summary(); got.Total != 1 || got.Passed != 1 {
			t.Fatalf("run %d summary = %#v", i, got)
		}
	}
	if got := passiveStandardRuntimeClassCache.builds; got != 1 {
		t.Fatalf("passive class template builds = %d, want 1 per process", got)
	}
}

// TestPassiveStandardRuntimeClassesAllocateLessThanFreshBuild is the
// allocation budget: a cached call copies maps but skips the symbol walk and
// method-name construction.
func TestPassiveStandardRuntimeClassesAllocateLessThanFreshBuild(t *testing.T) {
	passiveStandardRuntimeClasses(nil, nil)
	fresh := testing.AllocsPerRun(3, func() { freshPassiveStandardRuntimeClasses(nil, nil) })
	cached := testing.AllocsPerRun(3, func() { passiveStandardRuntimeClasses(nil, nil) })
	if cached > fresh*0.8 {
		t.Fatalf("cached passive classes allocate %.0f per call, fresh %.0f; want at most 80%%", cached, fresh)
	}
	t.Logf("allocs per call: fresh %.0f, cached %.0f", fresh, cached)
}

// TestRunPassiveStandardClassesReflectionParity runs the same project twice in
// one process. The first Run's VM stamps its passive class copies; the second
// must see the same reflection and Type.forName answers.
func TestRunPassiveStandardClassesReflectionParity(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/XmlTag.cls"), `
public class XmlTag {
  public String origin() {
    return 'project';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PluginImpl.cls"), `
public class PluginImpl implements Auth.AuthProviderPlugin {
  public String getCustomMetadataType() { return null; }
  public PageReference initiate(Map<String, String> config, String stateToPropagate) { return null; }
  public Auth.AuthProviderTokenResponse handleCallback(Map<String, String> config, Auth.AuthProviderCallbackState state) { return null; }
  public Auth.UserData getUserInfo(Map<String, String> config, Auth.AuthProviderTokenResponse response) { return null; }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PassiveReflectionTest.cls"), `
@isTest
private class PassiveReflectionTest {
  @isTest static void forNameResolvesPassivePlatformClass() {
    Type jwtType = Type.forName('Auth.JWT');
    System.assertNotEquals(null, jwtType);
    System.assertEquals('Auth.JWT', jwtType.getName());
    System.assertEquals(Auth.JWT.class, jwtType);
    System.assertEquals(jwtType, Type.forName('auth.jwt'));
  }

  @isTest static void forNameNewInstanceOfPassivePlatformClass() {
    Object jwt = Type.forName('Auth.JWT').newInstance();
    System.assert(jwt instanceof Auth.JWT);
  }

  @isTest static void passiveInterfaceIsAssignableFromProjectImplementation() {
    Object plugin = Type.forName('PluginImpl').newInstance();
    System.assert(plugin instanceof Auth.AuthProviderPlugin);
    System.assert(Auth.AuthProviderPlugin.class.isAssignableFrom(Type.forName('PluginImpl')));
    System.assertEquals(false, Auth.AuthProviderPlugin.class.isAssignableFrom(Type.forName('Auth.JWT')));
  }

  @isTest static void projectClassShadowsPassivePlatformClass() {
    Object tag = Type.forName('XmlTag').newInstance();
    System.assertEquals('project', ((XmlTag) tag).origin());
  }

  @isTest static void passiveEnumConstantsKeepTheirValues() {
    System.assertEquals('FINE', LoggingLevel.FINE.name());
    System.assertEquals(LoggingLevel.FINE, LoggingLevel.valueOf('FINE'));
    System.assertEquals('AFTER_INSERT', TriggerOperation.AFTER_INSERT.name());
  }
}
`)
	var outcomes []string
	for i := 0; i < 2; i++ {
		run := Run(loadTestIndex(t, root), Options{})
		var lines []string
		for _, suite := range run.Suites {
			for _, c := range suite.Cases {
				line := c.ClassName + "." + c.MethodName + " " + string(c.Status)
				if c.Problem != nil {
					line += " " + c.Problem.Message
				}
				lines = append(lines, line)
			}
		}
		got := run.Summary()
		if got.Total != 5 || got.Passed != 5 {
			t.Fatalf("run %d summary = %#v cases:\n%s", i, got, strings.Join(lines, "\n"))
		}
		outcomes = append(outcomes, strings.Join(lines, "\n"))
	}
	if outcomes[0] != outcomes[1] {
		t.Fatalf("second Run differs from first:\n%s\n---\n%s", outcomes[0], outcomes[1])
	}
	want := freshPassiveStandardRuntimeClasses(nil, nil)
	for i, template := range passiveStandardRuntimeClassTemplates() {
		if !reflect.DeepEqual(template.class, want[i]) {
			t.Fatalf("Run mutated passive template %s", want[i].Name)
		}
	}
}
