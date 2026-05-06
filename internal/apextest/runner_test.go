package apextest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		t.Fatalf("summary = %#v run=%#v", got, run)
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
		t.Fatalf("summary = %#v run=%#v", got, run)
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
		t.Fatalf("summary = %#v run=%#v", got, run)
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
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunDispatchesCreateStubToStubProvider(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Greeter.cls"), `
public class Greeter {
  public String greet(String name) {
    return 'real';
  }
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
		t.Fatalf("summary = %#v run=%#v", got, run)
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
    System.assertEquals(1, AsyncMarker.ran);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
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
    String queueId = System.enqueueJob(new FirstQueue());
    System.assertNotEquals('', batchId);
    System.assertNotEquals('', scheduleId);
    System.assertNotEquals('', queueId);
    System.assertEquals(0, AsyncState.futureRan);
    System.assertEquals(0, AsyncState.batchSum);
    System.assertEquals(0, AsyncState.scheduledRan);
    System.assertEquals(0, AsyncState.queueRan);
    Integer beforeRows = [SELECT COUNT() FROM Account];
    System.assertEquals(0, beforeRows);
    Test.stopTest();
    Integer afterRows = [SELECT COUNT() FROM Account];
    System.assertEquals(7, afterRows);
    List<AsyncApexJob> jobs = [SELECT Id, Status, JobType FROM AsyncApexJob];
    System.assertEquals(5, jobs.size());
    List<CronTrigger> crons = [SELECT Id, State FROM CronTrigger];
    System.assertEquals(1, crons.size());
    CronTrigger cron = crons.get(0);
    System.assertEquals('Complete', cron.State);
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
    System.assertEquals('system', UserInfo.getUserId());
    System.runAs(new User(Id = 'user-a', ProfileId = 'profile-a', Username = 'user-a@example.test')) {
      System.assertEquals('user-a', UserInfo.getUserId());
      System.assertEquals('profile-a', UserInfo.getProfileId());
      System.assertEquals('user-a@example.test', UserInfo.getUserName());
    }
    System.assertEquals('system', UserInfo.getUserId());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
