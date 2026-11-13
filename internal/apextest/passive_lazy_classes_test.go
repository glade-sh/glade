package apextest

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// passiveLazyMachines registers the passive platform classes twice: as lazy
// shells and as full declarations, the registration the shells stand in for.
func passiveLazyMachines(t testing.TB) (eager, lazy *vm.VM) {
	t.Helper()
	eager, lazy = vm.New(nil), vm.New(nil)
	if err := registerBaseRuntime(eager, nil, eagerClasses(passiveStandardRuntimeClasses(nil, nil)), nil); err != nil {
		t.Fatal(err)
	}
	if err := registerBaseRuntime(lazy, nil, passiveStandardRuntimeClasses(nil, nil), nil); err != nil {
		t.Fatal(err)
	}
	eager.FreezeClassLookup()
	lazy.FreezeClassLookup()
	return eager, lazy
}

// withoutStaticRefs returns the class with the heap identities of its static
// values cleared. Each registration allocates fresh identities.
func withoutStaticRefs(class vm.Class) vm.Class {
	if len(class.StaticFields) == 0 {
		return class
	}
	statics := make(map[string]vm.Field, len(class.StaticFields))
	for name, field := range class.StaticFields {
		field.Value.Ref, field.InitialValue.Ref = 0, 0
		statics[name] = field
	}
	class.StaticFields = statics
	return class
}

func TestPassiveLazyTemplatesHaveNoStaticState(t *testing.T) {
	storage.EnableStandardCatalogRelease()
	lazyCount := 0
	for _, template := range passiveStandardRuntimeClassTemplates() {
		fresh := template.class.FreshClass()
		if !template.lazy {
			// Enums stay eager even when they declare no constant.
			continue
		}
		lazyCount++
		if !template.class.IsLazy() {
			t.Fatalf("template %s is marked lazy and is not a shell", fresh.Name)
		}
		if len(fresh.StaticFields) != 0 || len(fresh.StaticFieldOrder) != 0 || len(fresh.EnumValues) != 0 ||
			len(fresh.StaticInitializers) != 0 || len(fresh.InstanceInitializers) != 0 {
			t.Errorf("lazy passive class %s has static state or initializers", fresh.Name)
		}
		shell := template.class
		if shell.Name != fresh.Name || shell.Namespace != fresh.Namespace || shell.APIVersion != fresh.APIVersion ||
			shell.SuperClass != fresh.SuperClass || shell.Access != fresh.Access || shell.IsAbstract != fresh.IsAbstract ||
			shell.IsInterface != fresh.IsInterface || shell.PlatformType != fresh.PlatformType || shell.IsTest != fresh.IsTest ||
			shell.Dependency != fresh.Dependency || shell.Nested != fresh.Nested ||
			!reflect.DeepEqual(shell.Interfaces, fresh.Interfaces) || !reflect.DeepEqual(shell.Modifiers, fresh.Modifiers) {
			t.Errorf("shell header of %s differs from its built class", fresh.Name)
		}
		if len(shell.Fields) != 0 || len(shell.Methods) != 0 || len(shell.Constructors) != 0 {
			t.Errorf("shell %s carries members", fresh.Name)
		}
	}
	if lazyCount < 5000 {
		t.Fatalf("lazy passive classes = %d; the platform view should yield thousands", lazyCount)
	}
}

// Every class the lazy registration holds is, once built, the class an eager
// registration holds, and the method tables list the same names.
func TestPassiveLazyRegistrationMatchesEagerClassByClass(t *testing.T) {
	storage.EnableStandardCatalogRelease()
	eager, lazy := passiveLazyMachines(t)
	if len(lazy.Classes) != len(eager.Classes) {
		t.Fatalf("classes = %d, want %d", len(lazy.Classes), len(eager.Classes))
	}
	want := make(map[string]vm.Class, len(eager.Classes))
	for name, class := range eager.ClassEntries() {
		want[name] = class
	}
	seen := 0
	for name, class := range lazy.ClassEntries() {
		expected, ok := want[name]
		if !ok {
			t.Fatalf("lazy registration holds %s, eager does not", name)
		}
		seen++
		if !reflect.DeepEqual(withoutStaticRefs(class), withoutStaticRefs(expected)) {
			t.Fatalf("class %s differs between lazy and eager registration", name)
		}
	}
	if seen != len(want) {
		t.Fatalf("enumerated %d classes, want %d", seen, len(want))
	}
	names := func(machine *vm.VM) []string {
		var out []string
		machine.RangeRegisteredMethods(func(method vm.Method) { out = append(out, method.Name) })
		sort.Strings(out)
		return out
	}
	if got, want := names(lazy), names(eager); !reflect.DeepEqual(got, want) {
		t.Fatalf("registered method names differ: %d lazy, %d eager", len(got), len(want))
	}
}

// Type.forName by every name form, instantiation and member dispatch answer the
// same on lazy and eager registrations, on the machine and on its clones.
func TestPassiveLazyRegistrationMatchesEagerExecution(t *testing.T) {
	storage.EnableStandardCatalogRelease()
	eager, lazy := passiveLazyMachines(t)
	var source strings.Builder
	templates := passiveStandardRuntimeClassTemplates()
	sampled := 0
	for i, template := range templates {
		if i%23 != 0 && !strings.HasPrefix(template.class.Name, "Auth.") && template.class.Name != "Reports.ReportMetadata" {
			continue
		}
		sampled++
		name := template.class.Name
		forms := []string{name, strings.ToLower(name), strings.ToUpper(name)}
		if dot := strings.IndexByte(name, '.'); dot > 0 {
			forms = append(forms, name[dot+1:])
			fmt.Fprintf(&source, "System.debug('ns2 %s ' + String.valueOf(Type.forName('%s', '%s') != null));\n", name, name[:dot], name[dot+1:])
		}
		for _, form := range forms {
			fmt.Fprintf(&source, "System.debug('forName %s ' + (Type.forName('%s') == null ? 'null' : Type.forName('%s').getName()));\n", form, form, form)
		}
		fmt.Fprintf(&source, "try { Object o = Type.forName('%s').newInstance(); System.debug('new %s ' + String.valueOf(o != null)); } catch (Exception e) { System.debug('ex %s ' + e.getTypeName()); }\n", name, name, name)
	}
	source.WriteString("System.debug('unknown ' + String.valueOf(Type.forName('Auth.NoSuchType') == null));\n")
	program, err := vm.CompileAnonymous(source.String())
	if err != nil {
		t.Fatal(err)
	}
	run := func(machine *vm.VM) string {
		result, err := machine.Execute(program)
		if err != nil {
			return "error: " + err.Error()
		}
		return strings.Join(result.Debug, "\n")
	}
	want := run(eager)
	if !strings.Contains(want, "forName Auth.JWT Auth.JWT") {
		t.Fatalf("eager baseline did not resolve Auth.JWT:\n%.600s", want)
	}
	for name, machine := range map[string]*vm.VM{
		"lazy":                lazy,
		"lazy clone":          lazy.CloneRuntimeFrozenShared(nil),
		"lazy full clone":     lazy.CloneRuntime(nil),
		"eager clone":         eager.CloneRuntimeFrozenShared(nil),
		"lazy after others":   lazy,
		"lazy clone of clone": lazy.CloneRuntimeFrozenShared(nil).CloneRuntime(nil),
	} {
		if got := run(machine); got != want {
			t.Errorf("%s output differs from eager registration (%d classes sampled)", name, sampled)
			wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
			for i := 0; i < len(wantLines) && i < len(gotLines); i++ {
				if wantLines[i] != gotLines[i] {
					t.Errorf("first difference at line %d:\n eager: %s\n %s: %s", i, wantLines[i], name, gotLines[i])
					break
				}
			}
		}
	}
}

// A cached runtime stores the full classes and restores the shells.
func TestPassiveLazyClassesRoundTripThroughStartupCachePayload(t *testing.T) {
	storage.EnableStandardCatalogRelease()
	classes := append([]vm.Class{{Name: "ProjectClass"}}, passiveStandardRuntimeClasses(nil, nil)...)
	persisted := PersistableClasses(classes)
	if len(persisted) != len(classes) {
		t.Fatalf("persisted %d classes, want %d", len(persisted), len(classes))
	}
	for i, class := range persisted {
		if class.IsLazy() {
			t.Fatalf("persisted class %d (%s) is a shell", i, class.Name)
		}
	}
	for i, class := range classes {
		if class.IsLazy() && len(persisted[i].Methods)+len(persisted[i].Fields)+len(persisted[i].Constructors) == 0 && len(class.Interfaces) == 0 && class.SuperClass == "" {
			continue
		}
	}
	restored := RestoreLazyClasses(persisted)
	if len(restored) != len(classes) {
		t.Fatalf("restored %d classes, want %d", len(restored), len(classes))
	}
	for i := range classes {
		if restored[i].IsLazy() != classes[i].IsLazy() || restored[i].Name != classes[i].Name {
			t.Fatalf("restored class %d = %s lazy=%v, want %s lazy=%v", i, restored[i].Name, restored[i].IsLazy(), classes[i].Name, classes[i].IsLazy())
		}
		if !reflect.DeepEqual(restored[i].FreshClass(), classes[i].FreshClass()) {
			t.Fatalf("restored class %s differs", classes[i].Name)
		}
	}
	// A slice with no shell is returned as is.
	plain := []vm.Class{{Name: "A"}, {Name: "B"}}
	if got := PersistableClasses(plain); &got[0] != &plain[0] {
		t.Fatal("PersistableClasses copied a slice with no shells")
	}
	if got := RestoreLazyClasses(plain); &got[0] != &plain[0] {
		t.Fatal("RestoreLazyClasses copied a slice with no platform classes")
	}
}
