package apextest

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// conformanceRunner holds an unexecuted linked base for one family/source API.
// It never executes a row on the base. Compilation and semantic analysis remain
// at the call site so rejected rows retain their existing checks and diagnostics.
type conformanceRunner struct {
	base    *vm.VM
	org     *storage.RuntimeTemplate
	sources map[string]conformanceSourceGeneration
}

type conformanceSourceGeneration struct {
	Digest     [32]byte
	APIVersion string
	Dependency bool
}

func conformanceSourceName(name, namespace string) string {
	if namespace != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(namespace)+".") {
		name = namespace + "." + name
	}
	return strings.ToLower(name)
}

type conformanceRunnerOptions struct {
	// LinkProject preserves the helper-backed families' method/class registration.
	// Families that previously used a bare VM leave this false.
	LinkProject bool
	Org         *storage.OrgState
	// Row-specific declarations extend a private clone of the linked API base.
	Base *conformanceRunner
	// Declaration rows use the request compiler rather than the test compiler.
	RequestRuntime *CompiledProjectRuntime
}

func newConformanceRunner(t *testing.T, index typesys.Index, options conformanceRunnerOptions) *conformanceRunner {
	t.Helper()
	releaseRuntimeCachesAfterTest(t)
	sources := make(map[string]conformanceSourceGeneration)
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() || (!options.LinkProject && options.RequestRuntime == nil) {
			continue
		}
		digest, ok := index.SourceDigest(typ.File)
		if !ok {
			t.Fatalf("missing conformance source digest: %s", typ.File)
		}
		name := conformanceSourceName(typ.Name, typ.Namespace)
		sources[name] = conformanceSourceGeneration{digest, typ.EffectiveAPIVersion, typ.Dependency}
	}
	if options.Base != nil {
		for name, generation := range sources {
			if previous, exists := options.Base.sources[name]; exists && previous != generation {
				// These callers pass a complete row index. A changed definition
				// replaces the old project rather than merging two generations'
				// fields, constructors and initializer lists in RegisterClass.
				options.Base = nil
				break
			}
		}
	}
	base := vm.New(nil)
	var include func(typesys.TypeSymbol) bool
	if options.Base != nil {
		base = options.Base.newMachine()
		// The base never executes. Detach the parent's org and schema caches so
		// this runner's stamp below never adopts caches built for another schema.
		base.SetOrg(nil)
		include = func(typ typesys.TypeSymbol) bool {
			if !typ.HasSourceSnapshot() {
				return false
			}
			_, alreadyLinked := options.Base.sources[conformanceSourceName(typ.Name, typ.Namespace)]
			return !alreadyLinked
		}
	}
	if options.LinkProject {
		methods := compileProjectMethodsWhere(index, include)
		classes := compileProjectClassesWhere(index, methods, include)
		for _, method := range methods {
			if err := base.RegisterMethod(method); err != nil {
				t.Fatal(err)
			}
		}
		for _, class := range classes {
			if err := base.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
		}
	}
	if options.RequestRuntime != nil {
		runtime := *options.RequestRuntime
		if options.Base != nil {
			// Link only new source types: re-registering an unchanged class
			// appends its static and instance initializers to the linked base.
			sourceTypes := make(map[string]bool)
			for _, typ := range index.Types {
				if include(typ) {
					sourceTypes[conformanceSourceName(typ.Name, typ.Namespace)] = true
				}
			}
			runtime.Methods = make(map[string]vm.Method)
			for key, method := range options.RequestRuntime.Methods {
				if sourceTypes[conformanceSourceName(method.ClassName, "")] {
					runtime.Methods[key] = method
				}
			}
			runtime.Classes = nil
			for _, class := range options.RequestRuntime.Classes {
				if sourceTypes[conformanceSourceName(class.Name, class.Namespace)] {
					runtime.Classes = append(runtime.Classes, class)
				}
			}
		}
		if err := RegisterCompiledProjectRuntimeForRequest(base, runtime); err != nil {
			t.Fatal(err)
		}
	}
	base.FreezeClassLookup()
	if options.Base != nil {
		for name, generation := range options.Base.sources {
			if _, exists := sources[name]; !exists {
				sources[name] = generation
			}
		}
	}
	runner := &conformanceRunner{base: base, sources: sources}
	if options.Org != nil {
		template := storage.NewRuntimeTemplate(*options.Org)
		vm.PrimeRuntimeTemplateSchema(&template)
		// Same contract as Run: clones of a primed base share describe and
		// JSON child-relationship caches for this one schema generation.
		base.PrimeMetadataSchema(&template.Org)
		runner.org = &template
	}
	return runner
}

func (r *conformanceRunner) newMachine() *vm.VM {
	// Frozen artifacts are shared; static fields detach on first access. Each
	// clone starts with fresh globals, limits, logs, request and async state.
	machine := r.base.CloneRuntimeFrozenShared(nil)
	if r.org != nil {
		org := r.newOrg()
		machine.SetRuntimeTemplateOrg(&org)
	}
	return machine
}

// newOrg clones the template org for one row. Records, indexes, ID sequences
// and transactions are private; definitions use the template's frozen boundary.
// OrgState.CloneRuntimeFrozenShared also isolates rows (see
// TestConformanceRunnerIsolatesRowDML) but measured slower here: both rebuild
// the Objects map, and it normalizes every object name twice per row.
func (r *conformanceRunner) newOrg() storage.OrgState {
	return r.org.CloneRuntimeOrg()
}

func (r *conformanceRunner) execute(program ir.Program, configure ...func(*vm.VM)) (vm.Result, error) {
	machine := r.newMachine()
	for _, setup := range configure {
		setup(machine)
	}
	return machine.Execute(program)
}

// Compare with the former per-row registration path while deliberately dirtying
// statics, collections, org records, governor counters, globals and debug logs.
func TestConformanceRunnerMatchesFreshRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			path := filepath.Join(root, "force-app/main/default/classes/ConformanceRowState.cls")
			writeFile(t, path, `public class ConformanceRowState {
 public static Integer calls = 0;
 public static List<String> entries = new List<String>{'seed'};
 static { ConformanceInitLog.entries.add('static'); }
 public Integer initialized = markInstance();
 private static Integer markInstance(){ConformanceInitLog.entries.add('instance'); return 1;}
}`)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			logPath := filepath.Join(root, "force-app/main/default/classes/ConformanceInitLog.cls")
			writeFile(t, logPath, "public class ConformanceInitLog { public static List<String> entries = new List<String>(); }")
			writeFile(t, logPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			// Exercise metadata-backed settings and the fixture clone used by
			// BusinessHours, SchemaProfiles and ExecutionContext.
			for name, kind := range map[string]string{"RunnerListSetting__c": "List", "RunnerHierarchySetting__c": "Hierarchy"} {
				objectPath := filepath.Join(root, "force-app/main/default/objects", name)
				writeFile(t, filepath.Join(objectPath, name+".object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Runner Setting</label><customSettingsType>`+kind+`</customSettingsType><visibility>Public</visibility></CustomObject>`)
				writeFile(t, filepath.Join(objectPath, "fields/Text__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Text__c</fullName><label>Text</label><length>80</length><type>Text</type></CustomField>`)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/RunnerMetadata__mdt/RunnerMetadata__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Runner Metadata</label><pluralLabel>Runner Metadata</pluralLabel><visibility>Public</visibility></CustomObject>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/RunnerMetadata__mdt/fields/Text__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Text__c</fullName><label>Text</label><length>80</length><type>Text</type></CustomField>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/customMetadata/RunnerMetadata.Seed.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>Seed</label><protected>false</protected><values><field>Text__c</field><value xsi:type="xsd:string">metadata seed</value></values></CustomMetadata>`)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			fixture := storage.NewFixture()
			fixture.Objects = []storage.FixtureObject{
				{Name: "Account", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"Name": storage.StringValue("conformance-runner-fixture")}}}},
				{Name: "RunnerListSetting__c", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"Name": storage.StringValue("Seed"), "Text__c": storage.StringValue("list seed")}}}},
				{Name: "RunnerHierarchySetting__c", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"SetupOwnerId": storage.StringValue(org.OrgID), "Text__c": storage.StringValue("hierarchy seed")}}}},
			}
			if err := storage.ApplyFixture(&org, fixture); err != nil {
				t.Fatal(err)
			}
			users := org.Objects["User"]
			user := users.Records["005000000000001"]
			user.Fields["LocaleSidKey"] = storage.StringValue("en_US")
			user.Fields["LanguageLocaleKey"] = storage.StringValue("en_US")
			user.Fields["TimeZoneSidKey"] = storage.StringValue("America/Los_Angeles")
			user.Fields["UserRoleId"] = storage.NullValue()
			users.Records[user.ID] = user
			org.Objects["User"] = users
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			methods := compileProjectMethods(index)
			classes := compileProjectClasses(index, methods)
			fresh := func(program ir.Program) (vm.Result, error) {
				machine := vm.New(nil)
				for _, method := range methods {
					if err := machine.RegisterMethod(method); err != nil {
						t.Fatal(err)
					}
				}
				for _, class := range classes {
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				rowOrg := org.Clone()
				machine.SetOrg(&rowOrg)
				return machine.Execute(program)
			}
			initial := `System.assertEquals(0, Limits.getDmlStatements());
System.assertEquals(0, Limits.getQueries());
System.assertEquals('list seed', RunnerListSetting__c.getInstance('Seed').Text__c);
System.assertEquals(1, RunnerListSetting__c.getAll().size());
System.assertEquals('hierarchy seed', RunnerHierarchySetting__c.getOrgDefaults().Text__c);
System.assertEquals('metadata seed', RunnerMetadata__mdt.getInstance('Seed').Text__c);
System.assertEquals(0, Limits.getQueries());
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('America/Los_Angeles', UserInfo.getTimeZone().getID());
System.assertEquals(0, ConformanceRowState.calls);
System.assertEquals(1, ConformanceRowState.entries.size());
ConformanceRowState initialized = new ConformanceRowState();
System.assertEquals(1, initialized.initialized);
System.assert('static,instance'.equals(String.join(ConformanceInitLog.entries,',')), 'initializers must run exactly once');
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'conformance-runner-row']);
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'conformance-runner-fixture']);
Object rawNull = null;`
			dirty := initial + `ConformanceRowState.calls++;
ConformanceRowState.entries.add('dirty');
Account rowAccount = new Account(Name = 'conformance-runner-row'); insert rowAccount;
RunnerListSetting__c listSetting = RunnerListSetting__c.getInstance('Seed'); listSetting.Text__c='dirty'; update listSetting;
RunnerHierarchySetting__c hierarchySetting = RunnerHierarchySetting__c.getOrgDefaults(); hierarchySetting.Text__c='dirty'; update hierarchySetting;
Account fixtureAccount = [SELECT Id, Name FROM Account WHERE Name = 'conformance-runner-fixture']; fixtureAccount.Name='dirty'; update fixtureAccount;
User currentUser = [SELECT Id FROM User WHERE Id=:UserInfo.getUserId()]; currentUser.LocaleSidKey='fr_FR'; update currentUser;
System.debug('dirty|' + ConformanceRowState.calls + '|' + ConformanceRowState.entries.size() + '|' + rowAccount.Id);`
			rows := []struct{ name, source string }{
				{"dirty", dirty},
				{"clean", initial + `String observed;
try { throw new IllegalArgumentException('runner text'); }
catch(Exception e) { observed = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); }
System.debug(observed);`},
				{"uncaught", "throw new IllegalArgumentException('runner uncaught text');"},
				{"metadata-read-only", "RunnerMetadata__mdt record = RunnerMetadata__mdt.getInstance('Seed'); record.put('Text__c','dirty');"},
				{"dirty-again", dirty},
			}
			// The request compiler used by anonymous declaration rows must
			// produce the same execution and isolation as the test compiler.
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			requestRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: runner, RequestRuntime: &runtime, Org: &org})
			var retained vm.Result
			var retainedVars []byte
			var retainedDebug []string
			encodeVars := func(result vm.Result) []byte {
				raw, err := json.Marshal(result.Vars)
				if err != nil {
					t.Fatal(err)
				}
				return raw
			}
			errorText := func(err error) string {
				if err == nil {
					return ""
				}
				return fmt.Sprintf("%T|%s", err, err.Error())
			}
			for i, row := range rows {
				t.Run(row.name, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(row.source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					got, gotErr := runner.execute(program)
					want, wantErr := fresh(program)
					requestResult, requestErr := requestRunner.execute(program)
					if errorText(requestErr) != errorText(wantErr) || string(encodeVars(requestResult)) != string(encodeVars(want)) || !reflect.DeepEqual(requestResult.Debug, want.Debug) {
						t.Fatalf("request runtime differs: error=%s vars=%s debug=%q; fresh error=%s vars=%s debug=%q", errorText(requestErr), encodeVars(requestResult), requestResult.Debug, errorText(wantErr), encodeVars(want), want.Debug)
					}
					if errorText(gotErr) != errorText(wantErr) {
						t.Fatalf("error=%s, fresh=%s", errorText(gotErr), errorText(wantErr))
					}
					if row.name == "uncaught" || row.name == "metadata-read-only" {
						if gotErr == nil {
							t.Fatal("uncaught row did not throw")
						}
					} else {
						if gotErr != nil {
							t.Fatal(gotErr)
						}
						if got.Vars["rawNull"].Kind != vm.ValueNull {
							t.Fatal("raw null was converted")
						}
					}
					if string(encodeVars(got)) != string(encodeVars(want)) || !reflect.DeepEqual(got.Debug, want.Debug) {
						t.Fatalf("output differs: got vars=%s debug=%q; fresh vars=%s debug=%q", encodeVars(got), got.Debug, encodeVars(want), want.Debug)
					}
					// CPUTimeMS measures elapsed execution time, not a deterministic counter.
					gotLimits, wantLimits := got.Limits, want.Limits
					gotLimits.CPUTimeMS, wantLimits.CPUTimeMS = 0, 0
					if gotLimits != wantLimits || got.LimitMode != want.LimitMode || !reflect.DeepEqual(got.LimitViolations, want.LimitViolations) {
						t.Fatalf("limits=%#v/%s/%v, fresh=%#v/%s/%v", gotLimits, got.LimitMode, got.LimitViolations, wantLimits, want.LimitMode, want.LimitViolations)
					}
					if i == 0 {
						retained, retainedVars = got, encodeVars(got)
						retainedDebug = append([]string(nil), got.Debug...)
					}
				})
			}
			if string(encodeVars(retained)) != string(retainedVars) || !reflect.DeepEqual(retained.Debug, retainedDebug) {
				t.Fatal("later rows mutated an earlier result")
			}
			t.Run("named-fixture-rows", func(t *testing.T) {
				// Compression inspects direct @IsTest output; BusinessHours uses
				// runCase with SeeAllData and an explicitly seeded fixture org.
				namedPath := filepath.Join(root, "force-app/main/default/classes/ConformanceNamedRow.cls")
				writeFile(t, namedPath, "@IsTest(SeeAllData=true) private class ConformanceNamedRow { @IsTest static void dirty(){System.assert(Test.isRunningTest());"+dirty+"} @IsTest static void clean(){System.assert(Test.isRunningTest());"+initial+"System.debug('clean');} }")
				writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				namedIndex := loadTestIndex(t, root)
				namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
				namedRuntime, err := CompileProjectRuntimeForRequestWithSourceDigests(namedIndex, nil)
				if err != nil {
					t.Fatal(err)
				}
				requestNamedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: runner, RequestRuntime: &namedRuntime, Org: &org})
				namedMethods := compileProjectMethods(namedIndex)
				namedClasses := compileProjectClasses(namedIndex, namedMethods)
				freshNamed := func() *vm.VM {
					machine := vm.New(nil)
					if err := registerBaseRuntime(machine, namedMethods, namedClasses, nil); err != nil {
						t.Fatal(err)
					}
					rowOrg := org.Clone()
					machine.SetOrg(&rowOrg)
					return machine
				}
				cases := Discover(namedIndex, Options{SelectedClasses: []string{"ConformanceNamedRow"}})
				if len(cases) != 2 {
					t.Fatalf("named discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				runtimeMethods := indexTestRuntimeMethods(methods)
				for _, name := range []string{"dirty", "clean", "dirty", "clean"} {
					for _, tc := range cases {
						if tc.MethodName != name {
							continue
						}
						key := testCaseKey(tc)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("%s compile: %v %v", key, methodErrors[key], programErrors[key])
						}
						invoke := func(machine *vm.VM) (vm.Result, error) {
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							machine.SetTestSeeAllData(tc.SeeAllData)
							return machine.ExecuteInClass(programs[key], tc.ClassName)
						}
						got, gotErr := invoke(namedRunner.newMachine())
						want, wantErr := invoke(freshNamed())
						if gotErr != nil || wantErr != nil || string(encodeVars(got)) != string(encodeVars(want)) || !reflect.DeepEqual(got.Debug, want.Debug) {
							t.Fatalf("%s named differs: got error=%v vars=%s debug=%q; fresh error=%v vars=%s debug=%q", name, gotErr, encodeVars(got), got.Debug, wantErr, encodeVars(want), want.Debug)
						}
						requestGot, requestErr := invoke(requestNamedRunner.newMachine())
						if requestErr != nil || string(encodeVars(requestGot)) != string(encodeVars(want)) || !reflect.DeepEqual(requestGot.Debug, want.Debug) {
							t.Fatalf("%s named request differs: error=%v vars=%s debug=%q; fresh vars=%s debug=%q", name, requestErr, encodeVars(requestGot), requestGot.Debug, encodeVars(want), want.Debug)
						}
						ordinary := func(base *vm.VM, seed storage.OrgState) testreport.Case {
							initializeTestOrg(&seed)
							return runCase(context.Background(), tc, methods[key], runtimeMethods[testMethodSourceKey(tc.ClassName, tc.File)], methodErrors[key], programs[key], programErrors[key], base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, newRunPerfCounters(false))
						}
						gotCase := ordinary(namedRunner.base, namedRunner.newOrg())
						wantCase := ordinary(freshNamed(), org.Clone())
						gotCase.DurationMS, wantCase.DurationMS = 0, 0
						if gotCase.Status != testreport.StatusPass || !reflect.DeepEqual(gotCase, wantCase) {
							t.Fatalf("%s runner case=%#v fresh=%#v", name, gotCase, wantCase)
						}
					}
				}
			})
			t.Run("source-overlays", func(t *testing.T) {
				emptyRoot := t.TempDir()
				writeFile(t, filepath.Join(emptyRoot, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				overlayRunner := newConformanceRunner(t, loadTestIndex(t, emptyRoot), conformanceRunnerOptions{LinkProject: true})
				linkedRunner := overlayRunner
				linkedRequestRunner := overlayRunner
				for _, text := range []string{"first", "first", "second", "second", "first"} {
					rowRoot := t.TempDir()
					writeFile(t, filepath.Join(rowRoot, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
					rowPath := filepath.Join(rowRoot, "force-app/main/default/classes/ConformanceOverlay.cls")
					writeFile(t, rowPath, "public class ConformanceOverlay {public static Integer calls=0; public static List<String> entries=new List<String>(); static {entries.add('static');} public Integer initialized=markInstance(); private static Integer markInstance(){entries.add('instance'); return 1;} public static String text(){return '"+text+"';}}")
					writeFile(t, rowPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					rowIndex := loadTestIndex(t, rowRoot)
					program, err := vm.CompileAnonymousWithOptions("System.assertEquals(0,ConformanceOverlay.calls); ConformanceOverlay.calls++; ConformanceOverlay initialized=new ConformanceOverlay(); System.assert('static,instance'.equals(String.join(ConformanceOverlay.entries,','))); System.assert('"+text+"'.equals(ConformanceOverlay.text())); System.debug(ConformanceOverlay.text());", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					rowRunner := newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: linkedRunner, LinkProject: true})
					got, gotErr := rowRunner.execute(program)
					fresh := vm.New(nil)
					methods := compileProjectMethods(rowIndex)
					if err := registerBaseRuntime(fresh, methods, compileProjectClasses(rowIndex, methods), nil); err != nil {
						t.Fatal(err)
					}
					want, wantErr := fresh.Execute(program)
					if gotErr != nil || wantErr != nil || string(encodeVars(got)) != string(encodeVars(want)) || !reflect.DeepEqual(got.Debug, want.Debug) {
						t.Fatalf("overlay %q differs: error=%v vars=%s debug=%q; fresh error=%v vars=%s debug=%q", text, gotErr, encodeVars(got), got.Debug, wantErr, encodeVars(want), want.Debug)
					}
					rowRuntime, err := CompileProjectRuntimeForRequestWithSourceDigests(rowIndex, nil)
					if err != nil {
						t.Fatal(err)
					}
					requestRowRunner := newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: linkedRequestRunner, RequestRuntime: &rowRuntime})
					requestGot, requestErr := requestRowRunner.execute(program)
					if requestErr != nil || string(encodeVars(requestGot)) != string(encodeVars(want)) || !reflect.DeepEqual(requestGot.Debug, want.Debug) {
						t.Fatalf("request overlay %q differs: error=%v vars=%s debug=%q; fresh vars=%s debug=%q", text, requestErr, encodeVars(requestGot), requestGot.Debug, encodeVars(want), want.Debug)
					}
					// Exercise unchanged deduplication and changed-definition replacement
					// against a base that already contains this source class.
					linkedRunner = rowRunner
					linkedRequestRunner = requestRowRunner
					if _, leaked := overlayRunner.base.Classes["ConformanceOverlay"]; leaked {
						t.Fatal("row registration changed the linked base")
					}
				}
			})
		})
	}
}

// Rows of a primed runner share schema caches. DML on business, settings, setup
// and current-user records must still stay in its row, both with the runner's
// template clone and with OrgState.CloneRuntimeFrozenShared, which shares
// immutable setup records with the template until a row writes them.
func TestConformanceRunnerIsolatesRowDML(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	objectPath := filepath.Join(root, "force-app/main/default/objects/RunnerIsolationSetting__c")
	writeFile(t, filepath.Join(objectPath, "RunnerIsolationSetting__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Runner Isolation Setting</label><customSettingsType>List</customSettingsType><visibility>Public</visibility></CustomObject>`)
	writeFile(t, filepath.Join(objectPath, "fields/Text__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Text__c</fullName><label>Text</label><length>80</length><type>Text</type></CustomField>`)
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	fixture := storage.NewFixture()
	fixture.Objects = []storage.FixtureObject{
		{Name: "Account", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"Name": storage.StringValue("isolation-fixture")}}}},
		{Name: "RunnerIsolationSetting__c", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"Name": storage.StringValue("Seed"), "Text__c": storage.StringValue("seed")}}}},
	}
	if err := storage.ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	users := org.Objects["User"]
	user := users.Records["005000000000001"]
	user.Fields["LocaleSidKey"] = storage.StringValue("en_US")
	users.Records[user.ID] = user
	org.Objects["User"] = users
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})

	permissionSet, ok := storage.ResolveObjectName(runner.org.Org, "PermissionSet")
	if !ok || !storage.IsImmutableMetadataObject(permissionSet) {
		t.Fatalf("PermissionSet must be an immutable setup object: %q %t", permissionSet, ok)
	}
	if rowOrg := runner.org.Org.CloneRuntimeFrozenShared(); reflect.ValueOf(rowOrg.Objects[permissionSet].Records).Pointer() != reflect.ValueOf(runner.org.Org.Objects[permissionSet].Records).Pointer() {
		t.Fatal("frozen-shared org does not share immutable setup records with the template")
	}
	records := func() []byte {
		snapshot := make(map[string]map[storage.ID]storage.Record)
		for name, object := range runner.org.Org.Objects {
			if len(object.Records) > 0 {
				snapshot[name] = object.Records
			}
		}
		raw, err := json.Marshal(struct {
			Records     map[string]map[storage.ID]storage.Record
			IDSequences map[string]uint64
		}{snapshot, runner.org.Org.IDSequences})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	before := records()

	clean := `System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'isolation-row']);
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'isolation-fixture']);
System.assertEquals('seed', RunnerIsolationSetting__c.getInstance('Seed').Text__c);
System.assertEquals(1, RunnerIsolationSetting__c.getAll().size());
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('en_US', [SELECT LocaleSidKey FROM User WHERE Id = :UserInfo.getUserId()].LocaleSidKey);
System.assertEquals(0, [SELECT COUNT() FROM PermissionSet WHERE Name = 'RunnerIsolation']);
`
	rows := []struct{ name, source string }{
		{"clean", clean},
		{"business-dml", clean + `insert new Account(Name = 'isolation-row');
Account fixture = [SELECT Id FROM Account WHERE Name = 'isolation-fixture']; fixture.Name = 'dirty'; update fixture;
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'isolation-row']);
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'isolation-fixture']);`},
		{"after-business-dml", clean},
		{"settings", clean + `RunnerIsolationSetting__c seed = RunnerIsolationSetting__c.getInstance('Seed'); seed.Text__c = 'dirty'; update seed;
insert new RunnerIsolationSetting__c(Name = 'Extra', Text__c = 'extra');
System.assertEquals('dirty', RunnerIsolationSetting__c.getInstance('Seed').Text__c);
System.assertEquals(2, RunnerIsolationSetting__c.getAll().size());`},
		{"after-settings", clean},
		{"current-user", clean + `User current = [SELECT Id FROM User WHERE Id = :UserInfo.getUserId()]; current.LocaleSidKey = 'fr_FR'; update current;
System.assertEquals('fr_FR', [SELECT LocaleSidKey FROM User WHERE Id = :UserInfo.getUserId()].LocaleSidKey);`},
		{"after-current-user", clean},
		{"setup-dml", clean + `insert new PermissionSet(Name = 'RunnerIsolation', Label = 'Runner Isolation');
System.assertEquals(1, [SELECT COUNT() FROM PermissionSet WHERE Name = 'RunnerIsolation']);`},
		{"after-setup-dml", clean},
	}
	clones := []struct {
		name  string
		clone func() storage.OrgState
	}{
		{"template", runner.newOrg},
		{"frozen-shared", runner.org.Org.CloneRuntimeFrozenShared},
	}
	for _, clone := range clones {
		t.Run(clone.name, func(t *testing.T) {
			for _, row := range rows {
				t.Run(row.name, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(row.source, vm.CompileOptions{APIVersion: "67.0"})
					if err != nil {
						t.Fatal(err)
					}
					machine := runner.base.CloneRuntimeFrozenShared(nil)
					org := clone.clone()
					machine.SetRuntimeTemplateOrg(&org)
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					if after := records(); string(after) != string(before) {
						t.Fatal("row DML changed the runner's template org")
					}
				})
			}
		})
	}
}

// A derived runner must detach caches warmed by rows of its parent before it
// primes a different schema. Both schemas have the same object/field counts.
func TestConformanceRunnerDerivedSchemaDoesNotReuseWarmParentCaches(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			makeOrg := func(relationship string) storage.OrgState {
				org := storage.NewOrgState()
				org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
					APIName: "Parent__c",
					Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
				}}
				org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
					APIName: "Child__c",
					Fields: map[string]storage.Field{
						"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r", ChildRelationshipName: relationship},
					},
				}}
				return org
			}
			program, err := vm.CompileAnonymousWithOptions(`List<String> relationships = new List<String>(); Object described = Schema.getGlobalDescribe().get('Parent__c').getDescribe(); for (Object relationship : described.getChildRelationships()) { relationships.add(relationship.getRelationshipName()); } relationships.sort(); String observed = String.join(relationships, ','); System.debug(observed);`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			parentOrg := makeOrg("ParentChildren__r")
			parent := newConformanceRunner(t, typesys.Index{}, conformanceRunnerOptions{Org: &parentOrg})
			checkRelationship := func(runner *conformanceRunner, relationship string) vm.Result {
				t.Helper()
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if got := result.Vars["observed"].Text; got != relationship {
					t.Fatalf("derived-schema relationship = %q, want %q", got, relationship)
				}
				return result
			}
			// Populate the parent base's shared child-relationship caches through a row
			// clone, keeping the base itself unexecuted.
			checkRelationship(parent, "ParentChildren__r")
			childOrg := makeOrg("ChildChildren__r")
			child := newConformanceRunner(t, typesys.Index{}, conformanceRunnerOptions{Base: parent, Org: &childOrg})
			if parent.org.RuntimeSchemaStamp == child.org.RuntimeSchemaStamp {
				t.Fatal("derived-schema control did not change the schema stamp")
			}
			got := checkRelationship(child, "ChildChildren__r")
			fresh := vm.New(nil)
			freshOrg := childOrg.Clone()
			fresh.SetOrg(&freshOrg)
			want, err := fresh.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if got.Vars["observed"].Text != want.Vars["observed"].Text || !reflect.DeepEqual(got.Debug, want.Debug) {
				t.Fatalf("derived-schema row differs from fresh execution: got relationship=%q debug=%q, want relationship=%q debug=%q", got.Vars["observed"].Text, got.Debug, want.Vars["observed"].Text, want.Debug)
			}
			checkRelationship(child, "ChildChildren__r")
			checkRelationship(parent, "ParentChildren__r")
		})
	}
}
