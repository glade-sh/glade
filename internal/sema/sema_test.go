package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/typesys"
)

func TestAnalyzeResolvesMemberTypes(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "List<Thing__c>"},
				},
			},
		},
		Objects: []schema.Object{{Name: "Thing__c"}},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeResolvesNamespaceQualifiedSchemaAliases(t *testing.T) {
	index := typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "List<pkg__Thing__c>"},
				},
			},
		},
		Triggers: []typesys.TriggerSymbol{{Name: "ThingTrigger", ObjectName: "pkg__Thing__c", File: "Thing.trigger"}},
		Objects: []schema.Object{{
			Name: "Thing__c",
			Fields: []schema.Field{
				{Name: "Parent__c", Type: "Lookup", ReferenceTo: []string{"pkg__Thing__c"}},
			},
		}},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUnknownMemberType(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "MissingType", Range: diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}}},
				},
			},
		},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "OAERSEMA002" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}
}

func TestAnalyzeMethodParameterTypes(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationMethod,
						Name: "run",
						Parameters: []apexast.Parameter{
							{Name: "accounts", Type: "List<Account>"},
							{Name: "missing", Type: "MissingType", Range: diagnostic.Range{Start: diagnostic.Position{Line: 2, Column: 20}}},
						},
					},
				},
			},
		},
		Objects: []schema.Object{{Name: "Account"}},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "OAERSEMA004" {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}

func TestAnalyzeRecognizesCallableAndStubProviderTypes(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Provider",
				File:       "Provider.cls",
				Interfaces: []string{"System.StubProvider"},
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationMethod,
						Name: "handleMethodCall",
						Type: "Object",
						Parameters: []apexast.Parameter{
							{Name: "stubbedObject", Type: "Object"},
							{Name: "stubbedMethodName", Type: "String"},
							{Name: "returnType", Type: "Type"},
							{Name: "listOfParamTypes", Type: "List<Type>"},
							{Name: "listOfParamNames", Type: "List<String>"},
							{Name: "listOfArgs", Type: "List<Object>"},
						},
					},
				},
			},
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Action",
				File:       "Action.cls",
				Interfaces: []string{"System.Callable"},
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationMethod,
						Name: "call",
						Type: "Object",
						Parameters: []apexast.Parameter{
							{Name: "action", Type: "String"},
							{Name: "args", Type: "Map<String, Object>"},
						},
					},
				},
			},
		},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProjectNamespaceQualifiedTypes(t *testing.T) {
	index := typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Helper",
				File: "Helper.cls",
			},
			{
				Kind: apexast.DeclarationClass,
				Name: "UsesHelper",
				File: "UsesHelper.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "pkg.Helper"},
					{
						Kind: apexast.DeclarationMethod,
						Name: "withParam",
						Parameters: []apexast.Parameter{
							{Name: "helper", Type: "pkg.Helper"},
						},
					},
				},
			},
		},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedTypeReferences(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public class Inner {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesInner.cls"), `
public class UsesInner {
  public Outer.Inner build() {
    Outer.Inner value = new Outer.Inner();
    return value;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Outer.cls"),
		filepath.Join(root, "UsesInner.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaSoapTypeAliases(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "UsesSoapType",
				File: "UsesSoapType.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationField, Name: "byAlias", Type: "Map<SoapType, Type>"},
					{Kind: apexast.DeclarationField, Name: "byQualifiedName", Type: "Map<Schema.SoapType, Type>"},
					{
						Kind: apexast.DeclarationMethod,
						Name: "accept",
						Parameters: []apexast.Parameter{
							{Name: "fieldType", Type: "SoapType"},
							{Name: "qualifiedFieldType", Type: "Schema.SoapType"},
						},
					},
				},
			},
		},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedTypeRelativeReferencesInsideOwner(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public interface Named {
    String name();
  }
  public class Inner {
    public Inner(Integer value) {}
  }
  public class NamedImpl implements Named {
    public String name() {
      return 'named';
    }
  }
  public static Inner build(Integer value) {
    Inner made = new Inner(value);
    return made;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Outer.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedStaticPropertyNestedFieldCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public static Factory Test { get; private set; }
  public class Factory {
    public MockDatabase Database = new MockDatabase();
  }
  public class MockDatabase {
    public void onUpdate(List<SObject> records, Map<Id, SObject> oldRecords) {}
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesOuter.cls"), `
public class UsesOuter {
  public void run(Opportunity opp) {
    Outer.Test.Database.onUpdate(new List<Opportunity> { opp }, new Map<Id, Opportunity> { opp.Id => opp });
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Outer.cls"),
		filepath.Join(root, "UsesOuter.cls"),
	}}, schema.Schema{
		Objects: []schema.Object{{Name: "Opportunity"}},
	})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestCallArgumentsAtKeepsMapInitializerFatArrowInCall(t *testing.T) {
	body := "Outer.Test.Database.onUpdate(new Opportunity[] { newOpp }, new Map<Id, SObject> { newOpp.Id => oldOpp } );\nSystem.assertEquals(true, Outer.Test.Database.hasRecords());"
	calleeEnd := strings.Index(body, "(")
	if calleeEnd < 0 {
		t.Fatal("missing call")
	}

	args, ok := callArgumentsAt(body, calleeEnd)
	if !ok {
		t.Fatal("expected arguments")
	}
	if len(args) != 2 {
		t.Fatalf("args = %#v", args)
	}
	if strings.Contains(args[1].text, "assertEquals") {
		t.Fatalf("second argument consumed following call: %q", args[1].text)
	}
}

func TestCallArgumentsAtKeepsNestedFormatAndListInitializer(t *testing.T) {
	body := `keyValuePairs.add(String.format('{0}{1}{2}', new List<String> {
                key,
                PARAM_SPLITTER,
                get(key)
            }));`
	calleeEnd := strings.Index(body, "(")
	if calleeEnd < 0 {
		t.Fatal("missing call")
	}

	args, ok := callArgumentsAt(body, calleeEnd)
	if !ok {
		t.Fatal("expected arguments")
	}
	if len(args) != 1 {
		t.Fatalf("args = %#v", args)
	}
	if !strings.Contains(args[0].text, "String.format") || !strings.Contains(args[0].text, "get(key)") {
		t.Fatalf("argument = %q", args[0].text)
	}
}

func TestAnalyzeDatetimeFormatCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatetime.cls"), `
public class UsesDatetime {
  public String run() {
    return System.now().format('yyyyMMdd');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDatetime.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeImplicitMethodArgumentSelectsSpecificOverload(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DescribeHelper.cls"), `
public class DescribeHelper {
  public static DescribeHelper getDescribe(String name) { return null; }
  public static DescribeHelper getDescribe(Schema.SObjectType token) { return null; }
  public static DescribeHelper getDescribe(SObject record) { return null; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Selector.cls"), `
public abstract class Selector {
  public abstract Schema.SObjectType getSObjectType();
  public DescribeHelper get() {
    return DescribeHelper.getDescribe(getSObjectType());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "DescribeHelper.cls"),
		filepath.Join(root, "Selector.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeThisFieldAccessIgnoresShadowingParameter(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ShadowedField.cls"), `
public class ShadowedField {
  private Set<String> fields;
  public String path(Schema.SObjectField token) {
    return 'Name';
  }
  public void selectFields(List<Schema.SObjectField> fields) {
    for (Schema.SObjectField token : fields) {
      this.fields.add(path(token));
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "ShadowedField.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInheritedOverloadRemainsVisible(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BaseBuilder.cls"), `
public virtual class BaseBuilder {
  public virtual String getStringValue() {
    return '';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ChildBuilder.cls"), `
public class ChildBuilder extends BaseBuilder {
  public String getStringValue(String prefix) {
    return prefix;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesChildBuilder.cls"), `
public class UsesChildBuilder {
  public String run(ChildBuilder builder) {
    return builder.getStringValue();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "BaseBuilder.cls"),
		filepath.Join(root, "ChildBuilder.cls"),
		filepath.Join(root, "UsesChildBuilder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUppercaseBooleanLiteral(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesBoolean.cls"), `
public class UsesBoolean {
  public Boolean run() {
    return TRUE;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesBoolean.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseSavepoint(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSavepoint.cls"), `
public class UsesSavepoint {
  public void run() {
    Savepoint sp = Database.setSavepoint();
    Database.rollback(sp);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSavepoint.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemSavepointAlias(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSystemSavepoint.cls"), `
public class UsesSystemSavepoint {
  public void run() {
    System.Savepoint sp = Database.setSavepoint();
    Database.rollback(sp);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSystemSavepoint.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectIdFieldPath(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectPath.cls"), `
public class UsesSObjectPath {
  public SObject relatedTo;
  public Id run() {
    return this.RelatedTo.Id;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSObjectPath.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSingleEmailMessageSetWhatId(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSingleEmail.cls"), `
public class UsesSingleEmail {
  public Messaging.SingleEmailMessage email;
  public SObject relatedTo;
  public void run() {
    this.email.setWhatId(this.relatedTo.Id);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSingleEmail.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSingleEmailMessageAssignableToMessagingEmail(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMessagingEmail.cls"), `
public class UsesMessagingEmail {
  public void registerEmail(Messaging.Email email) {}
  public void run() {
    Messaging.SingleEmailMessage email = new Messaging.SingleEmailMessage();
    registerEmail(email);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesMessagingEmail.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCommonSalesSObjectTypes(t *testing.T) {
	index := typesys.Index{Types: []typesys.TypeSymbol{{
		Kind: apexast.DeclarationClass,
		Name: "UsesSalesObjects",
		File: "UsesSalesObjects.cls",
		Members: []typesys.MemberSymbol{{
			Kind: apexast.DeclarationMethod,
			Name: "run",
			Parameters: []apexast.Parameter{
				{Name: "pbe", Type: "PricebookEntry"},
				{Name: "line", Type: "OpportunityLineItem"},
			},
		}},
	}}}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSalesSObjectAssignableToSObject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSalesRelationship.cls"), `
public class UsesSalesRelationship {
  public void registerRelationship(SObject record, Schema.SObjectField field, SObject parent) {}
  public void run(OpportunityLineItem line, PricebookEntry entry) {
    registerRelationship(line, OpportunityLineItem.PricebookEntryId, entry);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSalesRelationship.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedCallArgumentSelectsSObjectFieldOverload(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DescribeWrapper.cls"), `
public class DescribeWrapper {
  public static DescribeWrapper getDescribe(SObjectType objType) {
    return null;
  }
  public Schema.SObjectField getField(String name) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Security.cls"), `
public class Security {
  public static void checkFieldIsInsertable(SObjectType objType, String fieldName) {
    checkFieldIsInsertable(objType, DescribeWrapper.getDescribe(objType).getField(fieldName));
  }
  public static void checkFieldIsInsertable(SObjectType objType, SObjectField fieldToken) {}
  public static void checkFieldIsInsertable(SObjectType objType, DescribeFieldResult fieldDescribe) {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "DescribeWrapper.cls"),
		filepath.Join(root, "Security.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStringValueOfSelectsStringOverload(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StringValueBuilder.cls"), `
public class StringValueBuilder {
  public void add(List<String> values) {}
  public void add(String value) {}
  public void run(Schema.SObjectField fieldToken) {
    add(String.valueOf(fieldToken));
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StringValueBuilder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeFieldSetMemberPathSelectsStringOverload(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "FieldSetBuilder.cls"), `
public class FieldSetBuilder {
  public void add(List<String> values) {}
  public void add(String value) {}
  public void run(Schema.FieldSetMember member) {
    add(member.getFieldPath());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "FieldSetBuilder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeVisibilityBaseline(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind:      apexast.DeclarationClass,
				Name:      "Both",
				File:      "Both.cls",
				Modifiers: []string{"public", "global"},
			},
			{
				Kind: apexast.DeclarationInterface,
				Name: "IWorker",
				File: "IWorker.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "hidden", Modifiers: []string{"private"}},
				},
			},
		},
	}

	result := Analyze(index)
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	for _, diag := range result.Diagnostics {
		if diag.Code != "OAERSEMA005" {
			t.Fatalf("diagnostic = %#v", diag)
		}
	}
}

func TestAnalyzeMethodBodyBaseline(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void work() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  Integer field;
  public void run(String input) {
    Integer count = 1;
    Helper h = new Helper();
    h.work();
    field = count;
  }
  public void callRun() {
    run('x');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodBodyDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    MissingType item;
    MissingCtor built = new MissingCtor();
    missingValue = 1;
    missingCall();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	codes := map[string]bool{}
	for _, diag := range result.Diagnostics {
		codes[diag.Code] = true
	}
	for _, code := range []string{"OAERSEMA006", "OAERSEMA013", "OAERSEMA008"} {
		if !codes[code] {
			t.Fatalf("missing %s in diagnostics: %#v", code, result.Diagnostics)
		}
	}
}

func TestAnalyzeNonConstructableTypeDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `public abstract class Base {}`)
	writeSemaFile(t, filepath.Join(root, "IThing.cls"), `public interface IThing {}`)
	writeSemaFile(t, filepath.Join(root, "Mood.cls"), `public enum Mood { Happy }`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void run() {
    Base base = new Base();
    IThing thing = new IThing();
    Mood mood = new Mood();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "IThing.cls"),
		filepath.Join(root, "Mood.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA015" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("OAERSEMA015 count = %d, diagnostics = %#v", count, result.Diagnostics)
	}
}

func TestAnalyzeCustomExceptionUsesInheritedConstructors(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "NUException.cls"), `public class NUException extends Exception {}`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void run(Exception cause) {
    NUException empty = new NUException();
    NUException message = new NUException('blocked');
    NUException nested = new NUException(cause);
    throw new NUException('blocked', cause);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "NUException.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMultilineReturnExpression(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  private static final String TEMPLATE = '{0} {1} {2}';

  private static String build(String one, String two, String three) {
    return String.format(TEMPLATE, new List<String> {
      one, two, three
    });
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeIgnoresReturnWordInComments(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static void run(Boolean done) {
    // If done, return early.
    if (done) {
      return;
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeIgnoresCallShapeInsideStringLiteral(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static void run() {
    String message = 'failure (///_-) dude';
    String html = '<div><img src="notmyproblem.jpeg">Hello</div>';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" || diag.Code == "OAERSEMA006" {
			t.Fatalf("unexpected string-literal diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCatchVariableVisibleInCatchBlock(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    try {
      throw new DmlException('blocked');
    } catch (DmlException | QueryException e) {
      System.debug(e.getMessage());
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" {
			t.Fatalf("unexpected catch-variable diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeEnumValuesAsStaticFields(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DataRequest.cls"), `
public class DataRequest {
  public enum SharingMode {
    WithSharing,
    WithoutSharing
  }

  public SharingMode SharingModeToUse {
    get {
      if (this.SharingModeToUse == null) {
        this.SharingModeToUse = SharingMode.WithSharing;
      }
      return this.SharingModeToUse;
    }
    private set;
  }

  public static DataRequest.SharingMode qualifiedEnum() {
    return DataRequest.SharingMode.WithoutSharing;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "DataRequest.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedCollectionCallOnImplicitMethodResult(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static List<String> supported() {
    return new List<String>{ 'a' };
  }

  public static Boolean run(String key) {
    return !supported().contains(key);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTypeLiteralToStringCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static String run() {
    return Hello.class.toString();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeListBracketIndexUsesListReceiver(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  private static final List<String> KEYS = new List<String>{ 'a' };

  public static String run() {
    return KEYS[0];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCollectionCallOnFieldPath(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Request.cls"), `
public class Request {
  public List<String> Values { get; private set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static Boolean run(Request request) {
    return request.Values.isEmpty();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Request.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallWithListInitializerArgument(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static void accepts(String key, List<String> values) {}

  public static void run() {
    accepts('key', new List<String> { 'one', 'two' });
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMultilineMapInitializerLocal(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static void run() {
    Map<String, Object> values = new Map<String, Object> {
      'someString' => 'someValue',
      'someMap' => new Map<String, Object> {
        'someChild' => 'someChildValue'
      }
    };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAssignmentExpressionInCallArgument(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AssignInArg.cls"), `
public class AssignInArg {
  public static void putList(Map<String, List<String>> values) {
    List<String> items;
    values.put('items', items = new List<String>());
    items.add('x');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "AssignInArg.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStringReplaceAllCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static String run(String input) {
    return input.replaceAll('[^a-z]', '');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStringSplitCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static String run(String input) {
    List<String> parts = input.split('-', 2);
    return parts[0];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeBooleanExpressionWithStringCalls(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static Boolean run(String classPrefix) {
    Boolean isValid = ''.equals(classPrefix) || (classPrefix.length() > 2 && classPrefix.endsWith('.'));
    return isValid;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMetadataRetrieveCustomMetadataReturn(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static Integer run() {
    List<Metadata.CustomMetadata> records = Metadata.Operations.retrieve(
      Metadata.MetadataType.CustomMetadata,
      new List<String>{'Feature.Default'}
    );
    return records.size();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectDynamicRelationshipCalls(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static Object run(Object value, String property) {
    SObject parent = ((SObject)value).getSObject(property);
    List<SObject> children = parent.getSObjects('Contacts');
    return children.isEmpty() ? parent.get('Name') : children[0].get('Name');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCastNullSelectsOverload(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Parameters.cls"), `
public class Parameters {
  public static Parameters newInstance(String paramsString) {
    return new Parameters();
  }
  public static Parameters newInstance(Map<String, String> paramMap) {
    return new Parameters();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static Parameters run() {
    return Parameters.newInstance((String)null);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Parameters.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedInstanceFieldPath(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DataRequest.cls"), `
public class DataRequest {
  public String Query { get; private set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "SObjectData.cls"), `
public class SObjectData {
  private DataRequest request;
  public String run() {
    return this.request.Query;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "DataRequest.cls"),
		filepath.Join(root, "SObjectData.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeFluentChainAcrossNewlinesWithEqualsInArgument(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DataRequest.cls"), `
public class DataRequest {
  public enum SharingMode { WithSharing, WithoutSharing }
  public static DataRequest newInstance() { return new DataRequest(); }
  public DataRequest setQuery(String queryString) { return this; }
  public DataRequest setParam1(List<String> param1s) { return this; }
  public DataRequest withSharingMode(SharingMode sharingModeToSet) { return this; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static DataRequest run(String idValue) {
    DataRequest request = DataRequest.newInstance()
      .setQuery('SELECT Id FROM Account WHERE Id = :param1')
      .setParam1(new List<String>{ idValue })
      .withSharingMode(DataRequest.SharingMode.WithoutSharing);
    return request;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "DataRequest.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeRecordTypeInfoMethods(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static Id run(Map<String, Schema.RecordTypeInfo> recordTypes) {
    Schema.RecordTypeInfo info = recordTypes.get('Business');
    Boolean active = info.isActive();
    return active ? info.getRecordTypeId() : null;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStringArgumentMatchesIdParameter(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  private static Account byId(List<Account> accounts, Id recordId) {
    return accounts.isEmpty() ? null : accounts[0];
  }
  public static Account run(List<Account> accounts, String recordId) {
    return byId(accounts, recordId);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeIdValuesFitStringInitializer(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static List<String> run(Account account) {
    return new List<String>{ account.Name, account.Id };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedCustomSObjectSystemFieldPath(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SubtemplateWrapper.cls"), `
public class SubtemplateWrapper {
  public Account TemplateRecord { get; private set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static Id run(SubtemplateWrapper wrapper) {
    return wrapper.TemplateRecord.Id;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "SubtemplateWrapper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeRelationshipSObjectMatchesConcreteSObjectParameter(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Template.cls"), `
public class Template {
  private Account record;
  public Template ParentTemplate {
    get {
      return Template.newInstance(this.record.ParentTemplate__r);
    }
  }
  public static Template newInstance(Account templateRecord) {
    return new Template();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Template.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeEnumNameCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "TokenType.cls"), `
public enum TokenType {
  Add,
  Subtract
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static String run(TokenType op) {
    return op.name();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "TokenType.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStringChainFromThisField(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  private Dom.Document htmlDocumentInstance;
  private String SourceHtml;

  public String getHtml() {
    return this.htmlDocumentInstance.toXmlString()
      .substringAfter('<html>')
      .substringBeforeLast('</html>')
      .replace('{amp}', '&');
  }

  public String clean() {
    return this.SourceHtml.trim().toLowerCase().unescapeHtml4();
  }

  public Dom.XmlNode root() {
    this.htmlDocumentInstance.load('<html></html>');
    String htmlDocumentBody = String.format('<html>{0}</html>',
      new List<String> { clean() });
    return this.htmlDocumentInstance.getRootElement();
  }

  public void mutate(Dom.XmlNode node) {
    node.removeAttribute('href', null);
    node.setAttributeNs('href', 'x', null, null);
    node.addTextNode('text');
    node.addCommentNode('comment');
    node.addChildElement('span', null, null);
    node.removeChild(node);
    node.insertBefore(node, node);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeObjectAssignabilityUsesInheritanceAndInterfaces(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `public virtual class Base {}`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `public class Child extends Base implements Marker {}`)
	writeSemaFile(t, filepath.Join(root, "Marker.cls"), `public interface Marker {}`)
	writeSemaFile(t, filepath.Join(root, "Other.cls"), `public class Other {}`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void acceptsBase(Base base) {}
  public void acceptsMarker(Marker marker) {}
  public Base returnsBase() {
    return new Child();
  }
  public void run() {
    Base base = new Child();
    Marker marker = new Child();
    acceptsBase(new Child());
    acceptsMarker(new Child());
    base = new Other();
    acceptsBase(new Other());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
		filepath.Join(root, "Marker.cls"),
		filepath.Join(root, "Other.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	assignabilityDiagnostics := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" || diag.Code == "OAERSEMA009" {
			assignabilityDiagnostics++
		}
	}
	if assignabilityDiagnostics != 2 {
		t.Fatalf("expected two object assignability diagnostics, got %d: %#v", assignabilityDiagnostics, result.Diagnostics)
	}
}

func TestAnalyzeResolvesShortNestedNewExpressionsForInterfaceOverloads(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Matcher.cls"), `public interface Matcher {}`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public static void accept(Matcher matcher) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "FirstTest.cls"), `
public class FirstTest {
  private class LocalMatcher implements Matcher {}
  public void run() {
    Uses.accept(new LocalMatcher());
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "SecondTest.cls"), `
public class SecondTest {
  private class LocalMatcher implements Matcher {}
  public void run() {
    Uses.accept(new LocalMatcher());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Matcher.cls"),
		filepath.Join(root, "Uses.cls"),
		filepath.Join(root, "FirstTest.cls"),
		filepath.Join(root, "SecondTest.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsThisAsCallArgument(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StubProvider.cls"), `public interface StubProvider {}`)
	writeSemaFile(t, filepath.Join(root, "Test.cls"), `
public class Test {
  public static Object createStub(Type target, StubProvider provider) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Provider.cls"), `
public class Provider implements StubProvider {
  public Object mock(Type classToMock) {
    return Test.createStub(classToMock, this);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StubProvider.cls"),
		filepath.Join(root, "Test.cls"),
		filepath.Join(root, "Provider.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUsesThisMethodReturnTypeForOverloadResolution(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Mode.cls"), `public class Mode {}`)
	writeSemaFile(t, filepath.Join(root, "Verifier.cls"), `
public class Verifier {
  public Mode times(Integer count) {
    return new Mode();
  }
  public Object verify(Object target, Mode mode) {
    return target;
  }
  public Object verify(Object target, Integer count) {
    return target;
  }
  public Object run(Object target) {
    return verify(target, this.times(1));
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Mode.cls"),
		filepath.Join(root, "Verifier.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInfersArrayNewExpressionForOverloadResolution(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void add(String value) {}
  public void add(String[] values) {}
  public void run() {
    Uses target = new Uses();
    add('one');
    add(new String[] {'one'});
    target.add((String[])null);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSkipsThrowStatementAsCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Object run(Object value) {
    if (value instanceof Exception) {
      throw ((Exception) value);
    }
    return value;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeJSONParserMethodsOnThisField(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  private JSONParser parser;
  public Uses(JSONParser parser) {
    this.parser = parser;
    this.parser.nextToken();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedClassCanCallOuterPrivateHelper(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  private static void helper(String value) {}
  private class Inner {
    public void run() {
      helper('x');
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Outer.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectListGetSObjectType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Schema.SObjectType run(List<SObject> records) {
    return records.getSObjectType();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeIdGetSObjectTypeFromIndexedList(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Schema.SObjectType run(Set<Id> ids) {
    SObjectType domainSObjectType = new List<Id>(ids)[0].getSObjectType();
    return domainSObjectType;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCollectionCallOnGenericConstructorReceiver(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Set<Id> run(List<Account> accounts) {
    Set<Id> accountIds = new Map<Id, Account>(accounts).keySet();
    return accountIds;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeShortNestedTypeAssignmentFromConstructor(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public void run() {
    Inner value = new Inner();
  }
  private class Inner {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Outer.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeShortNestedLocalTypeCarriesInterfaces(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Matcher.cls"), `public interface Matcher {}`)
	writeSemaFile(t, filepath.Join(root, "Accepts.cls"), `
public class Accepts {
  public static void matches(Matcher matcher) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public void run() {
    Inner value = new Inner();
    Accepts.matches(value);
  }
  public class Inner implements Matcher {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Matcher.cls"),
		filepath.Join(root, "Accepts.cls"),
		filepath.Join(root, "Outer.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeGenericClassLiteralIsTypeReference(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void run() {
    Type listType = List<String>.class;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsDateArgumentForDatetimeParameter(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Uses(Datetime value, Boolean inclusive) {}
  public static void run(Date value) {
    new Uses(value, false);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsDoubleArgumentForDecimalParameter(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Uses(Decimal lower, Boolean inclusiveLower, Decimal upper, Boolean inclusiveUpper) {}
  public static void run(Double lower, Double upper) {
    new Uses(lower, false, upper, false);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCustomExceptionKeepsInheritedConstructors(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public class MyException extends Exception {
    public MyException(String value, Integer code) {}
  }
  public void run() {
    throw new MyException();
    throw new MyException('bad');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePlatformAliasOverloadScoring(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public static SObjectField accept(SObjectField field) {
    return field;
  }
  public static void run(Schema.SObjectField field) {
    accept(field);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectTypeTokenOverloadScoring(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Uses(Schema.SObjectType table) {}
  private Uses(Schema.ChildRelationship relationship) {}
  public static void run() {
    new Uses(Contact.SObjectType);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectTypeTokenNewSObject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public SObject run() {
    return TemplateSettings__c.SObjectType.newSObject();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectFieldTokenOverloadScoring(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void selectField(String fieldName) {}
  public void selectField(Schema.SObjectField field) {}
  public void run() {
    selectField('Name');
    selectField(Schema.Contact.SObjectType.fields.lastName);
    selectField(Contact.Email);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCustomDataStaticMethodsAreCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Feature__mdt one() {
    Feature__mdt cfg = feature__MDT.GETINSTANCE('Default');
    return cfg;
  }
  public Map<String,Feature__mdt> allRows() {
    return FEATURE__mdt.getALL();
  }
  public Feature__mdt valuesRow() {
    return FEATURE__mdt.getValues('Default');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "Feature__mdt"}},
	})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeFieldSetGetFieldsSignature(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public List<Schema.FieldSetMember> run(Object value) {
    return ((FieldSet)value).getFields();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInstanceOfGenericTypeStopsAtBooleanOperator(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Boolean run(Object arg) {
    return arg != null
      && arg instanceof List<Object>
      && ((List<Object>)arg).isEmpty();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTernaryBranchCastReceiverCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  private String expected;
  public Boolean run(Object arg) {
    return arg != null && arg instanceof String ? ((String)arg).contains(expected) : false;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCastedReceiverFieldCollectionCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  private Set<String> fields = new Set<String>();
  public Boolean run(Object obj) {
    return ((Uses)obj).fields.size() == this.fields.size();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeConstructsGroupStandardObject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  private static List<Group> groups = new List<Group>{
    new Group(Name = 'Queue', DeveloperName = 'Queue', Type = 'Queue')
  };
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMapLiteralArrowDoesNotLookLikeAssignment(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Map<Schema.SObjectField, Object> run(Schema.SObjectField idField, Account record) {
    return new Map<Schema.SObjectField, Object>{
      idField => record.Id
    };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeIteratorGenericMethods(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public String run(String fieldName) {
    Iterator<String> i = fieldName.split('\\.').iterator();
    while (i.hasNext()) {
      return i.next();
    }
    return null;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatetimeGetTime(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Long run() {
    String label = 'prefix' + System.now().getTime();
    return System.now().getTime();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Uses.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInfersKnownMethodCallReturnTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Factory.cls"), `
public class Factory {
  public Product make() {
    return new Product();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Product.cls"), `
public class Product {
  public String label() {
    return 'ready';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Product direct() {
    return new Factory().make();
  }
  public String chained() {
    return new Factory().make().label();
  }
  public void run() {
    Product product = new Factory().make();
    String label = product.label();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Factory.cls"),
		filepath.Join(root, "Product.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodBodyDiagnosticRangesPointAtTokens(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    MissingType item;
    missingValue = 1;
    missingCall();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	positions := map[string]int{}
	for _, diag := range result.Diagnostics {
		if diag.Range != nil {
			positions[diag.Code] = diag.Range.Start.Line
		}
	}
	if positions["OAERSEMA006"] != 4 {
		t.Fatalf("OAERSEMA006 line = %d diagnostics=%#v", positions["OAERSEMA006"], result.Diagnostics)
	}
	if positions["OAERSEMA013"] != 5 {
		t.Fatalf("OAERSEMA013 line = %d diagnostics=%#v", positions["OAERSEMA013"], result.Diagnostics)
	}
	if positions["OAERSEMA008"] != 6 {
		t.Fatalf("OAERSEMA008 line = %d diagnostics=%#v", positions["OAERSEMA008"], result.Diagnostics)
	}
}

func TestAnalyzeMethodBodyScopeDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void useIt(Integer value) {}
  public void run(Boolean flag) {
    Integer local = 1;
    Integer local = 2;
    useIt(missingArg);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	codes := map[string]bool{}
	for _, diag := range result.Diagnostics {
		codes[diag.Code] = true
	}
	for _, code := range []string{"OAERSEMA014", "OAERSEMA013"} {
		if !codes[code] {
			t.Fatalf("missing %s in diagnostics: %#v", code, result.Diagnostics)
		}
	}
}

func TestAnalyzeIRBodyDiagnosticsForUnknownVariableReads(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer run(Boolean flag) {
    Integer total = 1;
    if (missingFlag) {
      total = total + missingAmount;
    }
    return missingReturn;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	unknowns := map[string]bool{}
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" {
			unknowns[diag.Message] = true
		}
	}
	for _, name := range []string{"missingFlag", "missingAmount", "missingReturn"} {
		found := false
		for message := range unknowns {
			if strings.Contains(message, name) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing unknown read for %s: %#v", name, result.Diagnostics)
		}
	}
}

func TestAnalyzeIRBodyScopesNestedLocals(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Boolean flag) {
    if (flag) {
      Integer inside = 1;
      System.debug(inside);
    }
    System.debug(inside);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" && strings.Contains(diag.Message, "inside") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one out-of-scope diagnostic for inside, got %d: %#v", count, result.Diagnostics)
	}
}

func TestAnalyzeIRBodyConditionTypeDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Integer count, String label) {
    if (count) {
      System.debug(label);
    }
    while (label) {
      break;
    }
    for (Integer i = 0; label; i = i + 1) {
      break;
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA020" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("OAERSEMA020 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeIRBodyTypeChecksNestedAssignmentsAndReturns(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer run(Boolean flag) {
    if (flag) {
      Integer count = 'bad';
      count = 'also bad';
      return 'still bad';
    }
    return 1;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	codes := map[string]int{}
	for _, diag := range result.Diagnostics {
		codes[diag.Code]++
	}
	if codes["OAERSEMA018"] == 0 || codes["OAERSEMA019"] == 0 {
		t.Fatalf("missing IR type diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeIRBodyFieldShapeDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public class Base {
  public String inherited;
}
`)
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper extends Base {
  public String value;
  public void run(Helper other) {
    String ok = other.value;
    String alsoOk = other.inherited;
    String missing = other.nope;
    other.nope = 'x';
    String thisMissing = this.nope;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Helper.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA021" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("OAERSEMA021 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeIRBodyMethodCalls(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void use(Integer value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void localUse(String value) {}
  public void run(Boolean flag) {
    if (flag) {
      Helper helper = new Helper();
      helper.use('bad');
      helper.missing();
      localUse(1);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	counts := map[string]int{}
	for _, diag := range result.Diagnostics {
		counts[diag.Code]++
	}
	if counts["OAERSEMA008"] == 0 || counts["OAERSEMA009"] == 0 {
		t.Fatalf("missing IR call diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSearchAndSOSLUnsupportedDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Search.find('FIND {Acme} IN ALL FIELDS RETURNING Account(Id)');
    Object rows = [FIND 'Acme' IN ALL FIELDS RETURNING Account(Id)];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA028" && strings.Contains(diag.Message, "unsupported local") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("OAERSEMA028 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeIRBodyConstructorCalls(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public Helper(Integer value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "AbstractThing.cls"), `public abstract class AbstractThing {}`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper ok = new Helper(1);
    Helper wrongArgs = new Helper('bad');
    MissingType missing = new MissingType();
    AbstractThing abstractThing = new AbstractThing();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "AbstractThing.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	codes := map[string]int{}
	for _, diag := range result.Diagnostics {
		codes[diag.Code]++
	}
	for _, code := range []string{"OAERSEMA006", "OAERSEMA011", "OAERSEMA015"} {
		if codes[code] == 0 {
			t.Fatalf("missing %s diagnostics=%#v", code, result.Diagnostics)
		}
	}
}

func TestAnalyzeSimpleAssignmentTypeDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  Integer count;
  Decimal total;
  public void run(String name, Integer input) {
    Decimal widened = 1;
    Integer local = 'bad';
    count = input;
    total = count;
    count = 'bad';
    name = 1.5;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("OAERSEMA018 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeCoercionRules(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Double doubleMath(Double value) {
    return value + 1;
  }
  public void run(Integer count, String name, Boolean ready) {
    Long widenedLong = count;
    Decimal widenedDecimal = count;
    Double widenedDouble = widenedDecimal;
    Object anyValue = new List<String>();
    List<Decimal> decimals = new List<Decimal>();
    List<Integer> ints = new List<Decimal>();
    Integer badNarrow = widenedDecimal;
    Boolean badBoolean = name;
    String badString = ready;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" {
			count++
		}
	}
	if count != 4 {
		t.Fatalf("OAERSEMA018 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeGenericCollectionMethodReturnTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String first(List<String> names) {
    return names.get(0);
  }
  public void mapValues(Map<String, Account> byName) {
    Account account = byName.get('Acme');
    String badAccount = byName.get('Acme');
    Set<String> keys = byName.keySet();
    List<Account> values = byName.values();
    Integer keyCount = keys.size();
    Boolean hasAccount = values.contains(account);
    Integer badContains = keys.contains('Acme');
    byName.put('Other', account);
    byName.put(1, account);
    byName.put('Bad', 'not account');
    values.add(account);
    values.add('not account');
    values.add(0, account);
    values.add(0, 'not account');
    values.addAll(values);
    values.addAll(keys);
    Account removed = values.remove(0);
    String badRemoved = values.remove(0);
    values.set(0, account);
    values.set(0, 'not account');
    Integer accountIndex = values.indexOf(account);
    values.clear();
    values.sort();
    keys.addAll(new Set<String>{'Other'});
    keys.addAll(values);
    keys.removeAll(new Set<String>{'Other'});
    keys.retainAll(new List<String>{'Acme'});
    Account removedByName = byName.remove('Acme');
    String badMapRemove = byName.remove('Acme');
    byName.putAll(byName);
    byName.putAll(keys);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	unknownCalls := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" {
			count++
		}
		if diag.Code == "OAERSEMA008" {
			unknownCalls++
		}
	}
	collectionCalls := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA023" {
			collectionCalls++
		}
	}
	if count != 4 {
		t.Fatalf("OAERSEMA018 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
	if unknownCalls != 0 {
		t.Fatalf("collection methods should resolve without unknown-call diagnostics: %#v", result.Diagnostics)
	}
	if collectionCalls != 8 {
		t.Fatalf("OAERSEMA023 count = %d diagnostics=%#v", collectionCalls, result.Diagnostics)
	}
}

func TestAnalyzeRawCollectionsAndArraySyntax(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(List rawList, Map rawMap, Object[] values, SObject[] records) {
    rawList.add('value');
    Object first = rawList.get(0);
    Integer rawSize = rawList.size();
    rawMap.put('key', first);
    Object mapped = rawMap.get('key');
    values.add(first);
    Object value = values.get(0);
    Integer valueCount = values.size();
    for (Object item : values) {
      System.debug(item);
    }
    List<SObject> recordList = records;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		switch diag.Code {
		case "OAERSEMA008", "OAERSEMA023", "OAERSEMA024", "OAERSEMA018":
			t.Fatalf("raw collections and array syntax should be accepted, got %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectGenericCollectionAssignability(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void takesSObjects(List<SObject> records) {}
  public void run(List<Account> accounts, Account account) {
    takesSObjects(accounts);
    List<SObject> records = new List<Account>{account};
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA009" || diag.Code == "OAERSEMA018" || diag.Code == "OAERSEMA025" {
			t.Fatalf("SObject generic collection assignability should be accepted, got %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeEnhancedForGenericCollectionTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(List<Account> accounts, Set<String> names, Map<String, Account> byName, Account fallback) {
    for (Account account : accounts) {
      account = fallback;
    }
    for (Object anyName : names) {
      System.debug(anyName);
    }
    for (Account value : byName.values()) {
      value = fallback;
    }
    for (String badAccount : accounts) {
      System.debug(badAccount);
    }
    for (Account badMap : byName) {
      System.debug(badMap);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA024" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("OAERSEMA024 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeClassicForMultipleInitializerLocals(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static void run(List<String> values) {
    for (Integer i = 0, j = values.size(); i < j; i++) {
      String value = values[i];
      System.debug(value);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" {
			t.Fatalf("unexpected for-local diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeGenericCollectionInitializerTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Account account) {
    List<Account> accounts = new List<Account>{account};
    Set<String> names = new Set<String>{'Acme'};
    List<Account> copy = new List<Account>(accounts);
    Set<Account> accountSet = new Set<Account>(accounts);
    List<Account> copyFromSet = new List<Account>(accountSet);
    Map<Id, Account> byId = new Map<Id, Account>(accounts);
    List<Account> badAccounts = new List<Account>{'not account'};
    Set<String> badNames = new Set<String>{1};
    Map<String, Account> byName = new Map<String, Account>{account};
    List<String> badCopy = new List<String>(accountSet);
    Map<Id, String> badById = new Map<Id, String>(accounts);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA025" {
			count++
		}
	}
	if count != 5 {
		t.Fatalf("OAERSEMA025 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeConstructorAcceptsBareSchemaTypeAliases(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UnitOfWork.cls"), `
public class UnitOfWork {
  public UnitOfWork(List<Schema.SObjectType> types) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    UnitOfWork uow = new UnitOfWork(new List<SObjectType>{Account.SObjectType});
    List<Schema.DescribeFieldResult> fieldDescribes = new List<DescribeFieldResult>();
    List<Schema.ChildRelationship> childRelationships = new List<ChildRelationship>();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UnitOfWork.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeIgnoresSObjectConstructorNamedArgumentsAsAssignments(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Account account = new Account(
      Name = 'Acme',
      Phone = '555'
    );
    Contact contact = new Contact(LastName = 'Smith', AccountId = account.Id);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" {
			t.Fatalf("unexpected named-argument assignment diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeTernaryExpressionTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Account choose(Boolean pick, Account left, Account right) {
    Account selected = pick ? left : right;
    Object broader = pick ? left : 'fallback';
    Account nullable = pick ? left : null;
    String badLocal = pick ? left : right;
    return pick ? left : right;
  }
  public String badReturn(Boolean pick, Account account) {
    return pick ? account : null;
  }
  public void badConditionStillInfers(Integer pick, Account left, Account right) {
    String bad = pick ? left : right;
    Account okComparison = pick < 3 ? left : right;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	assignments := 0
	returns := 0
	conditions := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" {
			assignments++
		}
		if diag.Code == "OAERSEMA019" {
			returns++
		}
		if diag.Code == "OAERSEMA020" {
			conditions++
		}
	}
	if assignments != 2 || returns != 1 || conditions != 1 {
		t.Fatalf("ternary diagnostics assignments=%d returns=%d conditions=%d diagnostics=%#v", assignments, returns, conditions, result.Diagnostics)
	}
}

func TestAnalyzeCastAndInstanceOfExpressionTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Object raw, Account fallback) {
    Account castAccount = (Account) raw;
    Boolean accountLike = raw instanceof Account;
    Account selected = raw instanceof Account ? (Account) raw : fallback;
    String badCast = (Account) raw;
    Integer badInstanceof = raw instanceof Account;
    String parenthesized = ('a') + 'b';
    Integer parenthesizedMinus = (1) - 2;
    Object badUnknownCast = (MissingType) raw;
    Boolean badUnknownCheck = raw instanceof MissingType;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	unknownTypes := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" {
			count++
		}
		if diag.Code == "OAERSEMA006" {
			unknownTypes++
		}
	}
	if count != 2 || unknownTypes != 2 {
		t.Fatalf("cast diagnostics OAERSEMA018=%d OAERSEMA006=%d diagnostics=%#v", count, unknownTypes, result.Diagnostics)
	}
}

func TestAnalyzeSimpleReturnTypeDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer ok(Integer value) {
    return value;
  }
  public Decimal widened(Integer value) {
    return value;
  }
  public Integer badString() {
    return 'bad';
  }
  public void badVoid() {
    return 1;
  }
  public String missingReturn() {
    Integer value = 1;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA019" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("OAERSEMA019 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeIRBodyAllPathsReturnDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer bothBranches(Boolean flag) {
    if (flag) {
      return 1;
    } else {
      return 2;
    }
  }
  public Integer switchAll(Integer value) {
    switch on value {
      when 1 { return 1; }
      when else { return 2; }
    }
  }
  public Integer tryCatchAll(Boolean flag) {
    try {
      return 1;
    } catch (Exception e) {
      return 2;
    }
  }
  public Integer missingElse(Boolean flag) {
    if (flag) {
      return 1;
    }
  }
  public Integer missingSwitchElse(Integer value) {
    switch on value {
      when 1 { return 1; }
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA019" && strings.Contains(diag.Message, "on all paths") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("all-path return diagnostic count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeSimpleExpressionTypeDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void acceptDecimal(Decimal value) {}
  public void acceptBoolean(Boolean value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer add(Integer left, Integer right) {
    return left + right;
  }
  public void run(Integer count, String name, Boolean ready) {
    Helper h = new Helper();
    Decimal total = count + 1.5;
    Boolean ok = ready && true;
    h.acceptDecimal(count + 2);
    h.acceptBoolean(count > 0);
    count = name + 'x';
    ready = count + 1;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	counts := map[string]int{}
	for _, diag := range result.Diagnostics {
		counts[diag.Code]++
	}
	if counts["OAERSEMA018"] != 2 || counts["OAERSEMA009"] != 0 || counts["OAERSEMA019"] != 0 {
		t.Fatalf("diagnostic counts = %#v diagnostics=%#v", counts, result.Diagnostics)
	}
}

func TestAnalyzeMethodCallOverloadBaseline(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void pick(Integer value) {}
  public void pick(String value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    h.pick(1);
    h.pick('one');
    h.pick(true);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	var got bool
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA009" {
			got = true
		}
	}
	if !got {
		t.Fatalf("expected OAERSEMA009: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallNumericWideningBaseline(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void acceptInteger(Integer value) {}
  public void acceptLong(Long value) {}
  public void acceptDecimal(Decimal value) {}
  public void acceptDouble(Double value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    Integer count = 1;
    h.acceptLong(count);
    h.acceptDecimal(1);
    h.acceptDouble(1.5);
    h.acceptInteger(1.5);
    h.acceptDouble(count);
    h.acceptDecimal(true);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA009" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("OAERSEMA009 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeMethodCallNumericOverloadChoosesNarrowestWidening(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public String pick(Integer value) { return 'integer'; }
  public Boolean pick(Decimal value) { return true; }
  public String widen(Integer value) { return 'integer'; }
  public String widen(Long value) { return 'long'; }
  public Boolean widen(Decimal value) { return true; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    String exact = h.pick(1);
    String widened = h.widen(1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected no errors: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallObjectOverloadChoosesNearestAncestor(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Root.cls"), `public virtual class Root {}`)
	writeSemaFile(t, filepath.Join(root, "Parent.cls"), `public virtual class Parent extends Root {}`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `public class Child extends Parent {}`)
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public Boolean pick(Object value) { return true; }
  public Boolean pick(Root value) { return true; }
  public String pick(Parent value) { return 'parent'; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    String result = h.pick(new Child());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Root.cls"),
		filepath.Join(root, "Parent.cls"),
		filepath.Join(root, "Child.cls"),
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected no errors: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallOverloadUsesPairwiseSpecificity(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public Boolean pick(Integer count, Object label) { return true; }
  public Boolean pick(Long count, String label) { return true; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    h.pick(1, 'one');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA022" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected OAERSEMA022: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallNullUsesMostSpecificOverload(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public Boolean pick(Object value) { return true; }
  public String pick(String value) { return 'string'; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    String value = h.pick(null);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected no errors: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInheritedAndSuperMethodCalls(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Worker.cls"), `
public interface Worker {
  void work(Integer value);
}
`)
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public class Base {
  public void inherited(String value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends Base implements Worker {
  public void work(Integer value) {}
  public void run() {
    inherited('x');
    super.inherited('y');
    work(1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Worker.cls"),
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInheritedSuperReturnAndFieldTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public virtual class Base {
  public String label;
  public String inheritedLabel() {
    return label;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends Base {
  public String okThisCall() {
    return this.inheritedLabel();
  }
  public String okSuperCall() {
    return super.inheritedLabel();
  }
  public Integer badSuperReturn() {
    return super.inheritedLabel();
  }
  public void badSuperFieldAssign() {
    super.label = 1;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	counts := map[string]int{}
	for _, diag := range result.Diagnostics {
		counts[diag.Code]++
	}
	if counts["OAERSEMA018"] != 1 || counts["OAERSEMA019"] != 1 {
		t.Fatalf("diagnostic counts = %#v diagnostics=%#v", counts, result.Diagnostics)
	}
}

func TestAnalyzeInterfaceAndOverrideReturnInference(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Worker.cls"), `
public interface Worker {
  String work();
}
`)
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public virtual class Base {
  public virtual Object pick() {
    return new Object();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends Base implements Worker {
  public String work() {
    return 'work';
  }
  public override Object pick() {
    return 'child';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void run() {
    Worker worker = new Child();
    String label = worker.work();
    Base base = new Child();
    String bad = base.pick();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Worker.cls"),
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("OAERSEMA018 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeOverrideAndImplementationContracts(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Worker.cls"), `
public interface Worker {
  void work(Integer value);
}
`)
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public abstract class Base {
  public abstract String label();
  public virtual Integer score() { return 1; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Good.cls"), `
public class Good extends Base implements Worker {
  public override String label() { return 'ok'; }
  public void work(Integer value) {}
  public override Integer score() { return 2; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Bad.cls"), `
public class Bad extends Base implements Worker {
  public override void missing() {}
  public abstract void ownAbstract();
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Worker.cls"),
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Good.cls"),
		filepath.Join(root, "Bad.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	counts := map[string]int{}
	for _, diag := range result.Diagnostics {
		counts[diag.Code]++
	}
	if counts["OAERSEMA016"] != 1 || counts["OAERSEMA017"] != 3 {
		t.Fatalf("diagnostic counts = %#v diagnostics=%#v", counts, result.Diagnostics)
	}
}

func TestAnalyzeNestedSiblingOverrideSignatures(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ExprNode.cls"), `
public abstract class ExprNode {
  public abstract Object evaluate(Context context);
  public abstract class BinaryExprNode extends ExprNode {
  }
  public class AddNode extends BinaryExprNode {
    public override Object evaluate(Context context) {
      return null;
    }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Context.cls"), `
public interface Context {
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "ExprNode.cls"),
		filepath.Join(root, "Context.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA016" || diag.Code == "OAERSEMA017" {
			t.Fatalf("nested sibling inheritance should satisfy override contracts: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzePlatformOverrideSignatures(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Picklist.cls"), `
global class Picklist extends VisualEditor.DynamicPickList {
  global override VisualEditor.DataRow getDefaultValue() {
    return new VisualEditor.DataRow('None', '');
  }
  global override VisualEditor.DynamicPickListRows getValues() {
    return new VisualEditor.DynamicPickListRows();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Callback.cls"), `
public class Callback extends Metadata.DeployCallbackContext {
  public override Id getCallbackJobId() {
    return '000000000000001';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Picklist.cls"),
		filepath.Join(root, "Callback.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA016" {
			t.Fatalf("platform base overrides should satisfy override contracts: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeConstructorChainingBaseline(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public class Base {
  public Base(Integer value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends Base {
  public Child() {
    super(1);
  }
  public Child(String value) {
    this();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeConstructorChainingDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Plain.cls"), `
public class Plain {
  public void run() {
    this();
  }
  public Plain() {
    super(1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Plain.cls")}}, schema.Schema{})

	result := Analyze(index)
	var count int
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA011" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("OAERSEMA011 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeMethodCallVisibilityDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Secret.cls"), `
public class Secret {
  private void hidden() {}
  protected void guarded() {}
  public void ownAccess() {
    hidden();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ChildSecret.cls"), `
public class ChildSecret extends Secret {
  public void run() {
    guarded();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "GrandChildSecret.cls"), `
public class GrandChildSecret extends ChildSecret {
  public void runAgain() {
    guarded();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Intruder.cls"), `
public class Intruder {
  public void run() {
    Secret s = new Secret();
    s.hidden();
    s.guarded();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Secret.cls"),
		filepath.Join(root, "ChildSecret.cls"),
		filepath.Join(root, "GrandChildSecret.cls"),
		filepath.Join(root, "Intruder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	var count int
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA010" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("OAERSEMA010 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeTestVisibleMethodAccess(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Secret.cls"), `
public class Secret {
  @TestVisible private static void visibleForTests() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "SecretTest.cls"), `
@IsTest
private class SecretTest {
  @IsTest static void run() {
    Secret.visibleForTests();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Intruder.cls"), `
public class Intruder {
  public void run() {
    Secret.visibleForTests();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Secret.cls"),
		filepath.Join(root, "SecretTest.cls"),
		filepath.Join(root, "Intruder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA010" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("OAERSEMA010 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeAnnotationSemantics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "GoodRest.cls"), `
@RestResource(urlMapping='/good/*')
global class Good {
  @HttpGet global static void getIt() {}
  @future(callout=true) public static void later() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "GoodTest.cls"), `
@IsTest
private class GoodTest {
  @TestSetup static void seed() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "GoodInvocable.cls"), `
public class GoodInvocable {
  @InvocableMethod public static void run(List<String> names) {}
  @InvocableVariable public String name;
}
`)
	writeSemaFile(t, filepath.Join(root, "BadRest.cls"), `
@RestResource(urlMapping='/bad/*')
public interface BadRest {
}
`)
	writeSemaFile(t, filepath.Join(root, "BadAnnotations.cls"), `
public class BadAnnotations {
  @HttpPost public static void postIt() {}
  @TestSetup static void seed(String name) {}
  @future public static String later() { return 'x'; }
  @InvocableMethod public void run() {}
  @InvocableVariable public void notVariable() {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "GoodRest.cls"),
		filepath.Join(root, "GoodTest.cls"),
		filepath.Join(root, "GoodInvocable.cls"),
		filepath.Join(root, "BadRest.cls"),
		filepath.Join(root, "BadAnnotations.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA026" {
			count++
		}
	}
	if count != 6 {
		t.Fatalf("OAERSEMA026 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeStaticAndInstanceMethodAccess(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Worker.cls"), `
public class Worker {
  public static void stat() {}
  public void inst() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Caller.cls"), `
public class Caller {
  public void run() {
    Worker.stat();
    Worker w = new Worker();
    w.inst();
    Worker.inst();
    w.stat();
  }
  public static void runStatic() {
    helper();
  }
  public void helper() {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Worker.cls"),
		filepath.Join(root, "Caller.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA027" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("OAERSEMA027 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeFieldVisibilityDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Secret.cls"), `
public class Secret {
  private String code;
  protected String guarded;
  public String ownAccess() {
    return code;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ChildSecret.cls"), `
public class ChildSecret extends Secret {
  public String run() {
    return guarded;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Intruder.cls"), `
public class Intruder {
  public void run() {
    Secret s = new Secret();
    String a = s.code;
    String b = s.guarded;
    s.code = 'x';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Secret.cls"),
		filepath.Join(root, "ChildSecret.cls"),
		filepath.Join(root, "Intruder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA010" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("OAERSEMA010 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeTriggerObject(t *testing.T) {
	index := typesys.Index{
		Triggers: []typesys.TriggerSymbol{{Name: "ThingTrigger", ObjectName: "Missing__c", File: "Thing.trigger"}},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "OAERSEMA001" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}
}

func writeSemaFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractTypeNames(t *testing.T) {
	got := extractTypeNames("Map<String,List<Account>>")
	want := []string{"String", "Account"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v", got)
		}
	}
}

func TestIsSemaConstructorCallAtHonorsWhitespaceBeforeNew(t *testing.T) {
	body := "return new DomainBase.Context().value;"
	start := strings.Index(body, "DomainBase.Context")
	if start < 0 {
		t.Fatal("missing constructor call")
	}
	if !isSemaConstructorCallAt(body, start) {
		t.Fatalf("constructor call after whitespace was not recognized")
	}
	notConstructor := "return renew DomainBase.Context().value;"
	start = strings.Index(notConstructor, "DomainBase.Context")
	if start < 0 {
		t.Fatal("missing non-constructor call")
	}
	if isSemaConstructorCallAt(notConstructor, start) {
		t.Fatalf("identifier ending in new was recognized as a constructor call")
	}
}
