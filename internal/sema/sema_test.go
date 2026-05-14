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

func TestAnalyzeShortNestedTypeMatchesAnyCompatibleCandidate(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "One.cls"), `
public class One {
  public class Shared {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public virtual class Base {}
`)
	writeSemaFile(t, filepath.Join(root, "Two.cls"), `
public class Two {
  public class Shared extends Base {
    public String Ensured;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesShared.cls"), `
public class UsesShared {
  public static void run(Shared candidate) {
    Base baseValue = candidate;
    String ensured = candidate.Ensured;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "One.cls"),
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Two.cls"),
		filepath.Join(root, "UsesShared.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedTestDoubleStaticMockAssignment(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BaseMock.cls"), `
public virtual class BaseMock {
  @TestVisible private static BaseMock mockInstance;
  public BaseMock() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "OneTest.cls"), `
@IsTest
private class OneTest {
  private class MockChild extends BaseMock {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesMockTest.cls"), `
@IsTest
private class UsesMockTest {
  @IsTest static void run() {
    MockChild mockChild = new MockChild();
    BaseMock.mockInstance = mockChild;
    System.assert(mockChild.Ensured);
  }
  private class MockChild extends BaseMock {
    public Boolean Ensured = false;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "BaseMock.cls"),
		filepath.Join(root, "OneTest.cls"),
		filepath.Join(root, "UsesMockTest.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProductNamespaceGeneratedDeclarations(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesProductNamespaces.cls"), `
public class UsesProductNamespaces {
  public static void run() {
    connectapi.organizationsettings settings = connectapi.organization.getsettings();
    ConnectApi.TimeZone zone = settings.userSettings.timeZone;
    Metadata.DeployContainer container = new Metadata.DeployContainer();
    Metadata.CustomMetadata item = new Metadata.CustomMetadata();
    Metadata.CustomMetadataValue value = new Metadata.CustomMetadataValue();
    value.field = 'Enabled__c';
    value.value = true;
    item.values.add(value);
    container.addMetadata(item);
    Id deploymentId = Metadata.Operations.enqueueDeployment(container, null);
    Metadata.DeployResult result = Metadata.Operations.checkDeployStatus(deploymentId, true);
    Cache.OrgPartition partition = cache.org.getpartition('local');
    partition.put('zone', zone.id, 60, cache.visibility.all, false);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesProductNamespaces.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUserInfoStandardDeclarations(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesUserInfo.cls"), `
public class UsesUserInfo {
  public static void run() {
    Id userId = UserInfo.getUserId();
    Id profileId = USERINFO.getProfileId();
    String username = UserInfo.getUserName();
    String name = UserInfo.getName();
    String firstName = UserInfo.getFirstName();
    String lastName = UserInfo.getLastName();
    String email = UserInfo.getUserEmail();
    Id orgId = UserInfo.getOrganizationId();
    String userType = UserInfo.getUserType();
    String sessionId = UserInfo.getSessionId();
    String locale = UserInfo.getLocale();
    String language = UserInfo.getLanguage();
    TimeZone zone = UserInfo.getTimeZone();
    Boolean multiCurrency = UserInfo.isMultiCurrencyOrganization();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesUserInfo.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseDMLCollectionOverloads(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatabaseDML.cls"), `
public class UsesDatabaseDML {
  public static void run(List<Account> accounts, Account account, Id recordId, List<Id> recordIds, Database.DMLOptions opts) {
    List<Database.SaveResult> insertResults = Database.insert(accounts);
    List<Database.SaveResult> partialInsertResults = Database.insert(accounts, false);
    List<Database.SaveResult> optionInsertResults = Database.insert(accounts, opts);
    Database.SaveResult singleInsert = Database.insert(account, false);
    List<Database.SaveResult> userModeInsertResults = Database.insert(accounts, false, AccessLevel.USER_MODE);
    Database.SaveResult systemModeInsert = Database.insert(account, AccessLevel.SYSTEM_MODE);
    List<Database.SaveResult> updateResults = Database.update(accounts);
    List<Database.SaveResult> userModeUpdateResults = Database.update(accounts, false, AccessLevel.USER_MODE);
    List<Database.DeleteResult> deleteResults = Database.delete(accounts, false);
    List<Database.DeleteResult> userModeDeleteResults = Database.delete(accounts, false, AccessLevel.USER_MODE);
    Database.DeleteResult idDelete = Database.delete(recordId);
    Database.DeleteResult systemModeIdDelete = Database.delete(recordId, AccessLevel.SYSTEM_MODE);
    List<Database.DeleteResult> idDeleteResults = Database.delete(recordIds, false);
    List<Database.UpsertResult> upsertResults = Database.upsert(accounts, Account.External_Id__c, false);
    List<Database.UpsertResult> userModeExternalIdUpsert = Database.upsert(accounts, Account.External_Id__c, AccessLevel.USER_MODE);
    Database.UpsertResult singleUpsert = Database.upsert(account, Account.External_Id__c, false);
    Database.UpsertResult singleUserModeExternalIdUpsert = Database.upsert(account, Account.External_Id__c, AccessLevel.USER_MODE);
    List<Database.UpsertResult> systemModeUpsertResults = Database.upsert(accounts, AccessLevel.SYSTEM_MODE);
    Database.UpsertResult singleSystemModeUpsert = Database.upsert(account, AccessLevel.SYSTEM_MODE);
    List<Database.UpsertResult> systemModeUpsertNoExternalId = Database.upsert(accounts, true, AccessLevel.SYSTEM_MODE);
    List<Database.UpsertResult> userModeUpsertResults = Database.upsert(accounts, Account.External_Id__c, false, AccessLevel.USER_MODE);
    Database.UpsertResult singleUserModeUpsertNoExternalId = Database.upsert(account, false, AccessLevel.USER_MODE);
    Database.UpsertResult systemModeSingleUpsert = Database.upsert(account, Account.External_Id__c, false, AccessLevel.SYSTEM_MODE);
    Database.UndeleteResult idUndelete = Database.undelete(recordId);
    List<Database.UndeleteResult> idUndeleteResults = Database.undelete(recordIds, false, AccessLevel.USER_MODE);
    Database.EmptyRecycleBinResult idEmptyRecycleBin = Database.emptyRecycleBin(recordId);
    List<Database.EmptyRecycleBinResult> idEmptyRecycleBinResults = Database.emptyRecycleBin(recordIds);
    Database.MergeResult idMerge = Database.merge(account, recordId, false, AccessLevel.USER_MODE);
    List<Database.MergeResult> idMergeResults = Database.merge(account, recordIds, AccessLevel.SYSTEM_MODE);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDatabaseDML.cls"),
	}}, schema.Schema{Objects: []schema.Object{{
		Name: "Account",
		Fields: []schema.Field{
			{Name: "External_Id__c", Type: "Text"},
		},
	}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeBroadSystemStubShapes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesBroadSystemShapes.cls"), `
public class UsesBroadSystemShapes {
  public enum ParameterSet {
    SSN_PARAMETER_SET,
    EDUCATION_PARAMETER_SET
  }
  public static void run(HttpRequest request, Account account, EntityParticle particle, FieldDefinition fieldDefinition) {
    HttpRequest made = new HttpRequest();
    made.setEndpoint('callout:example');
    made.setMethod('GET');
    made.setHeader('X-Test', request.getHeader('X-Test'));
    String body = made.getBody();
    Integer namePos = account.Name.indexOf('School');
    Integer laterPos = account.Name.indexOf('School', namePos);
    Datetime gmtDate = Datetime.newInstanceGmt(2026, 5, 14);
    AsyncOptions asyncOptions = new AsyncOptions();
    asyncOptions.MaximumQueueableStackDepth = 5;
    Database.DMLOptions dmlOptions = new Database.DMLOptions();
    dmlOptions.OptAllOrNone = true;
    dmlOptions.EmailHeader.triggerUserEmail = false;
    dmlOptions.DuplicateRuleHeader.AllowSave = true;
    dmlOptions.AssignmentRuleHeader.UseDefaultRule = true;
    String street = account.BillingAddress.getStreet();
    Datetime parsed = Datetime.valueOf((Object) '2026-05-14 00:00:00');
    Schema.DescribeSObjectResult described = account.getSObjectType().getDescribe(SObjectDescribeOptions.DEFERRED);
    account.addError(Account.Name, 'Name is required');
    Iterable<SObjectField> iterableFields = new List<SObjectField>{ Account.Name };
    for (SObjectField field : iterableFields) {
      String fieldName = field.getDescribe().getName();
    }
    List<EntityDefinition> entityDefinitions = new List<SObject>();
    switch on 'equals' {
      when equals {
        body = 'equal';
      }
      when not_equals {
        body = 'not equal';
      }
    }
    ParameterSet parameterSet = ParameterSet.SSN_PARAMETER_SET;
    Boolean entityUpdateable = particle.FieldDefinition.EntityDefinition.RunningUserEntityAccess.IsUpdatable;
    Boolean fieldAccessible = fieldDefinition.RunningUserFieldAccess.IsAccessible;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesBroadSystemShapes.cls"),
	}}, schema.Schema{Objects: []schema.Object{{
		Name:   "Account",
		Fields: []schema.Field{{Name: "BillingAddress", Type: "Address"}},
	}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeGeneratedSystemStubsAreCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesGeneratedStubCase.cls"), `
public class UsesGeneratedStubCase {
  public static void run(Account account) {
    httprequest request = new HTTPRequest();
    request.SETendpoint('callout:example');
    request.setMETHOD('GET');
    String header = request.GetHEADER('X-Test');

    apexpages.Message message = new APEXPages.Message(
      SEVERITY = apexpages.Severity.ERROR,
      SUMMARY = 'Summary',
      DETAIL = 'Detail'
    );

    Schema.DisplayType displayType = DisplayType.sTrInG;
    Schema.DisplayType schemaDisplayType = schema.DisplayType.PICKLIST;
    Schema.SObjectType token = Account.SObjectType;

    Database.QueryLocatorIterator locatorIterator = null;
    system.Iterator<SObject> systemIterator = locatorIterator;
    Iterator<SObject> shortIterator = locatorIterator;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesGeneratedStubCase.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeConcreteSObjectRelationshipAccessors(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesConcreteSObjectAccessors.cls"), `
public class UsesConcreteSObjectAccessors {
  public static void run(Account record) {
    SObject parent = record.getSObject('Parent');
    List<SObject> children = record.getSObjects('Contacts');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesConcreteSObjectAccessors.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStandardSObjectRelationshipAccessors(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStandardSObjectAccessors.cls"), `
public class UsesStandardSObjectAccessors {
  public static void run(ContentDocumentLink documentLink) {
    Boolean matched = documentLink.ContentDocument.Title.startsWithIgnoreCase('Profile');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesStandardSObjectAccessors.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePolymorphicStandardSObjectRelationshipAccessors(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPolymorphicSObjectAccessors.cls"), `
public class UsesPolymorphicSObjectAccessors {
  public static void run(Task task, ContentDocumentLink link) {
    Id whoId = task.Who.Id;
    Id whatId = task.What.Id;
    Id linkedEntityId = link.LinkedEntity.Id;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesPolymorphicSObjectAccessors.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePolymorphicProjectSObjectRelationshipAccessors(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPolymorphicProjectSObjectAccessors.cls"), `
public class UsesPolymorphicProjectSObjectAccessors {
  public static void run(Activity_Link__c link) {
    SObject explicitParent = link.Related_To__r;
    Id explicitId = link.Related_To__r.Id;
    SObject inferredParent = link.Subject__r;
    Id inferredId = link.Subject__r.Id;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesPolymorphicProjectSObjectAccessors.cls"),
	}}, schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{Name: "Contact"},
		{Name: "Activity_Link__c", Fields: []schema.Field{
			{Name: "Related_To__c", Type: "Lookup", ReferenceTo: []string{"Account", "Contact"}, RelationshipName: "Related_To__r"},
			{Name: "Subject__c", Type: "Lookup", ReferenceTo: []string{"Account", "Contact"}},
		}},
	}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaDescribeSObjectResultMethods(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDescribe.cls"), `
public class UsesDescribe {
  public static void run(Schema.SObjectType token) {
    Schema.DescribeSObjectResult describe = token.getDescribe();
    String name = describe.getName();
    String label = describe.getLabel();
    String plural = describe.getLabelPlural();
    String prefix = describe.getKeyPrefix();
    Map<String, Schema.SObjectField> fields = describe.getFields();
    List<Schema.RecordTypeInfo> infos = describe.getRecordTypeInfos();
    Map<String, Schema.RecordTypeInfo> byName = describe.getRecordTypeInfosByName();
    Map<String, Schema.RecordTypeInfo> byDeveloperName = describe.getRecordTypeInfosByDeveloperName();
    List<Schema.ChildRelationship> children = describe.getChildRelationships();
    Schema.ChildRelationship child = children[0];
    String relationship = child.getRelationshipName();
    Schema.SObjectType childType = child.getChildSObject();
    Schema.SObjectField field = child.getField();
    Boolean cascade = child.isCascadeDelete();
    Boolean accessible = describe.isAccessible();
    Boolean creatable = describe.isCreateable();
    Boolean updateable = describe.isUpdateable();
    Boolean deletable = describe.isDeletable();
    Boolean queryable = describe.isQueryable();
    Boolean searchable = describe.isSearchable();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDescribe.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDateDatetimeStandardDeclarations(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDates.cls"), `
public class UsesDates {
  public static void run() {
    Date today = Date.today();
    Date made = Date.newInstance(2026, 5, 7);
    Date parsed = Date.valueOf('2026-05-07');
    Date parsedObject = Date.valueOf((Object) '2026-05-07');
    Date due = System.today().addDays(30);
    Date nextMonth = due.addMonths(1);
    Date nextYear = due.addYears(1);
    Integer days = today.daysBetween(due);
    Integer day = due.day();
    Integer month = due.month();
    Integer year = due.year();
    String formattedDate = due.format();
    Datetime nowStamp = Datetime.now();
    Datetime stamp = Datetime.newInstance(2026, 5, 7, 1, 2, 3);
    Datetime stampFromMillis = Datetime.newInstance(stamp.getTime());
    Datetime stampFromParts = Datetime.newInstance(today, Time.newInstance(1, 2, 3, 0));
    Datetime gmtStamp = Datetime.newInstanceGmt(2026, 5, 7, 1, 2, 3);
    Datetime parsedStamp = Datetime.valueOfGmt('2026-05-07T01:02:03Z');
    Datetime later = stamp.addDays(1).addHours(2).addMinutes(3).addSeconds(4).addMilliseconds(5);
    Date localDate = later.date();
    Date gmtDate = later.dateGmt();
    Time localTime = later.time();
    Time gmtTime = later.timeGmt();
    String formatted = later.format('yyyy-MM-dd', 'UTC');
    String gmtFormatted = later.formatGmt('yyyy-MM-dd');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDates.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemLabelReferencesAsStrings(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesLabels.cls"), `
public class UsesLabels {
  public class ResultData {
    public void addParameterField(String parameterType, String label, Object value) {}
  }
  public void run(ResultData resultData, Object value) {
    resultData.addParameterField('Name', System.Label.facilityResultsFacilityName, value);
    resultData.addParameterField('Name', Label.facilityResultsFacilityName, value);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesLabels.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectTypeFieldsTokens(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectTypeFields.cls"), `
public class UsesSObjectTypeFields {
  public void run() {
    List<Schema.SObjectField> fields = new List<Schema.SObjectField>{
      Account.SObjectType.fields.name,
      Account.SObjectType.fields.ParentId,
      Account.SObjectType.fields.ownerId,
      Contact.SObjectType.fields.accountId,
      Lead.SObjectType.fields.FirstNAMe,
      Lead.SObjectType.fields.cOMPANY
    };
    Map<String, Schema.SObjectField> accountFields = Account.SObjectType.fields.getMap();
    Schema.SObjectField nameField = Account.SObjectType.fields.get('Name');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSObjectTypeFields.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "OAERSEMA021" || diag.Code == "OAERSEMA008") && strings.Contains(diag.Message, "SObjectType.fields") {
			t.Fatalf("SObjectType.fields tokens should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedSObjectTypeToken(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesChainedSObjectType.cls"), `
public class UsesChainedSObjectType {
  public static void run() {
    Schema.SObjectType accountType = Account.SObjectType.SObjectType;
    Schema.SObjectType customMetadataType = Credentialing_Object_Setup__mdt.SObjectType.SObjectType;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesChainedSObjectType.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Credentialing_Object_Setup__mdt"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestExtractBodyForSemaSkipsCommentApostrophes(t *testing.T) {
	source := `public class Example {
  public static String run() {
    if (true) {
      // don't let this comment hide the nested block
      if (true) {
        return 'ok';
      }
    }
    return 'fallback';
  }
}`
	start := strings.Index(source, "public static String run")
	body, _, ok := extractBodyForSema(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: len(source) - 1},
	})
	if !ok {
		t.Fatalf("expected body extraction to succeed")
	}
	if !strings.Contains(body, "return 'fallback';") {
		t.Fatalf("expected body to include final return, got %q", body)
	}
}

func TestBlockBoundsAtSkipsCommentApostrophes(t *testing.T) {
	body := `{
  if (true) {
    // don't let this comment hide the close brace
    String rec = 'first';
  }
  if (true) {
    String rec = 'second';
  }
}`
	pos := strings.LastIndex(body, "String rec")
	start, end := blockBoundsAt(body, pos)
	block := body[start:end]
	if strings.Contains(block, "first") {
		t.Fatalf("expected second block scope only, got %q", block)
	}
}

func TestAnalyzeMultilineSOQLDoesNotDeclareLocals(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMultilineSOQL.cls"), `
public class UsesMultilineSOQL {
  public static void run(Id recordId) {
    Account updatedRec = [
      SELECT Id, Name
      FROM Account
      WHERE Id = :recordId
    ];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesMultilineSOQL.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA006" && (strings.Contains(diag.Message, "unknown type \"SELECT\"") || strings.Contains(diag.Message, "unknown type \"WHERE\"")) {
			t.Fatalf("SOQL query should not be treated as a local declaration: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSecurityStripInaccessibleDecision(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSecurityStripInaccessible.cls"), `
public class UsesSecurityStripInaccessible {
  public static List<SObject> run(List<SObject> records) {
    SObjectAccessDecision decision = Security.stripInaccessible(AccessType.READABLE, records, false);
    Set<Integer> modified = decision.getModifiedIndexes();
    Map<String, Set<String>> removed = decision.getRemovedFields();
    return decision.getRecords();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSecurityStripInaccessible.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeListSortComparator(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesListSortComparator.cls"), `
public class UsesListSortComparator {
  public class AccountComparator implements Comparator<Account> {
    public Integer compare(Account left, Account right) {
      return 0;
    }
  }
  public static void run(List<Account> accounts) {
    accounts.sort(new AccountComparator());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesListSortComparator.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatacloudDuplicateResultTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatacloudDuplicateResult.cls"), `
public class UsesDatacloudDuplicateResult {
  public static List<Id> run(Database.DuplicateError duplicateError) {
    Datacloud.DuplicateResult duplicateResult = duplicateError.getDuplicateResult();
    List<Datacloud.MatchRecord> matchRecords = duplicateResult.getMatchResults()[0].getMatchRecords();
    List<Id> ids = new List<Id>();
    for (Datacloud.MatchRecord match : matchRecords) {
      ids.add(match.getRecord().Id);
    }
    return ids;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDatacloudDuplicateResult.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeURLStandardDeclarations(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesURL.cls"), `
public class UsesURL {
  public String run() {
    return new URL(URL.getSalesforceBaseUrl(), '/apexrest/example').toExternalForm();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesURL.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNullCoalescingDoesNotReportSyntheticCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCoalesce.cls"), `
public class UsesCoalesce {
  public Integer run(Map<String, Integer> counts) {
    return (counts.get('spruce') ?? 0) + 1;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCoalesce.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "__coalesce") {
			t.Fatalf("null coalescing should not report synthetic call: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNumericSetScaleAndSObjectErrors(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPlatformMethods.cls"), `
public class UsesPlatformMethods {
  public void run(Account accountRecord, Decimal price) {
    Decimal rounded = price.setScale(2);
    Long asLong = rounded.longValue();
    Boolean hasErrors = accountRecord.hasErrors();
    Boolean noErrors = accountRecord.getErrors().isEmpty();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesPlatformMethods.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && (strings.Contains(diag.Message, "setScale") || strings.Contains(diag.Message, "getErrors") || strings.Contains(diag.Message, "hasErrors")) {
			t.Fatalf("platform methods should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDateArithmeticAndSObjectCloneOverloads(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "TermInfo.cls"), `
public class TermInfo {
  public Date StartDate { get; set; }
  public List<TermInfo> Children { get; set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesCloneAndDate.cls"), `
public class UsesCloneAndDate {
  public void run(Account accountRecord) {
    Date endDate = Date.today().addYears(1).addDays(-1);
    SObject shallow = accountRecord.clone(false, true);
    SObject deep = accountRecord.clone(false, true, false);
    TermInfo info = new TermInfo();
    info.StartDate = Date.today();
    Date nestedEnd = info.StartDate.addYears(1).addDays(-1);
    info.Children = new List<TermInfo>();
    info.Children.add(info);
  }
  private class Calculator {
    public TermInfo Calculate(List<Account> accounts) {
      TermInfo info = new TermInfo();
      info.StartDate = Date.today();
      info.Children = new List<TermInfo>();
      for (Account accountRecord : accounts) {
        info.Children.add(info);
        Date nestedEnd = info.StartDate.addYears(1).addDays(-1);
      }
      return info;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "TermInfo.cls"), filepath.Join(root, "UsesCloneAndDate.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "OAERSEMA008" || diag.Code == "OAERSEMA023") && (strings.Contains(diag.Message, "addYears") || strings.Contains(diag.Message, "addDays") || strings.Contains(diag.Message, "clone")) {
			t.Fatalf("date arithmetic and SObject clone overloads should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAdditionalPlatformSeams(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CustomException.cls"), `
public class CustomException extends Exception {
  public CustomException(String message, Boolean display) {
    this(message);
  }
  public String getUserMessage() {
    return getMessage();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesPlatformSeams.cls"), `
public class UsesPlatformSeams {
  public PageReference run(Folder folderRecord, Note noteRecord, PermissionSetAssignment assignment, User userRecord, Date startDate, String label, SalesTaxRequest request) {
    Integer compared = label.compareTo('next');
    Integer months = startDate.monthsBetween(Date.today());
    Id folderId = folderRecord.Id;
    Id noteId = noteRecord.Id;
    String assigneeEmail = assignment.Assignee.Email;
    Account portalAccount = userRecord.Contact.Account;
    Address addressCopy = request.Address.clone();
    return ApexPages.currentPage();
  }
  public void safeMap(Map<String, Object> values) {
    Object value = values?.get('key');
  }
  public Boolean outside(Date startDate, Date joinDate, Date endDate) {
    return joinDate < startDate ||
           joinDate > endDate;
  }
  public void addFieldError(SObject record) {
    record.addError(Account.Name, 'bad');
  }
  public List<Schema.SObjectField> fields() {
    return new List<Schema.SObjectField> {
      RecentlyViewed.LastViewedDate,
      RecentlyViewed.Type,
      RecentlyViewed.Name,
      RecentlyViewed.Id
    };
  }
  public Boolean hasValidConfigurations(String typeName) {
    switch on typeName {
      when 'A' {
        return true;
      }
      when 'B', 'C' {
        return false;
      }
    }
    return false;
  }
}
public class SalesTaxRequest {
  public Address Address { get; set; }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "CustomException.cls"), filepath.Join(root, "UsesPlatformSeams.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "PermissionSetAssignment"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		switch diag.Code {
		case "OAERSEMA006", "OAERSEMA008", "OAERSEMA011", "OAERSEMA019", "OAERSEMA021":
			t.Fatalf("platform seams should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFallbackPropertyNamesForDateAndList(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesFallbackProperties.cls"), `
public class UsesFallbackProperties {
  public void run(Object info, Object response) {
    Date endDate = info.StartDate.addYears(1).addDays(-1);
    response.MembershipTermInfos.add(info);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesFallbackProperties.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "OAERSEMA008" || diag.Code == "OAERSEMA023") && (strings.Contains(diag.Message, "addYears") || strings.Contains(diag.Message, "MembershipTermInfos")) {
			t.Fatalf("fallback property names should provide Date/List types: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCommaSeparatedLocalDeclarations(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCommaLocals.cls"), `
public class UsesCommaLocals {
  public void run() {
    CartItem__c membershipItem, primaryDonationItem, secondaryDonationItem;
    membershipItem = new CartItem__c();
    primaryDonationItem = new CartItem__c();
    secondaryDonationItem = new CartItem__c();
    System.assertEquals(null, membershipItem);
    System.assertEquals(null, primaryDonationItem);
    System.assertEquals(null, secondaryDonationItem);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCommaLocals.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "CartItem__c"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" && (strings.Contains(diag.Message, "membershipItem") || strings.Contains(diag.Message, "primaryDonationItem") || strings.Contains(diag.Message, "secondaryDonationItem")) {
			t.Fatalf("comma-separated locals should be visible: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeApexClassBodyFieldAsString(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesApexClassBody.cls"), `
public class UsesApexClassBody {
  private List<ApexClass> queryClasses() {
    return new List<ApexClass>();
  }
  private void initTestData() {
    for (ApexClass testDataApexClass : queryClasses()) {
      if (classImplementsITestData(testDataApexClass.Body)) {
        System.assertEquals(true, true);
      }
    }
  }
  private Boolean classImplementsITestData(String body) {
    return body != null;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesApexClassBody.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA009" && strings.Contains(diag.Message, "classImplementsITestData") {
			t.Fatalf("ApexClass.Body should match String overloads: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedWrapperRecordCollectionAdd(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "WrapperController.cls"), `
public class WrapperController {
  public class Setting {
    public BulkBillingSetting__c Record { get; private set; }
  }
  public List<Setting> Settings { get; private set; }
  public void save() {
    List<BulkBillingSetting__c> records = new List<BulkBillingSetting__c>();
    for (Setting wrapper : Settings) {
      validateRecord(wrapper.Record);
      records.add(wrapper.Record);
    }
  }
  private void validateRecord(BulkBillingSetting__c record) {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "WrapperController.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "BulkBillingSetting__c"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMetadataSObjectFieldsAndStringEquals(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMetadataObjects.cls"), `
public class UsesMetadataObjects {
  public void run(List<ApexPage> pages, String value) {
    Set<String> pageIds = new Set<String>();
    for (ApexPage page : pages) {
      pageIds.add(page.Id);
      if (!pageIds.contains(page.Id)) {
        pageIds.add(page.Name);
      }
    }
    Boolean same = value.equals(0);
    Schema.SObjectType pageType = ApexPage.SObjectType;
    Schema.SObjectType reportType = Report.SObjectType;
    Schema.SObjectType credentialType = NamedCredential.SObjectType;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMetadataObjects.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStaticStandardSObjectFieldTokensInListInitializer(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStandardFieldTokens.cls"), `
public class UsesStandardFieldTokens {
  public List<Schema.SObjectField> namedCredentialFields() {
    return new List<Schema.SObjectField> {
      NamedCredential.DeveloperName,
      NamedCredential.Endpoint,
      NamedCredential.MasterLabel,
      NamedCredential.NamespacePrefix
    };
  }
  public List<Schema.SObjectField> recentlyViewedFields() {
    return new List<Schema.SObjectField> {
      RecentlyViewed.LastViewedDate,
      RecentlyViewed.Type,
      RecentlyViewed.Name,
      RecentlyViewed.Id
    };
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStandardFieldTokens.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectFieldTokenDoesNotShadowClassStaticField(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order {
  public static final String EXTERNAL_TAX_STATUS_TRANSACTION_LOCKED = 'Locked';
  public String TaxTransactionStatus { get; set; }
  public void run() {
    this.TaxTransactionStatus = Order.EXTERNAL_TAX_STATUS_TRANSACTION_LOCKED;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "Order.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" && strings.Contains(diag.Message, "Schema.SObjectField") {
			t.Fatalf("unexpected field token diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChildRelationshipAddAllSpecificType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesChildRelationships.cls"), `
public class UsesChildRelationships {
  public void run(List<Account> accounts) {
    List<Affiliation__c> affiliations = new List<Affiliation__c>();
    List<Merchandise__c> merchandise = new List<Merchandise__c>();
    List<Registration2__c> registrations = new List<Registration2__c>();
    for (Account account : accounts) {
      affiliations.addAll(account.Affiliates__r);
      merchandise.addAll(account.Merchandise2__r);
      registrations.addAll(account.Registrations3__r);
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesChildRelationships.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}, {Name: "Affiliation__c"}, {Name: "Merchandise__c"}, {Name: "Registration2__c"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChildRelationshipAddAllToSObjectList(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesChildRelationships.cls"), `
public class UsesChildRelationships {
  public void run(List<Account> accounts, List<VfiProvider__c> providers) {
    List<SObject> records = new List<SObject>();
    for (Account account : accounts) {
      if (account.Affiliates__r?.isEmpty() == false) {
        records.addAll(account.Affiliates__r);
      }
    }
    for (VfiProvider__c provider : providers) {
      if (provider.VfiLicense__r?.isEmpty() == false) {
        records.addAll(provider.VfiLicense__r);
      }
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesChildRelationships.cls")},
	}, schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{Name: "Affiliation__c"},
		{Name: "VfiProvider__c"},
		{Name: "VfiLicense__c", Fields: []schema.Field{{
			Name:             "VfiProvider__c",
			Type:             "Lookup",
			ReferenceTo:      []string{"VfiProvider__c"},
			RelationshipName: "VfiLicense",
		}}},
	}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAmbiguousNullOverloadsAccepted(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesNullOverloads.cls"), `
public class UsesNullOverloads {
  public class Response {
    public Response(String message) {}
    public Response(Account record) {}
  }
  public void run() {
    Response response = new Response(null);
    addTransition(null);
  }
  public void addTransition(String name) {}
  public void addTransition(Account record) {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesNullOverloads.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemTypeAliasAssignment(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSystemType.cls"), `
public class UsesSystemType {
  public void run() {
    System.Type classType = Type.forName('Example');
    Boolean assignable = classType.isAssignableFrom(Type.forName('Other'));
    Object made = classType.newInstance();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSystemType.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeGeneratedStubMethodStaticAccess(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesGeneratedStubStaticAccess.cls"), `
public class UsesGeneratedStubStaticAccess {
  public void run() {
    Boolean invalidStatic = Database.SaveResult.isSuccess();
    Type classType = Type.forName('Example');
    Type invalidInstance = classType.forName('Other');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesGeneratedStubStaticAccess.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	var staticAccessDiagnostics int
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA027" {
			staticAccessDiagnostics++
		}
	}
	if staticAccessDiagnostics != 2 {
		t.Fatalf("expected 2 generated-stub static access diagnostics, got %d: %#v", staticAccessDiagnostics, result.Diagnostics)
	}
}

func TestAnalyzeObjectEqualsAndFluentPageReferenceRedirect(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPlatformSeams.cls"), `
public class UsesPlatformSeams {
  public Boolean compare(UsesPlatformSeams other) {
    return this.equals(other);
  }
  public PageReference redirect() {
    return (new PageReference('/home')).setRedirect(true);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesPlatformSeams.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedEnumValuesStaticCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Logger.cls"), `
public class Logger {
  public enum Level { INFO, WARN }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesLogger.cls"), `
public class UsesLogger {
  public List<Logger.Level> getLoggerLevels() {
    return Logger.Level.values();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "Logger.cls"), filepath.Join(root, "UsesLogger.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeKnownSObjectCompatibilityAliases(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesKnownSObjectAliases.cls"), `
public class UsesKnownSObjectAliases {
  public void run(Registration__c registration, Payment_Line__c line) {
    Registration2__c reg2 = registration;
    PaymentLine__c paymentLine = line;
    List<Payment__c> payments = new List<CreditCardRefundPayment__c>();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesKnownSObjectAliases.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Registration__c"}, {Name: "Registration2__c"}, {Name: "Payment_Line__c"}, {Name: "PaymentLine__c"}, {Name: "Payment__c"}, {Name: "CreditCardRefundPayment__c"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeArrayStyleAndWrappedLocalDeclarations(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesWrappedLocals.cls"), `
public class UsesWrappedLocals {
  public void run(Account testAccount) {
    Id[] fixedSearchResults = new Id[1];
    fixedSearchResults[0] = testAccount.Id;
    Iterable<UsesWrappedLocals.Context>
      iterable = start(null);
    iterable.iterator();
  }
  public Iterable<Context> start(Object value) {
    return new List<Context>();
  }
  public class Context {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesWrappedLocals.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsSearchQueryCompileOnly(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSearchQuery.cls"), `
public class UsesSearchQuery {
  public List<List<SObject>> run(String formattedQuery) {
    return Search.query(formattedQuery);
  }
  public void assign(String formattedQuery) {
    List<List<SObject>> results = Search.query(formattedQuery);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSearchQuery.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Search.query diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseQueryAssignsSingleAndListSObjectContexts(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatabaseQuery.cls"), `
public class UsesDatabaseQuery {
  public void run(String query, Map<String, Object> binds) {
    SObject single = Database.query(query);
    Account account = Database.query(query);
    List<SObject> records = Database.query(query);
    List<Account> accounts = Database.query(query);
    SObject singleWithBinds = Database.queryWithBinds(query, binds);
    List<Account> accountsWithBinds = Database.queryWithBinds(query, binds);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDatabaseQuery.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Database.query diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStillFlagsUnsupportedSearchSurface(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSearchFind.cls"), `
public class UsesSearchFind {
  public void run(String queryText) {
    search.FIND(queryText);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSearchFind.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA028" && strings.Contains(diag.Message, "search.FIND") {
			return
		}
	}
	t.Fatalf("expected unsupported Search.find diagnostic: %#v", result.Diagnostics)
}

func TestAnalyzeUserRecordAccessAndAddressComparison(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MergeLike.cls"), `
public class MergeLike {
  public class Address {
    public String Street { get; private set; }
  }
  public Boolean run(Object other, UserRecordAccess access) {
    Address otherAddress = (Address)other;
    return otherAddress != null && access.HasReadAccess;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "MergeLike.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerOperationEnumValueArgs(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesTriggerOperation.cls"), `
public class UsesTriggerOperation {
  private static void createTriggeringContext(TriggerOperation operation, List<Account> newRecords, List<Account> oldRecords) {}
  public static void run() {
    List<Account> newRecords = new List<Account>();
    List<Account> oldRecords = new List<Account>();
    createTriggeringContext(TriggerOperation.AFTER_UPDATE, newRecords, oldRecords);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesTriggerOperation.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestCollectionSignaturesStripTypeModifiers(t *testing.T) {
	sig, ok := semaCollectionMethodSignature("global List<MembershipTermInfo>", "add")
	if !ok {
		t.Fatal("global List<T> should be recognized as a collection")
	}
	if len(sig.params) == 0 || len(sig.params[0]) != 1 || sig.params[0][0] != "MembershipTermInfo" {
		t.Fatalf("unexpected add signature: %#v", sig)
	}
}

func TestAnalyzeNestedCollectionAddAfterForEachShadow(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MembershipTermInfo.cls"), `
global class MembershipTermInfo {
  public Id MembershipLinkId;
}
`)
	writeSemaFile(t, filepath.Join(root, "MembershipTermResponse.cls"), `
global class MembershipTermResponse {
  global List<MembershipTermInfo> MembershipTermInfos { get; set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "MembershipTermRequestInfo.cls"), `
global class MembershipTermRequestInfo {
  public Id MembershipLinkId;
}
`)
	writeSemaFile(t, filepath.Join(root, "MembershipTermRequest.cls"), `
global class MembershipTermRequest {
  public List<MembershipTermRequestInfo> MembershipTermRequestInfos;
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesNestedCollectionAdd.cls"), `
public class UsesNestedCollectionAdd {
  private class CustomCalculator {
    public MembershipTermResponse Calculate(MembershipTermRequest request) {
      MembershipTermResponse response = new MembershipTermResponse();
      response.MembershipTermInfos = new List<MembershipTermInfo>();
      Set<Id> membershipLinkIds = new Set<Id>();
      for (MembershipTermRequestInfo info : request.MembershipTermRequestInfos) {
        membershipLinkIds.add(info.MembershipLinkId);
      }
      List<MembershipTypeProductLink__c> membershipLinks = new List<MembershipTypeProductLink__c>();
      for (MembershipTypeProductLink__c link : membershipLinks) {
        MembershipTermInfo info = new MembershipTermInfo();
        info.MembershipLinkId = link.Id;
        response.MembershipTermInfos.add(info);
      }
      return response;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "MembershipTermInfo.cls"),
			filepath.Join(root, "MembershipTermResponse.cls"),
			filepath.Join(root, "MembershipTermRequestInfo.cls"),
			filepath.Join(root, "MembershipTermRequest.cls"),
			filepath.Join(root, "UsesNestedCollectionAdd.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "MembershipTypeProductLink__c"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA023" && strings.Contains(diag.Message, "add") {
			t.Fatalf("nested collection add should use the local element type: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNewListHelperAllowsUntypedExpressionArgument(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BaseTest.cls"), `
public class BaseTest {
  public static List<SObject> newList(SObject record) {
    return new List<SObject>{ record };
  }
  public static List<String> newList(String value) {
    return new List<String>{ value };
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesNewList.cls"), `
public class UsesNewList extends BaseTest {
  public void run(Account accountRecord) {
    Object holder = accountRecord;
    List<SObject> records = newList(holder.getSObject('Parent'));
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "BaseTest.cls"), filepath.Join(root, "UsesNewList.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA022" && strings.Contains(diag.Message, "newList") {
			t.Fatalf("newList helper should tolerate untyped expression arguments: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeEnumStaticMethods(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesEnum.cls"), `
public class UsesEnum {
  public enum Status { Ready, Done }
  public void run() {
    List<Status> allStatuses = Status.values();
    Status selected = Status.valueOf('Ready');
    Integer count = Status.values().size();
    for (Status status : Status.values()) {
      String name = status.name();
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesEnum.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && (strings.Contains(diag.Message, "values") || strings.Contains(diag.Message, "valueOf")) {
			t.Fatalf("enum static methods should be recognized: %#v", diag)
		}
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

func TestAnalyzeStringFormatAcceptsObjectListArguments(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StringFormatBuilder.cls"), `
public class StringFormatBuilder {
  public String withStringList(String key, String value) {
    return String.format('{0}{1}', new List<String>{ key, value });
  }
  public String withObjectList(String key, Integer count) {
    List<Object> args = new List<Object>{ key, count };
    return String.format('{0}{1}', args);
  }
  public String withMixedObjectLiteral(String key, Integer count) {
    return String.format('{0}{1}', new List<Object>{ key, count });
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StringFormatBuilder.cls"),
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
		t.Fatalf("expected 3 non-constructable diagnostics, got %d: %#v", count, result.Diagnostics)
	}
}

func TestAnalyzeCommaSeparatedLocalDeclaration(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String run() {
    String stepNames = '', delimiter = ', ';
    return stepNames.removeEnd(delimiter);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" && strings.Contains(diag.Message, "delimiter") {
			t.Fatalf("unexpected delimiter diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCustomExceptionConstructorInheritance(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MyException.cls"), `
public class MyException extends Exception {}
`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void run(Exception cause) {
    MyException empty = new MyException();
    MyException message = new MyException('blocked');
    MyException nested = new MyException(cause);
    throw new MyException('blocked', cause);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "MyException.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCallArgumentSeesBareSObjectTypeLocal(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SelectorFactory.cls"), `
public class SelectorFactory {
  public interface ISelector {
    List<SObject> selectSObjectsById(Set<Id> recordIds);
  }
  public class SelectorFactoryInner {
    public ISelector newInstance(SObjectType objectType) {
      return null;
    }
    public List<SObject> selectById(Set<Id> recordIds) {
      throw new DeveloperException('Invalid record Id\'s set');
      SObjectType domainSObjectType = new List<Id>(recordIds)[0].getSObjectType();
      throw new DeveloperException('Unable to determine SObjectType');
      return newInstance(domainSObjectType).selectSObjectsById(recordIds);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "SelectorFactory.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" && strings.Contains(diag.Message, "domainSObjectType") {
			t.Fatalf("unexpected SObjectType local diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCustomRelationshipFieldInfersReferencedObject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Product.cls"), `
public class Product {
  public static Product newInstance(Product__c record) {
    return new Product();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "OrderLine.cls"), `
public class OrderLine {
  public void run(OrderItemLine__c line) {
    Product product = Product.newInstance(line.Product2__r);
  }
}
`)
	index := typesys.Build(
		project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Product.cls"), filepath.Join(root, "OrderLine.cls")}},
		schema.Schema{Objects: []schema.Object{{
			Name: "OrderItemLine__c",
			Fields: []schema.Field{{
				Name:        "Product2__c",
				Type:        "Lookup",
				ReferenceTo: []string{"Product__c"},
			}},
		}, {Name: "Product__c"}}},
	)

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA009" && strings.Contains(diag.Message, "Product.newInstance") {
			t.Fatalf("unexpected relationship field overload diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMapLiteralValueChainedCallAfterArrow(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ITestData.cls"), `
public interface ITestData {
  SObject insertRecord();
}
`)
	writeSemaFile(t, filepath.Join(root, "TestContext.cls"), `
public class TestContext {
  public static TestContext Instance { get; }
  public ITestData build(Schema.SObjectType objectType) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "AffiliationTestData.cls"), `
public class AffiliationTestData {
  protected Map<Schema.SObjectField, Object> getDefaultValueMap() {
    return new Map<Schema.SObjectField, Object> {
      Account.Name => TestContext.Instance.build(Account.SObjectType).insertRecord().Id
    };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "ITestData.cls"),
		filepath.Join(root, "TestContext.cls"),
		filepath.Join(root, "AffiliationTestData.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "insertRecord") {
			t.Fatalf("unexpected map literal chained-call diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectAddErrorAndTriggerStaticFlags(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public void run(List<Affiliation__c> affiliations) {
    for (Affiliation__c affiliation : affiliations) {
      affiliation.addError('bad');
      affiliation.IsPrimaryContact__c.addError('bad');
      if (trigger.isInsert) {
        affiliation.addError(new Exception('bad'), false);
      }
    }
  }
}
`)
	index := typesys.Build(
		project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Handler.cls")}},
		schema.Schema{Objects: []schema.Object{{
			Name: "Affiliation__c",
			Fields: []schema.Field{{
				Name: "IsPrimaryContact__c",
				Type: "Checkbox",
			}},
		}}},
	)

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "OAERSEMA008" && strings.Contains(strings.ToLower(diag.Message), "adderror")) ||
			(diag.Code == "OAERSEMA013" && strings.Contains(strings.ToLower(diag.Message), "trigger.isinsert")) {
			t.Fatalf("unexpected SObject addError/Trigger diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDescribeFieldGetNameSelectsStringOverload(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CollectionUtil.cls"), `
public class CollectionUtil {
  public static Map<Id, List<SObject>> groupSObjectsByIdField(List<SObject> records, String field) {
    return new Map<Id, List<SObject>>();
  }
  public static Map<Id, List<SObject>> groupSObjectsByIdField(List<SObject> records, Schema.SObjectField field) {
    return new Map<Id, List<SObject>>();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public Map<Id, List<SObject>> run(List<Affiliation__c> affiliations) {
    return CollectionUtil.groupSObjectsByIdField(affiliations, Affiliation__c.Account__c.getDescribe().getName());
  }
}
`)
	index := typesys.Build(
		project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "CollectionUtil.cls"), filepath.Join(root, "Handler.cls")}},
		schema.Schema{Objects: []schema.Object{{
			Name: "Affiliation__c",
			Fields: []schema.Field{{
				Name:        "Account__c",
				Type:        "Lookup",
				ReferenceTo: []string{"Account"},
			}},
		}, {Name: "Account"}}},
	)

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA022" && strings.Contains(diag.Message, "groupSObjectsByIdField") {
			t.Fatalf("unexpected describe getName overload ambiguity: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectMapGetSObjectType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Context.cls"), `
public class Context {
  public Map<Id, SObject> NewRecordMap { get; set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public void run() {
    Context context = new Context();
    System.assertEquals(Account.SObjectType, context.NewRecordMap.getSObjectType());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Context.cls"),
		filepath.Join(root, "Handler.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "getSObjectType") {
			t.Fatalf("unexpected SObject map getSObjectType diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStandardApexClassSelectorPatterns(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "InvalidOperationException.cls"), `
public class InvalidOperationException extends Exception {}
`)
	writeSemaFile(t, filepath.Join(root, "ApexClassSelector.cls"), `
public class ApexClassSelector {
  public List<SObject> selectById(Set<Id> ids) {
    throw new InvalidOperationException();
  }
  protected Schema.SObjectType getSObjectType() {
    return ApexClass.SObjectType;
  }
  private List<Schema.SObjectField> getSObjectFieldList() {
    return new List<Schema.SObjectField> {
      ApexClass.Name,
      ApexClass.NamespacePrefix
    };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "InvalidOperationException.cls"),
		filepath.Join(root, "ApexClassSelector.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "OAERSEMA019" && strings.Contains(diag.Message, "selectById")) ||
			(diag.Code == "OAERSEMA021" && strings.Contains(diag.Message, "ApexClass")) ||
			(diag.Code == "OAERSEMA025" && strings.Contains(diag.Message, "Schema.SObjectField")) {
			t.Fatalf("unexpected ApexClass selector diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeConcreteSObjectGetByFieldName(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public void run(RecordType recordType) {
    Boolean value = (Boolean)recordType.get('IsPersonType');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Handler.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "recordType.get") {
			t.Fatalf("unexpected concrete SObject get diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectGetPopulatedFieldsAsMap(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Map<String, Object> run(Account account) {
    return account.getPopulatedFieldsAsMap();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "getPopulatedFieldsAsMap") {
			t.Fatalf("unexpected getPopulatedFieldsAsMap diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCronJobDetailAndCronTriggerStandardObjects(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run() {
    List<CronJobDetail> jobs = [SELECT Name, Id FROM CronJobDetail];
    CronTrigger trigger = [SELECT CronExpression, TimesTriggered, NextFireTime FROM CronTrigger];
    return !jobs.isEmpty() && trigger.TimesTriggered > 0;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA006" && (strings.Contains(diag.Message, "CronJobDetail") || strings.Contains(diag.Message, "CronTrigger")) {
			t.Fatalf("unexpected cron standard object diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectCloneAndAddError(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Account run(Account account) {
    Account cloned = account.clone(false, true, false, false);
    account.addError('bad');
    return cloned;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && (strings.Contains(diag.Message, "clone") || strings.Contains(diag.Message, "addError")) {
			t.Fatalf("unexpected SObject instance method diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeConstructorPrefersEnumOverObject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "EventHandlerResponse.cls"), `
public class EventHandlerResponse {
  public enum Status { NO_HANDLER, SUCCESS, ERROR }
  public EventHandlerResponse(Object data) {}
  public EventHandlerResponse(Status status) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public EventHandlerResponse run() {
    return new EventHandlerResponse(EventHandlerResponse.Status.NO_HANDLER);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "EventHandlerResponse.cls"),
		filepath.Join(root, "Handler.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA011" && strings.Contains(diag.Message, "EventHandlerResponse") {
			t.Fatalf("unexpected enum/Object constructor ambiguity: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeThisConstructorCallWithNestedGenericList(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AttachmentService.cls"), `
public class AttachmentService {
  public class Request {}
  public AttachmentService(Request request) {
    this(new List<Request>{ request });
  }
  public AttachmentService(List<Request> requests) {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "AttachmentService.cls")}}, schema.Schema{Objects: []schema.Object{{Name: "Request"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA011" && strings.Contains(diag.Message, "AttachmentService") {
			t.Fatalf("unexpected this constructor diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStaticFactoryOverloadWithSObjectInitializer(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Schedule.cls"), `
public class Schedule {
  public static Schedule newInstance(Schedule__c record) { return new Schedule(); }
  public static Schedule newInstance(List<ScheduleLine> lines) { return new Schedule(); }
}
`)
	writeSemaFile(t, filepath.Join(root, "ScheduleLine.cls"), `public class ScheduleLine {}`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Schedule run() {
    return Schedule.newInstance(new Schedule__c(Id = 'a00000000000001'));
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Schedule.cls"),
		filepath.Join(root, "ScheduleLine.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Schedule__c"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA022" && strings.Contains(diag.Message, "Schedule.newInstance") {
			t.Fatalf("unexpected Schedule.newInstance ambiguity: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedConstructorCallReturnType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AccountEvaluator.cls"), `
public class AccountEvaluator {
  public Boolean evaluate(Map<String, Object> data) { return true; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Boolean run(Map<String, Object> data) {
    Boolean result = new AccountEvaluator().evaluate(data);
    return result;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "AccountEvaluator.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" && strings.Contains(diag.Message, "AccountEvaluator") {
			t.Fatalf("unexpected chained constructor return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedGenericReturnShortName(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BillMe.cls"), `
public class BillMe {
  public class Status {}
  public List<Status> run() {
    List<Status> statuses = new List<Status>();
    return statuses;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "BillMe.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA019" && strings.Contains(diag.Message, "Status") {
			t.Fatalf("unexpected nested generic return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedCollectionCallAfterComparison(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(Map<Id, List<SObject>> itemsByCartId, List<Object> failures) {
    return failures.size() != itemsByCartId.keySet().size();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "size") {
			t.Fatalf("unexpected chained size diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedMapConstructorKeySet(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Set<Id> run(List<Account> accounts) {
    return new Map<Id, Account>(accounts).keySet();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "keySet") {
			t.Fatalf("unexpected chained keySet diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedStaticFactoryCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Selector.cls"), `
public class Selector {
  public static Selector newInstance() {
    return new Selector();
  }
  public List<Account> selectById(Set<Id> ids) {
    return new List<Account>();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public List<Account> run(Set<Id> ids) {
    return Selector.newInstance().selectById(ids);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Selector.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "selectById") {
			t.Fatalf("unexpected chained factory diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStandardSetControllerAllowsUnresolvedQueryLocatorArg(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public ApexPages.StandardSetController run() {
    return new ApexPages.StandardSetController(Manager.Instance.getQueryLocator());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA011" && strings.Contains(diag.Message, "StandardSetController") {
			t.Fatalf("unexpected StandardSetController diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeApexPagesHasMessagesSeverity(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run() {
    return ApexPages.hasMessages(ApexPages.Severity.Error);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA023" && strings.Contains(diag.Message, "hasMessages") {
			t.Fatalf("unexpected ApexPages.hasMessages diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFinalLocalVariable(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    final String TEST_FAILURE = 'Test Failure';
    System.assertEquals(TEST_FAILURE, TEST_FAILURE);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" && strings.Contains(diag.Message, "TEST_FAILURE") {
			t.Fatalf("unexpected final local variable diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeIRStaticPropertyChainCallArgType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UnitOfWork.cls"), `
public class UnitOfWork {
  public void registerDeleted(List<SObject> records) {}
  public void registerDeleted(SObject record) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "TestContext.cls"), `
public class TestContext {
  public static TestContext Instance;
  public SObject get(Schema.SObjectType type) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    UnitOfWork unit = new UnitOfWork();
    unit.registerDeleted(TestContext.Instance.get(Account.SObjectType));
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UnitOfWork.cls"),
		filepath.Join(root, "TestContext.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA022" && strings.Contains(diag.Message, "registerDeleted") {
			t.Fatalf("unexpected static property chain overload diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectGetSObjectFieldToken(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public SObject run(SObject record, Schema.SObjectField fieldToken) {
    return record.getSObject(fieldToken);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA023" && strings.Contains(diag.Message, "getSObject") {
			t.Fatalf("unexpected SObject.getSObject diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeReturnAfterCommentStartingWithReturn(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(Boolean value) {
    // Return value from the method.
    return value;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA019" && strings.Contains(diag.Message, "must return Boolean") {
			t.Fatalf("unexpected return diagnostic after comment: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeListDeepClonePreserveFlags(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public List<Account> run(List<Account> accounts) {
    return accounts.deepClone(true, true, true);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA023" && strings.Contains(diag.Message, "deepClone") {
			t.Fatalf("unexpected List.deepClone diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedCollectionCallAfterLessThan(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(List<Account> records) {
    for (Integer i = 1; i < records.size(); i++) {
      records.remove(i);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "size") {
			t.Fatalf("unexpected chained size diagnostic after less-than: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSummaryFieldReturnType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  private Cart__c record;
  public Decimal run() {
    return this.record.SubTotal__c;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{
			Name: "Cart__c",
			Fields: []schema.Field{{
				Name: "SubTotal__c",
				Type: "Summary",
			}},
		}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA019" && strings.Contains(diag.Message, "SubTotal__c") {
			t.Fatalf("unexpected summary field return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectPutSObject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(SObject record, SObject related) {
    record.putSObject('Parent__r', related);
  }
  public void runCustom(CartItemLine__c record, SObject related) {
    record.putSObject('Product__r', related);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "CartItemLine__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "putSObject") {
			t.Fatalf("unexpected SObject.putSObject diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMapInitializerValueChainedCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public class Base {
  protected Schema.RecordTypeInfo getRecordType(String name) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello extends Base {
  public Map<Schema.SObjectField, Object> run() {
    return new Map<Schema.SObjectField, Object> {
      Account.RecordTypeId => getRecordType('Default').getRecordTypeId()
    };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "getRecordTypeId") {
			t.Fatalf("unexpected map initializer chained call diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeKnownFluentWithCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Hello run() {
    return with(Account.RecordTypeId, '012000000000000AAA')
      .with(Account.Name, 'Acme')
      .withFirstName('Ada');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(strings.ToLower(diag.Message), "with") {
			t.Fatalf("unexpected fluent with diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedCallAfterNestedStaticFactoryArg(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Generator.cls"), `
public class Generator {
  public static Id generate(Schema.SObjectType typeToken) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Converter.cls"), `
public class Converter {
  public static Converter newInstance(Id idValue) {
    return new Converter();
  }
  public OrderItem__c convertRecord(SObject record) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public OrderItem__c run(CartItem__c cartItem) {
    return Converter.newInstance(
      Generator.generate(Order__c.SObjectType))
      .convertRecord(cartItem);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Generator.cls"),
		filepath.Join(root, "Converter.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{
		Objects: []schema.Object{{Name: "CartItem__c"}, {Name: "Order__c"}, {Name: "OrderItem__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "convertRecord") {
			t.Fatalf("unexpected nested static factory chained call diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeGetClassGetName(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String run() {
    return getClass().getName();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "getName") {
			t.Fatalf("unexpected getClass().getName diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeGetClassGetNameInsideChainedArgument(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Selector.cls"), `
public class Selector {
  public static Selector newInstance() {
    return new Selector();
  }
  public Account selectByClassName(String className) {
    return new Account();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run() {
    Account record = Selector.newInstance().selectByClassName(getClass().getName());
    return record != null;
  }
  protected Type getClass() {
    return Hello.class;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Selector.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "getName") {
			t.Fatalf("unexpected nested getClass().getName diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeBooleanReturnWithCastComparison(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  private static final String ERROR_CODE = 'E';
  public Boolean run() {
    return (String)this.getValueFromField(Account.Name) != ERROR_CODE;
  }
  private Object getValueFromField(Schema.SObjectField fieldToken) {
    return null;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA019" && strings.Contains(diag.Message, "String from Boolean") {
			t.Fatalf("unexpected cast comparison return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDateTimeFieldDateCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(Payment__c record) {
    return record.CreatedDate.Date() == Date.today();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "Payment__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "CreatedDate.Date") {
			t.Fatalf("unexpected datetime date diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAmbiguousOverloadSameReturnType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Map<Id, Decimal> run(Object records) {
    return getPrices(records);
  }
  private Map<Id, Decimal> getPrices(List<CartItem__c> records) {
    return new Map<Id, Decimal>();
  }
  private Map<Id, Decimal> getPrices(List<CartItemLine__c> records) {
    return new Map<Id, Decimal>();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "CartItem__c"}, {Name: "CartItemLine__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA022" && strings.Contains(diag.Message, "getPrices") {
			t.Fatalf("unexpected same-return overload ambiguity: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFallbackCustomStringFieldContains(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(SpecialPrice__c specialPrice, String priceClass) {
    return specialPrice.PriceClasses__c.contains(priceClass);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "SpecialPrice__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "PriceClasses__c.contains") {
			t.Fatalf("unexpected fallback string field contains diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeLocalDeclarationShadowsForEachVariable(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CartItem.cls"), `
public class CartItem {}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void save(List<CartItem__c> records, Map<String, CartItem> itemsByIdentifier) {
    for (CartItem__c item : records) {
      String key = 'existing';
    }
    CartItem item = itemsByIdentifier.get('missing');
    if (item == null) {
      item = new CartItem();
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "CartItem.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "CartItem__c"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA018" && strings.Contains(diag.Message, `variable "item"`) {
			t.Fatalf("unexpected shadowed item assignment diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAuraHandledExceptionConstructable(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    throw new AuraHandledException('blocked');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA006" && strings.Contains(diag.Message, "AuraHandledException") {
			t.Fatalf("unexpected AuraHandledException diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDescribeFieldDefaultValueFormula(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer run() {
    return Integer.valueOf(Account.Name.getDescribe().getDefaultValueFormula());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "getDefaultValueFormula") {
			t.Fatalf("unexpected getDefaultValueFormula diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeObjectToStringCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AddressUtil.cls"), `
public class AddressUtil {
  public static Object getAddress(Account account, String fieldName) {
    return account.get(fieldName);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(Account account) {
    return AddressUtil.getAddress(account, 'BillingStreet').toString() != '';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "AddressUtil.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "toString") {
			t.Fatalf("unexpected toString diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeScalarValueAndStringSliceMethods(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String run(Decimal amount, String name) {
    Integer sortOrder = amount.intValue();
    String formatted = amount.toPlainString();
    return name.left(sortOrder) + formatted.leftPad(4, '0');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" {
			t.Fatalf("unexpected unknown method diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeTypedListLiteralDoesNotReportUnknownNewlitCall(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PaymentLine__c.cls"), `public class PaymentLine__c {}`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(PaymentLine__c line) {
    List<PaymentLine__c> lines = new List<PaymentLine__c>{ line };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "PaymentLine__c.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "newlit:List<PaymentLine__c>") {
			t.Fatalf("unexpected newlit diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDomXmlNodeGetChildElement(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Dom.XmlNode run(Dom.XmlNode node) {
    return node.getChildElement('name', null);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "getChildElement") {
			t.Fatalf("unexpected getChildElement diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeTypeNewInstance(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Object run(Type typ) {
    return typ.newInstance();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "newInstance") {
			t.Fatalf("unexpected newInstance diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeUserInfoOrganizationName(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String run() {
    return UserInfo.getOrganizationName();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && strings.Contains(diag.Message, "getOrganizationName") {
			t.Fatalf("unexpected UserInfo diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMessagingSendEmailAllOrNothing(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(List<Messaging.SingleEmailMessage> messages) {
    Messaging.sendEmail(messages, false);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA009" && strings.Contains(diag.Message, "Messaging.sendEmail") {
			t.Fatalf("unexpected Messaging.sendEmail diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStandardExceptionSubtypeAssignable(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Logger.cls"), `
public class Logger {
  public static void log(Exception e) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    try {
      throw new AsyncException('blocked');
    } catch (AsyncException e) {
      Logger.log(e);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Logger.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA009" && strings.Contains(diag.Message, "Logger.log") {
			t.Fatalf("unexpected standard exception subtype diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeListReturnAssignableToIterable(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Iterable<Object> run() {
    return new List<Object>();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA019" && strings.Contains(diag.Message, "Iterable<Object>") {
			t.Fatalf("unexpected Iterable return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDatabaseBatchableMethods(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Database.Batchable<Object> batchable, Database.BatchableContext context) {
    Iterable<Object> records = batchable.start(context);
    batchable.execute(context, new List<Object>());
    batchable.finish(context);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && (strings.Contains(diag.Message, ".start") || strings.Contains(diag.Message, ".execute") || strings.Contains(diag.Message, ".finish")) {
			t.Fatalf("unexpected Database.Batchable diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeQueryLocatorAndIterableMethods(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Database.Batchable<Object> batch) {
    Database.QueryLocator locator = (Database.QueryLocator)batch.start(null);
    String query = locator.getQuery();
    Database.QueryLocatorIterator queryIterator = locator.iterator();
    queryIterator.hasNext();
    Object record = queryIterator.next();
    Iterator<SObject> sobjectIterator = locator.iterator();
    sobjectIterator.hasNext();
    SObject sobjectRecord = sobjectIterator.next();
    System.Iterator<SObject> qualifiedIterator = locator.iterator();
    qualifiedIterator.hasNext();
    Iterable<Object> iterable = batch.start(null);
    Iterator<Object> iterator = iterable.iterator();
    iterator.hasNext();
    Object value = iterator.next();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA008" && (strings.Contains(diag.Message, "getQuery") || strings.Contains(diag.Message, "iterator") || strings.Contains(diag.Message, "hasNext") || strings.Contains(diag.Message, "next")) {
			t.Fatalf("unexpected query locator/iterable diagnostic: %#v", result.Diagnostics)
		}
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

func TestAnalyzeNestedEnumOverloadDeclaredLater(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ProductFabricator.cls"), `
public class ProductFabricator {
  public enum RecordType { MERCHANDISE }
  public static List<ProductFabricator> createProducts(Integer count) {
    return createProducts(count, RecordType.MERCHANDISE);
  }
  public static List<ProductFabricator> createProducts(Integer count, RecordType recordType) {
    return new List<ProductFabricator>();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "ProductFabricator.cls")}}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected nested enum overload to resolve: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCallArgumentsIgnoreCommentedArgument(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "TestOrderItemLineManager.cls"), `
public class TestOrderItemLineManager {
  public static OrderItemLine__c insertNonDuesMembershipOLI(Id orderItemId, Id memberAcctId, Id membershipEnrollmentId) {
    return null;
  }
  public static void run(OrderItemLine__c cartMembershipOIL) {
    OrderItemLine__c nonDuesMembershipOIL =
      TestOrderItemLineManager.insertNonDuesMembershipOLI
        (cartMembershipOIL.OrderItem__c,
         //cartMembershipOIL.ShipTo__c,
         null,
         cartMembershipOIL.Membership__c);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "TestOrderItemLineManager.cls")}}, schema.Schema{
		Objects: []schema.Object{
			{Name: "OrderItemLine__c", Fields: []schema.Field{
				{Name: "OrderItem__c", Type: "Lookup", ReferenceTo: []string{"OrderItem__c"}},
				{Name: "Membership__c", Type: "Lookup", ReferenceTo: []string{"Membership__c"}},
			}},
			{Name: "OrderItem__c"},
			{Name: "Membership__c"},
		},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected commented argument to be ignored: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedMapGetFieldIndexReturnType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "OrderConfirmation.cls"), `
public class OrderConfirmation {
  private class OrderItemCollection {
    public List<OrderItem__c> Items { get; set; }
  }
  public Map<String, OrderItemCollection> OrderItemMap { get; set; }
  public OrderItem__c get() {
    return OrderItemMap.get('Merchandise').Items[0];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "OrderConfirmation.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "OrderItem__c"}},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected chained Map.get field index to infer OrderItem__c: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMultipleLocalDeclaratorsKeepDeclaredType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "TotalAffiliatedAccounts.cls"), `
public class TotalAffiliatedAccounts {
  private static Map<Id, Affiliation__c> cache = new Map<Id, Affiliation__c>();
  private static void getAffiliatedAccountDeltas(Id affiliationId, Map<Id, Affiliation__c> oldRecords) {
    Id parentId;
    Affiliation__c newRecord = null, oldRecord = null;
    oldRecord = cache.get(affiliationId);
    if (oldRecord == null) {
      oldRecord = oldRecords.get(affiliationId);
    }
    parentId = oldRecord.ParentAccount__c;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "TotalAffiliatedAccounts.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "Affiliation__c", Fields: []schema.Field{{Name: "ParentAccount__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}}},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected comma-declared local to keep Affiliation__c type: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMultipleUninitializedLocalDeclarators(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StandardRegistrationValidator.cls"), `
public class StandardRegistrationValidator {
  public void validate() {
    Registration2__c existing, cancelled;
    String link, linkText;
    if (existing != null) {
      link = '/' + existing.Id;
      linkText = existing.Name;
    }
    if (cancelled != null) {
      link = '/' + cancelled.Id;
      linkText = cancelled.Name;
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "StandardRegistrationValidator.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "Registration2__c", Fields: []schema.Field{
			{Name: "Name", Type: "String"},
		}}},
	})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA013" {
			t.Fatalf("unexpected unknown variable diagnostic for comma declarator: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeVisibleOverrideWinsOverProtectedBaseMethod(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SObjectWrapper.cls"), `
public virtual class SObjectWrapper {
  protected virtual Object getValueFromField(Schema.SObjectField field) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order extends SObjectWrapper {
  global override virtual Object getValueFromField(SObjectField field) {
    return super.getValueFromField(field);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Service.cls"), `
public class Service {
  private Order orderInstance;
  protected String getInvoiceNumber() {
    return (String)this.orderInstance.getValueFromField(Account.Name);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "SObjectWrapper.cls"),
		filepath.Join(root, "Order.cls"),
		filepath.Join(root, "Service.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "String"}}}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA010" {
			t.Fatalf("unexpected OAERSEMA010 diagnostics: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMultilineLocalDeclarationDoesNotRedeclareLaterUse(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PaymentSchedule.cls"), `
public class PaymentSchedule {
  public PaymentSchedule(Object record) {}
  public Integer getIntervalAmount() { return 1; }
  public String getIntervalUnit() { return 'Month'; }
}
`)
	writeSemaFile(t, filepath.Join(root, "PaymentScheduleLink__c.cls"), `
public class PaymentScheduleLink__c {
  public Object PaymentSchedule__r;
  public Integer ScheduleStartDayOverride__c;
}
`)
	writeSemaFile(t, filepath.Join(root, "Calculator.cls"), `
public class Calculator {
  public static void run(Integer amount, String unit, Integer overrideDay, Boolean flag) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Processor.cls"), `
public class Processor {
  private void calculateInstallment(PaymentScheduleLink__c paymentScheduleLink) {
    PaymentSchedule paymentSchedule = new PaymentSchedule(paymentScheduleLink.PaymentSchedule__r);

    Calculator.run(paymentSchedule.getIntervalAmount(),
        paymentSchedule.getIntervalUnit(),
        cartOrderData.StartDate,
        cartOrderData.EndDate,
        paymentSchedule == null ? null : paymentScheduleLink.ScheduleStartDayOverride__c,
        false);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "PaymentSchedule.cls"),
		filepath.Join(root, "PaymentScheduleLink__c.cls"),
		filepath.Join(root, "Calculator.cls"),
		filepath.Join(root, "Processor.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA014" {
			t.Fatalf("unexpected OAERSEMA014 diagnostics: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCommentedConstructorDoesNotTriggerConstructability(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SubscriptionManager.cls"), `
public abstract class SubscriptionManager {
}
`)
	writeSemaFile(t, filepath.Join(root, "TestSubscriptions.cls"), `
@isTest
public class TestSubscriptions {
  private static testMethod void SubscriptionDateValidationTest() {
    //SubscriptionManager sManager = new SubscriptionManager();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "SubscriptionManager.cls"),
		filepath.Join(root, "TestSubscriptions.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA015" {
			t.Fatalf("unexpected OAERSEMA015 diagnostics: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectInstanceFieldsUseValueTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesInstanceFields.cls"), `
public class UsesInstanceFields {
  public void run(List<OrderItem__c> orderItems, List<Event__c> events) {
    for (OrderItem__c orderItem : orderItems) {
      Id entityId = orderItem.Entity__c;
    }
    for (Event__c event : events) {
      String label = event.Name;
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "UsesInstanceFields.cls")}}, schema.Schema{
		Objects: []schema.Object{
			{Name: "OrderItem__c", Fields: []schema.Field{{Name: "Entity__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}},
			{Name: "Event__c", Fields: []schema.Field{{Name: "Name", Type: "Text"}}},
		},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
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

func TestAnalyzeTestVisibleConstructorAccess(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SecretCtor.cls"), `
public class SecretCtor {
  @TestVisible private SecretCtor(String value) {}
  public SecretCtor() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "SecretCtorTest.cls"), `
@IsTest
private class SecretCtorTest {
  @IsTest static void run() {
    SecretCtor value = new SecretCtor('test');
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "IntruderCtor.cls"), `
public class IntruderCtor {
  public void run() {
    SecretCtor value = new SecretCtor('test');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "SecretCtor.cls"),
		filepath.Join(root, "SecretCtorTest.cls"),
		filepath.Join(root, "IntruderCtor.cls"),
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

func TestAnalyzeLocalDoesNotShadowTypeInOwnInitializer(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Factory.cls"), `
public class Factory {
  public static Factory newInstance() {
    return new Factory();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Caller.cls"), `
public class Caller {
  public void run() {
    Factory factory = Factory.newInstance();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Factory.cls"),
		filepath.Join(root, "Caller.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA027" && strings.Contains(diag.Message, "Factory.newInstance") {
			t.Fatalf("local should not shadow its type inside its own initializer: %#v", result.Diagnostics)
		}
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

func TestSemaEnumValuesStripsComments(t *testing.T) {
	tests := []struct {
		name     string
		decl     string
		expected []string
	}{
		{
			name:     "no comments",
			decl:     `public enum E { A, B, C }`,
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "line comments",
			decl:     "public enum E {\nA, // comment\nB, // another\nC\n}",
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "block comments",
			decl:     "public enum E { /* block */ A, /* block */ B, C }",
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "mixed comments",
			decl:     "public enum E { /* block */ A, // line\nB, /* block */ C }",
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "comment before first value",
			decl:     "public enum E { /* header */ A, B }",
			expected: []string{"A", "B"},
		},
		{
			name:     "comment after last value",
			decl:     "public enum E { A, B /* trailing */ }",
			expected: []string{"A", "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "Test.cls")
			if err := os.WriteFile(path, []byte(tt.decl), 0644); err != nil {
				t.Fatal(err)
			}
			typ := typesys.TypeSymbol{
				Kind: "enum",
				Name: "E",
				File: path,
				Range: diagnostic.Range{
					Start: diagnostic.Position{Offset: 0},
					End:   diagnostic.Position{Offset: len(tt.decl)},
				},
			}
			got := semaEnumValues(typ)
			if !slicesEqual(got, tt.expected) {
				t.Errorf("semaEnumValues() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
