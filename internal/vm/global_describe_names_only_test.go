package vm

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// The global describe map reads object names only. On a lazy org it is built
// from the installed objects plus the ones the spread still owes, instead of
// completing the spread first. These tests compare that build with the old
// one (eagerGlobalDescribe) on both lazy routes.

var namesOnlyDescribeShapes = map[string][]string{
	"unspecified":    {project.UnspecifiedStandardFieldShape},
	"communities":    {"Communities"},
	"networks":       {"Communities", "NetworksEnabled", "Quotes", "MultiCurrency"},
	"orderManaged":   {"OrderManagement", "OrderManagementEnabled"},
	"mixed":          {"Sites", "PersonAccounts", "StateAndCountryPicklist", "ContactsToMultipleAccounts", "PlatformCache", "Territory2", "KnowledgeEnabled"},
	"plainDeveloper": {project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape},
	"capturedB2B":    {"B2BCommerce", "Communities", "NetworksEnabled", "CommerceEnabled"},
}

func namesOnlyCustomObject(name string) storage.ObjectState {
	return storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: name, Label: name, KeyPrefix: "a0" + fmt.Sprint(len(name)%10),
		Fields: map[string]storage.Field{"Id": {APIName: "Id", Label: "Record ID", Type: storage.FieldID}},
	}, Records: map[storage.ID]storage.Record{}}
}

// namesOnlyDescribeOrg builds a lazy org on one route. "source" is the
// source-backed spread of `glade test`; "legacy" is the lean org of
// `glade exec` that seeds what it installs and owes the rest.
func namesOnlyDescribeOrg(route string, features []string, namespace string) storage.OrgState {
	custom := func(org *storage.OrgState) {
		org.Namespace = namespace
		// An installed package object and a project object.
		org.Objects["pkg__Thing__c"] = namesOnlyCustomObject("pkg__Thing__c")
		if namespace != "" {
			org.Objects[namespace+"__Widget__c"] = namesOnlyCustomObject(namespace + "__Widget__c")
		} else {
			org.Objects["Widget__c"] = namesOnlyCustomObject("Widget__c")
		}
	}
	switch route {
	case "source":
		org := storage.NewOrgState()
		org.OrgID = storage.DefaultOrgID
		org.EnableStandardSpreadSource()
		storage.EnsureDeterministicPlatformData(&org)
		storage.ApplyProjectOrgShape(&org, features)
		custom(&org)
		return org
	case "legacy":
		lean := storage.NewOrgState()
		storage.EnsureDeterministicPlatformData(&lean)
		storage.ApplyProjectOrgShape(&lean, features)
		org := storage.NewOrgState()
		org.OrgID = storage.DefaultOrgID
		for _, name := range storage.StandardSpreadObjectsOf(lean) {
			storage.EnsureStandardObject(&org, name)
		}
		storage.EnsureDeterministicPlatformData(&org)
		storage.ApplyProjectOrgShape(&org, features)
		custom(&org)
		org.EnableLazyStandardSpread()
		return org
	}
	panic("unknown route " + route)
}

// namesOnlyProbeNames are the keys each run looks up: core, gated and derived
// names, the catalog's namespaced names, custom and package objects with and
// without their prefix, misses, and objects the shape does not allow.
func namesOnlyProbeNames(features []string) []string {
	names := []string{
		"Account", "account", "Contact", "Quote", "WebStore", "OrderSummary", "Network", "AsyncApexJob",
		"AccountShare", "AccountHistory", "AccountFeed", "AccountChangeEvent", "Knowledge__kav", "Knowledge__ka",
		"Widget__c", "nsx__Widget__c", "Widget__ChangeEvent", "nsx__Widget__ChangeEvent", "Widget__Share",
		"pkg__Thing__c", "Thing__c", "NoSuch__c", "NoSuchObject", "",
	}
	names = append(names, namespacedCatalogObjectNames()...)
	for _, name := range namespacedCatalogObjectNames() {
		if _, rest, ok := strings.Cut(name, "__"); ok {
			names = append(names, rest)
		}
	}
	// A few catalog objects the shape does not allow, taken from the catalog.
	shape := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&shape, features)
	disallowed := 0
	for _, name := range storage.KnownStandardObjectNames() {
		if disallowed == 3 {
			break
		}
		if !shape.AllowsStandardObject(name) {
			names = append(names, name)
			disallowed++
		}
	}
	return names
}

func namesOnlyProbeSource(probes []string) string {
	quoted := make([]string, len(probes))
	for i, name := range probes {
		quoted[i] = "'" + name + "'"
	}
	return `
Schema.DescribeSObjectResult pre = Contact.SObjectType.getDescribe();
String preText = pre.getName() + ':' + pre.fields.getMap().size() + ':' + pre.getKeyPrefix();
Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
List<String> ordered = new List<String>(g.keySet());
List<String> sorted = ordered.clone();
sorted.sort();
String out = g.size() + '|' + String.join(ordered, ',').hashCode() + '|' + String.join(sorted, ',').length() + '|' + String.join(sorted, ',').hashCode();
out += '|' + sorted[0] + '|' + sorted[sorted.size() - 1] + '|' + g.values().size();
for (String n : new List<String>{` + strings.Join(quoted, ",") + `}) {
	Boolean has = g.containsKey(n);
	out += '|' + n + '=' + has;
	if (has) {
		Schema.DescribeSObjectResult d = g.get(n).getDescribe();
		out += ':' + d.getName() + ':' + d.getLabel() + ':' + d.getKeyPrefix() + ':' + d.fields.getMap().size() + ':' + d.isCreateable() + ':' + d.isCustom() + ':' + String.valueOf(g.get(n));
	}
}
Map<String, Schema.SObjectType> again = Schema.getGlobalDescribe();
out += '|' + (again == g) + '|' + again.size() + '|' + String.valueOf(g).length() + '|' + String.valueOf(g).hashCode();
Schema.DescribeSObjectResult post = Contact.SObjectType.getDescribe();
out += '|' + preText + '|' + (post.getName() + ':' + post.fields.getMap().size() + ':' + post.getKeyPrefix());
out += '|' + Schema.getAppDescribe('x').size() + '|' + Schema.getModuleDescribe().size();
return out;
`
}

type namesOnlyCaller struct {
	name      string
	namespace string // org namespace
	caller    string // caller namespace
}

var namesOnlyCallers = []namesOnlyCaller{
	{"system", "", ""},
	{"owner", "nsx", "nsx"},
	{"ownerSystem", "nsx", ""},
}

var namesOnlyDescribeAPIs = []string{"62.0", "64.0", "65.0", "67.0", "68.0"}

func runNamesOnlyProbe(t testing.TB, org *storage.OrgState, eager bool, api string, caller namesOnlyCaller, named, testContext bool, source string) string {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(org)
	machine.eagerGlobalDescribe = eager
	machine.SetCurrentUser(org.Objects["User"].Records[storage.ID(storage.DefaultUserID)])
	if testContext {
		machine.EnableTestContext()
	}
	method := Method{Name: "Anonymous.run", APIVersion: api, SourceContextBound: true, Namespace: caller.caller, Program: program}
	if named {
		machine.Classes["NamesOnlyProbe"] = Class{Name: "NamesOnlyProbe", Namespace: caller.caller}
		method = Method{Name: "NamesOnlyProbe.run", ClassName: "NamesOnlyProbe", IsStatic: true, APIVersion: api, SourceContextBound: true, Namespace: caller.caller, Program: program}
	}
	got, err := machine.callMethod(method, nil, &Result{})
	if err != nil {
		return "error: " + err.Error()
	}
	return got.Text
}

// The names-only map answers what the completed-spread map answers: size,
// key order, keys, values, containsKey hits and misses, per-entry describes,
// equality, String.valueOf, app and module describes, and a describe taken
// before and after the call. Every shape, both lazy routes, system, owner and
// owner-org system callers, anonymous and named source, API 62 to 68.
func TestNamesOnlyGlobalDescribeMatchesCompletedSpread(t *testing.T) {
	shapeNames := make([]string, 0, len(namesOnlyDescribeShapes))
	for name := range namesOnlyDescribeShapes {
		shapeNames = append(shapeNames, name)
	}
	sort.Strings(shapeNames)
	for _, route := range []string{"source", "legacy"} {
		for _, shape := range shapeNames {
			features := namesOnlyDescribeShapes[shape]
			source := namesOnlyProbeSource(namesOnlyProbeNames(features))
			for _, caller := range namesOnlyCallers {
				t.Run(route+"/"+shape+"/"+caller.name, func(t *testing.T) {
					base := namesOnlyDescribeOrg(route, features, caller.namespace)
					for _, api := range namesOnlyDescribeAPIs {
						for _, named := range []bool{false, true} {
							for _, testContext := range []bool{false, true} {
								if testContext && !named && shape != "plainDeveloper" {
									continue
								}
								eagerOrg := base.Clone()
								lazyOrg := base.Clone()
								want := runNamesOnlyProbe(t, &eagerOrg, true, api, caller, named, testContext, source)
								got := runNamesOnlyProbe(t, &lazyOrg, false, api, caller, named, testContext, source)
								if strings.HasPrefix(want, "error: ") {
									t.Fatalf("api %s named=%v test=%v: eager run failed: %s", api, named, testContext, want)
								}
								if diff := probeDifference(want, got); diff != "" {
									t.Fatalf("api %s named=%v test=%v: names-only differs from the completed spread\n%s", api, named, testContext, diff)
								}
								if eagerOrg.LazyStandardSpreadEnabled() {
									t.Fatal("the eager route left the spread incomplete")
								}
								if !lazyOrg.LazyStandardSpreadEnabled() {
									t.Fatal("the names-only route completed the spread")
								}
							}
						}
					}
				})
			}
		}
	}
}

// probeDifference reports the first field where completed and names-only runs differ.
func probeDifference(want, got string) string {
	w, g := strings.Split(want, "|"), strings.Split(got, "|")
	if len(w) != len(g) {
		return fmt.Sprintf("field counts differ: completed %d, names-only %d", len(w), len(g))
	}
	for i := range w {
		if w[i] != g[i] {
			return fmt.Sprintf("field %d: completed %q, names-only %q", i, w[i], g[i])
		}
	}
	return ""
}

// Cost guard: one global describe and one entry describe leave the lazy org
// small. Completing the spread holds more than 5,000 objects.
func TestNamesOnlyGlobalDescribeKeepsSpreadLazy(t *testing.T) {
	for _, route := range []string{"source", "legacy"} {
		for _, caller := range namesOnlyCallers {
			t.Run(route+"/"+caller.name, func(t *testing.T) {
				org := namesOnlyDescribeOrg(route, []string{"Communities"}, caller.namespace)
				source := `
Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
return g.size() + ':' + g.get('Account').getDescribe().getName() + ':' + g.get('WorkOrder').getDescribe().getName();`
				got := runNamesOnlyProbe(t, &org, false, "67.0", caller, true, true, source)
				if !strings.HasSuffix(got, ":Account:WorkOrder") {
					t.Fatalf("probe = %q", got)
				}
				if held := len(org.Objects); held >= 1000 {
					t.Fatalf("org holds %d objects after one global describe, want fewer than 1,000", held)
				}
				if !org.LazyStandardSpreadEnabled() {
					t.Fatal("the global describe completed the spread")
				}
			})
		}
	}
}
