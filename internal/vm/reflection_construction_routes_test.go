package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// reflection_construction/org.tsv records the constructor and namespace cases
// at API 62/67 through anonymous and named test execution.
func TestReflectionConstructionVMAnonymousAndNamedRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named"} {
			for _, tc := range []struct{ name, source string }{
				{"privateConstructor", `try { Type.forName('RoutePrivate').newInstance(); System.assert(false); } catch (TypeException issue) { System.assertEquals('RoutePrivate does not have a no-arg constructor', issue.getMessage()); }`},
				{"privateConstructorLiteral", `try { RoutePrivate.class.newInstance(); System.assert(false); } catch (TypeException issue) { System.assertEquals('RoutePrivate does not have a no-arg constructor', issue.getMessage()); }`},
				{"privateConstructorVariable", `Type target = Type.forName('RoutePrivate'); try { target.newInstance(); System.assert(false); } catch (TypeException issue) { System.assertEquals('RoutePrivate does not have a no-arg constructor', issue.getMessage()); }`},
				{"protectedConstructor", `try { Type.forName('RouteProtected').newInstance(); System.assert(false); } catch (TypeException issue) { System.assertEquals('RouteProtected does not have a no-arg constructor', issue.getMessage()); }`},
				{"abstractValue", `try { Type.forName('RouteAbstract').newInstance(); System.assert(false); } catch (TypeException issue) { System.assertEquals('RouteAbstract cannot be constructed', issue.getMessage()); }`},
				{"contract", `try { Type.forName('RouteContract').newInstance(); System.assert(false); } catch (TypeException issue) { System.assertEquals('RouteContract cannot be constructed', issue.getMessage()); }`},
				{"throwingConstructor", `try { Type.forName('RouteThrowing').newInstance(); System.assert(false); } catch (Exception issue) { System.assertEquals('ConstructionException', issue.getTypeName()); System.assertEquals('construction interrupted', issue.getMessage()); }`},
				{"splitNested", `System.assertEquals(null, Type.forName('RouteOuter', 'Plain')); System.assertEquals(null, Type.forName('RouteOuter', 'RouteOuter.Plain')); System.assertEquals(null, Type.forName('missingzone', 'RouteOuter.Plain'));`},
				{"userInfo", `for (Type token : new List<Type>{Type.forName('UserInfo'), Type.forName('System.UserInfo'), Type.forName('System', 'UserInfo'), Type.forName('System', 'System.UserInfo'), Type.forName('', 'System.UserInfo'), Type.forName(null, 'System.UserInfo'), UserInfo.class}) { System.assertNotEquals(null, token); System.assertEquals('System.UserInfo', token.getName()); System.assertNotEquals(null, token.newInstance()); } System.assertEquals(null, Type.forName('missingzone', 'System.UserInfo'));`},
			} {
				t.Run(fmt.Sprintf("%s/%s/%s", api, route, tc.name), func(t *testing.T) {
					compile := func(source string) ir.Program {
						t.Helper()
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						return program
					}
					machine := New(nil)
					classes := []Class{
						{Name: "RoutePrivate", Access: "public", Constructors: []Method{{Name: "RoutePrivate.<init>", ClassName: "RoutePrivate", Access: "private", IsConstructor: true, Program: compile("")}}},
						{Name: "RouteProtected", Access: "public", Constructors: []Method{{Name: "RouteProtected.<init>", ClassName: "RouteProtected", Access: "protected", IsConstructor: true, Program: compile("")}}},
						{Name: "RouteAbstract", Access: "public", IsAbstract: true},
						{Name: "RouteContract", Access: "public", IsInterface: true},
						{Name: "ConstructionException", Access: "public", SuperClass: "Exception"},
						{Name: "RouteThrowing", Access: "public", Constructors: []Method{{Name: "RouteThrowing.<init>", ClassName: "RouteThrowing", Access: "public", IsConstructor: true, Program: compile(`throw new ConstructionException('construction interrupted');`)}}},
						{Name: "RouteOuter", Access: "public"},
						{Name: "RouteOuter.Plain", Access: "public"},
					}
					for _, class := range classes {
						class.APIVersion = api
						if err := machine.RegisterClass(class); err != nil {
							t.Fatal(err)
						}
					}
					source := tc.source
					if route == "named" {
						machine.EnableTestContext()
						if err := machine.RegisterClass(Class{Name: "ReflectionRouteReader", Access: "public", APIVersion: api, Methods: map[string]Method{"verify": {Name: "ReflectionRouteReader.verify", ClassName: "ReflectionRouteReader", Access: "public", APIVersion: api, IsStatic: true, ReturnType: "void", Program: compile(source)}}}); err != nil {
							t.Fatal(err)
						}
						source = "ReflectionRouteReader.verify();"
					}
					if _, err := machine.Execute(compile(source)); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
