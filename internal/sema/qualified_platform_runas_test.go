package sema

import (
	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

// These owned fixtures isolate the qualified User identity from the unrelated
// source class. They do not depend on any diagnosed project's source.
func TestQualifiedPlatformRunAsWithUserShadow(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, body string
			wantReject bool
		}{
			{"query", `Schema.User acting = [SELECT Id FROM User WHERE Id = :System.UserInfo.getUserId()]; System.runAs(acting) { System.assertEquals(acting.Id, System.UserInfo.getUserId()); }`, false},
			{"query rejection twin", `User acting = new User(); System.runAs(acting) {}`, true},
			{"constructor", `System.runAs(new Schema.User(Id = System.UserInfo.getUserId())) {}`, false},
			{"constructor rejection twin", `System.runAs(new User()) {}`, true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"User.cls": `public class User {}`,
					"Probe.cls": `@IsTest private class Probe {
 @IsTest static void checkIdentity() { ` + tc.body + ` }
}`,
				}, api)
				if tc.wantReject {
					if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
						t.Fatalf("expected runAs contract rejection, got %#v", result.Diagnostics)
					}
				} else if result.HasErrors() {
					t.Fatalf("qualified platform User rejected: %#v", result.Diagnostics)
				}
			})
		}
	}
}

// The source-expression validator and IR statement validator must use the same
// platform contract, including when an unrelated source User is in the model.
func TestQualifiedPlatformRunAsSourceExpressionContract(t *testing.T) {
	for _, shadow := range []bool{false, true} {
		index := typesys.Index{}
		if shadow {
			index.Types = []typesys.TypeSymbol{{Name: "User", Kind: apexast.DeclarationClass}}
		}
		model := buildSemaTypeMemberView(index)
		for _, tc := range []struct {
			arg    string
			reject bool
		}{
			{"Schema.User", false}, {"User", shadow},
		} {
			_, rejected := semaTestRunAsDiagnostic(typesys.TypeSymbol{Name: "Probe"}, tc.arg, 0, 0, "", model)
			if rejected != tc.reject {
				t.Errorf("shadow=%v arg=%s rejected=%v, want %v", shadow, tc.arg, rejected, tc.reject)
			}
		}
	}
}

// Class literals always produce platform Type values, including when a source
// class has that simple name. Their methods are unrelated to missing members on
// the source HttpRequest/SaveResult rejection twins in the native matrix.
func TestSourceClassLiteralMethodsWithPlatformShadows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shadowType := range []bool{false, true} {
			name, typeName := "ordinary", "Type"
			sources := map[string]string{
				"Foo.cls":         `public class Foo { public static String marker() { return 'foo'; } }`,
				"HttpRequest.cls": `public class HttpRequest {}`,
				"SaveResult.cls":  `public class SaveResult {}`,
			}
			if shadowType {
				name, typeName = "source Type shadow", "System.Type"
				sources["Type.cls"] = `public class Type { public static String marker() { return 'shadow'; } }`
			}
			sources["Probe.cls"] = `public class Probe { public static void run() {
    String marker = Foo.marker();
    String name = Foo.class.getName();
    ` + typeName + ` value = Foo.class;
    String storedName = value.getName();
   } }`
			t.Run(api+"/"+name, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, sources, api)
				if result.HasErrors() {
					t.Fatalf("class literal/Type methods rejected: %#v", result.Diagnostics)
				}
			})
		}
	}
}

func TestQualifiedPlatformEnumMethodsWithSourceShadow(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"LoggingLevel.cls": `public class LoggingLevel {}`,
				"Probe.cls": `public class Probe { public static void run(System.LoggingLevel level) {
      String name = level.name();
      String optionalName = level?.name();
      String conditionalName = (true ? System.LoggingLevel.DEBUG : System.LoggingLevel.ERROR).name();
    } }`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("qualified platform enum methods rejected: %#v", result.Diagnostics)
			}
		})
	}
}

// A receiver typed by a platform method's return keeps canonical acceptance
// beside a same-named source class; no native row observes chained receivers.
func TestChainedPlatformReturnReceiverWithSourceShadow(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"HttpResponse.cls": `public class HttpResponse {}`,
				"Matcher.cls":      `public class Matcher {}`,
				"Probe.cls": `public class Probe {
    public static String body(HttpRequest req) { return new Http().send(req).getBody(); }
    public static Boolean matches(String p, String s) { return Pattern.compile(p).matcher(s).matches(); }
  }`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("chained platform-return receiver rejected: %#v", result.Diagnostics)
			}
		})
	}
}
