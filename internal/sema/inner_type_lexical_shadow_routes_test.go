package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

const innerShadowScopeStore = `public virtual class ScopeStore {
    public virtual class EventBus {
        public virtual List<Database.SaveResult> publish(List<SObject> events) { return null; }
    }
    public class Reader {
        public List<Database.SaveResult> lexical() { return new EventBus().publish(new List<SObject>()); }
        public List<Database.SaveResult> explicitSystem() { return System.EventBus.publish(new List<SObject>()); }
    }
}
`

// The sources are the named sources captured in testdata/inner_type_scope/org.tsv
// and apextest/testdata/conformance/inner_type_shapes/org.tsv at API 62.0 and
// 67.0. Analyze runs the IR and text call checkers; each is checked alone so
// one route cannot conceal acceptance or a different message from the other.
func TestInnerTypeLexicalShadowIRAndTextRoutes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		files  map[string]string
		native string // "" accepts
	}{
		// dispatch_followups/org.tsv:12-13 accepts the permission-set Id call;
		// corpus_shape/native/org.tsv:6,11 accepts SObject type-token calls.
		{"platform_constant_instance_methods", map[string]string{"ConstantRoutes": `public class ConstantRoutes {
    public static void observe() {
        Id permissionSetId = null;
        AccessLevel bare = AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
        System.AccessLevel qualified = System.AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
        Schema.DescribeSObjectResult recordType = Account.SObjectType.getDescribe();
        Schema.DescribeFieldResult field = Account.Name.getDescribe();
    }
}`}, ""},
		// inner_type_scope/org.tsv:2-3 org_project.
		{"unrelated_system_receiver", map[string]string{
			"ScopeStore": innerShadowScopeStore,
			"ScopePublisher": `public class ScopePublisher {
    public List<SObject> records = new List<SObject>();
    public List<Database.SaveResult> publish() { return EventBus.publish(this.records); }
    public List<Database.SaveResult> mixedCase() { return eVeNtBuS.PuBlIsH(this.records); }
}`}, ""},
		// inner_type_scope/org.tsv:4-5 org_lexical_static_reject.
		{"lexical_static_reject", map[string]string{"ScopeLexicalReject": `public class ScopeLexicalReject {
    public class EventBus { public List<Database.SaveResult> publish(List<SObject> events) { return null; } }
    public class Reader { public List<Database.SaveResult> run() { return EventBus.publish(new List<SObject>()); } }
}`}, "Non static method cannot be referenced from a static context: List<Database.SaveResult> ScopeLexicalReject.EventBus.publish(List<SObject>)"},
		// inner_type_scope/org.tsv:6-7 org_lexical_mixed_reject.
		{"lexical_mixed_reject", map[string]string{"ScopeMixedReject": `public class ScopeMixedReject {
    public class EVENTBUS { public List<Database.SaveResult> publish(List<SObject> events) { return null; } }
    public class Reader { public List<Database.SaveResult> run() { return eVeNtBuS.publish(new List<SObject>()); } }
}`}, "Non static method cannot be referenced from a static context: List<Database.SaveResult> ScopeMixedReject.EVENTBUS.publish(List<SObject>)"},
		// inner_type_scope/org.tsv:34-35 review-org_inherited_static_reject.
		{"inherited_static_reject", map[string]string{
			"ScopeStore":           innerShadowScopeStore,
			"ScopeInheritedReject": `public class ScopeInheritedReject extends ScopeStore { public List<Database.SaveResult> run() { return EventBus.publish(new List<SObject>()); } }`,
		}, "Non static method cannot be referenced from a static context: List<Database.SaveResult> ScopeStore.EventBus.publish(List<SObject>)"},
		// inner_type_scope/org.tsv:30-31 static-org_inherited_static_success.
		{"inherited_static_success", map[string]string{
			"ScopeBusBase": `public virtual class ScopeBusBase { public static String publish() { return 'ok'; } }`,
			"ScopeStaticOuter": `public class ScopeStaticOuter {
 public class EventBus extends ScopeBusBase {}
 public class Reader { public String run() { return EventBus.publish(); } }
}`}, ""},
		// inner_type_scope/org.tsv:32-33 static-org_lexical_missing_reject.
		{"lexical_missing_reject", map[string]string{"ScopeMissingReject": `public class ScopeMissingReject {
 public class EventBus {}
 public class Reader { public List<Database.SaveResult> run() { return EventBus.publish(new List<SObject>()); } }
}`}, "Method does not exist or incorrect signature: void publish(List<SObject>) from the type ScopeMissingReject.EventBus"},
		// inner_type_scope/org.tsv:26-27 enum-org_lexical_enum_publish_reject.
		{"lexical_enum_publish_reject", map[string]string{"ScopeEnumReject": `public class ScopeEnumReject {
    public enum EventBus { One }
    public List<Database.SaveResult> run() { return EventBus.publish(new List<SObject>()); }
    public void mixed() { eVeNtBuS.pUbLiSh(new List<SObject>()); }
}`}, "Method does not exist or incorrect signature: void publish(List<SObject>) from the type ScopeEnumReject.EventBus"},
		// inner_type_scope/org.tsv:16-17 enum-org_enum_project: values() on a
		// System-named inner enum and qualified System publish stay valid.
		{"lexical_enum_values", map[string]string{"ScopeEnumPublisher": `public class ScopeEnumPublisher {
    public enum EventBus { One }
    public static Integer publish() { return System.EventBus.publish(new List<SObject>()).size(); }
    public static Integer enumValues() { return EventBus.values().size(); }
}`}, ""},
		// inner_type_shapes/org.tsv:74,77 lexical_type_reject (named).
		{"lexical_type_reject", map[string]string{"CapRej": `public class CapRej { public enum Type {A} void r(){ Type.forName('X'); } }`},
			"Method does not exist or incorrect signature: void forName(String) from the type CapRej.Type"},
		// inner_type_shapes/org.tsv:18-21 system_named_types and 22-25
		// system_enum_constant: dotted value paths are never static calls.
		{"system_named_value_paths", map[string]string{
			"CapSys": `public class CapSys { public class Request {} public enum Type {A,B}
  public static String c(){ return Type.B.name()+Type.values().size()+String.valueOf(Request.class)+new List<Request>{new Request()}.size(); } }`,
			"CapBus": `public class CapBus { public enum EventBus {publish,other} public static String value(){ return EventBus.publish.name(); } }`,
		}, ""},
		// Enum statics values() and valueOf(String) stay members of inner enums
		// named like non-implicit platform types, in the class and a subclass:
		// inner_type_scope/org.tsv:16-21 and inner_type_shapes/org.tsv:6-9,26-29.
		{"inner_enum_builtin_statics", map[string]string{
			"CapStatus": `public virtual class CapStatus {
    public enum Status { OPEN, DONE }
    public enum Message { HELLO }
    public static String value() { return String.valueOf(Status.values().size()) + Status.valueOf('DONE') + Message.values().size() + Message.valueOf('HELLO'); }
}`,
			"CapStatusChild": `public class CapStatusChild extends CapStatus {
    public static String child() { List<Status> all = Status.values(); return String.valueOf(all.size()) + Status.valueOf('OPEN') + Message.valueOf('HELLO'); }
}`,
		}, ""},
		// capture error-order-inner: a genuinely missing static method
		// on a lexical inner enum is still rejected.
		{"inner_enum_missing_static", map[string]string{"OrdA": `public class OrdA {
    void a() { X.m1(); }
    enum X { A }
}`}, "Method does not exist or incorrect signature: void m1() from the type OrdA.X"},
		// Tip behavior kept for a System-named inner subclass: simple-name
		// instanceof and cast bind to the inner type, as the native
		// dated_dispatch_cache and inner_type_shapes simple_instanceof/simple_cast
		// rows observe for non-colliding inner subclasses.
		{"colliding_inner_instanceof_cast", map[string]string{"CapReqBase": `public abstract class CapReqBase {
    public Boolean g() { return this instanceof Request; }
    public static Boolean c(CapReqBase b) { return ((Request) b) != null; }
    public class Request extends CapReqBase {}
}`}, ""},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for name, source := range tc.files {
					path := filepath.Join(root, name+".cls")
					writeSemaFile(t, path, source)
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				full := Analyze(index)
				if got := innerShadowNativeErrors(full.Diagnostics); (tc.native == "") != (len(got) == 0) || (tc.native != "" && !innerShadowHasNative(got, tc.native)) {
					t.Fatalf("Analyze: native %q; got %q (%v)", tc.native, got, full.Diagnostics)
				}
				for _, route := range []string{"IR", "text"} {
					a := NewAnalyzer()
					a.prepareAnalysisContext(index, AnalyzeOptions{})
					a.prepareAnalysisModel(index)
					model := buildSemaTypeMemberView(index)
					var diagnostics []diagnostic.Diagnostic
					for _, typ := range index.Types {
						source, err := os.ReadFile(typ.File)
						if err != nil {
							t.Fatal(err)
						}
						typeModel := semaModelWithCurrentType(model, typ)
						for _, member := range typ.Members {
							if member.Kind != apexast.DeclarationMethod {
								continue
							}
							body, offset, ok := semaBodyFromRange(string(source), member.BodyRange)
							if !ok {
								continue
							}
							member = semaNormalizeMemberTypes(typeModel, typ.Name, member)
							base := semaBodyBaseScope(typ, member, typeModel)
							if route == "IR" {
								diags, compiled := a.checkBodyIRWithCompileStatus(typ, member, body, offset, string(source), base, typeModel, nil)
								if !compiled {
									t.Fatalf("fixture body did not lower: %v", diags)
								}
								diagnostics = append(diagnostics, diags...)
							} else {
								scopes, diags := a.collectBodyScopes(typ, member, body, offset, string(source), base, typeModel)
								diagnostics = append(diagnostics, diags...)
								diagnostics = append(diagnostics, a.checkBodyCalls(typ, member, body, offset, string(source), scopes, typeModel)...)
							}
						}
					}
					got := innerShadowNativeErrors(diagnostics)
					if tc.native == "" && len(got) != 0 || tc.native != "" && !innerShadowHasNative(got, tc.native) {
						t.Errorf("%s route: native %q; got %q (%v)", route, tc.native, got, diagnostics)
					}
				}
			})
		}
	}
}

func innerShadowNativeErrors(diagnostics []diagnostic.Diagnostic) []string {
	var out []string
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		message := d.NativeMessage
		if message == "" {
			message = d.Message
		}
		out = append(out, message)
	}
	return out
}

func innerShadowHasNative(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// inner_type_shapes/org.tsv:58,61 field_enum_collision (named): the head of
// the initializer binds to the field being declared.
func TestInnerTypeStaticFieldSelfReferenceNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
			"CapCart.cls": "public class CapCart { public enum Level {LOW} public static Level level = Level.LOW; public static String f(){ return level.name(); } }",
		}, api)
		if got := innerShadowNativeErrors(result.Diagnostics); len(got) != 1 || got[0] != "Illegal forward reference: level" {
			t.Fatalf("%s: got %q", api, got)
		}
		// A different field name keeps the enum constant read valid.
		result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
			"CapCart.cls": "public class CapCart { public enum Level {LOW} public static Level current = Level.LOW; public static String f(){ return current.name(); } }",
		}, api)
		if got := innerShadowNativeErrors(result.Diagnostics); len(got) != 0 {
			t.Fatalf("%s control: got %q", api, got)
		}
	}
}
