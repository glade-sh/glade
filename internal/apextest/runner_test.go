package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	oaerschema "github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
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
		t.Fatalf("summary = %#v run=%#v", got, run)
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
		t.Fatalf("summary = %#v run=%#v", got, run)
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
    System.assertEquals(0, AsyncMarker.ran);
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

func TestRunAsSetsUserContextForBlock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RunAsTest.cls"), `
@isTest
private class RunAsTest {
  @isTest static void scopesCurrentUser() {
    System.assertEquals('system', UserInfo.getUserId());
    System.runAs('user-a') {
      System.assertEquals('user-a', UserInfo.getUserId());
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
	if len(problem.Stack) == 0 || problem.Stack[0].File != testFile || problem.Stack[0].Line == 0 {
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
