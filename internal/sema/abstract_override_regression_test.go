package sema

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// abstractOverrideErrors renders each error as code, file, line and native
// text in diagnostic order.
func abstractOverrideErrors(diagnostics []diagnostic.Diagnostic) []string {
	var out []string
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error || d.Code == "GLADESEMA_VERSION" {
			continue
		}
		message := d.NativeMessage
		if message == "" {
			message = d.Message
		}
		line := 0
		if d.NativeLine != nil {
			line = *d.NativeLine
		} else if d.Range != nil {
			line = d.Range.Start.Line
		}
		out = append(out, fmt.Sprintf("%s:%s:%d: %s", d.Code, filepath.Base(d.File), line, message))
	}
	return out
}

// analyzeClassAPIVersions writes each class in name order with its own
// -meta.xml API version.
func analyzeClassAPIVersions(t *testing.T, projectAPI string, files map[string]string, classAPIs map[string]string) Result {
	t.Helper()
	root := t.TempDir()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	paths := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(root, name+".cls")
		writeSemaFile(t, path, files[name])
		api := projectAPI
		if classAPI, ok := classAPIs[name]; ok {
			api = classAPI
		}
		writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		paths = append(paths, path)
	}
	return Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: projectAPI, ApexFiles: paths}, schema.Schema{}))
}

// Native AO R001/C001 match at API 62.0, 66.0 and 67.0 on both routes; PC07
// and PC11 add the missing-implementation error on check-only deploys. The
// 62.0 twin pins the requirement below the former API 66 boundary.
func TestAbstractImplementationOverrideRegression(t *testing.T) {
	t.Parallel()
	base := `public abstract class OracleAbstractBase { public abstract String value(); }`
	files := map[string]string{
		"OracleAbstractBase":  base,
		"OracleAbstractChild": "public class OracleAbstractChild extends OracleAbstractBase {\n  public String value() { return 'value'; }\n}\n",
	}
	withOverride := map[string]string{
		"OracleAbstractBase":  base,
		"OracleAbstractChild": "public class OracleAbstractChild extends OracleAbstractBase {\n  public override String value() { return 'value'; }\n}\n",
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("reject without override at API "+api, func(t *testing.T) {
			got := strings.Join(abstractOverrideErrors(analyzeClassAPIVersions(t, api, files, nil).Diagnostics), "\n")
			want := strings.Join([]string{
				"GLADESEMA016:OracleAbstractChild.cls:2: Method must use the override keyword: String OracleAbstractChild.value()",
				"GLADESEMA017:OracleAbstractChild.cls:1: Class OracleAbstractChild must implement the abstract method: String OracleAbstractBase.value()",
			}, "\n")
			if got != want {
				t.Fatalf("API %s abstract implementation without override:\n%s\nwant\n%s", api, got, want)
			}
		})
		t.Run("accept with override at API "+api, func(t *testing.T) {
			if result := analyzeClassAPIVersions(t, api, withOverride, nil); result.HasErrors() {
				t.Fatalf("abstract implementation with override was rejected at API %s: %#v", api, result.Diagnostics)
			}
		})
	}
}

// Salesforce's AuthProviderPluginClass example implements its provider
// callbacks without override. Keep the usual abstract-base rule as a control.
func TestAuthProviderPluginCallbacksMayOmitOverride(t *testing.T) {
	t.Parallel()
	provider := `global class ProviderChild extends Auth.AuthProviderPluginClass {
    global String getCustomMetadataType() { return 'Provider_Config__mdt'; }
    global PageReference initiate(Map<String,String> config, String state) { return new PageReference('/auth'); }
    global Auth.AuthProviderTokenResponse handleCallback(Map<String,String> config, Auth.AuthProviderCallbackState state) { return null; }
    global Auth.UserData getUserInfo(Map<String,String> config, Auth.AuthProviderTokenResponse response) { return null; }
}`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("provider callbacks at API "+api, func(t *testing.T) {
			result := analyzeClassAPIVersions(t, api, map[string]string{"ProviderChild": provider}, nil)
			if got := abstractOverrideErrors(result.Diagnostics); len(got) != 0 {
				t.Fatalf("documented provider callbacks without override were rejected: %q", got)
			}
		})

		t.Run("ordinary abstract method still requires override at API "+api, func(t *testing.T) {
			files := map[string]string{
				"OracleAbstractBase":  `public abstract class OracleAbstractBase { public abstract String value(); }`,
				"OracleAbstractChild": "public class OracleAbstractChild extends OracleAbstractBase {\n  public String value() { return 'value'; }\n}\n",
			}
			got := strings.Join(abstractOverrideErrors(analyzeClassAPIVersions(t, api, files, nil).Diagnostics), "\n")
			want := strings.Join([]string{
				"GLADESEMA016:OracleAbstractChild.cls:2: Method must use the override keyword: String OracleAbstractChild.value()",
				"GLADESEMA017:OracleAbstractChild.cls:1: Class OracleAbstractChild must implement the abstract method: String OracleAbstractBase.value()",
			}, "\n")
			if got != want {
				t.Fatalf("ordinary abstract override control at API %s:\n%s\nwant\n%s", api, got, want)
			}
		})
	}
}

// PB21/PB21O (base 63.0, subclass 66.0) and PB22/PB22O (base 62.0, subclass
// 67.0): a default-access abstract method is private to its top-level class.
// The subclass implements it without override; override is rejected with the
// native pair of errors per method. PB21C keeps explicit public abstract.
func TestDefaultAccessAbstractAcrossTopLevelClassesRegression(t *testing.T) {
	t.Parallel()
	base := "public abstract class FsBase {\n    abstract Schema.SObjectType getSObjectType();\n\n    abstract List<Schema.SObjectField> getSObjectFieldList();\n\n    public String observe() { return String.valueOf(getSObjectType()) + '|' + getSObjectFieldList().size(); }\n}\n"
	sub := func(modifier string) string {
		return "public class FsSub extends FsBase {\n    public " + modifier + "Schema.SObjectType getSObjectType() { return Account.SObjectType; }\n\n    public " + modifier + "List<Schema.SObjectField> getSObjectFieldList() { return new List<Schema.SObjectField>{ Account.Id }; }\n}\n"
	}
	for _, versions := range [][2]string{{"63.0", "66.0"}, {"62.0", "67.0"}} {
		baseAPI, subAPI := versions[0], versions[1]
		t.Run("accept without override "+baseAPI+"/"+subAPI, func(t *testing.T) {
			result := analyzeClassAPIVersions(t, subAPI, map[string]string{"FsBase": base, "FsSub": sub("")}, map[string]string{"FsBase": baseAPI})
			if got := abstractOverrideErrors(result.Diagnostics); len(got) != 0 {
				t.Fatalf("default-access abstract implementation was rejected: %q", got)
			}
		})
		t.Run("reject override "+baseAPI+"/"+subAPI, func(t *testing.T) {
			result := analyzeClassAPIVersions(t, subAPI, map[string]string{"FsBase": base, "FsSub": sub("override ")}, map[string]string{"FsBase": baseAPI})
			got := strings.Join(abstractOverrideErrors(result.Diagnostics), "\n")
			want := strings.Join([]string{
				"GLADESEMA016:FsSub.cls:2: @Override specified for non-overriding method: Schema.SObjectType FsSub.getSObjectType()",
				"GLADESEMA016:FsSub.cls:4: @Override specified for non-overriding method: List<Schema.SObjectField> FsSub.getSObjectFieldList()",
				"GLADESEMA017:FsSub.cls:2: Class FsSub must implement the abstract method: Schema.SObjectType FsBase.getSObjectType()",
				"GLADESEMA017:FsSub.cls:4: Class FsSub must implement the abstract method: List<Schema.SObjectField> FsBase.getSObjectFieldList()",
			}, "\n")
			if got != want {
				t.Fatalf("default-access abstract override:\n%s\nwant\n%s", got, want)
			}
		})
	}
	publicBase := strings.ReplaceAll(base, "    abstract ", "    public abstract ")
	if result := analyzeClassAPIVersions(t, "66.0", map[string]string{"FsBase": publicBase, "FsSub": sub("override ")}, map[string]string{"FsBase": "63.0"}); result.HasErrors() {
		t.Fatalf("public abstract override across versions was rejected: %#v", result.Diagnostics)
	}
}

// AO R011/C011 on the isTest route (API 62.0-67.0): a default-access virtual
// method in another top-level class is not inherited for override purposes.
func TestDefaultAccessVirtualAcrossTopLevelClassesRegression(t *testing.T) {
	t.Parallel()
	base := "public virtual class DvBase { virtual String tag() { return 'base'; } public String observe() { return tag(); } }"
	for _, api := range []string{"62.0", "67.0"} {
		result := analyzeClassAPIVersions(t, api, map[string]string{"DvBase": base, "DvSub": "public class DvSub extends DvBase { public String tag() { return 'sub'; } }"}, nil)
		if got := abstractOverrideErrors(result.Diagnostics); len(got) != 0 {
			t.Fatalf("API %s default-access virtual redefinition was rejected: %q", api, got)
		}
		result = analyzeClassAPIVersions(t, api, map[string]string{"DvBase": base, "DvSub": "public class DvSub extends DvBase { public override String tag() { return 'sub'; } }"}, nil)
		got := strings.Join(abstractOverrideErrors(result.Diagnostics), "\n")
		if want := "GLADESEMA016:DvSub.cls:1: @Override specified for non-overriding method: String DvSub.tag()"; got != want {
			t.Fatalf("API %s default-access virtual override:\n%s\nwant\n%s", api, got, want)
		}
	}
}

// M2 R035-R037/C035-C037 (top-level class, both routes, API 62.0 and 67.0).
func TestObjectMethodOverrideQuirksRegression(t *testing.T) {
	t.Parallel()
	for _, api := range []string{"62.0", "67.0"} {
		for _, test := range []struct {
			name, member, want string
		}{
			{"R035", "public Boolean equals(Object o) { return false; }", ""},
			{"C035", "public override Boolean equals(Object o) { return false; }", "GLADESEMA016:Quirk.cls:2: @Override specified for non-overriding method: Boolean Quirk.equals(Object)"},
			{"R036", "public Integer hashCode() { return 7; }", ""},
			{"C036", "public override Integer hashCode() { return 7; }", ""},
			{"R037", "public String toString() { return 'synthetic'; }", "GLADESEMA016:Quirk.cls:2: Method must use the override keyword: String Quirk.toString()"},
			{"C037", "public override String toString() { return 'synthetic'; }", ""},
		} {
			result := analyzeClassAPIVersions(t, api, map[string]string{"Quirk": "public class Quirk {\n    " + test.member + "\n}\n"}, nil)
			if got := strings.Join(abstractOverrideErrors(result.Diagnostics), "\n"); got != test.want {
				t.Fatalf("API %s %s: %q, want %q", api, test.name, got, test.want)
			}
		}
	}
}

// AOJ B01/B02 and B05/B06 (check-only deploys, API 62.0-67.0): a nested class
// of another top-level class, including a private inner class of an @IsTest
// class, implements a default-access abstract method without override at
// every API. With override it gets the native pair of errors.
func TestNestedDefaultAccessAbstractAcrossTopLevelClassesRegression(t *testing.T) {
	t.Parallel()
	base := "public abstract class NdBase { abstract String tag(); public String observe() { return tag(); } }"
	selector := func(modifier string) string {
		return "@IsTest private class NdTest {\n    private class NdSelector extends NdBase { public " + modifier + "String tag() { return 'inner'; } }\n}\n"
	}
	for _, api := range []string{"62.0", "64.0", "66.0", "67.0"} {
		result := analyzeClassAPIVersions(t, api, map[string]string{"NdBase": base, "NdTest": selector("")}, map[string]string{"NdBase": "62.0"})
		if got := abstractOverrideErrors(result.Diagnostics); len(got) != 0 {
			t.Fatalf("API %s nested default-access abstract implementation was rejected: %q", api, got)
		}
		result = analyzeClassAPIVersions(t, api, map[string]string{"NdBase": base, "NdTest": selector("override ")}, map[string]string{"NdBase": "62.0"})
		got := strings.Join(abstractOverrideErrors(result.Diagnostics), "\n")
		want := strings.Join([]string{
			"GLADESEMA016:NdTest.cls:2: @Override specified for non-overriding method: String NdTest.NdSelector.tag()",
			"GLADESEMA017:NdTest.cls:1: Class NdTest.NdSelector must implement the abstract method: String NdBase.tag()",
		}, "\n")
		if got != want {
			t.Fatalf("API %s nested default-access abstract override:\n%s\nwant\n%s", api, got, want)
		}
	}
}

// AOJ A03 (API 63.0, 64.0) and AOK A01/A03/A05/A11 (API 62.0, 67.0) deploys:
// a protected implementation without override in a non-virtual class also
// reports a new protected method, for protected, public and global bases. A
// virtual implementing class reports only the first two errors.
func TestProtectedAbstractImplementationWithoutOverrideRegression(t *testing.T) {
	t.Parallel()
	for _, api := range []string{"62.0", "64.0", "67.0"} {
		for _, test := range []struct {
			name, base, sub string
			newProtected    bool
		}{
			{"protected base", "public abstract class PaBase { protected abstract String tag(); public String observe() { return tag(); } }", "public class PaSub", true},
			{"public base", "public abstract class PaBase { public abstract String tag(); public String observe() { return tag(); } }", "public class PaSub", true},
			{"global base", "global abstract class PaBase { global abstract String tag(); global String observe() { return tag(); } }", "public class PaSub", true},
			{"virtual implementer", "public abstract class PaBase { protected abstract String tag(); public String observe() { return tag(); } }", "public virtual class PaSub", false},
		} {
			files := map[string]string{"PaBase": test.base, "PaSub": test.sub + " extends PaBase { protected String tag() { return 'sub'; } }"}
			want := []string{
				"GLADESEMA016:PaSub.cls:1: Method must use the override keyword: String PaSub.tag()",
				"GLADESEMA017:PaSub.cls:1: Class PaSub must implement the abstract method: String PaBase.tag()",
			}
			if test.newProtected {
				want = append(want, "GLADESEMA032:PaSub.cls:1: New protected methods cannot be defined in non-virtual classes")
			}
			if got := strings.Join(abstractOverrideErrors(analyzeClassAPIVersions(t, api, files, nil).Diagnostics), "\n"); got != strings.Join(want, "\n") {
				t.Fatalf("API %s %s:\n%s\nwant\n%s", api, test.name, got, strings.Join(want, "\n"))
			}
		}
	}
}

// AOK A04/A06 deploys (API 62.0, 67.0): a protected override of a public or
// global abstract method reduces its visibility and does not implement it.
func TestNarrowedAbstractOverrideRegression(t *testing.T) {
	t.Parallel()
	for _, api := range []string{"62.0", "67.0"} {
		files := map[string]string{
			"NvBase": "global abstract class NvBase { global abstract String tag(); global String observe() { return tag(); } }",
			"NvSub":  "public class NvSub extends NvBase { protected override String tag() { return 'sub'; } }",
		}
		want := strings.Join([]string{
			"GLADESEMA016:NvSub.cls:1: Cannot reduce the visibility of method: String NvSub.tag()",
			"GLADESEMA017:NvSub.cls:1: Class NvSub must implement the abstract method: String NvBase.tag()",
		}, "\n")
		if got := strings.Join(abstractOverrideErrors(analyzeClassAPIVersions(t, api, files, nil).Diagnostics), "\n"); got != want {
			t.Fatalf("API %s narrowed abstract override:\n%s\nwant\n%s", api, got, want)
		}
	}
}

// AOL A01 and B01-B06 (check-only deploys, API 62.0): narrowed overrides of a
// virtual method report only the visibility error; private implementations of
// abstract methods report the override or visibility error and must-implement.
func TestNarrowedVirtualAndPrivateImplementationRegression(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, base, sub string
		want            []string
	}{
		{"protected override of public virtual", "public virtual class NpBase { public virtual String tag() { return 'base'; } }", "protected override", []string{
			"GLADESEMA016:NpSub.cls:1: Cannot reduce the visibility of method: String NpSub.tag()",
		}},
		{"private implementation of public abstract", "public abstract class NpBase { public abstract String tag(); }", "private", []string{
			"GLADESEMA016:NpSub.cls:1: Method must use the override keyword: String NpSub.tag()",
			"GLADESEMA017:NpSub.cls:1: Class NpSub must implement the abstract method: String NpBase.tag()",
		}},
		{"private override of protected abstract", "public abstract class NpBase { protected abstract String tag(); }", "private override", []string{
			"GLADESEMA016:NpSub.cls:1: Cannot reduce the visibility of method: String NpSub.tag()",
			"GLADESEMA017:NpSub.cls:1: Class NpSub must implement the abstract method: String NpBase.tag()",
		}},
	} {
		files := map[string]string{"NpBase": test.base, "NpSub": "public class NpSub extends NpBase { " + test.sub + " String tag() { return 'sub'; } }"}
		if got := strings.Join(abstractOverrideErrors(analyzeClassAPIVersions(t, "62.0", files, nil).Diagnostics), "\n"); got != strings.Join(test.want, "\n") {
			t.Fatalf("%s:\n%s\nwant\n%s", test.name, got, strings.Join(test.want, "\n"))
		}
	}
	// AOL B02/B04/B06 bisect: the override access error follows from API 65.0.
	files := map[string]string{"NpBase": "public abstract class NpBase { public abstract String tag(); }", "NpSub": "public class NpSub extends NpBase { private override String tag() { return 'sub'; } }"}
	for api, access := range map[string]bool{"64.0": false, "65.0": true, "67.0": true} {
		want := []string{
			"GLADESEMA016:NpSub.cls:1: Cannot reduce the visibility of method: String NpSub.tag()",
			"GLADESEMA017:NpSub.cls:1: Class NpSub must implement the abstract method: String NpBase.tag()",
		}
		if access {
			want = append(want, "GLADESEMA032:NpSub.cls:1: override methods require at least one of the following global, public, protected")
		}
		if got := strings.Join(abstractOverrideErrors(analyzeClassAPIVersions(t, api, files, nil).Diagnostics), "\n"); got != strings.Join(want, "\n") {
			t.Fatalf("API %s private override:\n%s\nwant\n%s", api, got, strings.Join(want, "\n"))
		}
	}
}
