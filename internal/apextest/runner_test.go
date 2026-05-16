package apextest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	oaerschema "github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
)

func TestRunExecutesAnonymousSubsetTestMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathTest.cls"), `
@isTest
private class MathTest {
  @isTest static void adds() {
    Integer x = 1 + 1;
    System.assertEquals(2, x);
  }
  @TestSetup static void setup() {
    System.debug('setup');
  }
}

`)

	index := loadTestIndex(t, root)
	run := Run(index, Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestRunAllowsDeterministicHttpSendWithoutMockInTestContext(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/HttpHarnessTest.cls"), `
@isTest
private class HttpHarnessTest {
  @isTest static void sendsWithoutExternalNetwork() {
    HttpRequest req = new HttpRequest();
    req.setEndpoint('https://example.invalid/probe');
    req.setMethod('GET');
    HttpResponse res = new Http().send(req);
    System.assertEquals(200, res.getStatusCode());
    System.assertEquals('{}', res.getBody());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestDiscoverCapturesSeeAllDataAnnotation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SeeAllDataTest.cls"), `
@isTest
private class SeeAllDataTest {
  @isTest(SeeAllData=true) static void seesData() {}
  @isTest static void siloed() {}
}
`)

	cases := Discover(loadTestIndex(t, root), Options{})
	seen := map[string]bool{}
	for _, testCase := range cases {
		seen[testCase.MethodName] = testCase.SeeAllData
	}
	if !seen["seesData"] {
		t.Fatalf("SeeAllData annotation was not captured: %#v", cases)
	}
	if seen["siloed"] {
		t.Fatalf("plain @isTest method marked SeeAllData: %#v", cases)
	}
}

func TestCloneRuntimeOrgIsolatesRecordsAndDefinitions(t *testing.T) {
	org := storage.NewOrgState()
	org.OrgID = "00D000000000001"
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: "Text"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
			},
		},
	}

	cloned := cloneRuntimeOrg(org)
	account := cloned.Objects["Account"]
	account.Records["001000000000001"].Fields["Name"] = storage.StringValue("Changed")
	account.Definition.Fields["RuntimeOnly__c"] = storage.Field{APIName: "RuntimeOnly__c", Type: storage.FieldString}
	cloned.Objects["Account"] = account

	if got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String; got != "Acme" {
		t.Fatalf("clone shared records with base org: %q", got)
	}
	if _, ok := org.Objects["Account"].Definition.Fields["RuntimeOnly__c"]; ok {
		t.Fatalf("runtime clone shared definition fields with base org")
	}
}

func TestRunKeepsPageParametersAndDynamicSelectorBinds(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Template__c/Template__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Template</label><pluralLabel>Templates</pluralLabel><nameField><type>Text</type><label>Name</label></nameField></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Template__c/fields/SOQLQuery__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>SOQLQuery__c</fullName><label>SOQL Query</label><type>LongTextArea</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Template__c/fields/TemplateSource__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>TemplateSource__c</fullName><label>Template Source</label><type>LongTextArea</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateSelector.cls"), `
public class TemplateSelector {
  public static Template__c selectById(Id recordId) {
    Set<Id> escapedIdSet = new Set<Id>{ recordId };
    List<Template__c> rows = Database.query('SELECT Id, Name, SOQLQuery__c, TemplateSource__c FROM Template__c WHERE Id IN :escapedIdSet');
    if (rows.isEmpty()) {
      return null;
    }
    return rows[0];
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateController.cls"), `
public class TemplateController {
  public String Source { get; private set; }
  public String Query { get; private set; }
  public TemplateController() {
    Id templateId = ApexPages.currentPage().getParameters().get('templateId');
    Template__c row = TemplateSelector.selectById(templateId);
    if (row != null) {
      Source = row.TemplateSource__c;
      Query = row.SOQLQuery__c;
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateControllerTest.cls"), `
@isTest
private class TemplateControllerTest {
  @isTest static void loadsFromCurrentPageParameter() {
    Template__c tpl = new Template__c(Name = 'T', SOQLQuery__c = 'SELECT Id FROM Account', TemplateSource__c = 'Hello');
    insert tpl;
    PageReference pageRef = new PageReference('/apex/Template');
    Test.setCurrentPage(pageRef);
    ApexPages.currentPage().getParameters().put('templateId', tpl.Id);
    TemplateController controller = new TemplateController();
    System.assertEquals('Hello', controller.Source);
    System.assertEquals('SELECT Id FROM Account', controller.Query);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		var problem *testreport.Problem
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			problem = run.Suites[0].Cases[0].Problem
		}
		t.Fatalf("summary = %#v problem = %#v cases = %#v", summary, problem, run.Suites[0].Cases)
	}
}

func TestRunAllowsListReturnForIterableObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatch.cls"), `
public class IterableBatch {
  private List<Account> records;
  public IterableBatch(List<Account> records) {
    this.records = records;
  }
  public Iterable<Object> start() {
    return this.records;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatchTest.cls"), `
@isTest
private class IterableBatchTest {
  @isTest static void listSatisfiesIterableObject() {
    List<Account> records = new List<Account>{ new Account(Name = 'Acme') };
    Iterable<Object> items = new IterableBatch(records).start();
    Integer count = 0;
    for (Object item : items) {
      count++;
    }
    System.assertEquals(1, count);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v cases = %#v", summary, run.Suites[0].Cases)
	}
}

func TestRunDispatchesGeneratedProductCallbackImplementations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CommerceResolver.cls"), `
public class CommerceResolver implements CommerceExtension.ResolutionStrategy {
  public CommerceExtension.Resolution resolve() {
    return new CommerceExtension.Resolution(CommerceExtension.ResolutionStates.OFF);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ReadinessEvaluator.cls"), `
public class ReadinessEvaluator implements Readiness.ProductEvaluator {
  public Boolean isActive() {
    return true;
  }
  public List<Readiness.ProductScoreDetail> evaluateReadiness(Readiness.ProductEvaluationContext ctx) {
    return new List<Readiness.ProductScoreDetail>{
      new Readiness.ProductScoreDetail('01t000000000001AAA', 'local-rule', 100, 'ready')
    };
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ProductCallbackTest.cls"), `
@isTest
private class ProductCallbackTest {
  @isTest static void userCallbacksDispatchLocally() {
    CommerceExtension.ResolutionStrategy strategy = new CommerceResolver();
    System.assertEquals(CommerceExtension.ResolutionStates.OFF, strategy.resolve().getResolutionState());

    Readiness.ProductEvaluator evaluator = new ReadinessEvaluator();
    System.assertEquals(true, evaluator.isActive());
    Readiness.ProductEvaluationContext context =
      new Readiness.ProductEvaluationContext(new Set<Id>{ '01t000000000001AAA' });
    List<Readiness.ProductScoreDetail> scores = evaluator.evaluateReadiness(context);
    System.assertEquals(1, scores.size());
    System.assertEquals('local-rule', scores[0].getRuleName());
    System.assertEquals(100, scores[0].getRuleScore());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v cases = %#v", summary, run.Suites[0].Cases)
	}
}

func TestRuntimeEvaluatesTemplateLexemsWithInnerClassGaps(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MergeValues.cls"), `
public class MergeValues {
  private Map<String, Object> values = new Map<String, Object>();
  public MergeValues(Map<String, Object> values) {
    this.values.putAll(values);
  }
  public Object get(String path) {
    return values.get(path);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateEvaluator.cls"), `
public class TemplateEvaluator {
  private static final Pattern MERGE_FIELD_PATTERN = Pattern.compile('\\{!([\\w\\.]+)\\}');
  public final String content;
  private Object[] lexems;
  private TemplateEvaluator(String content) {
    this.content = content;
  }
  public static TemplateEvaluator newInstance(String content) {
    return new TemplateEvaluator(content);
  }
  public String evaluate(Map<String, Object> values) {
    compile();
    String buffer = '';
    MergeValues bag = new MergeValues(values);
    for (Object lexem : lexems) {
      Object value = evaluate(lexem, bag);
      buffer += value == null ? '' : String.valueOf(value);
    }
    return buffer;
  }
  private void compile() {
    lexems = new List<Object>();
    Matcher contentMatcher = MERGE_FIELD_PATTERN.matcher(content);
    Integer processedEnd = 0;
    while (contentMatcher.find()) {
      if (processedEnd < contentMatcher.start()) {
        lexems.add(content.substring(processedEnd, contentMatcher.start()));
      }
      lexems.add(new Gap(contentMatcher.group(1)));
      processedEnd = contentMatcher.end();
    }
    if (processedEnd < content.length()) {
      lexems.add(content.substring(processedEnd));
    }
  }
  private static Object evaluate(Object lexem, MergeValues values) {
    if (lexem instanceof String) {
      return lexem;
    }
    if (lexem instanceof Gap) {
      return values.get(((Gap)lexem).key);
    }
    return null;
  }
  private class Gap {
    public final String key;
    Gap(String key) {
      this.key = key;
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateEvaluatorTest.cls"), `
@isTest
private class TemplateEvaluatorTest {
  @isTest static void evaluatesGaps() {
    String result = TemplateEvaluator.newInstance('-start-{!valueA}-inner-{!valueB}-end-')
      .evaluate(new Map<String, Object>{ 'valueA' => 'A', 'valueB' => 'B' });
    System.assertEquals('-start-A-inner-B-end-', result);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		var problem *testreport.Problem
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			problem = run.Suites[0].Cases[0].Problem
		}
		t.Fatalf("summary = %#v problem = %#v cases = %#v", summary, problem, run.Suites[0].Cases)
	}
}

func TestRuntimeInstanceOfUsesConcreteRuntimeTypeForInterfaceValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MatcherProbe.cls"), `
public class MatcherProbe {
  public interface IMatcher {
    Boolean matches(Object value);
  }
  public class Captor implements IMatcher {
    public Object value;
    public Boolean matches(Object value) {
      this.value = value;
      return true;
    }
    public void store(List<Object> values) {
      values.add(value);
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MatcherProbeTest.cls"), `
@isTest
private class MatcherProbeTest {
  @isTest static void interfaceValueKeepsRuntimeType() {
    MatcherProbe.IMatcher original = new MatcherProbe.Captor();
    List<MatcherProbe.IMatcher> matchers = new List<MatcherProbe.IMatcher>{ original };
    System.assert(original === matchers[0], 'list literal should keep object identity');
    List<MatcherProbe.IMatcher> cloned = matchers.clone();
    System.assert(original === cloned[0], 'List.clone should be shallow');
    List<Object> values = new List<Object>();
    for (MatcherProbe.IMatcher matcher : cloned) {
      System.assert(matcher instanceof MatcherProbe.Captor);
      matcher.matches('Fred');
      ((MatcherProbe.Captor)matcher).store(values);
    }
    System.assertEquals('Fred', (String)values[0]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		var problem *testreport.Problem
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			problem = run.Suites[0].Cases[0].Problem
		}
		t.Fatalf("summary = %#v problem = %#v cases = %#v", summary, problem, run.Suites[0].Cases)
	}
}

func TestExtractMethodBodyHandlesBackslashEscapedApexStrings(t *testing.T) {
	source := `@IsTest
private class DataRequestTest {
    @IsTest
    private static void setParam1_validParams_expectSet() {
        List<String> testParams = new List<String> { 'it\'s', 'wednesday' };
        System.assertEquals('it\'s', testParams[0]);
    }

    @IsTest
    private static void nextTest() {
        System.assert(true);
    }
}`
	start := strings.Index(source, "private static void setParam1")
	end := strings.Index(source, "    @IsTest\n    private static void nextTest")
	if start < 0 || end < 0 {
		t.Fatal("test source markers not found")
	}
	body, err := extractMethodBody(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: end},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "nextTest") || !strings.Contains(body, `'it\'s'`) {
		t.Fatalf("body = %q", body)
	}
}

func TestRunCoversProtectedOverrideAndHandlerDispatchPatterns(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SelectorBase.cls"), `
public abstract class SelectorBase {
  public String run() {
    return getName();
  }
  protected abstract String getName();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteSelector.cls"), `
public class ConcreteSelector extends SelectorBase {
  protected override String getName() {
    return 'selector';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PrivateSelectorBase.cls"), `
public abstract class PrivateSelectorBase {
  public String run() {
    return fieldListString();
  }
  String fieldListString() {
    return getSObjectFieldList();
  }
  abstract String getSObjectFieldList();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PrivateSelectorChild.cls"), `
public class PrivateSelectorChild extends PrivateSelectorBase {
  private override String getSObjectFieldList() {
    return 'private-fields';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DMLHelper.cls"), `
public virtual class DMLHelper {
  public static DMLHelper Instance {
    get {
      if (Instance == null) {
        Instance = new WithoutSharing();
      }
      return Instance;
    }
  }
  public virtual String updateRecords(List<SObject> records) {
    return 'base';
  }
  private without sharing class WithoutSharing extends DMLHelper {
    public override String updateRecords(List<SObject> records) {
      return super.updateRecords(records) + '-without';
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TriggerHandlersBase.cls"), `
global virtual class TriggerHandlersBase {
  global virtual void onBeforeUpdate(Map<Id, SObject> newRecordMap, Map<Id, SObject> oldRecordMap) {
    DispatchState.Value = 'base';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteTriggerHandlers.cls"), `
public class ConcreteTriggerHandlers extends TriggerHandlersBase {
  public override void onBeforeUpdate(Map<Id, SObject> newRecordMap, Map<Id, SObject> oldRecordMap) {
    DispatchState.Value = 'child';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TriggerHandlerManager.cls"), `
public class TriggerHandlerManager {
  public static void executeHandlers(TriggerHandlersBase triggerHandler) {
    triggerHandler.onBeforeUpdate(new Map<Id, SObject>(), new Map<Id, SObject>());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchState.cls"), `
public class DispatchState {
  public static String Value;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchPatternsTest.cls"), `
@isTest
private class DispatchPatternsTest {
  @isTest static void dispatches() {
    System.assertEquals('selector', new ConcreteSelector().run());
    System.assertEquals('private-fields', new PrivateSelectorChild().run());
    System.assertEquals('base-without', DMLHelper.Instance.updateRecords(new List<Widget__c>()));
    TriggerHandlerManager.executeHandlers(new ConcreteTriggerHandlers());
    System.assertEquals('child', DispatchState.Value);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v case = %#v problem = %#v", summary, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunCoversNestedServiceFactoryWithTypeMap(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label><pluralLabel>Things</pluralLabel><nameField><type>Text</type><label>Name</label></nameField></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FactoryBase.cls"), `
public virtual class FactoryBase {
  public virtual class ServiceFactory {
    private Map<Type, Type> implByInterface;
    public ServiceFactory(Map<Type, Type> registrations) {
      implByInterface = registrations;
    }
    public Object newInstance(Type serviceInterfaceType) {
      Type impl = implByInterface.get(serviceInterfaceType);
      return impl.newInstance();
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Application.cls"), `
public class Application {
  private static final List<SObjectType> OBJECTS = new List<SObjectType>{ Thing__c.SObjectType };
  public static final FactoryBase.ServiceFactory Service = new FactoryBase.ServiceFactory(
    new Map<Type, Type>{ ILocatorService.class => LocatorServiceImpl.class, IOtherLocatorService.class => OtherLocatorServiceImpl.class }
  );
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ILocatorService.cls"), `
public interface ILocatorService {
  String name();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IOtherLocatorService.cls"), `
public interface IOtherLocatorService {
  String other();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorServiceImpl.cls"), `
public class LocatorServiceImpl implements ILocatorService {
  public String name() {
    return 'located';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OtherLocatorServiceImpl.cls"), `
public class OtherLocatorServiceImpl implements IOtherLocatorService {
  public String other() {
    return 'other';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorFacade.cls"), `
public class LocatorFacade {
  public static String name() {
    return ((ILocatorService) Application.Service.newInstance(ILocatorService.class)).name();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MapOverloadProbe.cls"), `
public class MapOverloadProbe {
  public static String choose(Map<Object, Type> values) {
    return 'object';
  }
  public static String choose(Map<SObjectType, Type> values) {
    return 'sobject';
  }
  public static Integer keyCount(Map<SObjectType, Type> values) {
    Integer count = 0;
    for (SObjectType key : values.keySet()) {
      count++;
    }
    return count;
  }
  public static String typeKeyRoundTrip(Map<Type, String> values) {
    for (Type key : values.keySet()) {
      return values.get(key);
    }
    return null;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorFactoryTest.cls"), `
@isTest
private class LocatorFactoryTest {
  @isTest static void locatesService() {
    System.assertEquals('located', LocatorFacade.name());
    System.assertEquals('sobject', MapOverloadProbe.choose(new Map<SObjectType, Type>{ Thing__c.SObjectType => LocatorServiceImpl.class }));
    System.assertEquals(1, MapOverloadProbe.keyCount(new Map<SObjectType, Type>{ Thing__c.SObjectType => LocatorServiceImpl.class }));
    System.assertEquals('locator', MapOverloadProbe.typeKeyRoundTrip(new Map<Type, String>{ LocatorServiceImpl.class => 'locator' }));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		problem := run.Suites[0].Cases[0].Problem
		if problem == nil {
			t.Fatalf("summary = %#v case = %#v problem = nil", summary, run.Suites[0].Cases[0])
		}
		t.Fatalf("summary = %#v case = %#v problem = %#v", summary, run.Suites[0].Cases[0], *problem)
	}
}

func TestRunExecutesJSONParserFieldOnInnerHandler(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JSONParserHolder.cls"), `
public class JSONParserHolder {
  private interface ParserEvents {
    String nextToken();
  }
  private class InjectChildrenEventHandler implements ParserEvents {
    private JSONParser childrenParser;
    public InjectChildrenEventHandler(JSONParser childrenParser) {
      this.childrenParser = childrenParser;
      this.childrenParser.nextToken();
    }
    public String nextToken() {
      JSONToken token = childrenParser.nextToken();
      return token == null ? null : token.name();
    }
  }
  public static String firstChildToken(String payload) {
    ParserEvents handler = new InjectChildrenEventHandler(JSON.createParser(payload));
    return handler.nextToken();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JSONParserHolderTest.cls"), `
@isTest
private class JSONParserHolderTest {
  @isTest static void storesParserOnInnerHandlerField() {
    System.assertEquals('VALUE_STRING', JSONParserHolder.firstChildToken('["child"]'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		problem := run.Suites[0].Cases[0].Problem
		if problem == nil {
			t.Fatalf("summary = %#v case = %#v problem = nil", summary, run.Suites[0].Cases[0])
		}
		t.Fatalf("summary = %#v case = %#v problem = %#v", summary, run.Suites[0].Cases[0], *problem)
	}
}

func TestExtractMethodBodyFallsBackPastShortRange(t *testing.T) {
	source := `public class BigClass {
  public static void run() {
    // a comment with { that should not count
    if (true) {
      System.debug('}');
    }
  }
}`
	start := strings.Index(source, "public static void run")
	shortEnd := strings.Index(source, "if (true)")
	body, err := extractMethodBody(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: shortEnd},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "System.debug('}')") {
		t.Fatalf("body = %q", body)
	}
}

func TestExtractMethodSourceRecoversOneLineSignature(t *testing.T) {
	source := `public class Hooks {
  public virtual void onApplyDefaults() { }
}`
	start := strings.Index(source, "{ }")
	text, err := extractMethodSource(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: start + len("{ }")},
	})
	if err != nil {
		t.Fatal(err)
	}
	params, err := parseParams(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 0 {
		t.Fatalf("params = %#v", params)
	}
	body, err := extractMethodBody(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start + 1},
		End:   diagnostic.Position{Offset: start + 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != "" {
		t.Fatalf("body = %q", body)
	}
}

func TestRunContextReportsCanceledCases(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CanceledTest.cls"), `
@isTest
private class CanceledTest {
  @isTest static void stops() {
    System.assert(true);
  }
}
`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	run := RunContext(ctx, loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Unsupported != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if run.Suites[0].Cases[0].Problem.Type != "Canceled" {
		t.Fatalf("case = %#v", run.Suites[0].Cases[0])
	}
}

func TestRunContextPerTestDeadlineDoesNotCancelFollowingTests(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PerTestTimeoutTest.cls"), `
@isTest
private class PerTestTimeoutTest {
  @isTest static void a_hangs() {
    while (true) {}
  }
  @isTest static void z_passes() {
    System.assert(true);
  }
}
`)

	run := RunContext(context.Background(), loadTestIndex(t, root), Options{TimeoutMS: 500})
	if got := run.Summary(); got.Total != 2 || got.Passed != 1 || got.Failed+got.Unsupported != 1 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
	cases := run.Suites[0].Cases
	if cases[0].Problem == nil {
		t.Fatalf("first case = %#v", cases[0])
	}
	if cases[1].Status != testreport.StatusPass {
		t.Fatalf("second case = %#v", cases[1])
	}
}

func TestRunReportsAssertionFailures(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FailingTest.cls"), `
@isTest
private class FailingTest {
  @isTest static void fails() {
    System.assertEquals(3, 1 + 1);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Failed != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if run.Suites[0].Cases[0].Status != testreport.StatusFail {
		t.Fatalf("case = %#v", run.Suites[0].Cases[0])
	}
}

func TestRunExecutesStaticHelperMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void adds() {
    System.assertEquals(3, MathUtil.add(1, 2));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v run=%#v", got, run.Suites[0].Cases[0].Problem, run)
	}
}

func TestRunExecutesStaticHelperMethodWithBranching(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer max(Integer a, Integer b) {
    if (a > b) {
      return a;
    } else {
      return b;
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void maxChoosesLargerValue() {
    System.assertEquals(5, MathUtil.max(5, 2));
    System.assertEquals(7, MathUtil.max(3, 7));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		index := loadTestIndex(t, root)
		methods := compileProjectMethods(index)
		for _, class := range compileProjectClasses(index, methods) {
			if class.Name == "ListDowncastDomain" {
				t.Logf("constructors=%#v fields=%#v", class.Constructors, class.Fields)
			}
		}
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesCaseFoldedOverloadWithNestedGenericParams(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CaseFoldHelper.cls"), `
public class CaseFoldHelper {
  public static Integer reapplyCartCoupons(Set<Id> ids, Boolean preventIfExpired) {
    Map<Id, Integer> results = new Map<Id, Integer>();
    Map<Id, List<Account>> records = new Map<Id, List<Account>>();
    return reapplyCartCoupons(results, ids, records, new Set<Id>(), preventIfExpired);
  }

  private static Integer reApplyCartCoupons(
      Map<Id, Integer> results,
      Set<Id> ids,
      Map<Id, List<Account>> records,
      Set<Id> seen,
      Boolean preventIfExpired) {
    return 7;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CaseFoldHelperTest.cls"), `
@isTest
private class CaseFoldHelperTest {
  @isTest static void helperOverloadRuns() {
    System.assertEquals(7, CaseFoldHelper.reapplyCartCoupons(new Set<Id>(), true));
  }
}
`)

	index := loadTestIndex(t, root)
	run := Run(index, Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		methods := compileProjectMethods(index)
		keys := make([]string, 0, len(methods))
		for key := range methods {
			keys = append(keys, key)
		}
		t.Logf("methods=%v", keys)
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestCompileProjectMethodsIncludesDependencyTestHelpers(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/SharedTestHelper.cls"), `
@isTest
public class SharedTestHelper {
  public static String value() {
    return 'dep';
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "oaer.yml"), `project:
  managedPackageDependencies: ["znu:../dep:1.0"]
`)
	index := loadTestIndex(t, consumerRoot)

	if cases := Discover(index, Options{}); len(cases) != 0 {
		t.Fatalf("discovered dependency test helpers as runnable cases: %#v", cases)
	}
	methods := compileProjectMethods(index)
	if _, ok := methods["SharedTestHelper.value#"]; !ok {
		t.Fatalf("dependency @isTest helper method was not compiled; methods=%#v", methods)
	}
}

func TestRunCallsInstanceMethodThroughStaticProperty(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Context.cls"), `
public class Context {
  public static Context Instance {
    get {
      if (Instance == null) {
        Instance = new Context();
      }
      return Instance;
    }
  }
  public Context() {
    Object duringConstruction = Context.Instance;
  }
  public String value(Schema.SObjectType typ) {
    return typ.getDescribe().getName();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextTest.cls"), `
@isTest
private class ContextTest {
  @isTest static void callsThroughProperty() {
    System.assertEquals('Account', Context.Instance.value(Account.SObjectType));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunCallsStaticPropertyReceiverInsideMapLiteral(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Context.cls"), `
public class Context {
  public static Context Instance {
    get {
      if (Instance == null) {
        Instance = new Context();
      }
      return Instance;
    }
  }
  public Id getId(Schema.SObjectType typ) {
    return '001000000000001AAA';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextTest.cls"), `
@isTest
private class ContextTest {
  @isTest static void callsThroughPropertyInMapLiteral() {
    Map<Schema.SObjectField, Object> values = new Map<Schema.SObjectField, Object>{
      Account.Name => Context.Instance.getId(Account.SObjectType)
    };
    System.assertEquals('001000000000001AAA', values.get(Account.Name));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunAllowsNestedInheritedPropertyGetterOnDifferentInstances(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseBuilder.cls"), `
public abstract class BaseBuilder {
  private Map<String, Object> defaultsPriv;
  private Map<String, Object> defaults {
    get {
      if (defaultsPriv == null) {
        defaultsPriv = getDefaults();
      }
      return defaultsPriv;
    }
  }
  protected abstract Map<String, Object> getDefaults();
  public Integer countDefaults() {
    return defaults.keySet().size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChildBuilder.cls"), `
public class ChildBuilder extends BaseBuilder {
  public static ChildBuilder Instance {
    get {
      if (Instance == null) {
        Instance = new ChildBuilder();
      }
      return Instance;
    }
  }
  protected override Map<String, Object> getDefaults() {
    Map<String, Object> values = new Map<String, Object>{'self' => 'ok'};
    values.put('other', OtherBuilder.Instance.countDefaults());
    return values;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OtherBuilder.cls"), `
public class OtherBuilder extends BaseBuilder {
  public static OtherBuilder Instance {
    get {
      if (Instance == null) {
        Instance = new OtherBuilder();
      }
      return Instance;
    }
  }
  protected override Map<String, Object> getDefaults() {
    return new Map<String, Object>{'other' => 'ok'};
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BuilderTest.cls"), `
@isTest
private class BuilderTest {
  @isTest static void nestedInheritedGetterUsesOwnReceiver() {
    System.assertEquals(2, ChildBuilder.Instance.countDefaults());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesStaticHelperMethodWithWhileLoop(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer sumTo(Integer n) {
    Integer total = 0;
    Integer i = 1;
    while (i <= n) {
      total = total + i;
      i = i + 1;
    }
    return total;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void sumsRange() {
    System.assertEquals(15, MathUtil.sumTo(5));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v", got, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesInstanceHelperMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Calculator.cls"), `
public class Calculator {
  public Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CalculatorTest.cls"), `
@isTest
private class CalculatorTest {
  @isTest static void instanceMethodAdds() {
    Calculator calc = new Calculator();
    System.assertEquals(7, calc.add(3, 4));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDispatchesCreateStubToStubProvider(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Greeter.cls"), `
public interface Greeter {
  String greet(String name);
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/GreeterProvider.cls"), `
private class GreeterProvider implements System.StubProvider {
  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    System.assertEquals('greet', stubbedMethodName);
    return 'stubbed';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/GreeterTest.cls"), `
@isTest
private class GreeterTest {
  @isTest static void routesThroughProvider() {
    Greeter greeter = Test.createStub(Greeter.class, new GreeterProvider());
    System.assertEquals('stubbed', greeter.greet('Ada'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunDispatchesCreateStubToStubProviderWithSystemTypeList(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Greeter.cls"), `
public interface Greeter {
  String greet(String name);
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/GreeterProvider.cls"), `
private class GreeterProvider implements System.StubProvider {
  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<System.Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    System.assertEquals('greet', stubbedMethodName);
    System.assertEquals(String.class, listOfParamTypes.get(0));
    return 'stubbed';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/GreeterTest.cls"), `
@isTest
private class GreeterTest {
  @isTest static void routesThroughProvider() {
    Greeter greeter = Test.createStub(Greeter.class, new GreeterProvider());
    System.assertEquals('stubbed', greeter.greet('Ada'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunInvokesCallableImplementation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocalCallable.cls"), `
public class LocalCallable implements System.Callable {
  public Object call(String action, Map<String, Object> args) {
    return action;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocalCallableTest.cls"), `
@isTest
private class LocalCallableTest {
  @isTest static void invokesCallable() {
    System.Callable callable = new LocalCallable();
    System.assert(callable instanceof System.Callable);
    System.assertEquals('go', callable.call('go', new Map<String, Object>()));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesClassStateConstructorLoopsAndExceptions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Counter.cls"), `
public class Counter {
  public Integer value { get; set; }
  public static Integer created = 0;

  public Counter(Integer seed) {
    value = seed;
    created++;
  }

  public Integer addAll(List<Integer> values) {
    for (Integer value : values) {
      if (value == 2) {
        continue;
      }
      this.value = this.value + value;
    }
    return this.value;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CounterTest.cls"), `
@isTest
private class CounterTest {
  @isTest static void statefulRuntimeFeaturesWork() {
    Counter c = new Counter(1);
    List<Integer> values = new List<Integer>{1, 2, 3};
    System.assertEquals(5, c.addAll(values));
    System.assertEquals(1, Counter.created);
    Integer cleanup = 0;
    try {
      throw new MyException();
    } catch (Exception e) {
      cleanup = cleanup + 1;
    } finally {
      cleanup = cleanup + 2;
    }
    System.assertEquals(3, cleanup);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesInitializerBlocks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InitCounter.cls"), `
public class InitCounter {
  public Integer value { get; set; }
  public static Integer seed = 0;

  static {
    seed = 4;
  }

  {
    value = seed + 1;
  }

  public Integer score() {
    return value;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InitCounterTest.cls"), `
@isTest
private class InitCounterTest {
  @isTest static void initializersRun() {
    System.assertEquals(4, InitCounter.seed);
    InitCounter counter = new InitCounter();
    System.assertEquals(5, counter.score());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesFieldInitializerExpressionsInSourceOrder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InitOrder.cls"), `
public class InitOrder {
  public static Integer seed = 2;
  public static Integer doubled = seed * 2;
  static {
    seed = doubled + 1;
  }

  public Integer first = seed + 1;
  public Integer second = first + 1;
  {
    second = second + 1;
  }

  public Integer score() {
    return second;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InitOrderTest.cls"), `
@isTest
private class InitOrderTest {
  @isTest static void fieldInitializersRunInOrder() {
    System.assertEquals(5, InitOrder.seed);
    System.assertEquals(4, InitOrder.doubled);
    InitOrder ordered = new InitOrder();
    System.assertEquals(8, ordered.score());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestFieldInitializerExprFallsBackToFullDeclarationStatement(t *testing.T) {
	source := "\t\tprivate List<Error> errorList = new List<Error>(); \n\t\tprivate Boolean enabled = false;\n"
	start := strings.Index(source, "new List<Error>()")
	end := strings.Index(source, "private Boolean")
	expr, ok := fieldInitializerExpr("errorList", diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: end},
	}, source)
	if !ok || expr != "new List<Error>()" {
		t.Fatalf("expr = %q ok=%v, want new List<Error>()", expr, ok)
	}
}

func TestTypeDeclarationSourceFallsBackToFullDeclarationLine(t *testing.T) {
	source := "\t\tpublic class InterfaceBackedFactory implements DomainFactory.IConstructable\n\t\t{\n\t\t\tpublic Object construct() { return null; }\n\t\t}\n"
	start := strings.Index(source, "InterfaceBackedFactory")
	end := strings.LastIndex(source, "}") + 1
	typeSource, err := typeDeclarationSource(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: end},
	})
	if err != nil {
		t.Fatal(err)
	}
	if interfaces := parseImplements(typeSource); len(interfaces) != 1 || interfaces[0] != "DomainFactory.IConstructable" {
		t.Fatalf("interfaces = %#v", interfaces)
	}
}

func TestCompileProjectClassesPrefersIndexedInterfaces(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Outer.cls")
	writeFile(t, file, "public class Outer {}\n")
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind:  apexast.DeclarationInterface,
			Name:  "Outer.Marker",
			File:  file,
			Range: diagnostic.Range{Start: diagnostic.Position{Offset: 0}, End: diagnostic.Position{Offset: 1}},
		},
		{
			Kind:       apexast.DeclarationClass,
			Name:       "Outer.Impl",
			File:       file,
			Interfaces: []string{"Outer.Marker"},
			Range:      diagnostic.Range{Start: diagnostic.Position{Offset: 0}, End: diagnostic.Position{Offset: 1}},
		},
	}}
	classes := compileProjectClasses(index, nil)
	for _, class := range classes {
		if class.Name == "Outer.Impl" {
			if len(class.Interfaces) != 1 || class.Interfaces[0] != "Outer.Marker" {
				t.Fatalf("interfaces = %#v", class.Interfaces)
			}
			return
		}
	}
	t.Fatal("Outer.Impl class not compiled")
}

func TestRunRegistersPassiveGeneratedSystemStubClasses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PassiveGeneratedStubTest.cls"), `
@IsTest
private class PassiveGeneratedStubTest {
  @IsTest static void generatedDtoAccessorsWork() {
    List<String> coupons = new List<String>{'WELCOME'};
    commercepromotions.PromotionRequest request = new commercepromotions.PromotionRequest(new Account(Name = 'Acme'), 'buyer-one', 'store-one', coupons);
    System.assertEquals('buyer-one', request.getBuyerAccountId());
    System.assertEquals('store-one', request.getWebStoreId());
    System.assertEquals('WELCOME', request.getCouponCodes().get(0));
    System.assertEquals('Acme', ((Account)request.getSalesTransaction()).Name);
    Map<String,Object> values = request.getAsMap();
    System.assertEquals('buyer-one', (String)values.get('buyerAccountId'));
    commercepromotions.PromotionRequest cloned = (commercepromotions.PromotionRequest)request.clone();
    System.assertEquals('buyer-one', cloned.getBuyerAccountId());
    System.assertEquals('INVALIDCOUPON', commercepromotions.ErrorCode.INVALIDCOUPON.name());
    commercepromotions.PromotionRequest namedRequest = new commercepromotions.PromotionRequest(salesTransaction = new Account(Name = 'Named'), buyerAccountId = 'named-buyer', webStoreId = 'named-store', couponCodes = coupons);
    System.assertEquals('named-buyer', namedRequest.getBuyerAccountId());
    System.assertEquals('Named', ((Account)namedRequest.getSalesTransaction()).Name);
    Invocable.Action action = Invocable.Action.createCustomAction('apex', 'pkg', 'DoIt');
    System.assertEquals('apex', action.getType());
    System.assertEquals('pkg', action.getNamespace());
    System.assertEquals('DoIt', action.getName());
    System.assertEquals('Audience', ConnectApi.AudienceCriteriaType.Audience.name());
    System.assertEquals('INVALIDCOUPON', commercepromotions.CouponInfo.ErrorCode.INVALIDCOUPON.name());
    System.assertEquals('NO_FILTER', Database.PaginationCursor.DeleteFilter.NO_FILTER.name());
    System.assertEquals(4, Database.PaginationCursor.DeleteFilter.values().size());
    System.assertEquals('EmailActivity', sfdatakit.DeployComponentBundleAccountEngagementConfig.AccountEngagmentDataStreamTypeEnum.EmailActivity.name());
    Slack.ApiTestRequest slackRequest = Slack.ApiTestRequest.builder().foo('bar').build();
    System.assert(slackRequest != null);
    Slack.ApiTestRequest.Builder slackBuilder = Slack.ApiTestRequest.builder();
    slackBuilder.foo('stored');
    slackBuilder.error('none');
    Slack.ApiTestRequest storedSlackRequest = slackBuilder.build();
    System.assert(storedSlackRequest != null);
    LoyaltyManagement.ChangeTierInputBuilder tierBuilder = new LoyaltyManagement.ChangeTierInputBuilder();
    tierBuilder.setProgramName('Rewards');
    tierBuilder.setTargetTierName('Gold');
    LoyaltyManagement.ChangeTierInput tierInput = tierBuilder.build();
    Map<String,Object> tierValues = tierInput.getAsMap();
    System.assertEquals('Rewards', (String)tierValues.get('programName'));
    System.assertEquals('Gold', (String)tierValues.get('targetTierName'));
    inventorypricing.GetInventoryPricing inventoryService = new inventorypricing.GetInventoryPricing();
    Object response = inventoryService.createResponse(new inventorypricing.InventoryPricingData());
    System.assertNotEquals(null, response);
    Map<String,Object> flowInputs = new Map<String,Object>{'recordId' => '001000000000001'};
    Flow.Interview interview = Flow.Interview.createInterview('Demo_Flow', flowInputs);
    interview.start();
    Map<String,Object> interviewValues = interview.getAsMap();
    System.assertEquals('Demo_Flow', (String)interviewValues.get('flowName'));
    System.assertEquals(true, (Boolean)interviewValues.get('started'));
    Flow.Interview namespacedInterview = Flow.Interview.createInterview('pkg', 'Demo_Flow', flowInputs);
    Map<String,Object> namespacedValues = namespacedInterview.getAsMap();
    System.assertEquals('pkg', (String)namespacedValues.get('namespace'));
    CartExtension.BuyerActionDetails.Builder actionBuilder = new CartExtension.BuyerActionDetails.Builder();
    actionBuilder.withCheckoutStarted(true);
    CartExtension.BuyerActionDetails details = actionBuilder.build();
    System.assertEquals(true, (Boolean)details.getAsMap().get('isCheckoutStarted'));
    CartExtension.BuyerActionDetails chainedDetails = new CartExtension.BuyerActionDetails.Builder()
      .withCouponChanges(new List<CartExtension.CouponChange>())
      .build();
    System.assertEquals(0, chainedDetails.getCouponChanges().size());
    CartExtension.Cart cart = CartExtension.CartTestUtil.createCart();
    System.assert(cart != null);
  }
}
`)
	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			t.Fatalf("summary = %#v problem=%#v", summary, run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v suites=%#v", summary, run.Suites)
	}
}

func TestExtractMethodSourceUsesByteOffsets(t *testing.T) {
	source := "// café comment before the method\npublic Integer runIt() {\n  return 7;\n}\n"
	start := strings.Index(source, "public Integer")
	end := len(source)
	methodSource, err := extractMethodSource(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: end},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(methodSource, "public Integer runIt()") {
		t.Fatalf("methodSource = %q", methodSource)
	}
}

func TestRunExecutesConstructorChaining(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseCounter.cls"), `
public class BaseCounter {
  public Integer base { get; set; }

  public BaseCounter(Integer seed) {
    base = seed;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChainedCounter.cls"), `
public class ChainedCounter extends BaseCounter {
  public Integer bonus { get; set; }

  public ChainedCounter() {
    this(4);
  }

  public ChainedCounter(Integer bonusSeed) {
    super(3);
    bonus = bonusSeed;
  }

  public Integer score() {
    return base + bonus;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChainedCounterTest.cls"), `
@isTest
private class ChainedCounterTest {
  @isTest static void constructorsChain() {
    ChainedCounter counter = new ChainedCounter();
    System.assertEquals(7, counter.score());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesPropertyAccessorBodies(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PropertyBox.cls"), `
public class PropertyBox {
  private String backing;

  public String Name {
    get {
      return backing + '!';
    }
    set {
      backing = value.toUpperCase();
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PropertyBoxTest.cls"), `
@isTest
private class PropertyBoxTest {
  @isTest static void accessorsRun() {
    PropertyBox box = new PropertyBox();
    box.Name = 'acme';
    System.assertEquals('ACME!', box.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesImplicitSuperBeforeSourceConstructorPropertySetters(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseWrapper.cls"), `
public abstract class BaseWrapper {
  private Map<String, String> values;

  protected BaseWrapper() {
    values = new Map<String, String>();
  }

  protected void setValue(String name, String value) {
    values.put(name, value);
  }

  public String getValue(String name) {
    return values.get(name);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteWrapper.cls"), `
public class ConcreteWrapper extends BaseWrapper {
  public ConcreteWrapper() {
    this.Name = 'Ada';
  }

  public String Name {
    set { setValue('name', value); }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteWrapperTest.cls"), `
@isTest
private class ConcreteWrapperTest {
  @isTest static void constructorRunsSuperBeforeSetter() {
    ConcreteWrapper wrapper = new ConcreteWrapper();
    System.assertEquals('Ada', wrapper.getValue('name'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunJSONDeserializeRunsZeroArgConstructorBeforePropertySetters(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseWrapper.cls"), `
public abstract class BaseWrapper {
  private Map<String, String> values;

  protected BaseWrapper() {
    values = new Map<String, String>();
  }

  protected void setValue(String name, String value) {
    values.put(name, value);
  }

  protected String getValueInternal(String name) {
    return values.get(name);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteWrapper.cls"), `
public class ConcreteWrapper extends BaseWrapper {
  public ConcreteWrapper() {}

  public String Name {
    get { return getValueInternal('name'); }
    set { setValue('name', value); }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteWrapperTest.cls"), `
@isTest
private class ConcreteWrapperTest {
  @isTest static void deserializeInitializesSetterState() {
    ConcreteWrapper wrapper = new ConcreteWrapper();
    wrapper.Name = 'Ada';
    String payload = '{"Name":"Ada"}';
    ConcreteWrapper decoded = (ConcreteWrapper)JSON.deserialize(payload, ConcreteWrapper.class);
    System.assertEquals('Ada', decoded.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesStaticPropertyNestedSubclassManagerDispatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ManagerBase.cls"), `
public abstract class ManagerBase {
  public abstract String required();
  public String callRequired() {
    return required();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BatchManager.cls"), `
public abstract class BatchManager extends ManagerBase {
  public static BatchManager Instance {
    get {
      if (Instance == null) {
        Instance = (BatchManager)new WithSharing();
      }
      return Instance;
    }
  }

  public virtual override String required() {
    return 'base';
  }

  public virtual String FindBatch(String source) {
    return callRequired() + ':' + source;
  }

  private class WithSharing extends BatchManager {
    public override String required() {
      return super.required();
    }
    public override String FindBatch(String source) {
      return super.FindBatch(source);
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BatchManagerTest.cls"), `
@isTest
private class BatchManagerTest {
  @isTest static void staticPropertyDispatches() {
    System.assertEquals('base:SS', BatchManager.Instance.FindBatch('SS'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesOverloadedMethodsByArgumentTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OverloadUtil.cls"), `
public class OverloadUtil {
  public static String pick(Integer value) {
    return 'int';
  }
  public static String pick(String value) {
    return 'string';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OverloadUtilTest.cls"), `
@isTest
private class OverloadUtilTest {
  @isTest static void choosesSpecificMethod() {
    System.assertEquals('int', OverloadUtil.pick(1));
    System.assertEquals('string', OverloadUtil.pick('one'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesInheritanceAndSuperDispatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseScore.cls"), `
public virtual class BaseScore {
  public virtual Integer score() {
    return 2;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BonusScore.cls"), `
public class BonusScore extends BaseScore {
  public override Integer score() {
    return super.score() + 3;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BonusScoreTest.cls"), `
@isTest
private class BonusScoreTest {
  @isTest static void dispatchesOverrideAndSuper() {
    BonusScore score = new BonusScore();
    System.assertEquals(5, score.score());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesEnumValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Mood.cls"), `
public enum Mood {
  Happy,
  Sad
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MoodTest.cls"), `
@isTest
private class MoodTest {
  @isTest static void enumValuesCompareByName() {
    System.assertEquals(Mood.Happy, Mood.Happy);
    System.assertNotEquals(Mood.Happy, Mood.Sad);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesNestedClassMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Outer.cls"), `
public class Outer {
  public class Inner {
    public static Integer count = 1;
    public static String staticLabel() {
      return 'static-inner';
    }
    public String label() {
      return 'inner';
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OuterTest.cls"), `
@isTest
private class OuterTest {
  @isTest static void nestedClassRuns() {
    Outer.Inner inner = new Outer.Inner();
    System.assertEquals('inner', inner.label());
    System.assertEquals('static-inner', Outer.Inner.staticLabel());
    Outer.Inner.count = 3;
    System.assertEquals(3, Outer.Inner.count);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunPrefersNestedInstanceFieldOverCaseFoldedInnerType(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Outer.cls"), `
public class Outer {
  public interface Filter {
    Boolean hasValue();
  }
  public class Impl implements Filter {
    public Boolean hasValue() {
      return true;
    }
  }
  public class Adapter {
    private Filter filter;
    public Adapter(Filter filter) {
      this.filter = filter;
    }
    public Boolean run() {
      return filter.hasValue();
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OuterTest.cls"), `
@isTest
private class OuterTest {
  @isTest static void nestedFieldWins() {
    System.assertEquals(true, new Outer.Adapter(new Outer.Impl()).run());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesLowercaseNestedClassStaticMethodFromInitializer(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchOuter.cls"), `
public class DispatchOuter {
  public static String initialized = v1.label('init');
  public class v1 {
    public static String label(String input) {
      return input + '-nested';
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchOuterTest.cls"), `
@isTest
private class DispatchOuterTest {
  @isTest static void lowercaseNestedStaticDispatches() {
    System.assertEquals('direct-nested', DispatchOuter.v1.label('direct'));
    System.assertEquals('init-nested', DispatchOuter.initialized);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesInstanceMethodOnStaticPropertyInitializedByStaticBlock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><label>Account</label><pluralLabel>Accounts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField><fullName>Name</fullName><label>Name</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchFacade.cls"), `
public class DispatchFacade {
  public static DispatchService v1 { get; private set; }
  static {
    v1 = new DispatchService();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchService.cls"), `
public class DispatchService {
  public String label(String input) {
    return input + '-instance';
  }
  public String describeRecords(List<SObject> records, SObjectField fieldToken) {
    return String.valueOf(records.size()) + ':' + fieldToken.getDescribe().getName();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchFacadeTest.cls"), `
@isTest
private class DispatchFacadeTest {
  @isTest static void staticPropertyReceiverDispatches() {
    System.assertEquals('direct-instance', DispatchFacade.v1.label('direct'));
    List<Account> records = new List<Account>{new Account(Name = 'Acme')};
    System.assertEquals('1:Name', DispatchFacade.v1.describeRecords(records, Account.Name));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunPersistsUnqualifiedStaticListMutation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticListRegistry.cls"), `
public class StaticListRegistry {
  private static List<String> values = new List<String>();

  public static void addOne(String value) {
    values.add(value);
  }

  public static Integer countValues() {
    return values.size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticListRegistryTest.cls"), `
@IsTest
private class StaticListRegistryTest {
  @IsTest static void staticListMutationPersistsAcrossStaticMethods() {
    StaticListRegistry.addOne('x');
    System.assertEquals(1, StaticListRegistry.countValues());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesStaticMapInitializerWithInnerClassTypeValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BindingShape.cls"), `
public abstract class BindingShape {
  public enum BindingType { Apex, Module }
  private static final Map<BindingType, Type> bindingImplsByType =
    new Map<BindingType, Type> {
      BindingType.Apex => ApexBinding.class,
      BindingType.Module => ApexBinding.class
    };

  public static String lookup() {
    return bindingImplsByType.get(BindingType.Apex).getName();
  }

  private class ApexBinding extends BindingShape {
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BindingShapeTest.cls"), `
@isTest
private class BindingShapeTest {
  @isTest static void staticMapTypeInitializerDispatches() {
    System.assertEquals('BindingShape.ApexBinding', BindingShape.lookup());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesLowercaseGenericListIsEmptyFromMethodReturn(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BindingCaseProbe.cls"), `
public class BindingCaseProbe {
  public class Binding {
    public String name;
  }

  public static list<Binding> retrieveBindings() {
    list<Binding> bindings = new list<Binding>();
    return bindings;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BindingCaseProbeTest.cls"), `
@isTest
private class BindingCaseProbeTest {
  @isTest static void lowercaseListReturnSupportsIsEmpty() {
    list<BindingCaseProbe.Binding> matchedBindings = BindingCaseProbe.retrieveBindings();
    System.assert(matchedBindings.isEmpty());
    matchedBindings.add(new BindingCaseProbe.Binding());
    System.assert(!matchedBindings.isEmpty());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesNestedTypesWithConstructorsInterfacesEnumsAndIdentity(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Outer.cls"), `
public class Outer {
  public static Integer seed = 2;
  public interface Named {
    String name();
  }
  public class Inner {
    public Integer value;
    public Inner(Integer input) {
      value = input + Outer.seed;
    }
    public String label() {
      return 'inner-' + value;
    }
  }
  public class NamedImpl implements Named {
    public String name() {
      return 'nested-iface';
    }
  }
  public static Inner makeInner(Integer input) {
    Inner made = new Inner(input);
    return made;
  }
  public enum Choice {
    One,
    Two
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OuterTypesTest.cls"), `
@isTest
private class OuterTypesTest {
  @isTest static void nestedTypesRun() {
    Outer.Inner first = new Outer.Inner(3);
    Outer.Inner alias = first;
    Outer.Inner second = new Outer.Inner(3);
    System.assertEquals(5, first.value);
    System.assertEquals('inner-5', first.label());
    System.assert(first == alias);
    System.assert(first != second);
    Outer.Named named = new Outer.NamedImpl();
    System.assertEquals('nested-iface', named.name());
    Outer.Inner made = Outer.makeInner(4);
    System.assertEquals(6, made.value);
    System.assertEquals('Two', Outer.Choice.Two.name());
    System.assertEquals(1, Outer.Choice.Two.ordinal());
    List<Outer.Choice> choices = Outer.Choice.values();
    System.assertEquals(2, choices.size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesTestSetupAndResetsStatics(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupState.cls"), `
public class SetupState {
  public static Integer value = 1;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupStateTest.cls"), `
@isTest
private class SetupStateTest {
  @TestSetup static void setup() {
    SetupState.value = 99;
  }

  @isTest static void first() {
    System.assertEquals(1, SetupState.value);
    SetupState.value = 2;
  }

  @isTest static void second() {
    System.assertEquals(1, SetupState.value);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunIsolatesDMLBetweenTestMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IsolationTest.cls"), `
@isTest
private class IsolationTest {
  @isTest static void insertsData() {
    insert new Account(Name = 'Acme');
    Integer rows = [SELECT COUNT() FROM Account];
    System.assertEquals(1, rows);
  }

  @isTest static void doesNotSeeOtherTestData() {
    Integer rows = [SELECT COUNT() FROM Account];
    System.assertEquals(0, rows);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunClonesTestSetupDataBetweenMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupDataTest.cls"), `
@isTest
private class SetupDataTest {
  @TestSetup static void seed() {
    insert new Account(Name = 'Seed');
  }

  @isTest static void canMutateSetupDataInOwnTransaction() {
    List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Seed'];
    System.assertEquals(1, rows.size());
    Account row = rows.get(0);
    row.Name = 'Changed';
    update row;
    insert new Account(Name = 'Extra');
    Integer total = [SELECT COUNT() FROM Account];
    System.assertEquals(2, total);
  }

  @isTest static void seesFreshSetupSnapshot() {
    Integer seedRows = [SELECT COUNT() FROM Account WHERE Name = 'Seed'];
    Integer changedRows = [SELECT COUNT() FROM Account WHERE Name = 'Changed'];
    Integer extraRows = [SELECT COUNT() FROM Account WHERE Name = 'Extra'];
    System.assertEquals(1, seedRows);
    System.assertEquals(0, changedRows);
    System.assertEquals(0, extraRows);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunContinuesDeterministicRandomStateAfterTestSetup(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Vuid__c.field-meta.xml"), `
<CustomField>
  <fullName>Vuid__c</fullName>
  <label>Vuid</label>
  <type>Text</type>
  <unique>true</unique>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupRandomTest.cls"), `
@isTest
private class SetupRandomTest {
  @TestSetup static void seed() {
    insert new Account(Name = 'Seed', Vuid__c = UUID.randomUUID().toString());
  }

  @isTest static void methodRandomDoesNotCollideWithSetupData() {
    insert new Account(Name = 'Method', Vuid__c = UUID.randomUUID().toString());
    System.assertEquals(2, [SELECT COUNT() FROM Account]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestProjectRuntimeResolvesCustomObjectFieldTokensFromMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"verifiable"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/VfiHospitalAffiliation__c/VfiHospitalAffiliation__c.object-meta.xml"), `
<CustomObject>
  <label>Hospital Affiliation</label>
  <pluralLabel>Hospital Affiliations</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/VfiHospitalAffiliation__c/fields/Type__c.field-meta.xml"), `
<CustomField>
  <fullName>Type__c</fullName>
  <label>Type</label>
  <type>Picklist</type>
  <valueSet>
    <valueSetDefinition>
      <value>
        <fullName>AdmittingPrivileges</fullName>
        <label>Admitting Privileges</label>
      </value>
    </valueSetDefinition>
  </valueSet>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SchemaTokenTest.cls"), `
@isTest
private class SchemaTokenTest {
  @isTest static void resolvesFieldToken() {
    System.assertNotEquals(null, VfiHospitalAffiliation__c.Type__c);
    System.assertEquals('verifiable__VfiHospitalAffiliation__c', VfiHospitalAffiliation__c.SObjectType.getDescribe().getName());
    System.assertEquals('VfiHospitalAffiliation__c', VfiHospitalAffiliation__c.SObjectType.getDescribe().getLocalName());
    System.assertEquals('verifiable__Type__c', VfiHospitalAffiliation__c.Type__c.getDescribe().getName());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunTestSetupRecordWinsOverSyntheticSetupDataDefault(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Setup_Data__c/Setup_Data__c.object-meta.xml"), `
<CustomObject>
  <label>Setup Data</label>
  <pluralLabel>Setup Data</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Setup_Data__c/fields/Data_Mappings__c.field-meta.xml"), `
<CustomField>
  <fullName>Data_Mappings__c</fullName>
  <label>Data Mappings</label>
  <type>LongTextArea</type>
  <length>32768</length>
  <visibleLines>3</visibleLines>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupDataDefaultTest.cls"), `
@isTest
private class SetupDataDefaultTest {
  @TestSetup static void seed() {
    insert new Setup_Data__c(Name = 'Test Setup', Data_Mappings__c = 'method');
  }

  @isTest static void unorderedLimitPrefersTestSetupRecord() {
    Setup_Data__c setup = [SELECT Data_Mappings__c FROM Setup_Data__c LIMIT 1];
    System.assertEquals('method', setup.Data_Mappings__c);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDrainsQueueableAtStopTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncMarker.cls"), `
public class AsyncMarker {
  public static Integer ran = 0;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MarkJob.cls"), `
public class MarkJob implements Queueable {
  public void execute(QueueableContext qc) {
    AsyncMarker.ran = AsyncMarker.ran + 1;
    insert new Account(Name = 'async ran');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MarkJobTest.cls"), `
@isTest
private class MarkJobTest {
  @isTest static void stopTestDrainsQueue() {
    Test.startTest();
    System.enqueueJob(new MarkJob());
    AsyncMarker.ran = 41;
    System.assertEquals(41, AsyncMarker.ran);
    Test.stopTest();
    System.assertEquals(42, AsyncMarker.ran);
    Integer asyncRows = [SELECT COUNT() FROM Account WHERE Name = 'async ran'];
    System.assertEquals(1, asyncRows);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunDoesNotDrainQueueableEnqueuedBeforeStartTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PreStartJob.cls"), `
public class PreStartJob implements Queueable {
  public void execute(QueueableContext qc) {
    insert new Account(Name = 'pre-start async ran');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PreStartJobTest.cls"), `
@isTest
private class PreStartJobTest {
  @isTest static void stopTestSkipsPreStartQueue() {
    System.enqueueJob(new PreStartJob());
    Test.startTest();
    Test.stopTest();
    System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'pre-start async ran']);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v", got, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesQueueableFinalizerAtStopTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FinalizerJob.cls"), `
public class FinalizerJob implements Queueable, Finalizer {
  public void execute(QueueableContext qc) {
    System.attachFinalizer(this);
    insert new Account(Name = 'queueable ran');
  }
  public void execute(FinalizerContext fc) {
    System.assertEquals(ParentJobResult.SUCCESS, fc.getResult());
    System.assertNotEquals('', fc.getAsyncApexJobId());
    System.assertEquals(null, fc.getException());
    insert new Account(Name = 'finalizer ran');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FinalizerJobTest.cls"), `
@isTest
private class FinalizerJobTest {
  @isTest static void stopTestRunsFinalizer() {
    Test.startTest();
    System.enqueueJob(new FinalizerJob());
    Test.stopTest();
    System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'queueable ran']);
	System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'finalizer ran']);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v", got, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDrainsFutureBatchScheduleAndChainedQueueables(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncState.cls"), `
public class AsyncState {
  public static Integer futureRan = 0;
  public static Integer batchSum = 0;
  public static Integer batchFinish = 0;
  public static Integer scheduledRan = 0;
  public static Integer queueRan = 0;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FutureWorker.cls"), `
public class FutureWorker {
  @future public static void mark(Integer amount) {
    AsyncState.futureRan = amount;
    insert new Account(Name = 'future');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CountingBatch.cls"), `
public class CountingBatch {
  public List<Integer> start(Object bc) {
    return new List<Integer>{1, 2, 3};
  }
  public void execute(Object bc, List<Integer> scope) {
    for (Integer value : scope) {
      AsyncState.batchSum = AsyncState.batchSum + value;
      insert new Account(Name = 'batch-' + value);
    }
  }
  public void finish(Object bc) {
    AsyncState.batchFinish = 1;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ScheduledWorker.cls"), `
public class ScheduledWorker {
  public void execute(Object sc) {
    AsyncState.scheduledRan = 1;
    insert new Account(Name = 'scheduled');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FirstQueue.cls"), `
public class FirstQueue {
  public void execute(Object qc) {
    AsyncState.queueRan = AsyncState.queueRan + 1;
    insert new Account(Name = 'queue-1');
    System.enqueueJob(new SecondQueue());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SecondQueue.cls"), `
public class SecondQueue {
  public void execute(Object qc) {
    AsyncState.queueRan = AsyncState.queueRan + 1;
    insert new Account(Name = 'queue-2');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncSemanticsTest.cls"), `
@isTest
private class AsyncSemanticsTest {
  @isTest static void drainsSupportedAsyncWork() {
    Test.startTest();
    FutureWorker.mark(7);
    String batchId = Database.executeBatch(new CountingBatch(), 2);
    String scheduleId = System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
    String scheduledBatchId = System.scheduleBatch(new CountingBatch(), 'batch later', 1, 2);
    String queueId = System.enqueueJob(new FirstQueue());
    System.assertNotEquals('', batchId);
    System.assertNotEquals('', scheduleId);
    System.assertNotEquals('', scheduledBatchId);
    System.assertNotEquals('', queueId);
    System.assertEquals(0, AsyncState.futureRan);
    System.assertEquals(0, AsyncState.batchSum);
    System.assertEquals(0, AsyncState.scheduledRan);
    System.assertEquals(0, AsyncState.queueRan);
    Integer beforeRows = [SELECT COUNT() FROM Account];
    System.assertEquals(0, beforeRows);
    Test.stopTest();
    Integer afterRows = [SELECT COUNT() FROM Account];
    System.assertEquals(9, afterRows);
    List<AsyncApexJob> jobs = [SELECT Id, Status, JobType FROM AsyncApexJob];
    System.assertEquals(6, jobs.size());
    List<CronTrigger> crons = [SELECT Id, State FROM CronTrigger];
    System.assertEquals(2, crons.size());
    CronTrigger cron = crons.get(0);
    System.assertEquals('Complete', cron.State);
    System.assertEquals(7, AsyncState.futureRan);
    System.assertEquals(12, AsyncState.batchSum);
    System.assertEquals(1, AsyncState.batchFinish);
    System.assertEquals(1, AsyncState.scheduledRan);
    System.assertEquals(1, AsyncState.queueRan);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunBatchStartCanReturnQueryLocator(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <fullName>Account</fullName>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <nameField><type>Text</type><label>Name</label></nameField>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorBatch.cls"), `
public class LocatorBatch {
  public Database.QueryLocator start(Object bc) {
    return Database.getQueryLocator('SELECT Id, Name FROM Account');
  }
  public void execute(Object bc, List<Account> scope) {
    for (Account row : scope) {
      insert new Account(Name = 'processed-' + row.Name);
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorBatchTest.cls"), `
@isTest
private class LocatorBatchTest {
  @isTest static void drainsQueryLocatorScope() {
    insert new Account(Name = 'seed');
    Test.startTest();
    Database.executeBatch(new LocatorBatch(), 200);
    Test.stopTest();
    Integer processed = [SELECT COUNT() FROM Account WHERE Name LIKE 'processed%'];
    System.assertEquals(1, processed);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunBatchStartCanReturnCustomIterable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatchState.cls"), `
public class IterableBatchState {
  public static Integer sum = 0;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatch.cls"), `
public class IterableBatch {
  public CounterIterable start(Object bc) {
    return new CounterIterable(3);
  }
  public void execute(Object bc, List<Integer> scope) {
    for (Integer value : scope) {
      IterableBatchState.sum = IterableBatchState.sum + value;
    }
  }
  public class CounterIterable implements Iterable<Integer>, Iterator<Integer> {
    private List<Integer> values;
    private Integer index = 0;
    public CounterIterable(Integer total) {
      values = new List<Integer>();
      for (Integer i = 0; i < total; i++) {
        values.add(i + 1);
      }
    }
    public Iterator<Integer> iterator() {
      return this;
    }
    public Boolean hasNext() {
      return index < values.size();
    }
    public Integer next() {
      return values[index++];
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatchTest.cls"), `
@isTest
private class IterableBatchTest {
  @isTest static void drainsCustomIterableScope() {
    Test.startTest();
    Database.executeBatch(new IterableBatch(), 2);
    Test.stopTest();
    System.assertEquals(6, IterableBatchState.sum);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunAppliesCustomObjectNameDefaultWhenTestSetsNull(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Widget__c/Widget__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Widget</label>
  <pluralLabel>Widgets</pluralLabel>
  <nameField><type>Text</type><label>Widget Name</label></nameField>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/WidgetNameDefaultTest.cls"), `
@isTest
private class WidgetNameDefaultTest {
  @isTest static void insertsWithNullNameInLocalTestContext() {
    Widget__c widget = new Widget__c(Name = null);
    insert widget;
    Widget__c loaded = [SELECT Name FROM Widget__c WHERE Id = :widget.Id];
    System.assertEquals('Widget', loaded.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunAsyncContextIdsAndJobFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><label>Account</label><pluralLabel>Accounts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField><fullName>Name</fullName><label>Name</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextQueue.cls"), `
public class ContextQueue {
  public void execute(QueueableContext qc) {
    insert new Account(Name = qc.getJobId());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextBatch.cls"), `
public class ContextBatch {
  public List<Integer> start(Database.BatchableContext bc) {
    return new List<Integer>{1, 2, 3};
  }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {
    insert new Account(Name = bc.getJobId());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextSchedule.cls"), `
public class ContextSchedule {
  public void execute(SchedulableContext sc) {
    insert new Account(Name = sc.getTriggerId());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncContextIdsTest.cls"), `
@isTest
private class AsyncContextIdsTest {
  @isTest static void contextsExposeDeterministicIds() {
    Test.startTest();
    String queueId = System.enqueueJob(new ContextQueue());
    String batchId = Database.executeBatch(new ContextBatch(), 2);
    String schedId = System.schedule('nightly', '0 0 0 * * ?', new ContextSchedule());
    System.assertEquals('707000000000001', queueId);
    System.assertEquals('707000000000002', batchId);
    System.assertEquals('08e000000000003', schedId);
    Test.stopTest();
    Integer batchRows = [SELECT COUNT() FROM Account WHERE Name = '707000000000002'];
    Integer queueRows = [SELECT COUNT() FROM Account WHERE Name = '707000000000001'];
    Integer triggerRows = [SELECT COUNT() FROM Account WHERE Name = '08e000000000003'];
    System.assertEquals(2, batchRows);
    System.assertEquals(1, queueRows);
    System.assertEquals(1, triggerRows);
    List<AsyncApexJob> batches = [SELECT Id, Status, JobType, TotalJobItems, JobItemsProcessed, NumberOfErrors FROM AsyncApexJob WHERE Id = '707000000000002'];
    System.assertEquals(1, batches.size());
    AsyncApexJob batch = batches.get(0);
    System.assertEquals('Completed', batch.Status);
    System.assertEquals('BatchApex', batch.JobType);
    System.assertEquals(2, batch.TotalJobItems);
    System.assertEquals(2, batch.JobItemsProcessed);
    System.assertEquals(0, batch.NumberOfErrors);
    List<CronTrigger> crons = [SELECT Id, State, CronExpression, CronJobDetail FROM CronTrigger];
    System.assertEquals(1, crons.size());
    CronTrigger cron = crons.get(0);
    System.assertEquals('Complete', cron.State);
    System.assertEquals('0 0 0 * * ?', cron.CronExpression);
    System.assertEquals('nightly', cron.CronJobDetail);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunScheduledApexExposesPendingJobWithCronTriggerRelationship(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/QueuedWorker.cls"), `
public class QueuedWorker {
  public void execute(QueueableContext qc) {
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ScheduledWorker.cls"), `
public class ScheduledWorker {
  public static Integer Ran = 0;
  public void execute(SchedulableContext sc) {
    Ran = Ran + 1;
    System.enqueueJob(new QueuedWorker());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ScheduledWorkerTest.cls"), `
@isTest
private class ScheduledWorkerTest {
  @isTest static void exposesScheduledJobRows() {
    Test.startTest();
    String scheduleId = System.schedule('nightly', '0 0 12 * * ?', new ScheduledWorker());
    Test.stopTest();
    System.assertEquals(1, ScheduledWorker.Ran);
    List<AsyncApexJob> jobs = [
      SELECT Id, Status, JobType, ApexClass.Name, CronTriggerId, CronTrigger.Id
      FROM AsyncApexJob
      WHERE ApexClass.Name = 'ScheduledWorker'
      AND Status IN ('Preparing', 'Processing', 'Queued', 'Holding')
      AND JobType = 'ScheduledApex'
    ];
    System.assertEquals(1, jobs.size());
    System.assertEquals(scheduleId, jobs.get(0).CronTriggerId);
    System.assertEquals(scheduleId, jobs.get(0).CronTrigger.Id);
    ApexClass queuedClass = [SELECT Id, Name, NamespacePrefix FROM ApexClass WHERE Name = 'QueuedWorker' AND NamespacePrefix = null LIMIT 1];
    List<AsyncApexJob> queuedJobs = [
      SELECT Id, Status, JobType, ApexClassId
      FROM AsyncApexJob
      WHERE ApexClassId = :queuedClass.Id
      AND Status IN ('Preparing', 'Processing', 'Queued', 'Holding')
      AND JobType = 'Queueable'
    ];
    System.assertEquals(1, queuedJobs.size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunAsyncContextFlagsReflectLocalDrainKind(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><label>Account</label><pluralLabel>Accounts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField><fullName>Name</fullName><label>Name</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FlagFuture.cls"), `
public class FlagFuture {
  @future public static void run() {
    System.assertEquals(true, System.isFuture());
    System.assertEquals(false, System.isBatch());
    System.assertEquals(false, System.isQueueable());
    System.assertEquals(false, System.isScheduled());
    insert new Account(Name = 'future');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FlagQueue.cls"), `
public class FlagQueue {
  public void execute(QueueableContext qc) {
    System.assertEquals(false, System.isFuture());
    System.assertEquals(false, System.isBatch());
    System.assertEquals(true, System.isQueueable());
    System.assertEquals(false, System.isScheduled());
    insert new Account(Name = 'queueable');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FlagBatch.cls"), `
public class FlagBatch {
  public List<Integer> start(Database.BatchableContext bc) {
    System.assertEquals(true, System.isBatch());
    System.assertEquals(false, System.isFuture());
    System.assertEquals(false, System.isQueueable());
    System.assertEquals(false, System.isScheduled());
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {
    System.assertEquals(true, System.isBatch());
    insert new Account(Name = 'batch');
  }
  public void finish(Database.BatchableContext bc) {
    System.assertEquals(true, System.isBatch());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FlagSchedule.cls"), `
public class FlagSchedule {
  public void execute(SchedulableContext sc) {
    System.assertEquals(false, System.isFuture());
    System.assertEquals(false, System.isBatch());
    System.assertEquals(false, System.isQueueable());
    System.assertEquals(true, System.isScheduled());
    insert new Account(Name = 'scheduled');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncFlagTest.cls"), `
@isTest
private class AsyncFlagTest {
  @isTest static void localDrainReportsAsyncContext() {
    System.assertEquals(false, System.isFuture());
    System.assertEquals(false, System.isBatch());
    System.assertEquals(false, System.isQueueable());
    System.assertEquals(false, System.isScheduled());
    Test.startTest();
    FlagFuture.run();
    System.enqueueJob(new FlagQueue());
    Database.executeBatch(new FlagBatch(), 1);
    System.schedule('nightly', '0 0 0 * * ?', new FlagSchedule());
    Test.stopTest();
    Integer rows = [SELECT COUNT() FROM Account];
    System.assertEquals(4, rows);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunAsSetsUserContextForBlock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RunAsTest.cls"), `
@isTest
private class RunAsTest {
  @isTest static void scopesCurrentUser() {
    System.assertEquals('005000000000001', UserInfo.getUserId());
    System.runAs(new User(Id = 'user-a', ProfileId = 'profile-a', Username = 'user-a@example.test')) {
      System.assertEquals('user-a', UserInfo.getUserId());
      System.assertEquals('profile-a', UserInfo.getProfileId());
      System.assertEquals('user-a@example.test', UserInfo.getUserName());
    }
    System.assertEquals('005000000000001', UserInfo.getUserId());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunAsDMLPersistsNonSetupRecordWithAuditUser(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/Thing__c.object-meta.xml"), `
<CustomObject>
  <label>Thing</label>
  <pluralLabel>Things</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RunAsDMLTest.cls"), `
@isTest
private class RunAsDMLTest {
  @isTest static void persistsRecord() {
    User u = new User(Id = '005000000000999', ProfileId = '00e000000000006', Username = 'user-a@example.test');
    System.runAs(u) {
      insert new Thing__c();
    }
    List<Thing__c> rows = [SELECT Id, LastModifiedById FROM Thing__c];
    System.assertEquals(1, rows.size());
    System.assertEquals('005000000000999', rows[0].LastModifiedById);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunAsDMLPersistsOuterAssignedSObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/Thing__c.object-meta.xml"), `
<CustomObject>
  <label>Thing</label>
  <pluralLabel>Things</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RunAsOuterAssignedDMLTest.cls"), `
@isTest
private class RunAsOuterAssignedDMLTest {
  @isTest static void persistsOuterAssignedRecord() {
    User u = new User(Id = '005000000000999', ProfileId = '00e000000000006', Username = 'user-a@example.test');
    Thing__c row;
    System.runAs(u) {
      row = new Thing__c();
      insert row;
    }
    System.assertNotEquals(null, row.Id);
    System.assertEquals(1, [SELECT Id FROM Thing__c].size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunReportsAssertionStackWithFileAndLine(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	testFile := filepath.Join(root, "force-app/main/classes/StackTraceTest.cls")
	writeFile(t, testFile, `
@isTest
private class StackTraceTest {
  @isTest static void failsWithStack() {
    System.assertEquals(1, 2);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Failed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
	problem := run.Suites[0].Cases[0].Problem
	if problem == nil || problem.Type != "System.AssertException" {
		t.Fatalf("problem = %#v", problem)
	}
	if len(problem.Stack) == 0 || problem.Stack[0].File != testFile || problem.Stack[0].Line != 5 || problem.Stack[0].Column != 5 {
		t.Fatalf("stack = %#v", problem.Stack)
	}
}

func TestRunExecutesSObjectSOQLDMLAndTriggers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountNameTrigger.trigger"), `
trigger AccountNameTrigger on Account (before insert) {
  for (Account a : Trigger.new) {
    a.Name = a.Name + '!';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DataRuntimeTest.cls"), `
@isTest
private class DataRuntimeTest {
  @isTest static void dmlSoqlAndTriggersWork() {
    Account a = new Account(Name = 'Acme');
    insert a;
    String wanted = 'Acme!';
    List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = :wanted];
    System.assertEquals(1, rows.size());
    Account row = rows.get(0);
    System.assertEquals('Acme!', row.Name);
    row.put('Name', 'Changed');
    update row;
    List<Account> changed = Database.query('SELECT Id, Name FROM Account');
    System.assertEquals(1, changed.size());
    delete row;
    List<Account> remaining = [SELECT Id FROM Account];
    System.assertEquals(0, remaining.size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestDiscoverFilter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ManyTest.cls"), `
@isTest
private class ManyTest {
  @isTest static void first() { System.assert(true); }
  @isTest static void second() { System.assert(true); }
}
`)

	cases := Discover(loadTestIndex(t, root), Options{Filter: "second"})
	if len(cases) != 1 || cases[0].MethodName != "second" {
		t.Fatalf("cases = %#v", cases)
	}
}

func TestDiscoverSkipsHelpersInIsTestClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/HelperTest.cls"), `
@isTest
private class HelperTest {
  static String helper() { return 'skip'; }
  @isTest static void runs() { System.assertEquals('skip', helper()); }
}
`)

	cases := Discover(loadTestIndex(t, root), Options{})
	if len(cases) != 1 || cases[0].MethodName != "runs" {
		t.Fatalf("cases = %#v", cases)
	}
}

func TestRunResolvesVisualforcePageReferencesAndControllerConstructors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/AccountView.page"), `<apex:page standardController="Account" extensions="AccountViewExtension">
  <c:AccountBadge value="{!Account.Name}" />
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/AccountBadge.component"), `<apex:component controller="AccountBadgeController">
  <apex:attribute name="value" type="String" assignTo="{!value}" />
</apex:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/AccountViewExtension.cls"), `
public class AccountViewExtension {
  public String name;
  public AccountViewExtension(ApexPages.StandardController controller) {
    Account account = (Account) controller.getRecord();
    name = account.Name;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/AccountBadgeController.cls"), `
public class AccountBadgeController {
  public String value { get; set; }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/VisualforceControllerContractTest.cls"), `
@isTest
private class VisualforceControllerContractTest {
  @isTest static void resolvesPageTokenAndExtensionConstructor() {
    Account account = new Account(Name = 'Acme');
    ApexPages.StandardController controller = new ApexPages.StandardController(account);
    AccountViewExtension extension = new AccountViewExtension(controller);
    System.assertEquals('Acme', extension.name);
    Test.setCurrentPage(Page.AccountView);
    ApexPages.currentPage().getParameters().put('id', '001000000000001AAA');
    System.assertEquals('/apex/AccountView', ApexPages.currentPage().getUrl());
    System.assertEquals('001000000000001AAA', ApexPages.currentPage().getParameters().get('id'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunResetsApexPagesStateBetweenTestMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ResetProbe.page"), `<apex:page/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/PageStateResetTest.cls"), `
@isTest
private class PageStateResetTest {
  @isTest static void addsMessageAndParameter() {
    Test.setCurrentPage(Page.ResetProbe);
    ApexPages.currentPage().getParameters().put('marker', 'dirty');
    ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.ERROR, 'Summary'));
    System.assert(ApexPages.hasMessages());
  }
  @isTest static void seesCleanPageState() {
    System.assert(!ApexPages.hasMessages());
    System.assertEquals(null, ApexPages.currentPage().getParameters().get('marker'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestProjectRuntimeCompilesStaticMapInitializerWithEscapedStrings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticMapProbe.cls"), `
public class StaticMapProbe {
  public static String lookup(String key) {
    return Values.get(key);
  }

  private static final Map<String, String> Values = new Map<String, String>{
    'US' => 'US',
    'L\'ANDORRE' => 'AD'
  };
}
`)
	machine := vm.New(nil)
	if err := RegisterProjectRuntimeForRequest(machine, loadTestIndex(t, root)); err != nil {
		t.Fatal(err)
	}
	value, err := machine.CallStatic("StaticMapProbe.lookup", []vm.Value{vm.String("L'ANDORRE")})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != vm.ValueString || value.Text != "AD" {
		t.Fatalf("lookup = %#v, want AD", value)
	}
}

func TestProjectRuntimeInitializesStaticFieldsInSourceOrder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticConstants.cls"), `
public class StaticConstants {
  public static final Boolean IsInternal = false;
  public static final String Endpoint = StaticEndpointService.getEndpoint();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticEndpointService.cls"), `
public class StaticEndpointService {
  public static String getEndpoint() {
    if (StaticConstants.IsInternal) {
      return 'internal';
    }
    return 'external';
  }
}
`)
	machine := vm.New(nil)
	if err := RegisterProjectRuntimeForRequest(machine, loadTestIndex(t, root)); err != nil {
		t.Fatal(err)
	}
	value, err := machine.CallStatic("StaticEndpointService.getEndpoint", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != vm.ValueString || value.Text != "external" {
		t.Fatalf("endpoint = %#v, want external", value)
	}
}

func TestRunAssignsDottedPathThroughStaticFieldRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/BaseControllerProbe.cls"), `
public virtual class BaseControllerProbe {
  private String marker;
  public String Marker {
    get { return marker; }
    set { marker = value; }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ConcreteControllerProbe.cls"), `
public class ConcreteControllerProbe extends BaseControllerProbe {
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DottedStaticRootAssignmentTest.cls"), `
@isTest
private class DottedStaticRootAssignmentTest {
  private static ConcreteControllerProbe controller;

  @isTest static void assignsInheritedPropertyThroughStaticField() {
    controller = new ConcreteControllerProbe();
    controller.Marker = 'set';
    System.assertEquals('set', controller.Marker);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunJSONDeserializeUsesPropertySetter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONPropertySetterPayload.cls"), `
public class JSONPropertySetterPayload {
  public Date StartDate { get; set; }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONPropertySetterEnvelope.cls"), `
public class JSONPropertySetterEnvelope {
  private JSONPropertySetterPayload payload;
  public JSONPropertySetterPayload Payload {
    get {
      if (payload == null) {
        payload = new JSONPropertySetterPayload();
      }
      return payload;
    }
    private set { payload = value; }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONPropertySetterTest.cls"), `
@isTest
private class JSONPropertySetterTest {
  @isTest static void deserializePopulatesBackingFieldThroughSetter() {
    JSONPropertySetterEnvelope envelope = (JSONPropertySetterEnvelope)JSON.deserialize(
      '{"Payload":{"StartDate":"2026-05-07"}}',
      JSONPropertySetterEnvelope.class
    );
    System.assertNotEquals(null, envelope.Payload.StartDate);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunJSONDeserializePopulatesCustomGetterAutoSetterListProperty(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONAutoSetterMapping.cls"), `
public class JSONAutoSetterMapping {
  public JSONAutoSetterMapping provider;
  public class Row {
    public String tpField;
    public String sfField;
    public Boolean isComplete() {
      return String.isNotBlank(tpField) && String.isNotBlank(sfField);
    }
  }
  public List<Row> rows {
    get {
      if (rows?.isEmpty() == false) {
        List<Row> validRows = new List<Row>();
        for (Row row : rows) {
          if (row.isComplete()) {
            validRows.add(row);
          }
        }
        rows = validRows;
      }
      return rows;
    }
    set;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONAutoSetterMappingTest.cls"), `
@isTest
private class JSONAutoSetterMappingTest {
  @isTest static void deserializePopulatesRows() {
    JSONAutoSetterMapping mapping = (JSONAutoSetterMapping)JSON.deserialize(
      '{"provider":{"rows":[{"tpField":"gender","sfField":"LeadSource"}]}}',
      JSONAutoSetterMapping.class
    );
    mapping = mapping.provider;
    System.assertEquals(1, mapping.rows.size());
    System.assertEquals('LeadSource', mapping.rows.get(0).sfField);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunPropertySetterSeesAutoPropertyAssignedInConstructor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SetterDispatchControllerProbe.cls"), `
public class SetterDispatchControllerProbe {
  public Account CurrentRecord { get; set; }
  public SetterDispatchControllerProbe() {
    CurrentRecord = new Account();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SetterDispatchBaseProbe.cls"), `
public virtual class SetterDispatchBaseProbe {
  private SetterDispatchControllerProbe c;
  public SetterDispatchControllerProbe Controller {
    get { return c; }
    set {
      c = value;
      OnControllerSet();
    }
  }
  public virtual void OnControllerSet() {}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SetterDispatchChildProbe.cls"), `
public class SetterDispatchChildProbe extends SetterDispatchBaseProbe {
  public override void OnControllerSet() {
    Controller.CurrentRecord.Name = 'set';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SetterDispatchPropertyTest.cls"), `
@isTest
private class SetterDispatchPropertyTest {
  private static SetterDispatchControllerProbe controller;
  private static SetterDispatchChildProbe child;

  @isTest static void setterDispatchSeesConstructorAssignedProperty() {
    controller = new SetterDispatchControllerProbe();
    System.assertNotEquals(null, controller.CurrentRecord);
    child = new SetterDispatchChildProbe();
    child.Controller = controller;
    System.assertEquals('set', controller.CurrentRecord.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunInstanceFieldLookupIsCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CaseInsensitiveFieldProbe.cls"), `
public class CaseInsensitiveFieldProbe {
  public Map<Object, List<Account>> RecordsByKey;
  public CaseInsensitiveFieldProbe() {
    RecordsByKey = new Map<Object, List<Account>>();
    System.assertEquals(0, recordsByKey.keySet().size());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CaseInsensitiveFieldLookupTest.cls"), `
@isTest
private class CaseInsensitiveFieldLookupTest {
  @isTest static void constructorReadsFieldWithDifferentCase() {
    new CaseInsensitiveFieldProbe();
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeInitializesNestedInstanceFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedInitializer.cls"), `
public class NestedInitializer {
  public static Inner StaticInner { get; private set; }
  public static TestFactory Test { get; private set; }
  static {
    StaticInner = new Inner();
    Test = new TestFactory();
  }
  public class Inner {
    private List<String> values = new List<String>();
    public Child child = new Child();
    public Child Database = new Child();
    private Inner() {
    }
    public Integer size() {
      values.add('x');
      return values.size();
    }
  }
  public class Child {
    public Integer value() {
      return 7;
    }
  }
  public class TestFactory {
    public MockDatabase Database = new MockDatabase();
    private TestFactory() {
    }
  }
  public class MockDatabase {
    private List<String> rows = new List<String>();
    private MockDatabase() {
    }
    public Boolean hasRecords() {
      return rows != null;
    }
  }
  public static Integer run() {
    return new Inner().size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedInitializerTest.cls"), `
@isTest
private class NestedInitializerTest {
  @isTest static void initializes() {
    System.assertEquals(1, NestedInitializer.run());
    System.assertEquals(7, NestedInitializer.StaticInner.child.value());
    System.assertEquals(7, NestedInitializer.StaticInner.Database.value());
    System.assert(NestedInitializer.Test.Database.hasRecords());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestProjectRuntimeMatchesSObjectListDowncastConstructors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ListDowncastDomain.cls"), `
public class ListDowncastDomain {
  public List<Opportunity> Records;
  public ListDowncastDomain(List<Opportunity> source) {
    Records = source;
  }
  public static Integer run() {
    List<SObject> records = new List<SObject>{ new Opportunity(Name = 'Test') };
    ListDowncastDomain domain = new ListDowncastDomain(records);
    return domain.Records.size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CustomListDowncastDomain.cls"), `
public class CustomListDowncastDomain {
  public List<Thing__c> Records;
  public CustomListDowncastDomain(List<Thing__c> source) {
    Records = source;
  }
  public static Integer run() {
    List<SObject> records = new List<SObject>{ new Thing__c(Name = 'Test') };
    CustomListDowncastDomain domain = new CustomListDowncastDomain(records);
    return domain.Records.size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Thing</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ListDowncastDomainTest.cls"), `
@isTest
private class ListDowncastDomainTest {
  @isTest static void constructs() {
    System.assertEquals(1, ListDowncastDomain.run());
    System.assertEquals(1, CustomListDowncastDomain.run());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeCallsImplicitDefaultSuperConstructor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ImplicitSuperBase.cls"), `
public virtual class ImplicitSuperBase {
  public List<String> values;
  public ImplicitSuperBase() {
    values = new List<String>();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ImplicitSuperChild.cls"), `
public class ImplicitSuperChild extends ImplicitSuperBase {
  public Integer size() {
    values.add('x');
    return values.size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ImplicitSuperChildTest.cls"), `
@isTest
private class ImplicitSuperChildTest {
  @isTest static void constructsBase() {
    System.assertEquals(1, new ImplicitSuperChild().size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeMatchesChainedConstructorWithSObjectTypeAlias(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChainedSObjectTypeCtor.cls"), `
public class ChainedSObjectTypeCtor {
  public SObjectType Captured;
  public ChainedSObjectTypeCtor(List<SObject> records) {
    this(records, records.getSObjectType());
  }
  public ChainedSObjectTypeCtor(List<SObject> records, SObjectType objectType) {
    Captured = objectType;
  }
  public static SObjectType run() {
    return new ChainedSObjectTypeCtor(new List<SObject>{ new Account(Name = 'Test') }).Captured;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChainedSObjectTypeCtorTest.cls"), `
@isTest
private class ChainedSObjectTypeCtorTest {
  @isTest static void constructs() {
    System.assertEquals(Account.SObjectType, ChainedSObjectTypeCtor.run());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeSuperclassThisConstructorChainsLexically(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LexicalBaseCtor.cls"), `
public virtual class LexicalBaseCtor {
  public Integer value;
  public LexicalBaseCtor(Integer seed) {
    this(seed, 2);
  }
  public LexicalBaseCtor(Integer seed, Integer multiplier) {
    value = seed * multiplier;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LexicalChildCtor.cls"), `
public class LexicalChildCtor extends LexicalBaseCtor {
  public LexicalChildCtor(Integer seed) {
    super(seed);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LexicalChildCtorTest.cls"), `
@isTest
private class LexicalChildCtorTest {
  @isTest static void constructs() {
    LexicalChildCtor child = new LexicalChildCtor(3);
    System.assertEquals(6, child.value);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeAssignsNestedInterfaceByShortName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedInterfaceOwner.cls"), `
public class NestedInterfaceOwner {
  public interface Worker {
    Integer run();
  }
  public class Impl implements Worker {
    public Integer run() {
      return 7;
    }
  }
  public static Integer execute() {
    Worker worker = new Impl();
    return worker.run();
  }
  public static Integer executeFromType() {
    Type implType = Type.forName('NestedInterfaceOwner.Impl');
    Worker worker = (Worker) implType.newInstance();
    return worker.run();
  }
  public static Integer executeFromInterfaceMap() {
    Map<String, Worker> workers = new Map<String, Worker>();
    workers.put('one', (Worker) Type.forName('NestedInterfaceOwner.Impl').newInstance());
    return workers.get('one').run();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedInterfaceOwnerTest.cls"), `
@isTest
private class NestedInterfaceOwnerTest {
  @isTest static void assignsShortName() {
    System.assertEquals(7, NestedInterfaceOwner.execute());
    System.assertEquals(7, NestedInterfaceOwner.executeFromType());
    System.assertEquals(7, NestedInterfaceOwner.executeFromInterfaceMap());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeStaticFieldInitializerCanReadHierarchyCustomSetting(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticSettings.cls"), `
public class StaticSettings {
  public static final Boolean IsInternal = Setup_Settings__c.getOrgDefaults().IsInternalOrg__c;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/Setup_Settings__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <customSettingsType>Hierarchy</customSettingsType>
  <label>Setup Settings</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/fields/IsInternalOrg__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>IsInternalOrg__c</fullName>
  <type>Checkbox</type>
</CustomField>
`)
	org := storage.NewOrgState()
	org.Objects["Setup_Settings__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "Setup_Settings__c",
			Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
			Fields: map[string]storage.Field{
				"SetupOwnerId":     {APIName: "SetupOwnerId", Type: storage.FieldString},
				"IsInternalOrg__c": {APIName: "IsInternalOrg__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0s000000000001": {
				ID:     "a0s000000000001",
				Object: "Setup_Settings__c",
				Fields: map[string]storage.Value{
					"SetupOwnerId":     storage.StringValue("00D000000000001"),
					"IsInternalOrg__c": storage.BooleanValue(false),
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if err := RegisterProjectRuntimeForRequest(machine, loadTestIndex(t, root)); err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymous(`System.assertEquals(false, StaticSettings.IsInternal);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRunHierarchyCustomSettingAbsentOrgDefaultsEqualsFreshEmptySObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupSettingsDefaultsTest.cls"), `
@isTest
private class SetupSettingsDefaultsTest {
  @isTest static void absentDefaultsEqualsFreshEmpty() {
    Setup_Settings__c defaults = Setup_Settings__c.getOrgDefaults();
    System.assertEquals(false, defaults.IsInternalOrg__c);
    System.assertEquals(new Setup_Settings__c(), defaults);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/Setup_Settings__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <customSettingsType>Hierarchy</customSettingsType>
  <label>Setup Settings</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/fields/IsInternalOrg__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>IsInternalOrg__c</fullName>
  <type>Checkbox</type>
  <defaultValue>false</defaultValue>
</CustomField>
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v", got, run.Suites[0].Cases[0])
	}
}

func loadTestIndex(t *testing.T, root string) typesys.Index {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := oaerschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}

func TestOrgFromIndexIncludesGeneratedStandardSchema(t *testing.T) {
	org := orgFromIndex(typesys.Index{})

	for objectName, fieldName := range map[string]string{
		"Account":             "AccountNumber",
		"Task":                "WhatId",
		"PricebookEntry":      "Product2Id",
		"OpportunityLineItem": "PricebookEntryId",
	} {
		state, ok := org.Objects[objectName]
		if !ok {
			t.Fatalf("%s object was not exposed", objectName)
		}
		if _, ok := state.Definition.Fields[fieldName]; !ok {
			t.Fatalf("%s.%s field was not exposed; fields=%#v", objectName, fieldName, state.Definition.Fields)
		}
	}
}

func TestOrgFromIndexIncludesProjectReferencedStandardFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AssetProbe.cls"), `
public class AssetProbe {
	public static void touch() {
		Asset asset = new Asset();
		asset.ExternalIdentifier = 'external';
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state, ok := org.Objects["Asset"]
	if !ok {
		t.Fatal("Asset object was not exposed")
	}
	field, ok := state.Definition.Fields["ExternalIdentifier"]
	if !ok {
		t.Fatalf("Asset.ExternalIdentifier was not inferred; fields=%#v", state.Definition.Fields)
	}
	if field.Type != storage.FieldString || field.DisplayType != "STRING" {
		t.Fatalf("Asset.ExternalIdentifier field = %#v", field)
	}
}

func TestOrgFromIndexDoesNotInferStandardParentRelationshipAsField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountProbe.cls"), `
public class AccountProbe {
	public static void touch(Account existingRecord) {
		Boolean linked = existingRecord.Parent?.IsPersonAccount == true;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state := org.Objects["Account"]
	if _, ok := state.Definition.Fields["Parent"]; ok {
		t.Fatalf("Account.Parent was inferred as a concrete field: %#v", state.Definition.Fields["Parent"])
	}
	field, ok := state.Definition.Fields["ParentId"]
	if !ok {
		t.Fatalf("Account.ParentId missing from standard fields")
	}
	if field.Type != storage.FieldReference || !parentRelationshipKnown(state.Definition, "Parent") {
		t.Fatalf("Account.ParentId relationship not preserved: field=%#v relations=%#v", field, state.Definition.Relations)
	}
}

func TestOrgFromIndexIncludesApexClassRows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/WidgetTestData.cls"), `
public class WidgetTestData {
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state, ok := org.Objects["ApexClass"]
	if !ok {
		t.Fatal("ApexClass object was not exposed")
	}
	if _, ok := state.Definition.Fields["ApiVersion"]; !ok {
		t.Fatalf("ApexClass.ApiVersion was not exposed: %#v", state.Definition.Fields)
	}
	if _, ok := state.Definition.Fields["LengthWithoutComments"]; !ok {
		t.Fatalf("ApexClass.LengthWithoutComments was not exposed: %#v", state.Definition.Fields)
	}
	var found bool
	for _, record := range state.Records {
		if record.Fields["Name"].String != "WidgetTestData" {
			continue
		}
		found = true
		if record.Fields["Body"].String == "" {
			t.Fatal("ApexClass.Body was empty")
		}
		if record.Fields["NamespacePrefix"].Kind != storage.ValueString {
			t.Fatalf("NamespacePrefix field = %#v", record.Fields["NamespacePrefix"])
		}
		if record.Fields["ApiVersion"].Decimal != "65.0" {
			t.Fatalf("ApiVersion field = %#v", record.Fields["ApiVersion"])
		}
		if record.Fields["LengthWithoutComments"].Integer == 0 {
			t.Fatalf("LengthWithoutComments field = %#v", record.Fields["LengthWithoutComments"])
		}
	}
	if !found {
		t.Fatalf("ApexClass row for WidgetTestData not found: %#v", state.Records)
	}
}

func TestOrgFromIndexIncludesCustomApplicationMenuRows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/applications/Apex_Recipes.app-meta.xml"), `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata"><label>Apex Recipes</label></CustomApplication>`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	apps := org.Objects["CustomApplication"]
	menu := org.Objects["AppMenuItem"]
	if len(apps.Records) != 1 || len(menu.Records) != 1 {
		t.Fatalf("application records = %d, menu records = %d", len(apps.Records), len(menu.Records))
	}
	var appID storage.ID
	for id, record := range apps.Records {
		appID = id
		if record.Fields["DeveloperName"].String != "Apex_Recipes" || record.Fields["Label"].String != "Apex Recipes" {
			t.Fatalf("CustomApplication fields = %#v", record.Fields)
		}
	}
	for _, record := range menu.Records {
		if record.Fields["Name"].String != "Apex_Recipes" || record.Fields["ApplicationId"].ID != appID {
			t.Fatalf("AppMenuItem fields = %#v, appID=%s", record.Fields, appID)
		}
	}
}

func TestOrgFromIndexIncludesProjectProfiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Nimble AMS Standard.profile-meta.xml"), `<Profile/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/Read_access_to_Account_Shipping_Address.permissionset-meta.xml"), `<PermissionSet>
  <label>Read access to Account Shipping Address</label>
  <fieldPermissions>
    <editable>false</editable>
    <field>Account.ShippingStreet</field>
    <readable>true</readable>
  </fieldPermissions>
  <objectPermissions>
    <allowCreate>false</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>false</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>Account</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
</PermissionSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsetgroups/Permission_Set_Group_for_testing.permissionsetgroup-meta.xml"), `<PermissionSetGroup/>`)

	org := orgFromIndex(loadTestIndex(t, root))
	profiles := org.Objects["Profile"].Records
	foundProfile := false
	for _, record := range profiles {
		if record.Fields["Name"].String == "Nimble AMS Standard" {
			foundProfile = true
			break
		}
	}
	if !foundProfile {
		t.Fatalf("project profile row was not created; records=%#v", profiles)
	}
	if !recordWithFieldValueExists(org.Objects["PermissionSet"], "Name", "Read_access_to_Account_Shipping_Address") {
		t.Fatalf("project permission set row was not created; records=%#v", org.Objects["PermissionSet"].Records)
	}
	if !recordWithFieldValueExists(org.Objects["ObjectPermissions"], "SObjectType", "Account") {
		t.Fatalf("project object permissions row was not created; records=%#v", org.Objects["ObjectPermissions"].Records)
	}
	if !recordWithFieldValueExists(org.Objects["FieldPermissions"], "Field", "Account.ShippingStreet") {
		t.Fatalf("project field permissions row was not created; records=%#v", org.Objects["FieldPermissions"].Records)
	}
	if !recordWithFieldValueExists(org.Objects["PermissionSetGroup"], "DeveloperName", "Permission_Set_Group_for_testing") {
		t.Fatalf("project permission set group row was not created; records=%#v", org.Objects["PermissionSetGroup"].Records)
	}
}

func recordWithFieldValueExists(state storage.ObjectState, fieldName, value string) bool {
	for _, record := range state.Records {
		if strings.EqualFold(record.Fields[fieldName].String, value) {
			return true
		}
	}
	return false
}

func TestOrgFromIndexIncludesProjectStaticResources(t *testing.T) {
	root := filepath.Join("..", "..", "example-projects", "src-nmb-nutpl-develop")
	if _, err := os.Stat(filepath.Join(root, "sfdx-project.json")); err != nil {
		t.Skip("example project is not available")
	}
	org := orgFromIndex(loadTestIndex(t, root))
	object := org.Objects["StaticResource"]
	for _, record := range object.Records {
		if record.Fields["Name"].String == "resetcss" {
			return
		}
	}
	t.Fatalf("resetcss StaticResource record was not created; records=%#v", object.Records)
}

func TestRuntimeCallsNUTPLMergeValuesPutSObject(t *testing.T) {
	root := filepath.Join("..", "..", "example-projects", "src-nmb-nutpl-develop")
	if _, err := os.Stat(filepath.Join(root, "sfdx-project.json")); err != nil {
		t.Skip("example project is not available")
	}
	index := loadTestIndex(t, root)
	methods := compileProjectMethods(index)
	found := false
	for _, method := range methods {
		if method.Name == "MergeValues.putSObject" {
			found = true
			if len(method.Params) != 2 || method.Params[0].Type != "String" || method.Params[1].Type != "Id" {
				t.Fatalf("MergeValues.putSObject params = %#v", method.Params)
			}
		}
	}
	if !found {
		t.Fatal("MergeValues.putSObject was not compiled")
	}
	org := orgFromIndex(index)
	machine := vm.New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := registerRuntime(machine, methods, compileProjectClasses(index, methods), nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := machine.Classes["MergeValues"]; !ok {
		t.Fatal("MergeValues class was not registered")
	}
	if candidates := machine.MethodOverloads["MergeValues.putSObject"]; len(candidates) == 0 {
		t.Fatalf("MergeValues.putSObject overloads were not registered; methods has %d entries", len(machine.Methods))
	}
	program, err := vm.CompileAnonymous(`
MergeValues bag = new MergeValues();
bag.registerFieldSecurely('User.FirstName');
bag.registerFieldSecurely('User.LastName');
bag.putSObject('User', UserInfo.getUserId());
System.assertEquals(UserInfo.getFirstName(), bag.get('User.FirstName'));
System.assertEquals(UserInfo.getLastName(), bag.get('User.LastName'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestParseParamsSkipsAnnotationArguments(t *testing.T) {
	params, err := parseParams(`
@AuraEnabled(Cacheable=true)
public static Id getAccountId() {
    return UserInfo.getUserId();
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 0 {
		t.Fatalf("params = %#v", params)
	}

	params, err = parseParams(`
@AuraEnabled(Cacheable=true)
public static String render(final Id templateId, Map<String, Object> values) {
    return '';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 || params[0].Type != "Id" || params[0].Name != "templateId" || params[1].Type != "Map<String, Object>" || params[1].Name != "values" {
		t.Fatalf("params = %#v", params)
	}

	params, err = parseParams(`
public static List<SObject> queryByIds(String query, /* do not remove param */ Set<Id> ids) {
    return Database.query(query);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 || params[0].Type != "String" || params[0].Name != "query" || params[1].Type != "Set<Id>" || params[1].Name != "ids" {
		t.Fatalf("params = %#v", params)
	}

	params, err = parseParams(`
private BulkPriceClassResponse getBulkPriceClassResponse(Map<Id, CartItemPricer>cartItemPricersByCartItemId) {
    return null;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 1 || params[0].Type != "Map<Id, CartItemPricer>" || params[0].Name != "cartItemPricersByCartItemId" {
		t.Fatalf("params = %#v", params)
	}
}

func TestProjectRuntimeResolvesNestedEnumConstantsInStaticInitializers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ObjectMappings.cls"), `
public class ObjectMappings {
  public enum MAPPING_OPERATION_TYPE {
    /**
     * Sets a field value.
     */
    setFieldValue,
    // Selects source records.
    sourceObjectSelectionCriteria,
    targetObjectSelectionCriteria
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MappingOperationFactory.cls"), `
public class MappingOperationFactory {
  private static final Map<ObjectMappings.MAPPING_OPERATION_TYPE, String> operationInstances =
    new Map<ObjectMappings.MAPPING_OPERATION_TYPE, String>{
      ObjectMappings.MAPPING_OPERATION_TYPE.setFieldValue => 'set',
      ObjectMappings.MAPPING_OPERATION_TYPE.sourceObjectSelectionCriteria => 'source',
      ObjectMappings.MAPPING_OPERATION_TYPE.targetObjectSelectionCriteria => 'target'
    };
  public static String get(ObjectMappings.MAPPING_OPERATION_TYPE operationType) {
    return operationInstances.get(operationType);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MappingOperationFactoryTest.cls"), `
@isTest
private class MappingOperationFactoryTest {
  @isTest static void resolvesNestedEnumConstants() {
    System.assertEquals('set', MappingOperationFactory.get(ObjectMappings.MAPPING_OPERATION_TYPE.setFieldValue));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
