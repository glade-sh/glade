package sema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

type scratchBackedSourceFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type scratchBackedRuleCase struct {
	ID           string                    `json:"id"`
	Area         string                    `json:"area"`
	APIVersion   int                       `json:"apiVersion"`
	SourceKind   string                    `json:"sourceKind"`
	Source       string                    `json:"source"`
	Dependencies []scratchBackedSourceFile `json:"dependencies"`
	ProjectFiles []scratchBackedSourceFile `json:"projectFiles"`
	Oracle       string                    `json:"oracle"`
}

func TestScratchBackedAnnotationRuleRegressions(t *testing.T) {
	runScratchBackedRuleCases(t, "annotations",
		"APEX-AUDIT-FUTURE-CALLS-FUTURE",
		"APEX-AUDIT-FUTURE-METHOD-IN-BATCHABLE-CLASS",
		"APEX-AUDIT-FUTURE-UNKNOWN-PROPERTY",
		"APEX-AUDIT-INVOCABLE-METHOD-INNER-CLASS",
		"APEX-AUDIT-INVOCABLE-METHOD-MAP-PARAMETER",
		"APEX-AUDIT-INVOCABLE-METHOD-MAP-RETURN",
		"APEX-AUDIT-INVOCABLE-METHOD-TWO-PARAMETERS",
		"APEX-AUDIT-INVOCABLE-METHOD-UNKNOWN-PROPERTY",
		"APEX-AUDIT-INVOCABLE-VARIABLE-FINAL",
		"APEX-AUDIT-INVOCABLE-VARIABLE-UNKNOWN-PROPERTY",
		"APEX-AUDIT-IS-TEST-CRITICAL-ON-METHOD",
		"APEX-AUDIT-IS-TEST-ENUM",
		"APEX-AUDIT-IS-TEST-GLOBAL-CLASS",
		"APEX-AUDIT-IS-TEST-INTERFACE",
		"APEX-AUDIT-IS-TEST-NESTED-CLASS",
		"APEX-AUDIT-IS-TEST-PARALLEL-SEE-ALL-DATA",
		"APEX-AUDIT-IS-TEST-TESTFOR-ON-METHOD",
		"APEX-AUDIT-IS-TEST-UNKNOWN-PROPERTY",
		"APEX-AUDIT-JSON-ACCESS-INVALID-VALUE",
		"APEX-AUDIT-NAMESPACE-ACCESSIBLE-INTERFACE-MEMBER",
		"APEX-AUDIT-NAMESPACE-ACCESSIBLE-INVOCABLE",
		"APEX-AUDIT-PRIVATE-INVOCABLE-METHOD",
		"APEX-AUDIT-PRIVATE-INVOCABLE-VARIABLE",
		"APEX-AUDIT-PROPERTY-INVOCABLE-VARIABLE",
		"APEX-AUDIT-REST-BLOB-PARAMETER",
		"APEX-AUDIT-REST-MAPPING-MISSING",
		"APEX-AUDIT-REST-NON-STRING-MAP-KEY",
		"APEX-AUDIT-REST-RESOURCE-INNER-CLASS",
		"APEX-AUDIT-REST-RESOURCE-UNKNOWN-PROPERTY",
		"APEX-AUDIT-UNKNOWN-METHOD-ANNOTATION",
		"APEX-AUDIT-WEBSERVICE-INTERFACE-METHOD",
		"APEX-AUDIT-WEBSERVICE-MAP-PARAMETER",
		"APEX-AUDIT-WEBSERVICE-SET-RETURN",
	)
}

func TestScratchBackedDeclarationRuleRegressions(t *testing.T) {
	runScratchBackedRuleCases(t, "declarations",
		"APEX-AUDIT-BARE-RETURN-FROM-NONVOID",
		"APEX-AUDIT-CONSTRUCTOR-THIS-NOT-FIRST",
		"APEX-AUDIT-DIAMOND-LIST-CONSTRUCTION",
		"APEX-AUDIT-FINAL-LOCAL-REASSIGNMENT",
		"APEX-AUDIT-FOR-NON-BOOLEAN-CONDITION",
		"APEX-AUDIT-GETTER-WITHOUT-RETURN",
		"APEX-AUDIT-IF-NON-BOOLEAN-CONDITION",
		"APEX-AUDIT-INNER-AS-NESTED-TYPE-IDENTIFIER",
		"APEX-AUDIT-ITERATOR-METHODS-WITHOUT-ACCESS",
		"APEX-AUDIT-OVERRIDE-METHOD-WITHOUT-ACCESS-V65",
		"APEX-AUDIT-QUEUEABLE-EXECUTE-WITHOUT-ACCESS",
		"APEX-AUDIT-RETURN-VALUE-FROM-VOID",
		"APEX-AUDIT-SUPER-OUTSIDE-OVERRIDE",
		"APEX-AUDIT-TEST-SETUP-WITH-SEE-ALL-DATA",
		"APEX-AUDIT-TRANSIENT-LOCAL-VARIABLE",
		"APEX-AUDIT-TRANSIENT-STATIC-FIELD",
		"APEX-AUDIT-UNINITIALIZED-LOCAL-READ",
		"APEX-AUDIT-WHILE-NON-BOOLEAN-CONDITION",
	)
}

func TestScratchBackedInheritanceRuleRegressions(t *testing.T) {
	runScratchBackedRuleCases(t, "inheritance",
		"APEX-AUDIT-BATCHABLE-NARROWER-EXECUTE-SCOPE",
	)
}

func TestScratchBackedInheritanceContractsRuleRegressions(t *testing.T) {
	runScratchBackedRuleCases(t, "inheritance-contracts",
		"APEX-ABSTRACT-IMPLEMENTATION-MISSING-OVERRIDE",
	)
}

func TestScratchBackedQueryRuleRegressions(t *testing.T) {
	runScratchBackedRuleCases(t, "query",
		"APEX-AUDIT-AGGREGATE-FUNCTION-IN-WHERE",
		"APEX-AUDIT-AGGREGATE-NON-GROUPED-FIELD",
		"APEX-AUDIT-AGGREGATE-QUERY-WRONG-TYPE",
		"APEX-AUDIT-GROUP-BY-NON-GROUPABLE-FIELD",
		"APEX-AUDIT-SOQL-FOR-WRONG-VARIABLE-TYPE",
		"APEX-AUDIT-SOQL-WRONG-ASSIGNMENT-TYPE",
		"APEX-AUDIT-SOSL-DUPLICATE-RETURNING-OBJECT",
		"APEX-AUDIT-SOSL-WITHOUT-RETURNING",
		"APEX-AUDIT-TYPEOF-WITH-GROUP-BY",
	)
}

func TestScratchBackedStatementRuleRegressions(t *testing.T) {
	runScratchBackedRuleCases(t, "statements",
		"APEX-AUDIT-DUPLICATE-CATCH-TYPE",
		"APEX-AUDIT-SWITCH-MULTIPLE-SOBJECT-TYPES-ONE-WHEN",
		"APEX-AUDIT-UNREACHABLE-AFTER-THROW-API40-CONTROL",
		"APEX-AUDIT-UNREACHABLE-AFTER-THROW-API41",
	)
}

func TestScratchBackedTypeRuleRegressions(t *testing.T) {
	runScratchBackedRuleCases(t, "types",
		"APEX-AUDIT-ANYTYPE-PSEUDO-TYPE-PARAMETER",
		"APEX-AUDIT-FINAL-PROPERTY",
		"APEX-AUDIT-PROPERTY-ON-INTERFACE",
	)
}

func runScratchBackedRuleCases(t *testing.T, area string, ids ...string) {
	t.Helper()
	var cases []scratchBackedRuleCase
	if err := json.Unmarshal([]byte(scratchBackedRuleCasesJSON), &cases); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]scratchBackedRuleCase, len(cases))
	for _, rule := range cases {
		byID[rule.ID] = rule
	}
	if len(ids) != scratchBackedRuleAreaCount(cases, area) {
		t.Fatalf("%s explicit IDs = %d, catalog cases = %d", area, len(ids), scratchBackedRuleAreaCount(cases, area))
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		rule, ok := byID[id]
		if !ok {
			t.Fatalf("explicit rule ID %q is absent from the product snapshot", id)
		}
		if rule.Area != area {
			t.Fatalf("rule %q area = %q, want %q", id, rule.Area, area)
		}
		if seen[id] {
			t.Fatalf("rule %q is enumerated more than once", id)
		}
		seen[id] = true
		t.Run(rule.ID, func(t *testing.T) {
			result := analyzeScratchBackedRuleCase(t, rule)
			gotReject := result.HasErrors()
			wantReject := rule.Oracle == "reject"
			if gotReject != wantReject {
				t.Fatalf("API %d %s result rejected=%v, want %v; diagnostics=%#v", rule.APIVersion, rule.SourceKind, gotReject, wantReject, result.Diagnostics)
			}
		})
	}
}

func TestAPI67ReleaseLanguageRuleRegressions(t *testing.T) {
	// These product-owned copies are bound to the Salesforce API 67 oracle
	// recorded by the glade-tools Apex language-rule catalog.
	for _, rule := range api67ReleaseLanguageRuleCases {
		rule := rule
		t.Run(rule.ID, func(t *testing.T) {
			result := analyzeScratchBackedRuleCase(t, rule)
			gotReject := result.HasErrors()
			wantReject := rule.Oracle == "reject"
			if gotReject != wantReject {
				t.Fatalf("API %d %s result rejected=%v, want %v; diagnostics=%#v", rule.APIVersion, rule.SourceKind, gotReject, wantReject, result.Diagnostics)
			}
		})
	}
}

var api67ReleaseLanguageRuleCases = []scratchBackedRuleCase{
	{
		ID:         "APEX-RELEASE-GETTER-SELF-ASSIGNMENT",
		Area:       "properties",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { public Integer Value { get { Value = 1; return Value; } } }`,
		Oracle:     "reject",
	},
	{
		ID:         "APEX-RELEASE-GETTER-EXTERNAL-ASSIGNMENT",
		Area:       "properties",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { public Integer Value { get { return 1; } } public static void run() { new Probe().Value = 2; } }`,
		Oracle:     "reject",
	},
	{
		ID:         "APEX-RELEASE-BACKSLASH-ESCAPED-ANNOTATION",
		Area:       "annotations",
		APIVersion: 67,
		SourceKind: "class",
		Source: `public class Probe {
    @InvocableVariable(
        Required=false
        Label='Email From Org-Wide Id'
        Description='The Salesforce Id of the Organization-Wide email address to use as the "From" in emails. If this isn\'t set, the email address of the user sending the email is used instead.'
    )
    public String emailFromOrgWideId;
}`,
		Oracle: "accept",
	},
	{
		ID:         "APEX-RELEASE-SIBLING-FOR-INITIALIZER-SCOPES",
		Area:       "statements",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { public static void run(Map<Id, Integer> firstById, Map<Id, Integer> secondById) { for (Integer index = 0; index < firstById.size(); index++) { Integer ignored = index; } for (Id cartId : firstById.keySet()) { Integer firstValue = firstById.get(cartId); if (firstValue != null) { System.debug(firstValue); } } Integer dmlOperations = Limits.getQueries(); for (Id cartId : secondById.keySet()) { Integer secondValue = secondById.get(cartId); if (secondValue != null) { System.debug(secondValue); } } System.debug(dmlOperations); } }`,
		Oracle:     "accept",
	},
	{
		ID:         "APEX-RELEASE-STATIC-METHOD-HIDING",
		Area:       "inheritance",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class ProbeStaticChild extends ProbeStaticBase { public static Integer size() { return 2; } }`,
		Dependencies: []scratchBackedSourceFile{{
			Path:    "force-app/main/default/classes/ProbeStaticBase.cls",
			Content: `public virtual class ProbeStaticBase { public static Integer size() { return 1; } }`,
		}},
		Oracle: "accept",
	},
	{
		ID:         "APEX-RELEASE-STATIC-GETTER-SELF-ASSIGNMENT",
		Area:       "properties",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { public static Probe Instance { get { if (Instance == null) { Instance = new Probe(); } return Instance; } } }`,
		Oracle:     "reject",
	},
	{
		ID:         "APEX-RELEASE-NESTED-INTERFACE-INSTANCEOF",
		Area:       "type-contracts",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { private class Implementation implements ProbeContract { public void calculate() { } } public static Boolean run() { ProbeContract calculator = new Implementation(); return calculator instanceof Implementation; } }`,
		Dependencies: []scratchBackedSourceFile{{
			Path:    "force-app/main/default/classes/ProbeContract.cls",
			Content: `public interface ProbeContract { void calculate(); }`,
		}},
		Oracle: "accept",
	},
	{
		ID:         "APEX-RELEASE-STANDARD-RELATIONSHIP-TRAVERSAL",
		Area:       "soql",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { public static void run() { List<Contact> rows = [SELECT Account.Name FROM Contact]; } }`,
		Oracle:     "accept",
	},
	{
		ID:         "APEX-RELEASE-STATIC-INHERITED-INSTANCE-NAME",
		Area:       "inheritance",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class ProbeStaticChild extends ProbeInstanceBase { private static Integer size() { return 2; } }`,
		Dependencies: []scratchBackedSourceFile{{
			Path:    "force-app/main/default/classes/ProbeInstanceBase.cls",
			Content: `public virtual class ProbeInstanceBase { public virtual Integer size() { return 1; } }`,
		}},
		Oracle: "accept",
	},
	{
		ID:         "APEX-RELEASE-INTERFACE-INSTANCEOF-IMPLEMENTATION",
		Area:       "type-contracts",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { public interface Contract { } public class Implementation implements Contract { } public static Boolean run(Contract value) { return value instanceof Implementation; } }`,
		Oracle:     "accept",
	},
	{
		ID:         "APEX-RELEASE-MERGE-ACCOUNT-ID-COLLECTION",
		Area:       "dml",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { public static void run(Id masterId, List<Id> duplicateIds) { merge new Account(Id = masterId) new List<Id>(duplicateIds); } }`,
		Oracle:     "accept",
	},
	{
		ID:         "APEX-RELEASE-FINALLY-RETURN-REACHABILITY",
		Area:       "statements",
		APIVersion: 67,
		SourceKind: "class",
		Source:     `public class Probe { public static String run() { try { return 'try'; } catch (Exception ex) { return 'catch'; } finally { System.debug('flush'); } return 'fallback'; } }`,
		Oracle:     "reject",
	},
}

func scratchBackedRuleAreaCount(cases []scratchBackedRuleCase, area string) int {
	count := 0
	for _, rule := range cases {
		if rule.Area == area {
			count++
		}
	}
	return count
}

func analyzeScratchBackedRuleCase(t *testing.T, rule scratchBackedRuleCase) Result {
	t.Helper()
	if rule.SourceKind != "class" {
		t.Fatalf("unsupported scratch-backed source kind %q", rule.SourceKind)
	}
	root := t.TempDir()
	mainPath := filepath.Join(root, "Probe.cls")
	writeSemaFile(t, mainPath, rule.Source)
	apexFiles := []string{mainPath}
	if len(rule.ProjectFiles) != 0 {
		t.Fatalf("scratch-backed product regression does not support project metadata")
	}
	for _, file := range rule.Dependencies {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, path, file.Content)
		apexFiles = append(apexFiles, path)
	}
	consumer := project.Project{
		Root:             root,
		SourceAPIVersion: fmt.Sprintf("%d.0", rule.APIVersion),
		ApexFiles:        apexFiles,
	}
	return Analyze(typesys.Build(consumer, schema.Schema{}))
}

// This is a product-owned snapshot of the scratch-backed catalog cases. Tests
// must not read glade-tools at runtime.
const scratchBackedRuleCasesJSON = `[
{"id":"APEX-AUDIT-AGGREGATE-FUNCTION-IN-WHERE","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeAggregateWhere { public void run() { List<Account> values = [SELECT Id FROM Account WHERE COUNT(Id) > 1]; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-AGGREGATE-NON-GROUPED-FIELD","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeAggregateUngrouped { public void run() { List<AggregateResult> values = [SELECT Name, COUNT(Id) total FROM Account]; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-AGGREGATE-QUERY-WRONG-TYPE","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeAggregateType { public void run() { List<Account> values = [SELECT COUNT(Id) total FROM Account]; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-ANYTYPE-PSEUDO-TYPE-PARAMETER","area":"types","apiVersion":66,"sourceKind":"class","source":"public class ProbeAnyTypeParam { public void run(AnyType value) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-BARE-RETURN-FROM-NONVOID","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeBareReturn { public Integer run() { return; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-ABSTRACT-IMPLEMENTATION-MISSING-OVERRIDE","area":"inheritance-contracts","apiVersion":67,"sourceKind":"class","source":"public class OracleAbstractChild extends OracleAbstractBase { public String value() { return 'value'; } }","dependencies":[{"path":"force-app/main/default/classes/OracleAbstractBase.cls","content":"public abstract class OracleAbstractBase { public abstract String value(); }"}],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-BATCHABLE-NARROWER-EXECUTE-SCOPE","area":"inheritance","apiVersion":66,"sourceKind":"class","source":"public class ProbeBatchNarrowScope implements Database.Batchable<SObject> { public Database.QueryLocator start(Database.BatchableContext context) { return Database.getQueryLocator([SELECT Id FROM Account]); } public void execute(Database.BatchableContext context, List<Account> scope) {} public void finish(Database.BatchableContext context) {} }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-CONSTRUCTOR-THIS-NOT-FIRST","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeCtorThisOrder { public ProbeCtorThisOrder() { Integer value = 1; this(value); } public ProbeCtorThisOrder(Integer value) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-DIAMOND-LIST-CONSTRUCTION","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeDiamondList { public void run() { List<String> values = new List<>(); } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-DUPLICATE-CATCH-TYPE","area":"statements","apiVersion":66,"sourceKind":"class","source":"public class ProbeDuplicateCatch { public void run() { try {} catch (DmlException first) {} catch (DmlException second) {} } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-FINAL-LOCAL-REASSIGNMENT","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeFinalAssignment { public void run() { final Integer value = 1; value = 2; } }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-FINAL-PROPERTY","area":"types","apiVersion":66,"sourceKind":"class","source":"public class ProbeFinalProperty { public final String Value { get; set; } }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-FOR-NON-BOOLEAN-CONDITION","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeForCondition { public void run() { for (Integer i = 0; 1; i++) {} } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-FUTURE-CALLS-FUTURE","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeFutureChain { @future public static void first() { second(); } @future public static void second() {} }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-FUTURE-METHOD-IN-BATCHABLE-CLASS","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeBatchFuture implements Database.Batchable<SObject> { public Database.QueryLocator start(Database.BatchableContext context) { return Database.getQueryLocator([SELECT Id FROM Account]); } public void execute(Database.BatchableContext context, List<SObject> scope) {} public void finish(Database.BatchableContext context) {} @future public static void later() {} }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-FUTURE-UNKNOWN-PROPERTY","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeFutureBadProperty { @future(definitelyNotApex=true) public static void run() {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-GETTER-WITHOUT-RETURN","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeGetterReturn { public String Value { get { String localValue = 'x'; } } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-GROUP-BY-NON-GROUPABLE-FIELD","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeGroupFieldType { public void run() { List<AggregateResult> values = [SELECT Description, COUNT(Id) total FROM Account GROUP BY Description]; } }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-IF-NON-BOOLEAN-CONDITION","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeIfCondition { public void run() { if (1) {} } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-INNER-AS-NESTED-TYPE-IDENTIFIER","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeInnerType { public class Inner {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-INVOCABLE-METHOD-INNER-CLASS","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeInvocableInner { public class Worker { @InvocableMethod public static void run(List<String> values) {} } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-INVOCABLE-METHOD-MAP-PARAMETER","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeInvokeMapParam { @InvocableMethod public static void run(Map<String, String> values) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-INVOCABLE-METHOD-MAP-RETURN","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeInvokeMapReturn { @InvocableMethod public static Map<String, String> run(List<String> values) { return new Map<String, String>(); } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-INVOCABLE-METHOD-TWO-PARAMETERS","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeInvocableParameters { @InvocableMethod public static void run(List<String> first, List<String> second) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-INVOCABLE-METHOD-UNKNOWN-PROPERTY","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeInvokeBadProperty { @InvocableMethod(definitelyNotApex='x') public static void run(List<String> values) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-INVOCABLE-VARIABLE-FINAL","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeInvokeFinalVar { @InvocableVariable public final String Value = 'x'; }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-INVOCABLE-VARIABLE-UNKNOWN-PROPERTY","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeInvokeVarBadProp { @InvocableVariable(definitelyNotApex=true) public String Value; }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-IS-TEST-CRITICAL-ON-METHOD","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@IsTest private class ProbeCriticalMethod { @IsTest(critical=true) static void run() {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-IS-TEST-ENUM","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@IsTest public enum ProbeTestEnum { First }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-IS-TEST-GLOBAL-CLASS","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@IsTest global class ProbeGlobalTest {}","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-IS-TEST-INTERFACE","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@IsTest public interface ProbeTestInterface {}","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-IS-TEST-NESTED-CLASS","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeNestedTest { @IsTest private class Worker {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-IS-TEST-PARALLEL-SEE-ALL-DATA","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@IsTest private class ProbeTestModes { @IsTest(SeeAllData=true IsParallel=true) static void run() {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-IS-TEST-TESTFOR-ON-METHOD","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@IsTest private class ProbeTestForMethod { @IsTest(testFor='ApexClass:ProbeTestForMethod') static void run() {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-IS-TEST-UNKNOWN-PROPERTY","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@IsTest(DefinitelyNotApex=true) private class ProbeTestBadProperty {}","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-ITERATOR-METHODS-WITHOUT-ACCESS","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeIteratorAccess implements Iterator<String> { Boolean hasNext() { return false; } String next() { return null; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-JSON-ACCESS-INVALID-VALUE","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@JsonAccess(serializable='sometimes') public class ProbeJsonBadValue {}","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-NAMESPACE-ACCESSIBLE-INTERFACE-MEMBER","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@NamespaceAccessible public interface ProbeNamespaceIface { @NamespaceAccessible void run(); }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-NAMESPACE-ACCESSIBLE-INVOCABLE","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeNamespaceInvoke { @InvocableMethod @NamespaceAccessible public static void run(List<String> values) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-OVERRIDE-METHOD-WITHOUT-ACCESS-V65","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeOverrideAccess extends ProbeOverrideBase { override void run() {} }","dependencies":[{"path":"force-app/main/default/classes/ProbeOverrideBase.cls","content":"public virtual class ProbeOverrideBase { public virtual void run() {} }"}],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-PRIVATE-INVOCABLE-METHOD","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbePrivateInvocable { @InvocableMethod private static void run(List<String> values) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-PRIVATE-INVOCABLE-VARIABLE","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbePrivateInvocableVar { @InvocableVariable private String Value; }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-PROPERTY-INVOCABLE-VARIABLE","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbePropertyInvocableVar { @InvocableVariable public String Value { get; set; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-PROPERTY-ON-INTERFACE","area":"types","apiVersion":66,"sourceKind":"class","source":"public interface ProbeInterfaceProperty { String Value { get; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-QUEUEABLE-EXECUTE-WITHOUT-ACCESS","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeQueueAccess implements Queueable { void execute(QueueableContext context) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-REST-BLOB-PARAMETER","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@RestResource(urlMapping='/pass2-blob-param/*') global class ProbeRestBlobParam { @HttpPost global static void run(Blob value) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-REST-MAPPING-MISSING","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@RestResource global class ProbeRestNoMapping { @HttpGet global static void run() {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-REST-NON-STRING-MAP-KEY","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@RestResource(urlMapping='/pass2-map-key/*') global class ProbeRestMapKey { @HttpPost global static void run(Map<Integer, String> values) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-REST-RESOURCE-INNER-CLASS","area":"annotations","apiVersion":66,"sourceKind":"class","source":"global class ProbeRestInner { @RestResource(urlMapping='/pass2-inner/*') global class Worker { @HttpGet global static void run() {} } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-REST-RESOURCE-UNKNOWN-PROPERTY","area":"annotations","apiVersion":66,"sourceKind":"class","source":"@RestResource(definitelyNotApex='/pass2-bad-property/*') global class ProbeRestBadProperty { @HttpGet global static void run() {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-RETURN-VALUE-FROM-VOID","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeVoidReturn { public void run() { return 1; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-SOQL-FOR-WRONG-VARIABLE-TYPE","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeSoqlForType { public void run() { for (Contact value : [SELECT Id FROM Account]) {} } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-SOQL-WRONG-ASSIGNMENT-TYPE","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeSoqlAssignment { public void run() { Contact value = [SELECT Id FROM Account LIMIT 1]; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-SOSL-DUPLICATE-RETURNING-OBJECT","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeSoslDuplicateObject { public void run() { List<List<SObject>> values = [FIND 'pass2' RETURNING Account(Id), Account(Name)]; } }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-SOSL-WITHOUT-RETURNING","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeSoslNoReturning { public void run() { List<List<SObject>> values = [FIND 'pass2']; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-SUPER-OUTSIDE-OVERRIDE","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeSuperContext extends ProbeSuperBase { public void other() { super.run(); } }","dependencies":[{"path":"force-app/main/default/classes/ProbeSuperBase.cls","content":"public virtual class ProbeSuperBase { public virtual void run() {} }"}],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-SWITCH-MULTIPLE-SOBJECT-TYPES-ONE-WHEN","area":"statements","apiVersion":66,"sourceKind":"class","source":"public class ProbeSwitchSObjects { public void run(SObject value) { switch on value { when Account accountValue, Contact contactValue {} when else {} } } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-TEST-SETUP-WITH-SEE-ALL-DATA","area":"declarations","apiVersion":66,"sourceKind":"class","source":"@IsTest(SeeAllData=true) private class ProbeSetupAllData { @TestSetup static void setup() {} @IsTest static void run() {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-TRANSIENT-LOCAL-VARIABLE","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeTransientLocal { public void run() { transient String value; } }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-TRANSIENT-STATIC-FIELD","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeTransientStatic { public transient static String Value; }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-TYPEOF-WITH-GROUP-BY","area":"query","apiVersion":66,"sourceKind":"class","source":"public class ProbeTypeofGroup { public void run() { List<AggregateResult> values = [SELECT TYPEOF What WHEN Account THEN Name END, COUNT(Id) total FROM Event GROUP BY What]; } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-UNINITIALIZED-LOCAL-READ","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeUninitializedLocal { public Integer run() { Integer value; return value; } }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-UNKNOWN-METHOD-ANNOTATION","area":"annotations","apiVersion":66,"sourceKind":"class","source":"public class ProbeUnknownMethodAnno { @DefinitelyNotApex public void run() {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-UNREACHABLE-AFTER-THROW-API40-CONTROL","area":"statements","apiVersion":40,"sourceKind":"class","source":"public class ProbeUnreachable40 { public void run() { throw new NullPointerException(); Integer value = 1; } }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-UNREACHABLE-AFTER-THROW-API41","area":"statements","apiVersion":41,"sourceKind":"class","source":"public class ProbeUnreachable41 { public void run() { throw new NullPointerException(); Integer value = 1; } }","dependencies":[],"projectFiles":[],"oracle":"accept"},
{"id":"APEX-AUDIT-WEBSERVICE-INTERFACE-METHOD","area":"annotations","apiVersion":66,"sourceKind":"class","source":"global interface ProbeWebInterface { webservice void run(); }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-WEBSERVICE-MAP-PARAMETER","area":"annotations","apiVersion":66,"sourceKind":"class","source":"global class ProbeWebMap { webservice static void run(Map<String, String> values) {} }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-WEBSERVICE-SET-RETURN","area":"annotations","apiVersion":66,"sourceKind":"class","source":"global class ProbeWebSet { webservice static Set<String> run() { return new Set<String>(); } }","dependencies":[],"projectFiles":[],"oracle":"reject"},
{"id":"APEX-AUDIT-WHILE-NON-BOOLEAN-CONDITION","area":"declarations","apiVersion":66,"sourceKind":"class","source":"public class ProbeWhileCondition { public void run() { while (1) {} } }","dependencies":[],"projectFiles":[],"oracle":"reject"}
]`

func TestScratchBackedRuleCatalogHasExpectedSize(t *testing.T) {
	var cases []scratchBackedRuleCase
	if err := json.Unmarshal([]byte(scratchBackedRuleCasesJSON), &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 69 {
		t.Fatalf("scratch-backed rule cases = %d, want 69", len(cases))
	}
}
