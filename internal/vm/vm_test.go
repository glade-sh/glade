package vm

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
)

func TestExecAssertEquals(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals(2, 1 + 1);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCommonSObjectTypeNamesIncludesGeneratedStandardObjects(t *testing.T) {
	foundApexClass := false
	foundAccount := false
	for _, name := range CommonSObjectTypeNames() {
		if strings.EqualFold(name, "ApexClass") {
			foundApexClass = true
		}
		if strings.EqualFold(name, "Account") {
			foundAccount = true
		}
	}
	if !foundApexClass {
		t.Fatalf("CommonSObjectTypeNames should include generated standard object ApexClass")
	}
	if !foundAccount {
		t.Fatalf("CommonSObjectTypeNames should preserve prefix-backed standard object Account")
	}
}

func TestSchemaGlobalDescribeIncludesOnlyOrgAvailableObjectShape(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)

	describe := machine.schemaGlobalDescribe()
	if token, ok := describe.Map[mapKey(String("Account"))]; !ok || token.Type != "Schema.SObjectType" {
		t.Fatalf("Account token = %#v ok=%v", token, ok)
	}
	if _, ok := describe.Map[mapKey(String("AIApplication"))]; ok {
		t.Fatalf("global describe should not include unloaded standard object AIApplication")
	}
}

func TestExecSchemaDescribeSObjectsRequiresOrgAvailableObjectShape(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
System.assertEquals(null, Schema.getGlobalDescribe().get('AIApplication'));
try {
	Schema.describeSObjects(new String[]{'AIApplication'});
	System.assert(false, 'expected describe failure');
} catch (Exception e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assertEquals('System.SObjectException:Schema.describeSObjects unknown object AIApplication', caught);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeFieldNameUsesCallerNamespaceWithoutOrgNamespace(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.currentNamespace = "NU"

	if got := machine.describeFieldName("IsActive__c"); got != "NU__IsActive__c" {
		t.Fatalf("describeFieldName() = %q, want NU__IsActive__c", got)
	}
}

func TestDescribeTabSObjectNameUsesCallerNamespaceWithoutOrgNamespace(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.currentNamespace = "NU"

	tab := machine.describeTabValue(storage.TabMetadata{
		Name:        "Order__c",
		Label:       "Orders",
		SObjectName: "Order__c",
		Custom:      true,
	})

	got := tab.Fields["sObjectName"]
	if got.Kind != ValueString || got.Text != "NU__Order__c" {
		t.Fatalf("DescribeTabResult.getSObjectName backing value = %#v, want NU__Order__c", got)
	}
}

func TestCoerceEmptyNonSObjectListToSObjectListFails(t *testing.T) {
	machine := New(nil)
	value := typedList("List<TriggerStep>")

	_, err := machine.coerceAssignable("List<SObject>", value)
	if err == nil {
		t.Fatal("expected empty List<TriggerStep> to fail assignment to List<SObject>")
	}
	if !strings.Contains(err.Error(), "Invalid conversion from runtime type List<TriggerStep> to List<SObject>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCoerceEmptySObjectListToSObjectListPasses(t *testing.T) {
	machine := New(nil)
	value := typedList("List<Account>")

	coerced, err := machine.coerceAssignable("List<SObject>", value)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Type != "List<SObject>" {
		t.Fatalf("coerced.Type = %q, want List<SObject>", coerced.Type)
	}
}

func TestRecordFromValueKeepsParentRelationshipShellForDMLValidationFormula(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Product__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Entity__c": {APIName: "Entity__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Product__c",
				Fields: map[string]storage.Value{
					"Entity__c": storage.StringValue("Member"),
				},
			},
		},
	}
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "CartItem__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Entity__c": {APIName: "Entity__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Line__c",
			KeyPrefix: "a03",
			Fields: map[string]storage.Field{
				"Product__c":  {APIName: "Product__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}},
				"CartItem__c": {APIName: "CartItem__c", Type: storage.FieldReference, ReferenceTo: []string{"CartItem__c"}},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "ProductEntityMatchesCartItem",
				Active:                true,
				ErrorConditionFormula: "ISBLANK(CartItem__r.Entity__c) || Product__r.Entity__c <> CartItem__r.Entity__c",
				ErrorMessage:          "The product entity must match the cart item's entity.",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}

	line := Object("Line__c")
	line.Fields["Product__c"] = String("a01000000000001")
	line.Fields["CartItem__c"] = String("a02000000000001")
	cartItem := Object("CartItem__c")
	cartItem.Fields["Entity__c"] = String("Member")
	line.Fields["CartItem__r"] = cartItem

	machine := New(nil)
	machine.Org = &org
	record, err := machine.recordFromValue(&line)
	if err != nil {
		t.Fatal(err)
	}
	engine := machine.newDMLEngine(&Result{})
	results := engine.Insert([]storage.Record{record})
	if !results[0].Success {
		t.Fatalf("insert with relationship shell = %#v; want success", results[0])
	}
	stored := org.Objects["Line__c"].Records[results[0].ID]
	if len(stored.ParentRelationships) != 0 {
		t.Fatalf("stored record kept parent relationship shell: %#v", stored.ParentRelationships)
	}
}

func TestRecordFromValueUsesPreparedDescribeCache(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Widget__c",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)

	value := Object("Widget__c")
	value.Fields["Name"] = String("Trail stove")
	if _, err := machine.recordFromValue(&value); err != nil {
		t.Fatal(err)
	}

	cached, ok := machine.describeDefCache["widget__c"]
	if !ok {
		t.Fatal("recordFromValue did not use prepared describe cache")
	}
	if cached.KeyPrefix == "" {
		t.Fatal("cached prepared definition has empty key prefix")
	}
	if got := org.Objects["Widget__c"].Definition.KeyPrefix; got != "" {
		t.Fatalf("org definition key prefix was mutated to %q", got)
	}
}

func TestSameAliasValueIgnoresMapIterationOrder(t *testing.T) {
	left := typedMap("Map<String,Object>")
	left.Ref = 10
	left.Map[mapKey(String("a"))] = String("A")
	left.Map[mapKey(String("b"))] = String("B")
	left.MapKeys[mapKey(String("a"))] = String("a")
	left.MapKeys[mapKey(String("b"))] = String("b")
	left.MapOrder = []string{mapKey(String("a")), mapKey(String("b"))}

	right := cloneValue(left)
	right.Ref = left.Ref
	right.MapOrder = []string{mapKey(String("b")), mapKey(String("a"))}
	if !sameAliasValue(left, right) {
		t.Fatalf("sameAliasValue should ignore map iteration order for unchanged alias content")
	}
}

func TestReplaceAliasSnapshotReplacesMatchingRefAndKind(t *testing.T) {
	target := Object("Account")
	target.Ref = 42
	root := List(target)
	snapshot := snapshotAlias(target)
	updated := target
	updated.Fields["Name"] = String("Acme")

	replaced, changed := replaceAliasSnapshot(root, snapshot, updated, make(map[uint64]bool))
	if !changed {
		t.Fatalf("replaceAliasSnapshot changed = false, want true")
	}
	if got := replaced.List[0].Fields["Name"].Text; got != "Acme" {
		t.Fatalf("replaced alias Name = %q, want Acme", got)
	}
}

func TestReplaceAliasSnapshotSeenMapCanBeReused(t *testing.T) {
	firstPrevious := Object("First")
	firstUpdated := firstPrevious
	firstUpdated.Fields["Name"] = String("updated")
	firstRoot := Object("Root")
	firstRoot.Fields["Child"] = firstPrevious

	seen := make(map[uint64]bool)
	firstReplaced, firstChanged := replaceAliasSnapshot(firstRoot, snapshotAlias(firstPrevious), firstUpdated, seen)
	if !firstChanged {
		t.Fatal("first replacement did not change")
	}
	if got := firstReplaced.Fields["Child"].Fields["Name"].Text; got != "updated" {
		t.Fatalf("first replacement name = %q", got)
	}

	clearRefSeen(seen)

	secondPrevious := Object("Second")
	secondUpdated := secondPrevious
	secondUpdated.Fields["Name"] = String("updated")
	secondRoot := Object("Root")
	secondRoot.Fields["Child"] = secondPrevious

	secondReplaced, secondChanged := replaceAliasSnapshot(secondRoot, snapshotAlias(secondPrevious), secondUpdated, seen)
	if !secondChanged {
		t.Fatal("second replacement did not change")
	}
	if got := secondReplaced.Fields["Child"].Fields["Name"].Text; got != "updated" {
		t.Fatalf("second replacement name = %q", got)
	}
}

func TestTriggerNamespaceByNameCachesCurrentNamespaceFallback(t *testing.T) {
	machine := New(nil)
	machine.currentNamespace = "pkg"
	if err := machine.RegisterTrigger(Trigger{Name: "AccountTrigger", Object: "Account", Timing: triggerTimingBefore, Operation: "insert"}); err != nil {
		t.Fatal(err)
	}
	if got := machine.triggerNamespaceByName("AccountTrigger"); got != "pkg" {
		t.Fatalf("trigger namespace = %q, want pkg", got)
	}
	if machine.triggerNamespaceCache == nil || len(machine.triggerNamespaceCache) == 0 {
		t.Fatal("trigger namespace lookup was not cached")
	}
}

func TestAliasSnapshotMutationPropagationSkipsNoopMetadataChange(t *testing.T) {
	original := Object("Account")
	original.Ref = 42
	original.Static = "Account"
	original.Runtime = "Account"
	original.Fields["Name"] = String("Acme")
	updated := original
	updated.Static = "Object"
	updated.Runtime = "Object"

	machine := New(nil)
	scope := map[string]Value{"account": original}
	changed := machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(updated), original, updated, false)
	if changed {
		t.Fatal("noop metadata coercion propagated as a mutation")
	}
	if got := scope["account"].Static; got != "Account" {
		t.Fatalf("account static type = %q, want Account", got)
	}
}

func TestAliasSnapshotMutationPropagationSkipsNestedNoopMetadataChange(t *testing.T) {
	child := List(String("Acme"))
	child.Ref = 43
	original := Object("Account")
	original.Ref = 42
	original.Static = "Account"
	original.Runtime = "Account"
	original.Fields["Names"] = child
	updated := original
	updated.Static = "Object"
	updated.Runtime = "Object"

	machine := New(nil)
	scope := map[string]Value{"account": original, "names": child}
	changed := machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(updated), original, updated, false)
	if changed {
		t.Fatal("nested noop metadata coercion propagated as a root mutation")
	}
	if got := scope["account"].Static; got != "Account" {
		t.Fatalf("account static type = %q, want Account", got)
	}
	if got := scope["names"].List[0].Text; got != "Acme" {
		t.Fatalf("names[0] = %q, want Acme", got)
	}
}

func TestAliasSnapshotMutationPropagationRefreshesNestedCollectionAlias(t *testing.T) {
	items := List(String("Tax"))
	items.Ref = 43
	original := Object("Holder")
	original.Ref = 42
	original.Fields["Items"] = items
	updated := original
	trimmed := items
	trimmed.List = trimmed.List[:0]
	updated.Fields["Items"] = trimmed

	machine := New(nil)
	scope := map[string]Value{"holder": original, "items": items}
	changed := machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(updated), original, updated, true)
	if changed {
		t.Fatal("nested collection alias refresh propagated as a root mutation")
	}
	if got := len(scope["items"].List); got != 0 {
		t.Fatalf("items size = %d, want 0", got)
	}
}

func TestAliasSnapshotMutationPropagationRefreshesStaleTopLevelObjectAlias(t *testing.T) {
	staleItems := List(Object("Account"))
	staleItems.Ref = 43
	stale := Object("Controller")
	stale.Ref = 42
	stale.Fields["Items"] = staleItems

	original := Object("Controller")
	original.Ref = stale.Ref
	updatedItems := List(Object("Account"))
	updatedItems.Ref = 44
	updated := original
	updated.Fields["Items"] = updatedItems

	machine := New(nil)
	scope := map[string]Value{"controller": stale}
	changed := machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(updated), original, updated, false)
	if !changed {
		t.Fatal("stale top-level object alias was not refreshed")
	}
	if got := scope["controller"].Fields["Items"].Ref; got != updatedItems.Ref {
		t.Fatalf("controller Items ref = %d, want %d", got, updatedItems.Ref)
	}
}

func TestAliasSnapshotMutationPropagationKeepsRealDataChange(t *testing.T) {
	original := Object("Account")
	original.Ref = 42
	original.Fields["Name"] = String("Acme")
	updated := Object("Account")
	updated.Ref = original.Ref
	updated.Fields["Name"] = String("Smith")

	machine := New(nil)
	scope := map[string]Value{"account": original}
	changed := machine.propagateAliasSnapshotMutationToScope(scope, snapshotAlias(updated), original, updated, false)
	if !changed {
		t.Fatal("data mutation did not propagate")
	}
	if got := scope["account"].Fields["Name"].Text; got != "Smith" {
		t.Fatalf("account Name = %q, want Smith", got)
	}
}

func TestExecGeneratedPlatformStaticMethodFallsBackToTypedDefault(t *testing.T) {
	program, err := CompileAnonymous(`
List<Id> similarIdeas = Ideas.findSimilar(new Idea(Title = 'Acme'));
System.assertEquals(0, similarIdeas.size());
System.assertEquals(0, Ideas.getAllRecentReplies('005000000000001', '0DB000000000001').size());
System.assertEquals(0, Ideas.getReadRecentReplies('005000000000001', '0DB000000000001').size());
System.assertEquals(0, Ideas.getUnreadRecentReplies('005000000000001', '0DB000000000001').size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectTypeMapDoesNotMatchDifferentStandardObjectFromIdPrefix(t *testing.T) {
	program, err := CompileAnonymous(`
Map<SObjectType, String> names = new Map<SObjectType, String>{
	Contact.SObjectType => 'contact'
};
Id accountId = '001000000000001';
System.assertEquals(null, names.get(accountId.getSObjectType()));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnumSwitchNullFallsThroughToElse(t *testing.T) {
	program, err := CompileAnonymous(`
VerifiableDataset.Dataset datasetType = null;
Boolean caught = false;
try {
	switch on datasetType {
		when Npi {
		}
		when else {
			throw new AuraHandledException('Unsupported object type.');
		}
	}
} catch (Exception e) {
	caught = true;
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VerifiableDataset.Dataset", EnumValues: []string{"Npi"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnumSwitchNullDoesNotMatchNullStaticCaseExpression(t *testing.T) {
	program, err := CompileAnonymous(`
Probe.Kind datasetType = null;
Boolean caught = false;
try {
	switch on datasetType {
		when Npi {
		}
		when else {
			throw new AuraHandledException('Unsupported object type.');
		}
	}
} catch (Exception e) {
	caught = true;
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:         "Probe",
		StaticFields: map[string]Field{"Npi": {Name: "Npi", Type: "Probe.Kind", Static: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Probe.Kind", EnumValues: []string{"Npi"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "Probe"); err != nil {
		t.Fatal(err)
	}
}

func TestExecSwitchNullMatchesWhenNull(t *testing.T) {
	program, err := CompileAnonymous(`
String branch = 'none';
switch on null {
	when null {
		branch = 'null';
	}
	when else {
		branch = 'else';
	}
}
System.assertEquals('null', branch);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecIdeasReplyServiceSurfacesRemainUnsupported(t *testing.T) {
	for _, source := range []string{
		`Ideas.markRead('087000000000001');`,
	} {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "unsupported call") {
			t.Fatalf("%s error = %v, want unsupported call", source, err)
		}
	}
}

func TestExecConnectApiSetTestMethodsAreLocalNoops(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.LibraryOutput output = new ConnectApi.LibraryOutput();
ConnectApi.AiGroundingLibrary.setTestGetLibrary('local-library', output);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedPlatformConstructsPassiveValueObjectWithProperties(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.QuestionAndAnswersSuggestions suggestions =
	new ConnectApi.QuestionAndAnswersSuggestions();
System.assertEquals(null, suggestions.articles);
suggestions.questions = 'placeholder';
System.assertEquals('placeholder', suggestions.questions);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedPlatformConstructorInitializesPassiveProperties(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.QuestionAndAnswersCapabilityInput input =
	new ConnectApi.QuestionAndAnswersCapabilityInput(bestAnswerId = '0D5000000000001');
System.assertEquals('0D5000000000001', input.bestAnswerId);
System.assertEquals(null, input.questionTitle);
input.questionTitle = 'Solved';
System.assertEquals('Solved', input.questionTitle);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSchemaPassiveDTOGettersAndSetters(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DataCategoryGroupSobjectTypePair pair =
	new Schema.DataCategoryGroupSobjectTypePair();
pair.setDataCategoryGroupName('Products');
pair.setSobject('Knowledge__kav');
System.assertEquals('Products', pair.getDataCategoryGroupName());
System.assertEquals('Knowledge__kav', pair.getSobject());

Schema.DataCategory category = new Schema.DataCategory();
category.name = 'Hardware';
category.label = 'Hardware';
System.assertEquals('Hardware', category.getName());
System.assertEquals('Hardware', category.getLabel());
System.assertEquals(0, category.getChildCategories().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedPlatformFallbackSelectsTypeAwareOverload(t *testing.T) {
	original := generatedPlatformMethodIndex
	generatedPlatformMethodIndex = map[string]map[string][]Method{
		"generated.overload": {
			"pick": {
				{
					Name:       "Generated.Overload.pick",
					ClassName:  "Generated.Overload",
					ReturnType: "Integer",
					Params:     []Param{{Name: "value", Type: "Integer"}},
					IsStatic:   true,
				},
				{
					Name:       "Generated.Overload.pick",
					ClassName:  "Generated.Overload",
					ReturnType: "Boolean",
					Params:     []Param{{Name: "value", Type: "Boolean"}},
					IsStatic:   true,
				},
			},
		},
	}
	defer func() { generatedPlatformMethodIndex = original }()

	machine := New(nil)
	value, handled := machine.generatedPlatformStaticDefault("Generated.Overload.pick", []Value{Bool(true)})
	if !handled || value.Kind != ValueBool || value.Bool {
		t.Fatalf("Boolean overload default = %#v, handled %v; want false Boolean", value, handled)
	}

	value, handled = machine.generatedPlatformStaticDefault("Generated.Overload.pick", []Value{Int(1)})
	if !handled || value.Kind != ValueInt || value.Int != 0 {
		t.Fatalf("Integer overload default = %#v, handled %v; want zero Integer", value, handled)
	}
}

func TestGeneratedPlatformInstanceMethodFallsBackToTypedDefault(t *testing.T) {
	machine := New(nil)
	receiver := Object("ApexPages.IdeaStandardSetController")
	result := &Result{}

	value, handled, err := machine.callValueMember("controller", receiver, "getHasNext", nil, result)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || value.Kind != ValueBool || value.Bool {
		t.Fatalf("getHasNext = %#v, handled %v; want false Boolean default", value, handled)
	}

	value, handled, err = machine.callValueMember("controller", receiver, "getResultSize", nil, result)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || value.Kind != ValueInt || value.Int != 0 {
		t.Fatalf("getResultSize = %#v, handled %v; want zero Integer default", value, handled)
	}

	value, handled, err = machine.callValueMember("controller", receiver, "getListViewOptions", nil, result)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || value.Kind != ValueList || value.Type != "List<SelectOption>" {
		t.Fatalf("getListViewOptions = %#v, handled %v; want typed List<SelectOption>", value, handled)
	}
}

func TestExecSfsqlquerySqlTesterBacksMockRowIterator(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.QuerySqlRow raw = new ConnectApi.QuerySqlRow();
raw.row = new List<Object>{'first'};
ConnectApi.QuerySqlMetadataItem metadata = new ConnectApi.QuerySqlMetadataItem();
metadata.name = 'Name';
sfsqlquery.SqlTester.setMockRows(new List<ConnectApi.QuerySqlRow>{raw});
sfsqlquery.SqlTester.setMockMetadata(new List<ConnectApi.QuerySqlMetadataItem>{metadata});
sfsqlquery.QueryHandle handle = sfsqlquery.QueryHandle.create('query-1', 'default');
sfsqlquery.SqlRowIterator rows = new sfsqlquery.SqlRowIterator(handle);
System.assertEquals('query-1', rows.getQueryId());
System.assertEquals(1, rows.getMetadata().size());
System.assertEquals('Name', rows.getColumnNames().get(0));
System.assertEquals(true, rows.hasNext());
sfsqlquery.Row first = rows.next();
ConnectApi.QuerySqlRow firstRaw = first.getRawRow();
System.assertEquals('first', ((List<Object>)firstRaw.row).get(0));
System.assertEquals(false, rows.hasNext());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSfsqlquerySqlStatementExecuteUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`
sfsqlquery.SqlStatement statement = sfsqlquery.SqlStatement.create('select Name from Account', 'default');
statement.execute();
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" ||
		runtimeErr.Message != `unsupported call "sfsqlquery.SqlStatement.execute local SQL service"` {
		t.Fatalf("error = %#v, want unsupported SqlStatement.execute", err)
	}
}

func TestExecGeneratedPlatformPassiveListWrapperMutatesItems(t *testing.T) {
	program, err := CompileAnonymous(`
CartExtension.CartAdjustmentBasisList items = new CartExtension.CartAdjustmentBasisList();
CartExtension.CartAdjustmentBasis item = new CartExtension.CartAdjustmentBasis();
System.assertEquals(0, items.size());
System.assertEquals(true, items.isEmpty());
items.add(item);
System.assertEquals(1, items.size());
System.assertEquals(false, items.isEmpty());
System.assertEquals(0, items.indexOf(item));
System.assertEquals(item, items.get(0));
Iterator<Object> iter = items.iterator();
System.assertEquals(true, iter.hasNext());
System.assertEquals(item, iter.next());
System.assertEquals(false, iter.hasNext());
items.remove(item);
System.assertEquals(0, items.size());
items.add(item);
items.clear();
System.assertEquals(true, items.isEmpty());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedPlatformOptionalWrapperEmptyAndOf(t *testing.T) {
	program, err := CompileAnonymous(`
CartExtension.CartAdjustmentBasis item = new CartExtension.CartAdjustmentBasis();
CartExtension.OptionalCartAdjustmentBasis present = CartExtension.OptionalCartAdjustmentBasis.of(item);
System.assertEquals(true, present.isPresent());
System.assertEquals(item, present.get());
CartExtension.OptionalCartAdjustmentBasis empty = CartExtension.OptionalCartAdjustmentBasis.empty();
System.assertEquals(false, empty.isPresent());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedPlatformOptionalWrapperGetEmptyIsExplicitUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`
CartExtension.OptionalCartAdjustmentBasis empty = CartExtension.OptionalCartAdjustmentBasis.empty();
empty.get();
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "unsupported call") {
		t.Fatalf("OptionalCartItem.get(empty) error = %v, want explicit unsupported", err)
	}
}

func TestExecSlackTestHarnessLocalFactories(t *testing.T) {
	program, err := CompileAnonymous(`
Slack.State state = new Slack.State();
Slack.TestHarness.Enterprise enterprise = state.createEnterprise('E1', 'Example');
Slack.TestHarness.Team team = state.createTeam('example', enterprise);
Slack.TestHarness.User user = state.createUser('muser', 'M User', team, 'en_US');
Slack.TestHarness.Channel channel = state.createPublicChannel(team, 'general', 'en_US');
Slack.TestHarness.UserSession session = state.createUserSession(user, channel);
System.assertNotEquals(null, session.getState());
System.assertNotEquals(null, session.getUser());
System.assertNotEquals(null, session.getOpenChannel());
System.assertEquals(0, session.getMessageCount());
List<Slack.TestHarness.Message> initialMessages = session.getMessages();
List<Slack.TestHarness.Modal> initialModalStack = session.getModalStack();
System.assertEquals(0, initialMessages.size());
System.assertEquals(0, initialModalStack.size());
System.assertEquals(null, session.getTopModal());
Slack.TestHarness.Home home = session.openAppHome(new Slack.App());
Slack.TestHarness.Channel opened = session.openChannel('C1');
Slack.TestHarness.Message message = session.postMessage('hello');
System.assertNotEquals(null, enterprise);
System.assertNotEquals(null, team);
System.assertNotEquals(null, user);
System.assertNotEquals(null, channel);
System.assertNotEquals(null, session);
System.assertNotEquals(null, home);
System.assertNotEquals(null, session.getAppHome());
System.assertNotEquals(null, opened);
System.assertNotEquals(null, session.getOpenChannel());
System.assertNotEquals(null, message);
System.assertEquals(1, session.getMessageCount());
List<Slack.TestHarness.Message> messages = session.getMessages();
System.assertEquals(1, messages.size());
state.clearAllClientMocks();
session.closeModal();
session.closeAllModals();
List<Slack.TestHarness.Modal> modalStack = session.getModalStack();
System.assertEquals(0, modalStack.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedPlatformFallbackDoesNotMaskExplicitUnsupportedRuntimeMethods(t *testing.T) {
	machine := New(nil)
	result := &Result{}

	content, handled, err := machine.callValueMember("page", newPageReference("/apex/example"), "getContent", nil, result)
	if !handled {
		t.Fatalf("PageReference.getContent was not handled")
	}
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "PageReference.getContent local Visualforce page rendering surface"` || content.Kind != ValueNull {
		t.Fatalf("PageReference.getContent = %#v err=%v, want UnsupportedFeature", content, err)
	}

	for _, args := range [][]Value{
		{String("RSA"), Object("Dom.XmlNode"), String("id"), String("cert")},
		{String("RSA"), Object("Dom.XmlNode"), String("id"), String("cert"), Object("Dom.XmlNode")},
	} {
		_, err = machine.call("Crypto.signXml", args, nil, result)
		if err == nil || err.Error() != `unsupported call "Crypto.signXml local XML signature surface"` {
			t.Fatalf("Crypto.signXml error = %v, want explicit unsupported", err)
		}
	}
}

func TestExecSystemAssertEqualsIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous("system.assertEquals(2, 1 + 1);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemAssertClassAliases(t *testing.T) {
	program, err := CompileAnonymous(`
System.Assert.areEqual(2, 1 + 1);
System.Assert.areNotEqual(3, 1 + 1);
System.Assert.isTrue(1 < 2);
System.Assert.isFalse(2 < 1);
System.Assert.isNull(null);
System.Assert.isNotNull('value');
System.Assert.isInstanceOfType('value', String.class, 'type');
System.Assert.isNotInstanceOfType('value', Account.class, 'type');
SYSTEM.assert.AREEQUAL('trail', 'trail');
Assert.areEqual('short', 'short');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemAssertClassFailures(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "areEqual",
			source: "System.Assert.areEqual('left', 'right', 'mismatch');",
			want:   "expected <left>, actual <right>: mismatch",
		},
		{
			name:   "areNotEqual",
			source: "System.Assert.areNotEqual('same', 'same', 'duplicate');",
			want:   "values should not be equal: <same>: duplicate",
		},
		{
			name:   "isFalse",
			source: "System.Assert.isFalse(true, 'truthy');",
			want:   "assertion failed: truthy",
		},
		{
			name:   "isNull",
			source: "System.Assert.isNull('value', 'not null');",
			want:   "expected null, actual <value>: not null",
		},
		{
			name:   "isNotNull",
			source: "System.Assert.isNotNull(null, 'missing');",
			want:   "value should not be null: missing",
		},
		{
			name:   "isNotInstanceOfType",
			source: "System.Assert.isNotInstanceOfType(new Account(), Account.class, 'type');",
			want:   "expected not instance of <Account>, actual <Account>: type",
		},
		{
			name:   "fail",
			source: "System.Assert.fail('forced');",
			want:   "assertion failed: forced",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) {
				t.Fatalf("error type = %T, want *RuntimeError", err)
			}
			if runtimeErr.Type != "System.AssertException" || runtimeErr.Message != tt.want {
				t.Fatalf("runtime error = (%q, %q), want System.AssertException %q", runtimeErr.Type, runtimeErr.Message, tt.want)
			}
		})
	}
}

func TestExecApexStackLowRiskBehavior(t *testing.T) {
	program, err := CompileAnonymous(`
Apex.Stack stack = new Apex.Stack();
System.assert(stack.empty());
System.assertEquals('one', stack.push('one'));
stack.push('two');
System.assert(!stack.empty());
System.assertEquals('two', stack.peek());
System.assertEquals('two', stack.pop());
System.assertEquals('one', stack.pop());
System.assert(stack.empty());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecApexStackEmptyPopRaisesException(t *testing.T) {
	program, err := CompileAnonymous(`
Apex.Stack stack = new Apex.Stack();
try {
	stack.pop();
	System.assert(false);
} catch (Apex.EmptyStackException e) {
	System.assertEquals('Apex.EmptyStackException', e.getTypeName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecKeywordsAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Object value = NULL;
System.assertEquals(null, value);
Object built = NEW Account(Name = 'Acme');
System.assert(built != NULL);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNumericEqualityCrossIntegerDecimal(t *testing.T) {
	program, err := CompileAnonymous(`
Object integerValue = 100;
Object decimalValue = 100.0;
System.assert(integerValue == decimalValue);
System.assert(!(integerValue != decimalValue));
System.assertEquals(integerValue, decimalValue);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalStringConcatenationStripsInsignificantTrailingZeros(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('12', '' + 12.0);
System.assertEquals('12.34', '' + 12.3400);
System.assertEquals('12.3400', String.valueOf(12.3400));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNumericLiteralSuffixes(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(1.1, 1.1d);
System.assertEquals(2.2, 2.2D);
System.assertEquals(3.3, 3.3f);
System.assertEquals(100.0, 1e2);
System.assertEquals(100.0, 1E2d);
System.assertEquals(10000000000, 10000000000L);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserTypesAndMembersAreCaseInsensitive(t *testing.T) {
	echo, err := CompileAnonymous("return 'ok';")
	if err != nil {
		t.Fatal(err)
	}
	shout, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
caseprobe.Count = 2;
System.assertEquals(2, CASEPROBE.count);
CaseProbe p = new caseprobe();
p.name = 'Ada';
System.assertEquals('Ada', p.NAME);
System.assertEquals('ok', caseprobe.echo());
System.assertEquals('go', p.SHOUT('go'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "CaseProbe",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
		StaticFields: map[string]Field{
			"Count": {Name: "Count", Type: "Integer"},
		},
		Methods: map[string]Method{
			"echo":  {Name: "CaseProbe.echo", ClassName: "CaseProbe", IsStatic: true, ReturnType: "String", Program: echo},
			"shout": {Name: "CaseProbe.shout", ClassName: "CaseProbe", ReturnType: "String", Params: []Param{{Name: "value", Type: "String"}}, Program: shout},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformMembersAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> values = new List<Integer>();
values.ADD(1);
System.assertEquals(1, values.SIZE());
try {
	throw new DmlException('blocked');
} catch (Exception e) {
	System.assertEquals('blocked', e.getMEssage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteStopsWhenContextCanceled(t *testing.T) {
	program, err := CompileAnonymous("System.assert(true);")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	machine := New(nil)
	machine.SetContext(ctx)
	if _, err := machine.Execute(program); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func traceHas(events []trace.Event, name, category string) bool {
	for _, event := range events {
		if event.Name == name && event.Category == category {
			return true
		}
	}
	return false
}

func TestExecVariablesAndDebug(t *testing.T) {
	program, err := CompileAnonymous(`
Integer x = 1 + 1;
x = x * 3;
System.debug('x=' + x);
System.assertEquals(6, x);
`)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	result, err := Execute(program, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "x=6" {
		t.Fatalf("stdout = %q", got)
	}
	if len(result.Debug) != 1 || result.Debug[0] != "x=6" {
		t.Fatalf("debug = %#v", result.Debug)
	}
}

func TestCompileSkipsCommentsAndSafeNavigation(t *testing.T) {
	program, err := CompileAnonymous(`
String value = 'trail';
// A line comment should not become divide tokens.
/* Nor should a block comment. */
System.assertEquals('trail', value?.toString());
String missing = null;
System.assertEquals(null, missing?.toString());
System.assertEquals(null, missing?.length());
System.assertEquals(null, missing?.replace('a', 'b').replace('b', 'c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileApexNotEqualAngleOperator(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(1 <> 2);
System.assert(!(2 <> 2));
System.assert('left' <> 'right');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSafeNavigationAssignmentUsesPlainNull(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Object> values = null;
String providerId = (String) values?.get('providerId');
System.assertEquals(null, providerId);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSafeNavigationReadsFieldAfterMethodCall(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme', ExternalId__c = 'v1');
Map<Id, Account> accounts = new Map<Id, Account>();
accounts.put('001000000000001AAA', account);
String externalId = accounts.get('001000000000001AAA')?.ExternalId__c;
System.assertEquals('v1', externalId);
System.assertEquals(null, accounts.get('001000000000002AAA')?.ExternalId__c);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDateTimeMinusIntegerAndMathExceptionAreCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime current = Datetime.valueOfGmt('2024-02-29 12:00:00');
Datetime next = current + 1;
Datetime prior = current - 5;
System.assertEquals('2024-03-01 12:00:00', String.valueOf(next));
System.assertEquals('2024-02-24 12:00:00', String.valueOf(prior));
Date day = Date.newInstance(2024, 2, 29);
System.assertEquals(Date.newInstance(2024, 3, 1), day + 1);
System.assertEquals(Date.newInstance(2024, 2, 28), day - 1);
Boolean caught = false;
try {
	Integer result = 5 / 0;
} catch (Exception e) {
	caught = true;
	System.assertEquals('System.MathException', e.getTypeName());
	System.assert(e.getMessage().contains('Divide by 0'));
}
System.assert(caught);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDateAndDatetimeComparisonsUseMidnightInstant(t *testing.T) {
	program, err := CompileAnonymous(`
Date day = Date.newInstance(2024, 2, 29);
Datetime midnight = Datetime.newInstanceGmt(2024, 2, 29, 0, 0, 0);
Datetime noon = Datetime.newInstanceGmt(2024, 2, 29, 12, 0, 0);
Datetime prior = Datetime.newInstanceGmt(2024, 2, 28, 23, 59, 59);
System.assert(day == midnight);
System.assert(day < noon);
System.assert(day <= midnight);
System.assert(day > prior);
System.assert(day >= midnight);
System.assert(day > '2024-02-28');
System.assert('2024-03-01' > day);
		`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileStringEndingWithEscapedQuote(t *testing.T) {
	program, err := CompileAnonymous(`String value = 'BYELARUS\''; System.assertEquals('BYELARUS''', value);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapObjectKeysUseUserClassIdentity(t *testing.T) {
	program, err := CompileAnonymous(`
Probe first = new Probe();
first.Key = 'one';
Probe second = new Probe();
second.Key = 'one';
Map<Object, Object> values = new Map<Object, Object>();
values.put(first, 'first');
values.put(second, 'second');
System.assertEquals(2, values.size());
System.assertEquals('first', values.get(first));
System.assertEquals('second', values.get(second));
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "Probe",
		Fields: map[string]Field{"Key": {Name: "Key", Type: "String"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecReturnedMapPreservesUserObjectKeys(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
Map<Probe, String> values = new Map<Probe, String>();
values.put(first, 'first');
values.put(second, 'second');
return values;
	`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Probe first = new Probe();
first.Key = 'same';
Probe second = new Probe();
second.Key = 'same';
Map<Probe, String> values = Holder.make(first, second);
System.assertEquals(2, values.size());
System.assertEquals('first', values.get(first));
System.assertEquals('second', values.get(second));
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "Probe",
		Fields: map[string]Field{"Key": {Name: "Key", Type: "String"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Holder"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "Holder.make",
		ClassName:  "Holder",
		IsStatic:   true,
		ReturnType: "Map<Probe,String>",
		Params: []Param{
			{Name: "first", Type: "Probe"},
			{Name: "second", Type: "Probe"},
		},
		Program: methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapObjectKeysUseUserClassIdentityForSObjectNameShadow(t *testing.T) {
	program, err := CompileAnonymous(`
OrderItem first = new OrderItem();
first.Key = 'one';
OrderItem second = new OrderItem();
second.Key = 'one';
Map<Object, Object> values = new Map<Object, Object>();
values.put(first, 'first');
values.put(second, 'second');
System.assertEquals(2, values.size());
System.assertEquals('first', values.get(first));
System.assertEquals('second', values.get(second));
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Wrapper"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "OrderItem",
		SuperClass: "Wrapper",
		Fields:     map[string]Field{"Key": {Name: "Key", Type: "String"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListOfConcreteObjectsAssignsToListObject(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{ new Account(Name = 'Acme'), new Account(Name = 'Global') };
List<Object> objects = accounts;
System.assertEquals(2, objects.size());
System.assertEquals('Acme', ((Account)objects[0]).Name);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecInheritedMethodDispatchesUnqualifiedVirtualCallToReceiverType(t *testing.T) {
	baseHasIds, err := CompileAnonymous(`return ids() != null;`)
	if err != nil {
		t.Fatal(err)
	}
	childIds, err := CompileAnonymous(`return new Set<Id>{ '001000000000001AAA' };`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base value = new Child();
System.assert(value.hasIds());
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Methods: map[string]Method{
			"hasIds": {
				Name:       "Base.hasIds",
				ClassName:  "Base",
				ReturnType: "Boolean",
				Program:    baseHasIds,
			},
			"ids": {
				Name:       "Base.ids",
				ClassName:  "Base",
				ReturnType: "Set<Id>",
				Modifiers:  []string{"abstract"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Base",
		Methods: map[string]Method{
			"ids": {
				Name:       "Child.ids",
				ClassName:  "Child",
				ReturnType: "Set<Id>",
				Modifiers:  []string{"override"},
				Program:    childIds,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsLongLiteralSuffix(t *testing.T) {
	program, err := CompileAnonymous(`System.assertEquals(9, 9L);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsFinalLocalDeclaration(t *testing.T) {
	program, err := CompileAnonymous(`
final Account insertedOpp = new Account(Name = 'Original');
System.assertEquals('Original', insertedOpp.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsApexCastSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
Object raw = 'trail';
String value = (String)raw;
System.assertEquals('trail', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsListIndexSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> values = new List<String>{'spruce', 'birch'};
System.assertEquals('spruce', values[0]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsTernarySyntax(t *testing.T) {
	program, err := CompileAnonymous(`
String value = true ? 'spruce' : null.toString();
System.assertEquals('spruce', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNullCoalescingSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Integer> values = new Map<String, Integer>();
values.put('spruce', (values.get('spruce') ?? 0) + 1);
System.assertEquals(1, values.get('spruce'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsPrefixIncrementInForUpdate(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
for (Integer i = 0; i < 3; ++i) {
	total += i;
}
System.assertEquals(3, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecForInitializerWithMultipleVariableDeclarations(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
for (Integer i = 0, j = 3; i < j; i++) {
	total += i;
}
System.assertEquals(3, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecForUpdateWithMultipleExpressions(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
for (Integer i = 0, j = 3; i < j; i++, j--) {
	total += i + j;
}
System.assertEquals(6, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTripleEqualsIdentityOperators(t *testing.T) {
	program, err := CompileAnonymous(`
Object left = new Account(Name = 'Acme');
Object same = left;
Object right = new Account(Name = 'Acme');
System.assert(left === same);
System.assert(left !== right);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecObjectEqualityUsesEqualsOverride(t *testing.T) {
	equalsProgram, err := CompileAnonymous(`
Probe that = (Probe) other;
return this.Key == that.Key;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Probe left = new Probe();
left.Key = 'one';
Probe sameValue = new Probe();
sameValue.Key = 'one';
Probe differentValue = new Probe();
differentValue.Key = 'two';
System.assert(left == sameValue);
System.assert(left != differentValue);
System.assert(left !== sameValue);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "Probe",
		Fields: map[string]Field{"Key": {Name: "Key", Type: "String"}},
		Methods: map[string]Method{
			"equals": {
				Name:       "Probe.equals",
				ClassName:  "Probe",
				ReturnType: "Boolean",
				Params:     []Param{{Name: "other", Type: "Object"}},
				Program:    equalsProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringConcatenationUsesToStringOverride(t *testing.T) {
	toStringProgram, err := CompileAnonymous("return 'Probe(' + this.Key + ')';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Probe probe = new Probe();
probe.Key = 'one';
System.assertEquals('value=Probe(one).', 'value=' + probe + '.');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "Probe",
		Fields: map[string]Field{"Key": {Name: "Key", Type: "String"}},
		Methods: map[string]Method{
			"toString": {
				Name:       "Probe.toString",
				ClassName:  "Probe",
				ReturnType: "String",
				Program:    toStringProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringConcatenationFormatsTypeLists(t *testing.T) {
	program, err := CompileAnonymous(`
List<Type> args = new List<Type>{String.class, Integer.class};
System.assertEquals('call(String, Integer)', 'call' + args);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringComparisonOperators(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert('B' > 'A');
System.assert('A' < 'B');
System.assert('A' <= 'A');
System.assert('B' >= 'A');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecBareBlockStatement(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
{
  Integer local = 2;
  total += local;
}
System.assertEquals(2, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileBareConstructorExpressionStatement(t *testing.T) {
	program, err := CompileAnonymous(`new Account(Name = 'Acme');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGroupIsSObjectLike(t *testing.T) {
	program, err := CompileAnonymous(`
SObject record = new Group(Name = 'Queue');
SObject[] records = new List<Group>{new Group(Name = 'Queue 1'), new Group(Name = 'Queue 2')};
System.assertEquals(2, records.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPostfixDecrementExpressionInListIndex(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names = new List<String>{'Account', 'Contact', 'Case'};
Integer objectIdx = names.size() - 1;
System.assertEquals('Case', names[objectIdx--]);
System.assertEquals(1, objectIdx);
System.assertEquals('Contact', names[objectIdx--]);
System.assertEquals(0, objectIdx);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPrefixDecrementExpressionInListIndex(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names = new List<String>{'Account', 'Contact', 'Case'};
Integer objectIdx = names.size();
System.assertEquals('Case', names[--objectIdx]);
System.assertEquals(2, objectIdx);
System.assertEquals('Contact', names[--objectIdx]);
System.assertEquals(1, objectIdx);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapLiteralInitializer(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, String> params = new Map<String, String> { 'orderId' => '001000000000001AAA', 'L\'ANDORRE' => 'AD' };
System.assertEquals('001000000000001AAA', params.get('orderId'));
System.assertEquals('AD', params.get('L\'ANDORRE'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectListAliasMutatesTypedList(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{ new Account(Name = 'One') };
List<SObject> records = accounts;
records.add(new Account(Name = 'Two'));
System.assertEquals(2, accounts.size());
System.assertEquals('Two', accounts[1].get('Name'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsPostfixFieldAccess(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'spruce');
Object raw = account;
System.assertEquals('spruce', ((Account)raw).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecLocalVariablesAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = '001000000000001AAA';
System.assertEquals(accountId, accountid);
accountID = '001000000000002AAA';
System.assertEquals('001000000000002AAA', accountId);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectMembersAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
System.assertEquals('Acme', account.GET('name'));
System.assertEquals(false, account.ISSET('Phone'));
account.PUT('phone', '1112223333');
System.assertEquals(true, account.ISSET('PHONE'));
System.assertEquals('1112223333', account.GET('Phone'));
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertyGetterChainUsesComputedGetterValue(t *testing.T) {
	developerGetter, err := CompileAnonymous(`
if (DeveloperName == null && MasterLabel != null) {
    DeveloperName = MasterLabel.deleteWhitespace();
}
return DeveloperName;
`)
	if err != nil {
		t.Fatal(err)
	}
	qualifiedGetter, err := CompileAnonymous(`
if (QualifiedApiName == null) {
    QualifiedApiName = DeveloperName;
}
return QualifiedApiName;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
MetadataName name = new MetadataName();
name.MasterLabel = 'Large Burger';
System.assertEquals('LargeBurger', name.QualifiedApiName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "MetadataName",
		Fields: map[string]Field{
			"MasterLabel": {Name: "MasterLabel", Type: "String"},
			"DeveloperName": {Name: "DeveloperName", Type: "String", Getter: &Method{
				Name: "MetadataName.getDeveloperName", ClassName: "MetadataName", ReturnType: "String", Program: developerGetter,
			}},
			"QualifiedApiName": {Name: "QualifiedApiName", Type: "String", Getter: &Method{
				Name: "MetadataName.getQualifiedApiName", ClassName: "MetadataName", ReturnType: "String", Program: qualifiedGetter,
			}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedAssignmentThroughGetterPreservesReturnedObjectAlias(t *testing.T) {
	developerGetter, err := CompileAnonymous(`
if (DeveloperName == null && MasterLabel != null) {
    DeveloperName = MasterLabel.deleteWhitespace();
}
return DeveloperName;
`)
	if err != nil {
		t.Fatal(err)
	}
	qualifiedGetter, err := CompileAnonymous(`
if (QualifiedApiName == null) {
    QualifiedApiName = DeveloperName;
}
return QualifiedApiName;
`)
	if err != nil {
		t.Fatal(err)
	}
	masterLabelSetter, err := CompileAnonymous(`
if (String.isNotBlank(MasterLabel) && MasterLabel != value && String.isBlank(Id)) {
    DeveloperName = null;
}
MasterLabel = value;
`)
	if err != nil {
		t.Fatal(err)
	}
	cardGetter, err := CompileAnonymous(`return (MDT_Card)Record;`)
	if err != nil {
		t.Fatal(err)
	}
	recordGetter, err := CompileAnonymous(`
if (Record != null) {
    return Record;
}
Record = new MDT_Card();
return Record;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Controller controller = new Controller();
controller.Card.MasterLabel = 'Large Burger';
System.assertEquals('LargeBurger', controller.Card.QualifiedApiName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{
			Name: "MDT_Base",
			Fields: map[string]Field{
				"Id": {Name: "Id", Type: "String"},
				"MasterLabel": {Name: "MasterLabel", Type: "String", Setter: &Method{
					Name: "MDT_Base.setMasterLabel", ClassName: "MDT_Base", Params: []Param{{Name: "value", Type: "String"}}, Program: masterLabelSetter,
				}},
				"DeveloperName": {Name: "DeveloperName", Type: "String", Getter: &Method{
					Name: "MDT_Base.getDeveloperName", ClassName: "MDT_Base", ReturnType: "String", Program: developerGetter,
				}},
				"QualifiedApiName": {Name: "QualifiedApiName", Type: "String", Getter: &Method{
					Name: "MDT_Base.getQualifiedApiName", ClassName: "MDT_Base", ReturnType: "String", Program: qualifiedGetter,
				}},
			},
		},
		{Name: "MDT_Card", SuperClass: "MDT_Base"},
		{
			Name: "Controller",
			Fields: map[string]Field{
				"Record": {Name: "Record", Type: "MDT_Base", Getter: &Method{
					Name: "Controller.getRecord", ClassName: "Controller", ReturnType: "MDT_Base", Program: recordGetter,
				}},
				"Card": {Name: "Card", Type: "MDT_Card", Getter: &Method{
					Name: "Controller.getCard", ClassName: "Controller", ReturnType: "MDT_Card", Program: cardGetter,
				}},
			},
		},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnhancedForMutatesSObjectsFromGetterReturnedList(t *testing.T) {
	rowsGetter, err := CompileAnonymous(`return Rows;`)
	if err != nil {
		t.Fatal(err)
	}
	updateRows, err := CompileAnonymous(`
Rows = null;
Rows = new List<Account>{ new Account(Name = 'Acme') };
for (Account row : getRows()) {
    row.Name = null;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	runHarness, err := CompileAnonymous(`
controller = new Controller();
controller.Rows = new List<Account>{ new Account(Name = 'Old') };
System.runAs(new User(Id = '005000000000999')) {
    Test.startTest();
    controller.updateRows();
    System.assertEquals(null, controller.Rows[0].Name);
    Test.stopTest();
    System.assertEquals(null, controller.Rows[0].Name);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`Harness.run();`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Controller",
		Fields: map[string]Field{
			"Rows": {Name: "Rows", Type: "List<Account>", Getter: &Method{
				Name: "Controller.getRows", ClassName: "Controller", ReturnType: "List<Account>", Program: rowsGetter,
			}},
		},
		Methods: map[string]Method{
			"getRows":    {Name: "Controller.getRows", ClassName: "Controller", ReturnType: "List<Account>", Program: rowsGetter},
			"updateRows": {Name: "Controller.updateRows", ClassName: "Controller", Program: updateRows},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Harness",
		StaticFields: map[string]Field{
			"controller": {Name: "controller", Type: "Controller", Static: true},
		},
		Methods: map[string]Method{
			"run": {Name: "Harness.run", ClassName: "Harness", IsStatic: true, Program: runHarness},
		},
	}); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedAssignmentThroughMethodReturnedFieldAlias(t *testing.T) {
	getAccount, err := CompileAnonymous(`return ActiveAccount;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Controller controller = new Controller();
controller.ActiveAccount = new Account(ShippingCountry = 'United States', ShippingCountryCode = 'US');
controller.getActiveAccount().ShippingCountry = null;
System.assertEquals(null, controller.ActiveAccount.ShippingCountry);
System.assertEquals(null, controller.ActiveAccount.ShippingCountryCode);
System.assertEquals(null, controller.ActiveAccount.get('ShippingCountry'));
System.assertEquals(null, controller.ActiveAccount.get('ShippingCountryCode'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Controller",
		Fields: map[string]Field{
			"ActiveAccount": {Name: "ActiveAccount", Type: "Account"},
		},
		Methods: map[string]Method{
			"getActiveAccount": {Name: "Controller.getActiveAccount", ClassName: "Controller", ReturnType: "Account", Program: getAccount},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertyGetterRecomputesAfterReturnedSObjectMutation(t *testing.T) {
	getAddress, err := CompileAnonymous(`
Addr address = new Addr();
address.Country = (String) ActiveAccount.get('ShippingCountry');
ShippingAddress = address;
return ShippingAddress;
`)
	if err != nil {
		t.Fatal(err)
	}
	getAccount, err := CompileAnonymous(`return ActiveAccount;`)
	if err != nil {
		t.Fatal(err)
	}
	isAddressCountryNull, err := CompileAnonymous(`return ShippingAddress.Country == null;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Controller controller = new Controller();
controller.ActiveAccount = new Account(ShippingCountry = 'United States');
System.assertEquals('United States', controller.ShippingAddress.Country);
controller.getActiveAccount().ShippingCountry = null;
System.assertEquals(null, controller.ActiveAccount.get('ShippingCountry'));
System.assertEquals(null, controller.ShippingAddress.Country);
System.assertEquals(true, controller.isAddressCountryNull());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Addr",
		Fields: map[string]Field{
			"Country": {Name: "Country", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Controller",
		Fields: map[string]Field{
			"ActiveAccount": {Name: "ActiveAccount", Type: "Account"},
			"ShippingAddress": {
				Name:      "ShippingAddress",
				Type:      "Addr",
				Property:  true,
				Getter:    &Method{Name: "Controller.ShippingAddress.get", ClassName: "Controller", ReturnType: "Addr", Program: getAddress},
				HasSetter: true,
			},
		},
		Methods: map[string]Method{
			"getActiveAccount":     {Name: "Controller.getActiveAccount", ClassName: "Controller", ReturnType: "Account", Program: getAccount},
			"isAddressCountryNull": {Name: "Controller.isAddressCountryNull", ClassName: "Controller", ReturnType: "Boolean", Program: isAddressCountryNull},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNullAddressCountryClearsCountryCodeAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(ShippingCountry = 'United States', ShippingCountryCode = 'US');
account.ShippingCountry = null;
System.assertEquals(null, account.ShippingCountry);
System.assertEquals(null, account.ShippingCountryCode);

Account putAccount = new Account();
putAccount.put('ShippingCountryCode', 'US');
putAccount.ShippingCountry = null;
System.assertEquals(null, putAccount.get('ShippingCountryCode'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesCollectionElementType(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return values.size();`)
	if err != nil {
		t.Fatal(err)
	}
	listStringProgram, err := CompileAnonymous(`return values.size() + 10;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<String> names = new List<String>{'spruce'};
System.assertEquals(11, Util.count(names));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<String>"}}, Program: listStringProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectListParameterCoercionDoesNotLeakIntoCallerOverloadResolution(t *testing.T) {
	getIDsProgram, err := CompileAnonymous(`return new Set<Id>();`)
	if err != nil {
		t.Fatal(err)
	}
	accountProgram, err := CompileAnonymous(`return 'accounts';`)
	if err != nil {
		t.Fatal(err)
	}
	contactProgram, err := CompileAnonymous(`return 'contacts';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{ new Account(Name = 'Acme') };
CollectionUtil.getSObjectIds(accounts);
System.assertEquals('accounts', ProductPricingRequest.pick(accounts));
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if err := machine.RegisterMethod(Method{Name: "CollectionUtil.getSObjectIds", ReturnType: "Set<Id>", Params: []Param{{Name: "records", Type: "List<SObject>"}}, Program: getIDsProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "ProductPricingRequest.pick", ReturnType: "String", Params: []Param{{Name: "records", Type: "List<Account>"}}, Program: accountProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "ProductPricingRequest.pick", ReturnType: "String", Params: []Param{{Name: "records", Type: "List<Contact>"}}, Program: contactProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSObjectTypePreservesConcreteRuntimeType(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>();
System.assertEquals(Account.SObjectType, accounts.getSObjectType());
List<SObject> records = accounts;
System.assertEquals(Account.SObjectType, records.getSObjectType());
records.add(new Account(Name = 'Acme'));
System.assertEquals(Account.SObjectType, records.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListAddHonorsConcreteRuntimeElementType(t *testing.T) {
	program, err := CompileAnonymous(`
Type accountListType = Type.forName('List<Account>');
List<Object> records = (List<Object>)accountListType.newInstance();
Boolean caught = false;
try {
	records.add(new Contact(LastName = 'Child'));
} catch (System.TypeException e) {
	caught = e.getMessage().contains('Collection store exception adding Contact to List<Account>');
}
System.assertEquals(true, caught);
records.add(new Account(Name = 'Acme'));
System.assertEquals(1, records.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSObjectTypePreservesEmptyQueryRuntimeTypeAfterSObjectCast(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> records = (List<SObject>) Database.query('SELECT Id FROM Account WHERE Name = \'Missing\'');
System.assertEquals(Account.SObjectType, records.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSObjectTypePreservesEmptyQueryRuntimeTypeAfterSObjectReturn(t *testing.T) {
	queryProgram, err := CompileAnonymous(`
return Database.query('SELECT Id FROM Account WHERE Name = \'Missing\'');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<SObject> records = Util.queryAccounts();
System.assertEquals(Account.SObjectType, records.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if err := machine.RegisterMethod(Method{Name: "Util.queryAccounts", ReturnType: "List<SObject>", Program: queryProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSObjectTypeInfersEmptySelectorReturnFromSOQL(t *testing.T) {
	selectProgram, err := CompileAnonymous(`
return [SELECT Id FROM Account WHERE Id IN :ids];
	`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Set<Id> ids = new Set<Id>{ Id.valueOf('001B000001DVM9t') };
List<SObject> records = Selector.selectById(ids);
System.assertEquals(Account.SObjectType, records.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if err := machine.RegisterMethod(Method{Name: "Selector.selectById", ReturnType: "List<SObject>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: selectProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnhancedForUsesCustomIterableIterator(t *testing.T) {
	iteratorProgram, err := CompileAnonymous(`
return new List<String>{'first', 'second'}.iterator();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Client client = new Client();
String joined = '';
for (String item : client) {
	joined += item + ';';
}
System.assertEquals('first;second;', joined);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Client",
		Interfaces: []string{"Iterable<String>"},
		Methods: map[string]Method{
			"iterator": {
				Name:       "Client.iterator",
				ClassName:  "Client",
				ReturnType: "Iterator<String>",
				Program:    iteratorProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAddAllUsesCustomIterableIterator(t *testing.T) {
	iteratorProgram, err := CompileAnonymous(`
return new List<String>{'first', 'second'}.iterator();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Client client = new Client();
List<String> values = new List<String>{'zero'};
values.addAll(client);
System.assertEquals(3, values.size());
System.assertEquals('second', values[2]);
Set<String> uniqueValues = new Set<String>{'zero'};
System.assert(uniqueValues.addAll(client));
System.assert(uniqueValues.contains('first'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Client",
		Interfaces: []string{"Iterable<String>"},
		Methods: map[string]Method{
			"iterator": {
				Name:       "Client.iterator",
				ClassName:  "Client",
				ReturnType: "Iterator<String>",
				Program:    iteratorProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecObjectMutationPropagatesIntoSetElementAliases(t *testing.T) {
	addState, err := CompileAnonymous(`
this.states = new Set<String>();
this.states.add(value);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Holder holder = new Holder();
holder.nodes = new Set<Node>();
Node node = new Node();
holder.nodes.add(node);
node.addState('Cart');
for (Node current : holder.nodes) {
	System.assert(current.states.contains('Cart'), String.valueOf(current.states));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Node",
		Fields: map[string]Field{
			"states": {Name: "states", Type: "Set<String>"},
		},
		Methods: map[string]Method{
			"addState": {Name: "Node.addState", ClassName: "Node", Params: []Param{{Name: "value", Type: "String"}}, Program: addState},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		Fields: map[string]Field{
			"nodes": {Name: "nodes", Type: "Set<Node>"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecShiftOperatorsAndCompoundAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
Integer value = 3;
value <<= 2;
System.assertEquals(12, value);
value >>= 1;
System.assertEquals(6, value);
value %= 4;
System.assertEquals(2, value);
System.assertEquals(16, 1 << 4);
Boolean ready = true;
ready &= false;
System.assertEquals(false, ready);
ready |= true;
System.assertEquals(true, ready);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSwitchTypeCaseBindsSObject(t *testing.T) {
	program, err := CompileAnonymous(`
SObject record = new Account(Name = 'Acme');
String name;
switch on record {
    when Contact contact {
        name = contact.LastName;
    }
    when Account account {
        name = account.Name;
    }
    when else {
        name = 'else';
    }
}
System.assertEquals('Acme', name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamedEnumEquals(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(true, Schema.DisplayType.STRING.equals(Schema.DisplayType.STRING));
System.assertEquals(false, Schema.DisplayType.STRING.equals(Schema.DisplayType.DATE));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSwitchMatchesUnqualifiedUserEnumCase(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('provider', SwitchProbe.describe(SwitchProbe.Kind.PROVIDER));
System.assertEquals('license', SwitchProbe.describe(SwitchProbe.Kind.LICENSE));
`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous(`
String out;
switch on kind {
    when PROVIDER {
        out = 'provider';
    }
    when LICENSE {
        out = 'license';
    }
}
return out;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "SwitchProbe.Kind", EnumValues: []string{"PROVIDER", "LICENSE"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "SwitchProbe.describe", ClassName: "SwitchProbe", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "kind", Type: "SwitchProbe.Kind"}}, Program: providerProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesTypedCollectionArgument(t *testing.T) {
	listProgram, err := CompileAnonymous(`return 'list';`)
	if err != nil {
		t.Fatal(err)
	}
	setProgram, err := CompileAnonymous(`return 'set';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<String> fields = new List<String>{'Name'};
System.assertEquals('list', QueryFactory.selectFields(fields));
System.assertEquals('list', QueryFactory.selectFields(new List<String>()));
System.assertEquals('set', QueryFactory.selectFields(new Set<String>{'Name'}));
System.assertEquals('set', QueryFactory.selectFields(new Set<String>()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "Set<String>"}}, Program: setProgram},
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "List<String>"}}, Program: listProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCatchVariableStaticTypeDrivesOverloadResolution(t *testing.T) {
	exceptionProgram, err := CompileAnonymous(`return 'Exception';`)
	if err != nil {
		t.Fatal(err)
	}
	dmlProgram, err := CompileAnonymous(`return 'DmlException';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
    throw new DmlException('blocked');
} catch (Exception e) {
    System.assertEquals('Exception', CatchProbe.describe(e));
}
try {
    throw new DmlException('blocked');
} catch (DmlException e) {
    System.assertEquals('DmlException', CatchProbe.describe(e));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "CatchProbe.describe", ReturnType: "String", Params: []Param{{Name: "value", Type: "Exception"}}, Program: exceptionProgram},
		{Name: "CatchProbe.describe", ReturnType: "String", Params: []Param{{Name: "value", Type: "DmlException"}}, Program: dmlProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionAllowsSetForIterableParameterByElementValues(t *testing.T) {
	iterableProgram, err := CompileAnonymous(`return 'iterable';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('iterable', QueryFactory.selectFields(new Set<String>{'Name'}));
System.assertEquals('iterable', QueryFactory.selectFields(new List<String>{'Name'}));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "QueryFactory.selectFields",
		ReturnType: "String",
		Params:     []Param{{Name: "fields", Type: "Iterable<String>"}},
		Program:    iterableProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesSchemaFieldCollectionElementType(t *testing.T) {
	stringSetProgram, err := CompileAnonymous(`return 'string set';`)
	if err != nil {
		t.Fatal(err)
	}
	stringListProgram, err := CompileAnonymous(`return 'string list';`)
	if err != nil {
		t.Fatal(err)
	}
	fieldSetProgram, err := CompileAnonymous(`return 'field set';`)
	if err != nil {
		t.Fatal(err)
	}
	fieldListProgram, err := CompileAnonymous(`return 'field list';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fieldMap = Schema.SObjectType.Account.fields.getMap();
Set<String> stringSet = new Set<String>{'Name'};
List<String> stringList = new List<String>{'Name'};
Set<Schema.SObjectField> fieldSet = new Set<Schema.SObjectField>{fieldMap.get('Name')};
List<Schema.SObjectField> fieldList = new List<Schema.SObjectField>{fieldMap.get('Name')};

System.assertEquals('string set', QueryFactory.selectFields(stringSet));
System.assertEquals('string list', QueryFactory.selectFields(stringList));
System.assertEquals('field set', QueryFactory.selectFields(fieldSet));
System.assertEquals('field list', QueryFactory.selectFields(fieldList));
System.assertEquals('field set', QueryFactory.selectFields(new Set<Schema.SObjectField>()));
System.assertEquals('field list', QueryFactory.selectFields(new List<Schema.SObjectField>()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	for _, method := range []Method{
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "Set<String>"}}, Program: stringSetProgram},
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "List<String>"}}, Program: stringListProgram},
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "Set<Schema.SObjectField>"}}, Program: fieldSetProgram},
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "List<Schema.SObjectField>"}}, Program: fieldListProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSchemaFieldTokensCompareByObjectAndField(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Account.Name, Account.Name);
System.assert(Account.Name == Account.Name);
System.assert(Account.Name != Account.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPicklistValuesAssignToUnqualifiedPicklistEntryList(t *testing.T) {
	program, err := CompileAnonymous(`
List<PicklistEntry> entries = Account.Rating.getDescribe().getPicklistValues();
System.assertEquals(1, entries.size());
System.assertEquals('Warm', entries[0].getValue());
System.assertEquals('Warm', entries[0].getLabel());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	storage.EnsureStandardObjectFields(&account.Definition)
	account.Definition.Fields["Rating"] = storage.Field{
		APIName:     "Rating",
		Type:        storage.FieldPicklist,
		DisplayType: "PICKLIST",
		PicklistValues: []storage.PicklistValue{{
			Value:   "Warm",
			Label:   "Warm",
			Active:  true,
			Default: true,
		}},
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestVMCoerceAssignableAllowsUnqualifiedPicklistEntry(t *testing.T) {
	machine := New(nil)
	entry := Object("Schema.PicklistEntry")
	entry.Fields["value"] = String("Warm")
	value := List(entry)

	coerced, err := machine.coerceAssignable("List<PicklistEntry>", value)
	if err != nil {
		t.Fatal(err)
	}
	if got := coerced.List[0].Type; got != "Schema.PicklistEntry" {
		t.Fatalf("coerced entry type = %q, want Schema.PicklistEntry", got)
	}
}

func TestVMResolveTypeNameInClassMapsPicklistEntryToSchemaType(t *testing.T) {
	machine := New(nil)
	machine.RegisterClass(Class{Name: "ProductBundling"})

	if got := machine.resolveTypeNameInClass("ProductBundling", "List<PicklistEntry>"); got != "List<Schema.PicklistEntry>" {
		t.Fatalf("resolved type = %q, want List<Schema.PicklistEntry>", got)
	}
}

func TestExecSchemaDisplayTypeReferenceComparesCaseInsensitively(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Schema.DisplayType.Reference, Contact.AccountId.getDescribe().getType());
System.assert(Contact.AccountId.getDescribe().getType() == Schema.DisplayType.Reference);
System.assert(Contact.FirstName.getDescribe().getType() != Schema.DisplayType.Reference);
System.assertEquals('REFERENCE', Schema.DisplayType.Reference.name());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserInfoGetDefaultCurrency(t *testing.T) {
	program, err := CompileAnonymous(`System.assertEquals('USD', UserInfo.getDefaultCurrency());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserInfoOrganizationName(t *testing.T) {
	program, err := CompileAnonymous(`System.assertEquals('GLADE Local Org', UserInfo.getOrganizationName());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLUpdateRequiredFieldToNullFails(t *testing.T) {
	program, err := CompileAnonymous(`
Account acct = new Account(Name = 'Acme');
insert acct;
acct.Name = null;
try {
    update acct;
    System.assert(false, 'expected required field failure');
} catch (Exception e) {
    System.assert(e.getMessage().contains('REQUIRED_FIELD_MISSING'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	storage.EnsureStandardObjectFields(&account.Definition)
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLUpdateRequiredFieldToBlankFails(t *testing.T) {
	program, err := CompileAnonymous(`
Account acct = new Account(Name = 'Acme');
insert acct;
acct.Name = '';
try {
    update acct;
    System.assert(false, 'expected required field failure');
} catch (DmlException e) {
    System.assert(e.getMessage().contains('REQUIRED_FIELD_MISSING'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	storage.EnsureStandardObjectFields(&account.Definition)
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedEnumShortNameFromSiblingSubclass(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
System.assertEquals(QConditionGroup.LogicalOperator.OR_x, LogicalOperator.OR_x);
return LogicalOperator.OR_x;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`System.assertEquals(QConditionGroup.LogicalOperator.OR_x, QOrGroup.value());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "QConditionGroup"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "QConditionGroup.LogicalOperator", EnumValues: []string{"AND_x", "OR_x"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "QOrGroup", SuperClass: "QConditionGroup"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "QOrGroup.value", ClassName: "QOrGroup", ReturnType: "QConditionGroup.LogicalOperator", IsStatic: true, Program: methodProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSchemaFieldSetMember(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.FieldSetMember member = (Schema.FieldSetMember)JSON.deserialize('{"fieldPath":"BillingStreet","label":"Billing Street","required":false,"dbRequired":false}', Schema.FieldSetMember.class);
System.assertEquals('BillingStreet', member.getFieldPath());
System.assertEquals('Billing Street', member.getLabel());
System.assertEquals(false, member.getRequired());
System.assertEquals(false, member.getDbRequired());
FieldSetMember shortMember = (FieldSetMember)JSON.deserialize('{"fieldPath":"Name","label":"Name"}', FieldSetMember.class);
System.assertEquals('Name', shortMember.getFieldPath());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapSObjectFieldKeysRoundTripToSObjectGet(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
Schema.SObjectField idField = fields.get('Id');
Schema.SObjectField nameField = fields.get('Name');
Schema.SObjectField createdDateField = fields.get('CreatedDate');
System.assertNotEquals(null, createdDateField);
Account record = new Account(Id = '001000000000001AAA', Name = 'Acme');
Map<Schema.SObjectField, Object> expected = new Map<Schema.SObjectField, Object>{
  idField => record.Id,
  nameField => record.get('Name')
};
for (Schema.SObjectField fieldToken : expected.keySet()) {
  System.assertEquals(expected.get(fieldToken), record.get(fieldToken));
}
System.assert(record.get(createdDateField) != System.now());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectPutFieldTokenCoercesReferenceID(t *testing.T) {
	program, err := CompileAnonymous(`
SObject record = Account.SObjectType.newSObject();
record.put(Account.OwnerId, UserInfo.getUserId());
Account account = (Account)record;
System.assertEquals(UserInfo.getUserId(), account.OwnerId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalAdditionTreatsNullOperandAsZero(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal total = 0;
Decimal amount;
total += amount;
System.assertEquals(0, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectListSortUsesNameBeforeId(t *testing.T) {
	program, err := CompileAnonymous(`
List<Button__c> buttons = new List<Button__c>{
    new Button__c(Id = 'a00000000000003AAA', Name = 'Third'),
    new Button__c(Id = 'a00000000000001AAA', Name = 'First'),
    new Button__c(Id = 'a00000000000002AAA', Name = 'Second')
};
buttons.sort();
System.assertEquals('First', buttons[0].Name);
System.assertEquals('Second', buttons[1].Name);
System.assertEquals('Third', buttons[2].Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Button__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Button__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectPutNullFieldTokenThrowsNullPointer(t *testing.T) {
	program, err := CompileAnonymous(`
Account record = new Account();
Schema.SObjectField fieldToken = Account.SObjectType.getDescribe().fields.getMap().get('Missing__c');
try {
  record.put(fieldToken, null);
  System.assert(false, 'null field token should throw');
} catch (System.NullPointerException e) {
  System.assertEquals('Argument cannot be null.', e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTaskOwnerDescribeKeepsPolymorphicReferenceTargets(t *testing.T) {
	program, err := CompileAnonymous(`
List<Schema.SObjectType> references = Task.OwnerId.getDescribe().getReferenceTo();
System.assertEquals(2, references.size(), String.valueOf(references));
System.assertEquals('Group', references[0].getDescribe().getName());
System.assertEquals('User', references[1].getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Task")
	storage.EnsureStandardObject(&org, "Group")
	storage.EnsureStandardObject(&org, "User")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedSObjectFieldMapKeysStayDistinct(t *testing.T) {
	program, err := CompileAnonymous(`
Map<Schema.SObjectField, Object> values = new Map<Schema.SObjectField, Object>();
values.put(pkg__Product__c.pkg__RevenueGLAccount__c, 'gl');
values.put(pkg__Product__c.pkg__Entity__c, 'entity');
System.assertEquals(2, values.size(), String.valueOf(values.keySet()));
System.assertEquals('gl', values.get(pkg__Product__c.pkg__RevenueGLAccount__c));
System.assertEquals('entity', values.get(pkg__Product__c.pkg__Entity__c));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__Product__c",
			KeyPrefix: "a12",
			Fields: map[string]storage.Field{
				"pkg__RevenueGLAccount__c": {APIName: "pkg__RevenueGLAccount__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__GLAccount__c"}},
				"pkg__Entity__c":           {APIName: "pkg__Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Entity__c"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedSObjectFieldSetRemoveAllMatchesEquivalentTokens(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> described = pkg__Product__c.SObjectType.getDescribe().fields.getMap();
Map<Schema.SObjectField, Object> defaults = new Map<Schema.SObjectField, Object>{
  described.get('pkg__RevenueGLAccount__c') => 'default gl',
  described.get('pkg__Entity__c') => 'default entity'
};
Map<Schema.SObjectField, Object> custom = new Map<Schema.SObjectField, Object>{
  pkg__Product__c.pkg__RevenueGLAccount__c => 'custom gl'
};
Set<Schema.SObjectField> fields = defaults.keySet().clone();
fields.removeAll(custom.keySet());
System.assertEquals(false, fields.contains(pkg__Product__c.pkg__RevenueGLAccount__c), String.valueOf(fields));
System.assertEquals(true, fields.contains(pkg__Product__c.pkg__Entity__c), String.valueOf(fields));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__Product__c",
			KeyPrefix: "a12",
			Fields: map[string]storage.Field{
				"pkg__RevenueGLAccount__c": {APIName: "pkg__RevenueGLAccount__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__GLAccount__c"}},
				"pkg__Entity__c":           {APIName: "pkg__Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Entity__c"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBuildNamespacedSObjectFromDefaultAndCustomFieldMaps(t *testing.T) {
	program, err := CompileAnonymous(`
Map<Schema.SObjectField, Object> defaults = new Map<Schema.SObjectField, Object>{
  pkg__Product__c.pkg__RevenueGLAccount__c => 'default gl',
  pkg__Product__c.pkg__Entity__c => 'default entity'
};
Map<Schema.SObjectField, Object> custom = new Map<Schema.SObjectField, Object>();
custom.put(pkg__Product__c.pkg__RevenueGLAccount__c, 'custom gl');
custom.put(pkg__Product__c.pkg__Entity__c, 'custom entity');
SObject instance = pkg__Product__c.SObjectType.newSObject(null, true);
Set<Schema.SObjectField> defaultFields = defaults.keySet().clone();
defaultFields.removeAll(custom.keySet());
for (Schema.SObjectField field : defaultFields) {
  instance.put(field, defaults.get(field));
}
for (Schema.SObjectField field : custom.keySet()) {
  instance.put(field, custom.get(field));
}
System.assertEquals('custom gl', instance.get(pkg__Product__c.pkg__RevenueGLAccount__c), String.valueOf(instance.getPopulatedFieldsAsMap()));
System.assertEquals('custom entity', instance.get(pkg__Product__c.pkg__Entity__c), String.valueOf(instance.getPopulatedFieldsAsMap()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__Product__c",
			KeyPrefix: "a12",
			Fields: map[string]storage.Field{
				"pkg__RevenueGLAccount__c": {APIName: "pkg__RevenueGLAccount__c", Type: storage.FieldString},
				"pkg__Entity__c":           {APIName: "pkg__Entity__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCastsSObjectListMapValuesToConcreteSObjectLists(t *testing.T) {
	program, err := CompileAnonymous(`
Id ownerId = '005000000000001AAA';
Account account = new Account(Id = '001000000000001AAA', OwnerId = ownerId, Name = 'Acme');
Map<Id, List<SObject>> raw = new Map<Id, List<SObject>>();
raw.put(ownerId, new List<SObject>{ account });
Map<Id, List<Account>> typed = (Map<Id, List<Account>>)raw;
System.assertNotEquals(null, typed.get(ownerId));
System.assertEquals('Acme', typed.get(ownerId)[0].Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertAppliesDefaultsWhenGeneratedSObjectFieldsAreImplicitNull(t *testing.T) {
	program, err := CompileAnonymous(`
Widget__c rec = new Widget__c();
insert rec;
Widget__c stored = [SELECT Status__c FROM Widget__c LIMIT 1];
System.assertEquals('Pending', stored.Status__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Status__c": {APIName: "Status__c", Type: storage.FieldPicklist, DefaultValue: `"Pending"`},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "Widget__c",
		Fields: map[string]Field{
			"Status__c": {Name: "Status__c", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertedListSObjectFieldTokenValuesStayDistinct(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Group.SObjectType.getDescribe().fields.getMap();
Schema.SObjectField idField = fields.get('Id');
Schema.SObjectField nameField = fields.get('Name');
List<Group> groups = new List<Group>{
  new Group(Name = 'MatcherGroup0', DeveloperName = 'MatcherGroup0', Type = 'Queue'),
  new Group(Name = 'MatcherGroup1', DeveloperName = 'MatcherGroup1', Type = 'Queue')
};
insert groups;
List<Group> rawGroups = new List<Group>{
  new Group(Name = 'ArrayZero', DeveloperName = 'MatcherGroupArray0', Type = 'Queue'),
  new Group(Name = 'ArrayOne', DeveloperName = 'MatcherGroupArray1', Type = 'Queue')
};
System.assertNotEquals(rawGroups[0].get(nameField), rawGroups[1].get(nameField));
SObject[] sobjectArray = rawGroups;
System.assertNotEquals(sobjectArray[0].get(nameField), sobjectArray[1].get(nameField));
System.assertNotEquals(sobjectArray.get(0).get(nameField), sobjectArray.get(1).get(nameField));
insert sobjectArray;
System.assertNotEquals(sobjectArray[0].Id, sobjectArray[1].Id);
System.assertNotEquals(sobjectArray[0].get(nameField), sobjectArray[1].get(nameField));
System.assertNotEquals(groups[0].Id, groups[1].Id);
System.assertNotEquals(groups[0].get(nameField), groups[1].get(nameField));
Map<Schema.SObjectField, Object> expected = new Map<Schema.SObjectField, Object>{
  idField => groups[0].Id,
  nameField => groups[0].get('Name')
};
List<Map<Schema.SObjectField, Object>> expectedList = new List<Map<Schema.SObjectField, Object>>{
  expected,
  new Map<Schema.SObjectField, Object>{
    idField => groups[1].Id,
    nameField => groups[1].get('Name')
  }
};
System.assertEquals(expected.get(idField), expectedList[0].get(idField));
System.assertNotEquals(expectedList[0].get(idField), expectedList[1].get(idField));
System.assertNotEquals(expected.get(idField), groups[1].get(idField));
System.assertNotEquals(expected.get(nameField), groups[1].get(nameField));
List<SObject> swapped = new List<SObject>{groups[1], groups[0]};
System.assertNotEquals(expected.get(idField), swapped[0].get(idField));
System.assertNotEquals(expected.get(nameField), swapped[0].get(nameField));
Boolean firstMatch = true;
for (Schema.SObjectField f : expectedList[0].keySet()) {
  if (swapped[0].get(f) != expectedList[0].get(f)) {
    firstMatch = false;
  }
}
System.assert(!firstMatch);
Boolean mismatch = false;
for (Schema.SObjectField f : expected.keySet()) {
  Object valueToMatch = expected.get(f);
  if (groups[1].get(f) != valueToMatch) {
    mismatch = true;
  }
}
System.assert(mismatch);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLListLiteralPreservesSObjectAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
Account alias = account;
insert new List<Account>{ account };
System.assertNotEquals(null, account.Id);
System.assertEquals(account.Id, alias.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringValueOfSObjectFieldUsesFieldAPIName(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('Name', String.valueOf(Account.Name));
System.assertEquals('AccountNumber', String.valueOf(Account.AccountNumber));
System.assertEquals('{Alpha, Beta}', String.valueOf(new Set<String>{'Alpha', 'Beta'}));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringValueOfSObjectFieldUsesNamespacedFieldAPIName(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
String tokenName = String.valueOf(Account.PrimaryAffiliation__c);
System.assertEquals(Account.PrimaryAffiliation__c.getDescribe().getName(), tokenName);
System.assert(fields.keySet().contains(tokenName.toLowerCase()), tokenName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "NU"
	storage.EnsureStandardObject(&org, "Account")
	account := org.Objects["Account"]
	account.Definition.Fields["PrimaryAffiliation__c"] = storage.Field{
		APIName:          "PrimaryAffiliation__c",
		Type:             storage.FieldReference,
		ReferenceTo:      []string{"Account"},
		RelationshipName: "PrimaryAffiliation__r",
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldTokenCanReferenceLookupTargetField(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.SObjectField fieldToken = Invoice__c.Account__c.Name;
System.assertEquals('Name', String.valueOf(fieldToken));
System.assertEquals('Name', fieldToken.getDescribe().getName());
System.assertEquals('Account', fieldToken.getDescribe().getSObjectType().getDescribe().getName());
Schema.SObjectField relationshipToken = Invoice__c.Account__r.Name;
System.assertEquals('Name', String.valueOf(relationshipToken));
System.assertEquals('Account', relationshipToken.getDescribe().getSObjectType().getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Invoice__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldTokenUsesStandardOverlayFallback(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals('FirstName', String.valueOf(Account.FirstName));")
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	definition := org.Objects["Account"].Definition
	storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
	if _, ok := definition.Fields["FirstName"]; !ok {
		t.Fatalf("standard Account overlay missing FirstName")
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCalculatedFieldDescribeNotCreateableOrUpdateable(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult fullName = Account.FullName__c.getDescribe();
System.assertEquals(true, fullName.isCalculated());
System.assertEquals(false, fullName.isCreateable());
System.assertEquals(false, fullName.isUpdateable());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["FullName__c"] = storage.Field{
		APIName:     "FullName__c",
		Label:       "Full Name",
		Type:        storage.FieldCalculated,
		DisplayType: "STRING",
	}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStandardSObjectDescribeUsesGeneratedOverlayWithoutOrgObject(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Account.SObjectType, Schema.SObjectType.account);
System.assertEquals('Account', Schema.SObjectType.account.getDescribe().getName());
System.assertEquals('AccountNumber', Account.accountnumber.getDescribe().getName());
System.assertEquals('AccountNumber', String.valueOf(Account.SObjectType.fields.accountnumber));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&storage.OrgState{Objects: map[string]storage.ObjectState{}})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStubSObjectDescribeUsesGeneratedOverlayWithoutOrgObject(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(ApexClass.SObjectType, Schema.SObjectType.apexclass);
System.assertEquals('ApexClass', Schema.SObjectType.apexclass.getDescribe().getName());
System.assertEquals('Name', ApexClass.name.getDescribe().getName());
System.assertEquals(Schema.DisplayType.String, ApexClass.Name.getDescribe().getType());
System.assertEquals('Name', String.valueOf(ApexClass.SObjectType.fields.name));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&storage.OrgState{Objects: map[string]storage.ObjectState{}})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestStaticSObjectFieldDefaultsToFieldToken(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "PaymentLine__c",
		StaticFields: map[string]Field{
			"CreatedDate": {Name: "CreatedDate", Type: "Schema.SObjectField", Static: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('CreatedDate', PaymentLine__c.SObjectType.getDescribe().fields.getMap().get('CreatedDate').getDescribe().getName());
System.assertEquals('CreatedDate', PaymentLine__c.CreatedDate.getDescribe().getName());
System.assertEquals(Schema.DisplayType.Datetime, PaymentLine__c.CreatedDate.getDescribe().getType());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["PaymentLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "PaymentLine__c",
			Fields: map[string]storage.Field{
				"CreatedDate": {Type: storage.FieldDateTime},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if err := machine.ResetStatics(); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestClassStaticFieldWinsOverSyntheticSObjectFieldToken(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "CartItem__c",
			KeyPrefix: "a00",
			Fields:    map[string]storage.Field{},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "CartItem",
		StaticFields: map[string]Field{
			"NO_DELETABLE_LINES": {Name: "NO_DELETABLE_LINES", Type: "String", Static: true, InitialValue: String("Cart item contains no item lines to delete.")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals('Cart item contains no item lines to delete.', CartItem.NO_DELETABLE_LINES);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeMapsMatchUnqualifiedNamespaceTokens(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.SObjectType objectType = Schema.getGlobalDescribe().get('Widget__c');
Map<String, Schema.SObjectField> fields = objectType.getDescribe().fields.getMap();
System.assert(fields.containsKey('Thing__c'));
System.assertEquals('pkg__Thing__c', fields.get('Thing__c').getDescribe().getName());

Schema.SObjectType standardType = Schema.getGlobalDescribe().get('pkg__Opportunity');
System.assertNotEquals(null, standardType);
Map<String, Schema.SObjectField> standardFields = standardType.getDescribe().fields.getMap();
System.assertNotEquals(null, standardFields.get('pkg__ContactId'));
System.assertEquals('ContactId', standardFields.get('pkg__ContactId').getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	storage.EnsureStandardObject(&org, "Opportunity")
	org.Objects["pkg__Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Widget__c",
			Fields: map[string]storage.Field{
				"pkg__Thing__c": {APIName: "pkg__Thing__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGlobalDescribeUnqualifiedCustomObjectPrefersCurrentNamespace(t *testing.T) {
	program, err := CompileAnonymous(`
	Map<String, Schema.SObjectType> globalDescribe = Schema.getGlobalDescribe();
	Schema.SObjectType localType = globalDescribe.get('StateTransitionCallback__mdt');
	Map<String, Schema.SObjectField> localFields = localType.getDescribe().fields.getMap();
	System.assertNotEquals(null, localFields.get('pkg__TriggeringTransition__c'));
	Schema.SObjectType dependencyType = Schema.getGlobalDescribe().get('dep__StateTransitionCallback__mdt');
	Map<String, Schema.SObjectField> dependencyFields = dependencyType.getDescribe().fields.getMap();
	System.assertNotEquals(null, dependencyFields.get('dep__TriggeringTransition__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["StateTransitionCallback__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{Fields: map[string]storage.Field{}},
		Records:    make(map[storage.ID]storage.Record),
	}
	org.Objects["pkg__StateTransitionCallback__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__StateTransitionCallback__mdt",
			Fields: map[string]storage.Field{
				"pkg__TriggeringTransition__c": {APIName: "pkg__TriggeringTransition__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__StateTransition__mdt"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["dep__StateTransitionCallback__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "dep__StateTransitionCallback__mdt",
			Fields: map[string]storage.Field{
				"dep__TriggeringTransition__c": {APIName: "dep__TriggeringTransition__c", Type: storage.FieldReference, ReferenceTo: []string{"dep__StateTransition__mdt"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStandardSObjectCastDescribeMapKeepsOpportunityFields(t *testing.T) {
	program, err := CompileAnonymous(`
Opportunity opp = new Opportunity(Name = 'Test Opportunity', Amount = 15000.05,
    CloseDate = Date.today().addDays(-30), StageName = 'Prospecting',
    IsPrivate = false);
SObject record = (SObject)opp;
Map<String, Schema.SObjectField> fields = record.getSObjectType().getDescribe().fields.getMap();
for (String fieldName : new List<String>{
    'CloseDate',
    'ExpectedRevenue',
    'IsPrivate',
    'IqScore',
    'TotalOpportunityQuantity',
    'StageName'
}) {
    System.assertNotEquals(null, fields.get(fieldName), fieldName);
    System.assertNotEquals(null, fields.get(fieldName).getDescribe(), fieldName);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSObjectResultLocalNameStripsOrgNamespace(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = Widget__c.SObjectType.getDescribe();
System.assertEquals('pkg__Widget__c', describe.getName());
System.assertEquals('Widget__c', describe.getLocalName());
System.assertEquals('pkg__', describe.getName().remove(describe.getLocalName()));
DescribeSObjectResult unqualifiedDescribe = Widget__c.SObjectType.getDescribe();
System.assertEquals('pkg__Widget__c', unqualifiedDescribe.getName());
System.assertEquals('Widget__c', unqualifiedDescribe.getLocalName());
System.assertEquals('pkg__', unqualifiedDescribe.getName().remove(unqualifiedDescribe.getLocalName()));
Schema.DescribeFieldResult fieldDescribe = Widget__c.Lookup__c.getDescribe();
System.assertEquals('pkg__Lookup__c', fieldDescribe.getName());
System.assertEquals('Lookup__c', fieldDescribe.getLocalName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Widget__c",
			Fields: map[string]storage.Field{
				"Lookup__c": {APIName: "Lookup__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecForeignNamespacedSObjectDescribeNameKeepsNamespace(t *testing.T) {
	program, err := CompileAnonymous(`
	pkg__OrderItem__c record = new pkg__OrderItem__c();
	Schema.DescribeSObjectResult describe = record.getSObjectType().getDescribe();
	System.assertEquals('pkg__OrderItem__c', describe.getName());
	System.assertEquals('pkg__OrderItem__c', describe.getLocalName());
	Schema.DescribeFieldResult fieldDescribe = pkg__OrderItem__c.pkg__Amount__c.getDescribe();
	System.assertEquals('pkg__Amount__c', fieldDescribe.getName());
	System.assertEquals('pkg__Amount__c', fieldDescribe.getLocalName());
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "namz"
	org.Objects["pkg__OrderItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__OrderItem__c",
			Fields: map[string]storage.Field{
				"pkg__Amount__c": {APIName: "pkg__Amount__c", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringValueOfSObjectTypeUsesOrgNamespace(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('pkg__Widget__c', String.valueOf(Widget__c.SObjectType));
System.assertEquals('pkg__Widget__c', String.valueOf(Widget__c.class));
System.assertEquals('Account', String.valueOf(Account.SObjectType));
System.assertEquals('Account', String.valueOf(Account.class));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Widget__c"},
		Records:    make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedListInitializerPreservesInnerList(t *testing.T) {
	program, err := CompileAnonymous(`
List<List<Contact>> nested = new List<List<Contact>>{ new List<Contact>{ new Contact(LastName = 'One'), new Contact(LastName = 'Two') } };
System.assertEquals(1, nested.size());
System.assertEquals(2, nested[0].size());
System.assertEquals('Two', nested[0][1].LastName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields:  map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecFixedSizeArrayAllocation(t *testing.T) {
	program, err := CompileAnonymous(`
Id[] ids = new Id[2];
System.assertEquals(2, ids.size());
System.assertEquals(null, ids[0]);
ids[1] = '001000000000001AAA';
System.assertEquals('001000000000001AAA', ids[1]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectGetWrongFieldTokenIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectType> globalDescribe = Schema.getGlobalDescribe();
Schema.SObjectType accountType = globalDescribe.get('Account');
SObject inserted = accountType.newSObject();
inserted.put('Name', 'Acme');
insert inserted;
Id accountId = inserted.Id;
SObject queried = Database.query('SELECT Id, Name FROM Account WHERE Id = :accountId LIMIT 1');
System.assertNotEquals(null, queried.Id);
System.assertNotEquals(null, queried.get('Name'));
Map<String, Schema.SObjectField> accountFields = accountType.getDescribe().fields.getMap();
Map<Schema.SObjectField, Object> expected = new Map<Schema.SObjectField, Object>{
  accountFields.get('Id') => queried.Id,
  accountFields.get('Name') => queried.get('Name')
};
for (Schema.SObjectField fieldToken : expected.keySet()) {
  System.assertEquals(expected.get(fieldToken), queried.get(fieldToken));
}
Boolean unqueriedCaught = false;
try {
  queried.get(accountFields.get('CreatedDate'));
} catch (Exception e) {
  unqueriedCaught = true;
}
System.assert(unqueriedCaught);
Boolean caught = false;
try {
  globalDescribe.get('Opportunity').newSObject().get(accountFields.get('Id'));
} catch (Exception e) {
  caught = true;
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLSObjectGetTreatsAuditFieldsAsUnqueried(t *testing.T) {
	program, err := CompileAnonymous(`
Account record = new Account(Name = 'Acme');
insert record;
System.assert(record.CreatedDate != null);
Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
System.assertEquals('Acme', record.get(fields.get('Name')));
Boolean caught = false;
try {
  record.get(fields.get('CreatedDate'));
} catch (Exception e) {
  caught = true;
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLSObjectGetAllowsLastModifiedById(t *testing.T) {
	program, err := CompileAnonymous(`
Account record = new Account(Name = 'Acme');
insert record;
System.assertNotEquals(null, record.LastModifiedById);
System.assertNotEquals(null, record.get(Account.LastModifiedById));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSchemaSObjectTypeFieldsPathReturnsFieldToken(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Account.SObjectType, Schema.Account.SObjectType);
System.assertEquals(Schema.SObjectType.Account, Schema.Account.SObjectType);
System.assertEquals(Contact.LastName, Schema.Contact.SObjectType.fields.lastName);
System.assertEquals(Account.AccountNumber, Schema.Account.SObjectType.fields.AccountNumber);
System.assertEquals(Account.AccountNumber, Account.SObjectType.getDescribe().fields.getMap().get('AccountNumber'));
System.assertEquals(Account.AccountNumber, Account.SObjectType.getDescribe().fields.getMap().get('accountnumber'));
System.assertEquals(null, Account.SObjectType.getDescribe().fields.getMap().get('Contacts'));
Schema.SObjectField missingChildRelationshipField = Account.SObjectType.getDescribe().fields.getMap().get('Contacts');
System.assert(missingChildRelationshipField == null);
if (missingChildRelationshipField == null) {
  System.assert(true);
} else {
  System.assert(false, 'missing child relationship field should enter null branch');
}
String accountTypeName = String.valueOf(Account.class);
Schema.SObjectField globalDescribeMissingChildRelationshipField = Schema.getGlobalDescribe()
  .get(accountTypeName)
  .getDescribe()
  .fields
  .getMap()
  .get('Contacts');
System.assertEquals(null, globalDescribeMissingChildRelationshipField);
System.assert(globalDescribeMissingChildRelationshipField == null);
System.assertEquals(Account.SObjectType, Schema.Account.getSObjectType());
Boolean sawContacts = false;
for (Schema.ChildRelationship relationship : Account.SObjectType.getDescribe().getChildRelationships()) {
  if (relationship.getRelationshipName() == 'Contacts') {
    sawContacts = true;
  }
}
System.assert(sawContacts);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSObjectDoesNotDeriveSelfReferenceChildRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean sawDerivedPlural = false;
for (Schema.ChildRelationship relationship : Self_Link__c.SObjectType.getDescribe().getChildRelationships()) {
  if (relationship.getRelationshipName() == 'SelfLinks__r') {
    sawDerivedPlural = true;
  }
}
System.assertEquals(false, sawDerivedPlural);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Self_Link__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Self_Link__c",
			Label:       "Self Link",
			PluralLabel: "Self Links",
			KeyPrefix:   "a01",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Self_Link__c"}, RelationshipName: "Parent__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringReturnFromUserMethodWorksAsSchemaMapKey(t *testing.T) {
	ctorProgram, err := CompileAnonymous(`this.sType = sType;`)
	if err != nil {
		t.Fatal(err)
	}
	nameProgram, err := CompileAnonymous(`return String.valueOf(sType);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String accountTypeName = new TypeNameHolder(Account.class).name();
System.assertEquals('Account', accountTypeName);
Schema.SObjectField childRelationshipAsField = Schema.getGlobalDescribe()
  .get(accountTypeName)
  .getDescribe()
  .fields
  .getMap()
  .get('Contacts');
System.assertEquals(null, childRelationshipAsField);
System.assert(childRelationshipAsField == null);
Schema.SObjectField nullSafeChildRelationshipAsField = Schema.getGlobalDescribe()
  ?.get(accountTypeName)
  ?.getDescribe()
  ?.fields
  ?.getMap()
  .get('Contacts');
System.assertEquals(null, nullSafeChildRelationshipAsField);
System.assert(nullSafeChildRelationshipAsField == null);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "TypeNameHolder",
		Fields: map[string]Field{
			"sType": {Name: "sType", Type: "Type"},
		},
		Constructors: []Method{{
			Name:          "TypeNameHolder",
			ClassName:     "TypeNameHolder",
			IsConstructor: true,
			Params:        []Param{{Name: "sType", Type: "Type"}},
			Program:       ctorProgram,
		}},
		Methods: map[string]Method{
			"name": {Name: "TypeNameHolder.name", ClassName: "TypeNameHolder", ReturnType: "String", Program: nameProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSObjectRecordTypeMapsUseStableKeys(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = Account.SObjectType.getDescribe();
Map<String, Schema.RecordTypeInfo> byName = describe.getRecordTypeInfosByName();
Map<String, Schema.RecordTypeInfo> byDeveloperName = describe.getRecordTypeInfosByDeveloperName();
Map<Id, Schema.RecordTypeInfo> byId = describe.getRecordTypeInfosById();
Schema.RecordTypeInfo business = byName.get('Business Account');
System.assertEquals('Business', business.getDeveloperName());
System.assertEquals(business, byDeveloperName.get('Business'));
System.assertEquals(business, byDeveloperName.get('business'));
System.assertEquals(business, byId.get(business.getRecordTypeId()));
System.assert(byName.keySet().contains('Business Account'));
System.assert(byId.containsKey(business.getRecordTypeId()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{
		{
			ID:            "012000000000001AAA",
			DeveloperName: "Business",
			Name:          "Business Account",
			Active:        true,
			Available:     true,
			Default:       true,
		},
		{
			ID:            "012000000000002AAA",
			DeveloperName: "Household",
			Name:          "Household Account",
			Active:        true,
			Available:     true,
		},
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectListGetSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> records = new List<SObject>{ new Account(Name = 'Test') };
System.assertEquals(Account.SObjectType, records.getSObjectType());
List<Account> accounts = new List<Account>();
System.assertEquals(Account.SObjectType, accounts.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectMapGetSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
Map<Id, Account> accounts = new Map<Id, Account>();
System.assertEquals(Account.SObjectType, accounts.getSObjectType());

Map<Id, SObject> assignedGeneric = accounts;
System.assertEquals(Account.SObjectType, assignedGeneric.getSObjectType());

Map<Id, SObject> genericRecords = new Map<Id, SObject>();
try {
  genericRecords.getSObjectType();
  System.assert(false, 'expected TypeException');
} catch (System.TypeException e) {
  System.assert(true);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFrameworkQualifiedMethodKeysEqualTreatsSObjectListTypesCovariant(t *testing.T) {
	machine := New(nil)
	left := Object("framework_QualifiedMethod")
	left.Fields["typeName"] = String("DML__sfdc_ApexStub")
	left.Fields["methodName"] = String("updateRecords")
	left.Fields["methodArgTypes"] = List(platformScalar("Type", "List<Account>"))

	right := Object("framework_QualifiedMethod")
	right.Fields["typeName"] = String("DML__sfdc_ApexStub")
	right.Fields["methodName"] = String("updateRecords")
	right.Fields["methodArgTypes"] = List(platformScalar("Type", "List<SObject>"))

	matched, handled := machine.frameworkQualifiedMethodKeysEqual(left, right)
	if !handled || !matched {
		t.Fatalf("frameworkQualifiedMethodKeysEqual matched=%v handled=%v", matched, handled)
	}
}

func TestFrameworkMatchesAllArgsTreatsAnySObjectAsSObjectListElementMatcher(t *testing.T) {
	machine := New(nil)
	methodArg := Object("framework_MethodArgValues")
	records := List(Object("Account"))
	records.Type = "List<Account>"
	methodArg.Fields["argValues"] = List(records)
	matchers := List(Object("fflib_MatcherDefinitions.AnySObject"))

	matched, handled, err := machine.frameworkMatchesAllArgs(methodArg, matchers)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !matched {
		t.Fatalf("frameworkMatchesAllArgs matched=%v handled=%v", matched, handled)
	}
}

func TestFrameworkMatcherEquivalentTreatsPagePathsCaseInsensitive(t *testing.T) {
	if !frameworkMatcherEquivalent(String("/testPage"), String("/TESTPAGE")) {
		t.Fatal("page path strings should match without case sensitivity")
	}
	if frameworkMatcherEquivalent(String("testPage"), String("TESTPAGE")) {
		t.Fatal("non-path strings should remain case-sensitive")
	}
}

func TestExecInstanceOfSObjectCollectionGenerics(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{new Account(Name = 'Test')};
Object accountObject = accounts;
System.assert(accountObject instanceof List<SObject>, 'List<Account> should be List<SObject>');
System.assert(accountObject instanceOf List<Account>, 'List<Account> should be List<Account>');
System.assert(!(accountObject instanceof List<Contact>), 'List<Account> should not be List<Contact>');

List<AggregateResult> aggregateRows = new List<AggregateResult>();
Object aggregateObject = aggregateRows;
System.assert(aggregateObject instanceof List<SObject>, 'List<AggregateResult> should be List<SObject>');

	List<String> names = new List<String>{'Test'};
	Object namesObject = names;
	System.assert(!(namesObject instanceof List<SObject>), 'List<String> should not be List<SObject>');
	System.assert(namesObject instanceof List<Object>, 'List<String> should be List<Object>');

	Set<String> stringSet = new Set<String>{'foo'};
	Object stringSetObject = stringSet;
	System.assert(!(stringSetObject instanceof Set<Id>), 'Set<String> should not be Set<Id>');
	System.assert(stringSetObject instanceof Set<Object>, 'Set<String> should be Set<Object>');

	Map<String, Account> byName = new Map<String, Account>{'Test' => new Account(Name = 'Test')};
	Object mapObject = byName;
	System.assert(mapObject instanceof Map<String, SObject>, 'Map<String,Account> should be Map<String,SObject>');
	System.assert(!(mapObject instanceof Map<Integer, SObject>), 'Map<String,Account> should not be Map<Integer,SObject>');

	List<LocalModel> models = new List<LocalModel>{new LocalModel()};
	List<Object> modelObjects = models;
	System.assert(modelObjects instanceof List<LocalModel>, 'List<Object> should keep the runtime custom element type');
	System.assert(!(modelObjects instanceof List<String>), 'List<Object> runtime custom element type should not match String');
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInstanceOfNullCollectionIsFalse(t *testing.T) {
	program, err := CompileAnonymous(`
Object values = null;
System.assert(!(values instanceof List<Object>));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestEvalInstanceOfNullHonorsLegacyAPIVersion(t *testing.T) {
	machine := New(nil)
	machine.currentMethod = Method{Name: "Legacy.run", APIVersion: "31.0"}
	if got := machine.evalInstanceOf(Null, "List<SObject>"); got.Kind != ValueBool || !got.Bool {
		t.Fatalf("legacy null instanceof = %v, want true", got)
	}
	machine.currentMethod = Method{Name: "Modern.run", APIVersion: "32.0"}
	if got := machine.evalInstanceOf(Null, "List<SObject>"); got.Kind != ValueBool || got.Bool {
		t.Fatalf("modern null instanceof = %v, want false", got)
	}
}

func TestExecInstanceOfHonorsNumericWidening(t *testing.T) {
	program, err := CompileAnonymous(`
Integer count = 3;
Long longer = 3L;
Decimal amount = 3.5;
System.assert(count instanceof Decimal);
System.assert(count instanceof Double);
System.assert(longer instanceof Decimal);
System.assert(amount instanceof Double);
System.assert(!(amount instanceof Integer));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInstanceOfHonorsScalarRuntimeTypes(t *testing.T) {
	program, err := CompileAnonymous(`
Object longValue = 3L;
Object integerValue = 3;
Object dateValue = Date.today();
Object idString = '001000000000001AAA';
Id typedId = '001000000000001AAA';
System.assert(longValue instanceof Long);
System.assert(!(longValue instanceof Integer));
System.assert(integerValue instanceof Integer);
System.assert(dateValue instanceof Datetime);
System.assert(idString instanceof Id);
System.assert(typedId instanceof String);
System.assert(!('bob' instanceof Id));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if got := machine.evalInstanceOf(platformScalar("Id", "001000000000001AAA"), "String"); got.Kind != ValueBool || !got.Bool {
		t.Fatalf("platform Id instanceof String = %v, want true", got)
	}
}

func TestFrameworkNativeMatcherPrimitiveAndReferenceSemantics(t *testing.T) {
	machine := New(nil)
	one := List(String("bob"), String("tom"))
	two := List(String("bob"), String("tom"))
	refEq := Object("framework_MatcherDefinitions.RefEq")
	refEq.Fields["toMatch"] = one
	if matched, _, err := machine.frameworkMatcherMatches(refEq, one); err != nil || !matched {
		t.Fatalf("RefEq same reference matched=%v err=%v", matched, err)
	}
	if matched, _, err := machine.frameworkMatcherMatches(refEq, two); err != nil || matched {
		t.Fatalf("RefEq equal different list matched=%v err=%v", matched, err)
	}

	anyDatetime := Object("framework_MatcherDefinitions.AnyDatetime")
	if matched, _, err := machine.frameworkMatcherMatches(anyDatetime, platformScalar("Date", "2024-02-29")); err != nil || !matched {
		t.Fatalf("AnyDatetime date matched=%v err=%v", matched, err)
	}
	if matched, _, err := machine.frameworkMatcherMatches(anyDatetime, platformScalar("Datetime", "2024-02-29T12:34:56Z")); err != nil || !matched {
		t.Fatalf("AnyDatetime datetime matched=%v err=%v", matched, err)
	}

	anyInteger := Object("framework_MatcherDefinitions.AnyInteger")
	longValue := Int(9)
	longValue.Type = "Long"
	if matched, _, err := machine.frameworkMatcherMatches(anyInteger, Int(9)); err != nil || !matched {
		t.Fatalf("AnyInteger integer matched=%v err=%v", matched, err)
	}
	if matched, _, err := machine.frameworkMatcherMatches(anyInteger, longValue); err != nil || matched {
		t.Fatalf("AnyInteger long matched=%v err=%v", matched, err)
	}
	if matched, _, err := machine.frameworkMatcherMatches(anyInteger, Decimal(9.99)); err != nil || matched {
		t.Fatalf("AnyInteger decimal matched=%v err=%v", matched, err)
	}
}

func TestExecStringEqualityOperatorIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert('Prepayment refunds are not allowed.' == 'Prepayment Refunds are not allowed.');
System.assert(!('Prepayment refunds are not allowed.' != 'Prepayment Refunds are not allowed.'));
System.assert('areaOfSpecialty' == 'AreaOfSpecialty');
System.assert(new List<String>{'A'}.contains('A'));
System.assert(!new List<String>{'A'}.contains('a'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalArithmeticSuppressesBinaryFloatNoise(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal balance = 100 - 91.63;
System.assertEquals(8.37, balance);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecLeadingDotDecimalLiteral(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal half = .5;
System.assertEquals(0.5, half);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNullableBooleanLogicalOperands(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean flag;
System.assertEquals(false, flag && true);
System.assertEquals(true, !flag);
Boolean other = true;
System.assertEquals(true, flag || other);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecLogicalOrShortCircuitsTypedNullGetter(t *testing.T) {
	childGetter, err := CompileAnonymous(`return Child;`)
	if err != nil {
		t.Fatal(err)
	}
	checkProgram, err := CompileAnonymous(`return this.Child == null || String.isBlank(this.Child.Name);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Controller controller = new Controller();
System.assertEquals(true, controller.check());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{
			Name: "Child",
			Fields: map[string]Field{
				"Name": {Name: "Name", Type: "String"},
			},
		},
		{
			Name: "Controller",
			Fields: map[string]Field{
				"Child": {Name: "Child", Type: "Child", Getter: &Method{
					Name: "Controller.getChild", ClassName: "Controller", ReturnType: "Child", Program: childGetter,
				}},
			},
			Methods: map[string]Method{
				"check": {Name: "Controller.check", ClassName: "Controller", ReturnType: "Boolean", Program: checkProgram},
			},
		},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGetterAssignmentWritesBackingFieldWithoutCallingSetter(t *testing.T) {
	tokenGetter, err := CompileAnonymous(`
if (Token == null) {
    Token = 'loaded';
}
return Token;
`)
	if err != nil {
		t.Fatal(err)
	}
	tokenSetter, err := CompileAnonymous(`
SetterCalled = true;
Token = value;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Controller controller = new Controller();
System.assertEquals('loaded', controller.Token);
System.assertEquals(null, controller.SetterCalled);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Controller",
		Fields: map[string]Field{
			"SetterCalled": {Name: "SetterCalled", Type: "Boolean"},
			"Token": {Name: "Token", Type: "String", Getter: &Method{
				Name: "Controller.getToken", ClassName: "Controller", ReturnType: "String", Program: tokenGetter,
			}, Setter: &Method{
				Name: "Controller.setToken", ClassName: "Controller", Params: []Param{{Name: "value", Type: "String"}}, Program: tokenSetter,
			}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssignmentExpressionInCallArgument(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, List<String>> values = new Map<String, List<String>>();
List<String> current = values.get('items');
if (current == null) {
  values.put('items', current = new List<String>());
}
current.add('one');
System.assertEquals(1, current.size());
System.assertEquals(1, values.get('items').size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMultiplicativeCompoundFieldAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account();
account.AnnualRevenue = 10;
account.AnnualRevenue *= 2;
System.assertEquals(20, account.AnnualRevenue);
account.AnnualRevenue /= 4;
System.assertEquals(5, account.AnnualRevenue);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecIncrementDottedFieldTarget(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Account> accounts = new Map<String, Account>{
	'one' => new Account(AnnualRevenue = 2)
};
accounts.get('one').AnnualRevenue++;
System.assertEquals(3, accounts.get('one').AnnualRevenue);
accounts.get('one').AnnualRevenue--;
System.assertEquals(2, accounts.get('one').AnnualRevenue);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecChainedIndexedFieldAssignmentExpression(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{ new Account(), new Account() };
accounts[0].Fax = accounts[1].Fax = '1112223333';
System.assertEquals('1112223333', accounts[0].Fax);
System.assertEquals('1112223333', accounts[1].Fax);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesTypedNullVariables(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return 1;`)
	if err != nil {
		t.Fatal(err)
	}
	listIDProgram, err := CompileAnonymous(`return 2;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Id> ids = null;
System.assertEquals(2, Util.count(ids));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Id>"}}, Program: listIDProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLListAssignmentKeepsDeclaredType(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return 1;`)
	if err != nil {
		t.Fatal(err)
	}
	listStringProgram, err := CompileAnonymous(`return 2;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> accounts = [SELECT Id FROM Account LIMIT 1];
System.assertEquals(1, Util.count(accounts));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}, Records: map[storage.ID]storage.Record{}}
	machine := New(nil)
	machine.Org = &org
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<String>"}}, Program: listStringProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExpandSOQLBindsKeepsBooleanAndNullLiterals(t *testing.T) {
	machine := New(nil)
	got, err := machine.expandSOQLBindsWith(
		"SELECT Id FROM Account WHERE TaxExempt__c = : true AND IsDeleted = :false AND ParentId = :NULL AND Id IN :ids",
		func(name string) (Value, error) {
			if name == "ids" {
				return Value{Kind: ValueList, List: []Value{{Kind: ValueString, Text: "001000000000001"}}}, nil
			}
			return Null, errors.New("unexpected lookup")
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT Id FROM Account WHERE TaxExempt__c = true AND IsDeleted = false AND ParentId = null AND Id IN ('001000000000001')"
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestExpandSOQLBindsEvaluatesIndexedMemberExpression(t *testing.T) {
	machine := New(nil)
	first := Object("Account")
	first.Fields["Id"] = platformScalar("Id", "001000000000001AAA")
	second := Object("Account")
	second.Fields["Id"] = platformScalar("Id", "001000000000002AAA")
	machine.Globals["accounts"] = List(first, second)
	got, err := machine.expandSOQLBinds("SELECT Id FROM Account WHERE Id = : accounts [ 1 ] . Id")
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT Id FROM Account WHERE Id = '001000000000002AAA'"
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestExpandSOQLBindsEvaluatesChainedStaticCallExpression(t *testing.T) {
	machine := New(nil)
	machine.fakeNow = time.Date(2026, 5, 13, 10, 30, 0, 0, time.UTC)
	got, err := machine.expandSOQLBinds("SELECT Id FROM FlowdownQueue__c WHERE LastModifiedDate <= :DateTime.now().addDays(-1)")
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT Id FROM FlowdownQueue__c WHERE LastModifiedDate <= 2026-05-12T10:30:00Z"
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestExpandSOQLBindsEvaluatesInstanceMethodCall(t *testing.T) {
	program, err := CompileAnonymous(`return IdValue;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Line", Fields: map[string]Field{
		"IdValue": {Name: "IdValue", Type: "Id"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Line.getId", ClassName: "Line", ReturnType: "Id", Program: program}); err != nil {
		t.Fatal(err)
	}
	line := Object("Line")
	line.Fields["IdValue"] = platformScalar("Id", "a00000000000001AAA")
	machine.Globals["line"] = line
	got, err := machine.expandSOQLBinds("SELECT Id FROM PaymentLine__c WHERE Id = :line.getId()")
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT Id FROM PaymentLine__c WHERE Id = 'a00000000000001AAA'"
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestExecSOQLSingleSObjectAssignmentAndReturn(t *testing.T) {
	selectorProgram, err := CompileAnonymous(`return [SELECT Id, Name FROM Account LIMIT 1];`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account assigned;
assigned = [SELECT Id, Name, Custom__c FROM Account LIMIT 1];
System.assertEquals('Acme', assigned.Name);
System.assertEquals('selected', assigned.Custom__c);
Account returned = Selector.get();
System.assertEquals('Acme', returned.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := singleSOQLAssignmentOrg()
	machine.Org = &org
	if err := machine.RegisterClass(Class{Name: "Selector"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Selector.get", ReturnType: "Account", Program: selectorProgram, ClassName: "Selector", IsStatic: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDynamicSOQLMarksQueriedNamespacedField(t *testing.T) {
	program, err := CompileAnonymous(`
Contact contact = (Contact)Database.query('SELECT Id, pkg__SelectedDate__c FROM Contact LIMIT 1');
System.assertEquals(Date.newInstance(2026, 5, 2), contact.pkg__SelectedDate__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
			"pkg__SelectedDate__c": {APIName: "pkg__SelectedDate__c", Type: storage.FieldDate},
		}},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{
				"pkg__SelectedDate__c": storage.DateValue("2026-05-02"),
			}},
		},
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLQueriedUnsetSystemFieldIsAccessibleNull(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = [SELECT Id, LastModifiedById FROM Account WHERE Name = 'Acme' LIMIT 1];
System.assertEquals(null, account.LastModifiedById);
System.assertEquals(null, account.get('Account.LastModifiedById'));
System.assertEquals('LastModifiedById', Account.SObjectType.fields.getMap().get('LastModifiedById').getDescribe().getName());
System.assertEquals('LastModifiedById', Account.SObjectType.fields.getMap().get('Account.LastModifiedById').getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := singleSOQLAssignmentOrg()
	machine := New(nil)
	machine.Org = &org
	result, err := machine.Execute(program)
	t.Logf("debug=%v", result.Debug)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldMapSynthesizesStandardFieldToken(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectType> globalDescribe = Schema.getGlobalDescribe();
Schema.SObjectType boardCertification = globalDescribe.get('Credentialification');
Map<String, Schema.SObjectField> fields = boardCertification.getDescribe().fields.getMap();
Schema.SObjectField createdBy = fields.get('CreatedById');
System.assertNotEquals(null, createdBy);
System.assertEquals('CreatedById', createdBy.getDescribe().getName());
System.assertEquals('CreatedById', fields.get('Credentialification.CreatedById').getDescribe().getName());
System.assertEquals(null, fields.get('PractitionerId'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Credentialification"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Credentialification",
			Fields:  map[string]storage.Field{},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	result, err := machine.Execute(program)
	t.Logf("debug=%v", result.Debug)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldMapDoesNotSynthesizeUnknownMetadataField(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Widget__c.SObjectType.getDescribe().fields.getMap();
System.assertEquals(false, fields.containsKey('Missing__c'));
System.assertEquals(null, fields.get('Missing__c'));
System.assertEquals(null, fields.get('Widget__c.Missing__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Widget__c",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	result, err := machine.Execute(program)
	t.Logf("debug=%v", result.Debug)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldMapAcceptsNamespaceAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Credentialing_Workflow__c.SObjectType.getDescribe().fields.getMap();
System.assertEquals(Credentialing_Workflow__c.ChecklistNotes__c, fields.get('verifiable__ChecklistNotes__c'));
System.assertEquals(Credentialing_Workflow__c.ChecklistNotes__c, fields.get('VERIFIABLE__CHECKLISTNOTES__C'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "verifiable"
	org.Objects["Credentialing_Workflow__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Credentialing_Workflow__c",
			Fields: map[string]storage.Field{
				"ChecklistNotes__c": {APIName: "ChecklistNotes__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	result, err := machine.Execute(program)
	t.Logf("debug=%v", result.Debug)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldMapKeySetOrderUsesCanonicalFieldsBeforeAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
List<String> names = new List<String>(fields.keySet());
System.assertNotEquals(null, fields.get('nu__primaryaffiliation__c'));
System.assertEquals('NU__PrimaryAffiliation__c', fields.get('nu__primaryaffiliation__c').getDescribe().getName());
System.assert(names.indexOf('ownerid') < names.indexOf('nu__primaryaffiliation__c'), String.valueOf(names));
System.assert(names.indexOf('parentid') < names.indexOf('nu__primaryaffiliation__c'), String.valueOf(names));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "NU"
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"OwnerId":               {APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
				"ParentId":              {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent"},
				"PrimaryAffiliation__c": {APIName: "PrimaryAffiliation__c", Type: storage.FieldReference, ReferenceTo: []string{"Affiliation__c"}, RelationshipName: "PrimaryAffiliation__r"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldMapIncludesReferenceRelationshipAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = License_Verification__c.SObjectType.getDescribe().fields.getMap();
System.assertEquals(null, fields.get('createdby'));
Schema.DescribeFieldResult createdBy = fields.get('createdbyid').getDescribe();
System.assertEquals('CreatedById', createdBy.getName());
System.assertEquals(Schema.SOAPType.ID, createdBy.getSoapType());
System.assertEquals('CreatedBy', createdBy.getRelationshipName());
Schema.DescribeFieldResult provider = fields.get('Provider__c').getDescribe();
System.assertEquals('NU__Provider__c', provider.getName());
System.assertEquals('NU__Provider__r', provider.getRelationshipName());
Schema.DescribeFieldResult providerRelationship = fields.get('NU__Provider__r').getDescribe();
System.assertEquals('NU__Provider__c', providerRelationship.getName());
System.assertEquals('NU__Provider__r', providerRelationship.getRelationshipName());
Schema.DescribeFieldResult billTo = fields.get('BillTo__c').getDescribe();
System.assertEquals('NU__BillTo__r', billTo.getRelationshipName());
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "NU"
	org.Objects["License_Verification__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "License_Verification__c",
			Fields: map[string]storage.Field{
				"Provider__c": {APIName: "Provider__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Provider__r"},
				"BillTo__c":   {APIName: "BillTo__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Carts"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldMapValuesExposeNamespacedCustomMetadataParentRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = StateTransitionCallback__mdt.SObjectType.getDescribe().fields.getMap();
String relationshipName = StateTransitionCallback__mdt.TriggeringTransition__c.getDescribe().getRelationshipName();
System.assertEquals('pkg__TriggeringTransition__r', relationshipName);
System.assertNotEquals(null, fields.get(relationshipName));
Boolean found = false;
for (Schema.SObjectField field : fields.values()) {
    Schema.DescribeFieldResult describe = field.getDescribe();
    if (describe.getRelationshipName() == relationshipName && describe.getReferenceTo().size() > 0) {
        found = true;
    }
}
System.assertEquals(true, found);
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["StateTransition__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "StateTransition__mdt",
			Metadata: map[string]string{"kind": "customMetadata"},
			Fields: map[string]storage.Field{
				"QualifiedApiName": {APIName: "QualifiedApiName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["StateTransitionCallback__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "StateTransitionCallback__mdt",
			Metadata: map[string]string{"kind": "customMetadata"},
			Fields: map[string]storage.Field{
				"TriggeringTransition__c": {
					APIName:          "TriggeringTransition__c",
					Type:             storage.FieldReference,
					ReferenceTo:      []string{"StateTransition__mdt"},
					RelationshipName: "TriggeringTransition__r",
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldMapDoesNotSynthesizeMissingScalarCustomFieldOnPartialCustomObject(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Invoice__c.SObjectType.getDescribe().fields.getMap();
Schema.DescribeFieldResult known = fields.get('Known__c').getDescribe();
System.assertEquals('Known__c', known.getName());
System.assertEquals(null, fields.get('TotalPayment__c'));
System.assertEquals(null, fields.get('InexistentField__c'));
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Invoice__c",
			Fields: map[string]storage.Field{
				"Known__c": {APIName: "Known__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeStringFieldWithRelationshipLabelDoesNotExposeRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult jigsaw = Contact.JigsawContactId.getDescribe();
System.assertEquals(0, jigsaw.getReferenceTo().size());
System.assertEquals(null, jigsaw.getRelationshipName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldMapDoesNotSynthesizeMissingCustomRelationshipOnPartialCustomObject(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = pkg__Product__c.SObjectType.getDescribe().fields.getMap();
System.assertEquals(null, fields.get('pkg__Event2__c'));
System.assertEquals(null, fields.get('pkg__Event2__r'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Product__c",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["pkg__Event__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Event__c",
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectGetSObjectUsesSyntheticReferenceFieldRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account();
User user = new User(Id = UserInfo.getUserId(), Email = 'created@example.test');
account.putSObject('CreatedBy', user);
Schema.SObjectField createdById = Schema.SObjectType.Account.fields.getMap().get('CreatedById');
System.assertEquals('created@example.test', (String) account.getSObject(createdById).get('Email'));
System.assertEquals(null, new Account(CreatedById = UserInfo.getUserId()).getSObject(createdById));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "User")
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecContactChildRelationshipSubqueriesStaySeparated(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
insert account;
Contact contact = new Contact(LastName = 'Provider', AccountId = account.Id);
insert contact;
Event eventRecord = new Event(Subject = 'license', WhoId = contact.Id);
insert eventRecord;
Case caseRecord = new Case(Subject = 'case', ContactId = contact.Id);
insert caseRecord;
Asset assetRecord = new Asset(Name = 'asset', ContactId = contact.Id);
insert assetRecord;

Contact queried = [
  SELECT Id,
    (SELECT Id, Subject FROM Events),
    (SELECT Id, Subject FROM Cases),
    (SELECT Id, Name FROM Assets)
  FROM Contact
  WHERE Id = :contact.Id
  LIMIT 1
];
System.assertEquals(1, queried.getSObjects('Events').size());
System.assertEquals(1, queried.getSObjects('Cases').size());
System.assertEquals(1, queried.getSObjects('Assets').size());
System.assertEquals('Event', queried.getSObjects('Events')[0].getSObjectType().getDescribe().getName());
System.assertEquals('Case', queried.getSObjects('Cases')[0].getSObjectType().getDescribe().getName());
System.assertEquals('Asset', queried.getSObjects('Assets')[0].getSObjectType().getDescribe().getName());
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	for _, objectName := range []string{"Account", "Asset", "Case", "Contact", "Event", "User"} {
		storage.EnsureStandardObject(&org, objectName)
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSObjectIncludesCustomChildRelationships(t *testing.T) {
	program, err := CompileAnonymous(`
List<Schema.ChildRelationship> relationships = Account.SObjectType.getDescribe().getChildRelationships();
Boolean found = false;
for (Schema.ChildRelationship relationship : relationships) {
    if (relationship.getChildSObject() == Invoice__c.SObjectType &&
        relationship.getField().getDescribe().getName() == 'Account__c' &&
        relationship.getRelationshipName() == 'Invoices__r') {
        found = true;
    }
}
System.assert(found, 'custom child relationship');
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Invoice__c",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r", ChildRelationshipName: "Invoices__r"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeChildRelationshipMatchesNamespacedSObjectTypeToken(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.SObjectType childType = Schema.getGlobalDescribe().get('pkg__Invoice__c');
System.assertNotEquals(null, childType);
System.assertEquals(childType, Invoice__c.SObjectType);
SObjectType unqualifiedChildType = Invoice__c.SObjectType;
System.assertEquals(childType, unqualifiedChildType);
Boolean found = false;
for (Schema.ChildRelationship relationship : Account.SObjectType.getDescribe().getChildRelationships()) {
    if (relationship.getChildSObject() == unqualifiedChildType &&
        relationship.getField().getDescribe().getName() == 'pkg__Account__c' &&
        relationship.getRelationshipName() == 'pkg__Invoices__r') {
        found = true;
    }
}
System.assert(found, 'namespaced custom child relationship token');
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	storage.EnsureStandardObject(&org, "Account")
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Invoice__c",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r", ChildRelationshipName: "Invoices__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Account__c",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account__r",
				ChildRelationship:  "Invoices__r",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertyAssignmentIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Mapping mapping = new Mapping();
mapping.checkListNotes = Account.Name;
System.assertEquals(Account.Name, mapping.checklistNotes);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Mapping",
		Fields: map[string]Field{
			"checklistNotes": {Name: "checklistNotes", Type: "Schema.SObjectField", Property: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInheritedFieldAssignmentIsCaseInsensitiveInInstanceMethod(t *testing.T) {
	program, err := CompileAnonymous(`
WidgetInfo model = new WidgetInfo();
Datetime stamp = Datetime.newInstance(2025, 6, 1, 12, 0, 0);
model.lastUpdatedAt = stamp;
System.assertEquals(stamp, model.toRecord().LastUpdatedAt__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	toRecordProgram, err := CompileAnonymous(`
return new Widget__c(LastUpdatedAt__c = this.LastUpdatedAt);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Widget__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Widget__c",
				Fields: map[string]storage.Field{
					"LastUpdatedAt__c": {APIName: "LastUpdatedAt__c", Type: storage.FieldDateTime},
				},
			},
			Records: make(map[storage.ID]storage.Record),
		},
	}}
	machine := New(nil)
	machine.Org = &org
	if err := machine.RegisterClass(Class{
		Name: "BaseInfo",
		Fields: map[string]Field{
			"LastUpdatedAt": {Name: "LastUpdatedAt", Type: "DateTime"},
			"lastUpdatedAt": {Name: "lastUpdatedAt", Type: "DateTime"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "WidgetInfo", SuperClass: "BaseInfo"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "WidgetInfo.toRecord", ClassName: "WidgetInfo", ReturnType: "Widget__c", Program: toRecordProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterClassStampsPropertyAccessorOwner(t *testing.T) {
	getterProgram, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "Owner",
		Namespace: "pkg",
		StaticFields: map[string]Field{
			"Mock": {Name: "Mock", Type: "Object", Static: true, Property: true, Getter: &Method{Name: "Owner.Mock.get", ReturnType: "Object", Program: getterProgram}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	field := machine.Classes["Owner"].StaticFields["Mock"]
	if field.Getter == nil || field.Getter.ClassName != "pkg.Owner" {
		t.Fatalf("getter ClassName = %#v, want pkg.Owner", field.Getter)
	}
}

func TestConstructNamespacedSourceClassCollectionFieldsDefaultNull(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "Model",
		Namespace: "pkg",
		Access:    "global",
		Fields: map[string]Field{
			"Names": {Name: "Names", Type: "List<String>", Property: true},
		},
	}); err != nil {
		t.Fatal(err)
	}

	value, err := machine.constructValue("pkg.Model", nil, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	names := value.Fields["Names"]
	if names.Kind != ValueNull {
		t.Fatalf("Names default = %#v, want null", names)
	}
}

func TestPageTokenReferenceURLIncludesMutatedParameters(t *testing.T) {
	page := newPageTokenReference("/apex/Order")
	params := page.Fields["parameters"]
	key := mapKey(String("id"))
	params.Map[key] = String("001000000000001")
	params.MapKeys[key] = String("id")
	page.Fields["parameters"] = params

	if got := pageReferenceURL(page).String(); got != "/apex/Order?id=001000000000001" {
		t.Fatalf("page URL = %q", got)
	}
}

func TestSOQLQueriedFieldsTracksDottedSelections(t *testing.T) {
	machine := New(nil)
	org := singleSOQLAssignmentOrg()
	machine.Org = &org
	fields := machine.queriedSObjectFields("SELECT Id, Account.LastModifiedById FROM Account")
	if !fields["account.lastmodifiedbyid"] {
		t.Fatalf("queried fields = %#v, want dotted field", fields)
	}
}

func TestSOQLQueriedFieldsTracksToLabelSelection(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.Org = &org
	fields := machine.queriedSObjectFields("SELECT Id, toLabel(Salutation) FROM Contact")
	if !fields["salutation"] {
		t.Fatalf("queried fields = %#v, want Salutation from toLabel", fields)
	}
}

func TestExecSOQLChildRelationshipRowsAreTypedAndCarryQueriedSystemFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = [SELECT Id, (SELECT Id, LastModifiedById FROM Children__r) FROM Account WHERE Name = 'Acme' LIMIT 1];
System.assertEquals(1, account.Children__r.size());
Child__c child = account.Children__r[0];
System.assertEquals(null, child.LastModifiedById);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := singleSOQLAssignmentOrg()
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Child__c",
			Fields: map[string]storage.Field{
				"Name":       {APIName: "Name", Type: storage.FieldString},
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r", ChildRelationshipName: "Children__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Account__c",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account__r",
				ChildRelationship:  "Children__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Child__c",
				Fields: map[string]storage.Value{
					"Name":       storage.StringValue("Child"),
					"Account__c": storage.IDValue("001000000000001"),
				},
			},
		},
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLLoadedCustomMetadataChildRelationshipDotAccess(t *testing.T) {
	program, err := CompileAnonymous(`
List<StateConfiguration__mdt> configs = [
	SELECT QualifiedApiName, IsActive__c,
		(SELECT QualifiedApiName, IsActive__c, FromStates__c, ToState__c FROM StateTransitions__r WHERE IsActive__c = TRUE)
	FROM StateConfiguration__mdt
	WHERE IsActive__c = TRUE
];
Integer transitionCount = 0;
for (StateConfiguration__mdt config : configs) {
	if (config.QualifiedApiName == 'OrderGraph') {
		for (StateTransition__mdt transitionRecord : config.StateTransitions__r) {
			if (transitionRecord.QualifiedApiName == 'order_submit_as_proforma') {
				transitionCount++;
				System.assertEquals('Cart', transitionRecord.FromStates__c);
				System.assertEquals('Pro forma', transitionRecord.ToState__c);
			}
		}
	}
}
System.assertEquals(1, transitionCount);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["StateConfiguration__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "StateConfiguration__mdt",
			Fields: map[string]storage.Field{
				"IsActive__c": {APIName: "IsActive__c", Type: storage.FieldBoolean},
			},
			Metadata: map[string]string{"kind": "customMetadata"},
		},
		Records: map[storage.ID]storage.Record{
			"a0j000000000001": {
				ID:     "a0j000000000001",
				Object: "StateConfiguration__mdt",
				Fields: map[string]storage.Value{
					"DeveloperName":      storage.StringValue("OrderGraph"),
					"QualifiedApiName":   storage.StringValue("OrderGraph"),
					"NamespacePrefix":    storage.StringValue("pkg"),
					"MasterLabel":        storage.StringValue("Order Graph"),
					"Label":              storage.StringValue("Order Graph"),
					"IsActive__c":        storage.BooleanValue(true),
					"SupportedStates__c": storage.StringValue("Cart,Pro forma"),
				},
			},
		},
	}
	org.Objects["StateTransition__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "StateTransition__mdt",
			Fields: map[string]storage.Field{
				"IsActive__c":           {APIName: "IsActive__c", Type: storage.FieldBoolean},
				"FromStates__c":         {APIName: "FromStates__c", Type: storage.FieldString},
				"ToState__c":            {APIName: "ToState__c", Type: storage.FieldString},
				"StateConfiguration__c": {APIName: "StateConfiguration__c", Type: storage.FieldReference, ReferenceTo: []string{"StateConfiguration__mdt"}, RelationshipName: "StateConfiguration__r", ChildRelationshipName: "StateTransitions__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "StateConfiguration__c",
				ParentObjects:      []string{"StateConfiguration__mdt"},
				ParentRelationship: "StateConfiguration__r",
				ChildRelationship:  "StateTransitions__r",
			}},
			Metadata: map[string]string{"kind": "customMetadata"},
		},
		Records: map[storage.ID]storage.Record{
			"a0l000000000002": {
				ID:     "a0l000000000002",
				Object: "StateTransition__mdt",
				Fields: map[string]storage.Value{
					"DeveloperName":         storage.StringValue("order_submit_as_proforma"),
					"QualifiedApiName":      storage.StringValue("order_submit_as_proforma"),
					"NamespacePrefix":       storage.StringValue("pkg"),
					"MasterLabel":           storage.StringValue("Submit As Proforma"),
					"Label":                 storage.StringValue("Submit As Proforma"),
					"IsActive__c":           storage.BooleanValue(true),
					"FromStates__c":         storage.StringValue("Cart"),
					"ToState__c":            storage.StringValue("Pro forma"),
					"StateConfiguration__c": storage.IDValue("a0j000000000001"),
				},
			},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLSingleSObjectNoRowsAndMultiRowsStayExplicit(t *testing.T) {
	noRowsProgram, err := CompileAnonymous(`Account account; account = [SELECT Id FROM Account WHERE Name = 'Missing'];`)
	if err != nil {
		t.Fatal(err)
	}
	multiRowsProgram, err := CompileAnonymous(`Account account; account = [SELECT Id FROM Account];`)
	if err != nil {
		t.Fatal(err)
	}
	org := singleSOQLAssignmentOrg()
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(noRowsProgram); err == nil || !strings.Contains(err.Error(), "List has no rows") {
		t.Fatalf("no-row err = %v, want List has no rows", err)
	}
	machine = New(nil)
	machine.Org = &org
	if _, err := machine.Execute(multiRowsProgram); err == nil || !strings.Contains(err.Error(), "List has more than 1 row") {
		t.Fatalf("multi-row err = %v, want List has more than 1 row", err)
	}
}

func TestExecSOQLSingleSObjectStaticAndDottedFieldAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
Constants.ORG = [SELECT Id, Name FROM Organization LIMIT 1];
Container c = new Container();
c.account = [SELECT Id, Name FROM Account LIMIT 1];
System.assertEquals('Acme', c.account.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	multiRowsProgram, err := CompileAnonymous(`
Container c = new Container();
c.account = [SELECT Id FROM Account];
`)
	if err != nil {
		t.Fatal(err)
	}
	org := singleSOQLAssignmentOrg()
	machine := New(nil)
	machine.Org = &org
	registerAssignmentTargetClasses(t, machine)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	field := machine.Classes["Constants"].StaticFields["ORG"]
	if field.Value.Kind != ValueObject || field.Value.Type != "Organization" {
		t.Fatalf("ORG = %#v, want Organization object", field.Value)
	}
	machine = New(nil)
	machine.Org = &org
	registerAssignmentTargetClasses(t, machine)
	if _, err := machine.Execute(multiRowsProgram); err == nil || !strings.Contains(err.Error(), "List has more than 1 row") {
		t.Fatalf("multi-row dotted assignment err = %v, want List has more than 1 row", err)
	}
}

func TestExecSOQLReferenceFieldInvalidStringReturnsNoRows(t *testing.T) {
	program, err := CompileAnonymous(`
List<Contact> contacts = Database.query('SELECT Id FROM Contact WHERE AccountId IN (\'not-an-id\')');
System.assertEquals(0, contacts.size());
Boolean caught = false;
try {
  Database.query('SELECT Id FROM Contact WHERE Id IN (\'not-an-id\')');
} catch (QueryException e) {
  caught = true;
}
System.assertEquals(true, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
			"Id":        {APIName: "Id", Type: storage.FieldID},
			"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		}},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{
				"AccountId": storage.IDValue("001000000000001"),
			}},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecImplicitThisSObjectFieldPathUsesInstanceField(t *testing.T) {
	program, err := CompileAnonymous(`return OrderRecord.Entity__c;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := singleSOQLAssignmentOrg()
	machine.Org = &org
	if err := machine.RegisterClass(Class{Name: "Controller", Fields: map[string]Field{
		"OrderRecord": {Name: "OrderRecord", Type: "Order__c"},
	}}); err != nil {
		t.Fatal(err)
	}
	method := Method{Name: "Controller.entityId", ReturnType: "Id", Program: program, ClassName: "Controller"}
	receiver := Object("Controller")
	receiver.Fields["OrderRecord"] = Object("Order__c")
	receiver.Fields["OrderRecord"].Fields["Entity__c"] = platformScalar("Id", "a00000000000001")
	value, err := machine.callMethodWithReceiver(method, receiver, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := platformScalarText(value, "Id"); text != "a00000000000001" {
		t.Fatalf("entity id = %#v", value)
	}
}

func TestExecSOQLSingleSObjectImplicitThisDottedFieldAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
c.account = [SELECT Id, Name FROM Account LIMIT 1];
return c.account.Name;
`)
	if err != nil {
		t.Fatal(err)
	}
	multiRowsProgram, err := CompileAnonymous(`c.account = [SELECT Id FROM Account];`)
	if err != nil {
		t.Fatal(err)
	}
	org := singleSOQLAssignmentOrg()
	machine := New(nil)
	machine.Org = &org
	registerAssignmentTargetClasses(t, machine)
	if err := machine.RegisterClass(Class{Name: "Controller", Fields: map[string]Field{
		"c": {Name: "c", Type: "Container"},
	}}); err != nil {
		t.Fatal(err)
	}
	receiver := Object("Controller")
	receiver.Fields["c"] = Object("Container")
	method := Method{Name: "Controller.assignAccount", ReturnType: "String", Program: program, ClassName: "Controller"}
	value, err := machine.callMethodWithReceiver(method, receiver, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueString || value.Text != "Acme" {
		t.Fatalf("assigned account name = %#v", value)
	}
	machine = New(nil)
	machine.Org = &org
	registerAssignmentTargetClasses(t, machine)
	if err := machine.RegisterClass(Class{Name: "Controller", Fields: map[string]Field{
		"c": {Name: "c", Type: "Container"},
	}}); err != nil {
		t.Fatal(err)
	}
	receiver = Object("Controller")
	receiver.Fields["c"] = Object("Container")
	method = Method{Name: "Controller.assignAccount", Program: multiRowsProgram, ClassName: "Controller"}
	if _, err := machine.callMethodWithReceiver(method, receiver, nil, &Result{}); err == nil || !strings.Contains(err.Error(), "List has more than 1 row") {
		t.Fatalf("multi-row implicit this assignment err = %v, want List has more than 1 row", err)
	}
}

func TestAssignmentTargetTypeWalksStorageReferenceFields(t *testing.T) {
	org := singleSOQLAssignmentOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
			"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		}},
	}
	machine := New(nil)
	machine.Org = &org
	machine.Globals["contact"] = Object("Contact")
	machine.VarTypes["contact"] = "Contact"
	if got := machine.assignmentTargetType("contact.Account__c.Custom__c"); got != "String" {
		t.Fatalf("target type = %q, want String", got)
	}
}

func TestExecUnaryPlusNoOpForStringConcatenation(t *testing.T) {
	program, err := CompileAnonymous(`
String value = 'a';
value += + 'b';
System.assertEquals('ab', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMultipleVariableDeclarationStatement(t *testing.T) {
	program, err := CompileAnonymous(`
Integer left = 1, right = 2;
System.assertEquals(3, left + right);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExplicitDecimalToIntegerCast(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal countValue = 5;
Integer existingCount = (Integer)countValue;
System.assertEquals(5, existingCount);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssertEqualsMatchesFifteenAndEighteenCharacterIDs(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('001000000000001', '001000000000001AAA');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func singleSOQLAssignmentOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
			"Name":      {APIName: "Name", Type: storage.FieldString},
			"Custom__c": {APIName: "Custom__c", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{
				"Name":      storage.StringValue("Acme"),
				"Custom__c": storage.StringValue("selected"),
			}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Other"),
			}},
		},
	}
	org.Objects["Organization"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Organization", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"00D000000000001": {ID: "00D000000000001", Object: "Organization", Fields: map[string]storage.Value{"Name": storage.StringValue("Local")}},
		},
	}
	return org
}

func registerAssignmentTargetClasses(t *testing.T, machine *VM) {
	t.Helper()
	if err := machine.RegisterClass(Class{Name: "Constants", StaticFields: map[string]Field{
		"ORG": {Name: "ORG", Type: "Organization", Static: true},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Container", Fields: map[string]Field{
		"account": {Name: "account", Type: "Account"},
	}}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteIDsToSObjectsUsesIDPrefix(t *testing.T) {
	machine := New(nil)
	converted, ok := machine.deleteIDsToSObjects(List(platformScalar("Id", "001000000000001AAA")))
	if !ok {
		t.Fatal("delete id list was not converted")
	}
	if converted.Kind != ValueList || len(converted.List) != 1 {
		t.Fatalf("converted = %#v", converted)
	}
	if converted.List[0].Type != "Account" {
		t.Fatalf("type = %q, want Account", converted.List[0].Type)
	}
	if id, _ := platformScalarText(converted.List[0].Fields["Id"], "Id"); id != "001000000000001AAA" {
		t.Fatalf("id = %#v", id)
	}
}

func TestExecCoercesNumericVariablesAndCollections(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal total = 1;
total = 2;
System.assertEquals(2.5, total + 0.5);
List<Decimal> totals = new List<Decimal>();
totals.add(1);
System.assertEquals(1.25, totals.get(0) + 0.25);
Map<String,Decimal> byName = new Map<String,Decimal>();
byName.put('one', 1);
System.assertEquals(1.75, byName.get('one') + 0.75);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRejectsInvalidCoercions(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"decimal to integer assignment", "Integer count = 1.5;"},
		{"string to boolean assignment", "Boolean ready = 'true';"},
		{"decimal list item", "List<Integer> counts = new List<Integer>(); counts.add(1.5);"},
		{"integer map key", "Map<String,Integer> counts = new Map<String,Integer>(); counts.put(1, 2);"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err == nil {
				t.Fatalf("expected coercion error")
			}
		})
	}
}

func TestExecAssertFailure(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals(3, 1 + 1);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("expected assertion failure")
	}
}

func TestExecRuntimeErrorStackUsesStatementSourcePosition(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(3, 1 + 1);
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("err = %#v", err)
	}
	if len(runtimeErr.Stack) == 0 || runtimeErr.Stack[0].Line != 2 || runtimeErr.Stack[0].Column != 1 {
		t.Fatalf("stack = %#v", runtimeErr.Stack)
	}
}

func TestExecNullDereferenceRuntimeErrorIncludesMemberContext(t *testing.T) {
	program, err := CompileAnonymous(`
String name = null;
name.toUpperCase();
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("err = %#v, want RuntimeError", err)
	}
	if runtimeErr.Type != "NullPointerException" {
		t.Fatalf("runtime error type = %q", runtimeErr.Type)
	}
	for _, want := range []string{"Attempt to de-reference a null object", "name.toUpperCase", "null receiver name"} {
		if !strings.Contains(runtimeErr.Message, want) {
			t.Fatalf("runtime error message = %q, want %q", runtimeErr.Message, want)
		}
	}
}

func TestExecCollectionsAndTrace(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2};
xs.add(3);
Set<String> names = new Set<String>();
names.add('a');
names.add('a');
Map<String,Integer> counts = new Map<String,Integer>();
counts.put('a', xs.size());
System.assertEquals(3, xs.get(2));
System.assertEquals(1, names.size());
System.assertEquals(3, counts.get('a'));
System.assert(counts.containsKey('a'));
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trace) != len(program.Instructions)+1 {
		t.Fatalf("trace length = %d, want %d", len(result.Trace), len(program.Instructions)+1)
	}
	if result.TraceFormat != "chrome-trace-event" {
		t.Fatalf("trace format = %q", result.TraceFormat)
	}
	first := result.Trace[0]
	if first.Name != "apex.statement.declare" || first.Category != "apex.statement" || first.Phase != "i" {
		t.Fatalf("trace event shape = %#v", first)
	}
	if first.Args["sourceOffset"] == 0 {
		t.Fatalf("trace missing source offset: %#v", first)
	}
	if first.Args["line"] != 2 || first.Args["column"] != 1 {
		t.Fatalf("trace source position = %#v", first.Args)
	}
	last := result.Trace[len(result.Trace)-1]
	if last.Name != "apex.limits" || last.Category != "apex.limits" {
		t.Fatalf("trace missing limits summary: %#v", last)
	}
}

func TestExecMethodParameterMapPropagatesNestedCollectionAliases(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
Set<String> values = (Set<String>)context.get('values');
values.add('applied');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Set<String> values = new Set<String>();
Map<String, Object> context = new Map<String, Object>{'values' => values};
Util.apply(context);
System.assertEquals(1, values.size());
System.assertEquals(1, ((Set<String>)context.get('values')).size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Util.apply",
		ClassName:  "Util",
		IsStatic:   true,
		ReturnType: "void",
		Params:     []Param{{Name: "context", Type: "Map<String,Object>"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMethodParameterListRemovePropagatesToCaller(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
values.remove(0);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<String> values = new List<String>{'trail'};
Util.removeFirst(values);
System.assert(values.isEmpty());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Util.removeFirst",
		ClassName:  "Util",
		IsStatic:   true,
		ReturnType: "void",
		Params:     []Param{{Name: "values", Type: "List<String>"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInstanceMethodParameterListRemovePropagatesToCaller(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
values.remove(0);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<String> values = new List<String>{'trail'};
new Util().removeFirst(values);
System.assert(values.isEmpty());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Util",
		Methods: map[string]Method{
			"removeFirst": {
				Name:       "Util.removeFirst",
				ClassName:  "Util",
				ReturnType: "void",
				Params:     []Param{{Name: "values", Type: "List<String>"}},
				Program:    methodProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInstanceMethodIndexedListRemovePropagatesToCaller(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
Item item;
for (Integer i = 0; i < values.size(); i++) {
	item = values[i];
	if (item.Name.equals(target)) {
		values.remove(i);
		break;
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Item item = new Item();
item.Name = 'trail';
List<Item> values = new List<Item>{item};
new Util().removeMatching(values, 'trail');
System.assert(values.isEmpty());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Item",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Util",
		Methods: map[string]Method{
			"removeMatching": {
				Name:       "Util.removeMatching",
				ClassName:  "Util",
				ReturnType: "void",
				Params:     []Param{{Name: "values", Type: "List<Item>"}, {Name: "target", Type: "String"}},
				Program:    methodProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecForBreakContinueDoWhileSwitchAndEnhancedFor(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2, 3, 4};
Integer total = 0;
for (Integer i = 0; i < xs.size(); i++) {
	if (i == 1) {
		continue;
	}
	if (i == 3) {
		break;
	}
	total = total + xs.get(i);
}
Integer seen = 0;
for (Integer x : xs) {
	seen = seen + x;
}
Integer once = 0;
do {
	once++;
} while (once < 1);
String label = '';
switch on total {
	when 1 { label = 'one'; }
	when 4 { label = 'four'; }
	when else { label = 'other'; }
}
System.assertEquals(4, total);
System.assertEquals(10, seen);
System.assertEquals(1, once);
System.assertEquals('four', label);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnhancedForNullCollectionIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names;
Boolean caught = false;
try {
	for (String name : names) {
		System.debug(name);
	}
} catch (NullPointerException e) {
	caught = true;
}
System.assertEquals(true, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNullBooleanConditionIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean flag;
Boolean caught = false;
try {
	if (flag) {
		System.debug('true');
	}
} catch (System.NullPointerException e) {
	caught = true;
}
System.assertEquals(true, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecListIndexOutOfBoundsIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names = new List<String>();
Boolean caught = false;
try {
	names.remove(-1);
} catch (ListException e) {
	caught = true;
}
System.assertEquals(true, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSwitchBreakOnlyExitsSwitchAndContinueReachesLoop(t *testing.T) {
	program, err := CompileAnonymous(`
Integer seen = 0;
Integer afterSwitch = 0;
for (Integer i = 0; i < 4; i++) {
	switch on i {
		when 0 {
			seen = seen + 1;
			break;
		}
		when 1 {
			continue;
		}
		when else {
			seen = seen + 10;
		}
	}
	afterSwitch++;
}
System.assertEquals(21, seen);
System.assertEquals(3, afterSwitch);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecFinallyPreservesAndOverridesLoopSignals(t *testing.T) {
	program, err := CompileAnonymous(`
Integer cleaned = 0;
Integer seen = 0;
for (Integer i = 0; i < 4; i++) {
	try {
		if (i == 1) {
			continue;
		}
		if (i == 2) {
			break;
		}
		seen++;
	} finally {
		cleaned++;
	}
}
System.assertEquals(1, seen);
System.assertEquals(3, cleaned);
Integer overridden = 0;
while (overridden < 1) {
	try {
		break;
	} finally {
		overridden++;
		continue;
	}
}
System.assertEquals(1, overridden);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnhancedForBreakContinueAndFinallyThrowOverride(t *testing.T) {
	throwingReturn, err := CompileAnonymous(`
try {
	return 7;
} finally {
	throw new DmlException('finally wins');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Integer> values = new List<Integer>{1, 2, 3, 4};
Integer total = 0;
Integer cleaned = 0;
for (Integer value : values) {
	try {
		if (value == 2) {
			continue;
		}
		if (value == 4) {
			break;
		}
		total = total + value;
	} finally {
		cleaned++;
	}
}
String message = '';
try {
	Util.throwingReturn();
} catch (DmlException e) {
	message = e.getMessage();
}
System.assertEquals(4, total);
System.assertEquals(4, cleaned);
System.assertEquals('finally wins', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.throwingReturn", ReturnType: "Integer", Program: throwingReturn}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTryCatchFinallyThrow(t *testing.T) {
	program, err := CompileAnonymous(`
Integer cleaned = 0;
try {
	throw new MyException();
} catch (Exception e) {
	cleaned = cleaned + 1;
} finally {
	cleaned = cleaned + 2;
}
System.assertEquals(3, cleaned);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredCustomExceptionUsesMessageConstructor(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
	throw new LocalException('boom');
} catch (Exception e) {
	message = e.getMessage();
}
System.assertEquals('boom', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "LocalException", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredCustomExceptionWithoutMessageHasDefaultMessage(t *testing.T) {
	program, err := CompileAnonymous(`
Exception e = new localexception();
String message = e.getMessage();
System.assert(message != null);
System.assert(message.length() > 0);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "localexception", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedCustomExceptionIsExceptionInstance(t *testing.T) {
	program, err := CompileAnonymous(`
Object e = new Outer.NestedException('blocked');
System.assert(e instanceof Exception);
try {
	throw ((Exception)e);
} catch (Exception caught) {
	System.assertEquals('blocked', caught.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer.NestedException", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUnqualifiedNestedExceptionThrownFromOwnerCarriesMessage(t *testing.T) {
	raiseProgram, err := CompileAnonymous(`throw new NestedException('blocked');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String message = '';
try {
	Outer.raise();
} catch (Outer.NestedException e) {
	message = e.getMessage();
}
System.assertEquals('blocked', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Outer",
		Methods: map[string]Method{
			"raise": {Name: "Outer.raise", ClassName: "Outer", IsStatic: true, ReturnType: "void", Program: raiseProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer.NestedException", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestResolveUniqueNestedTypeNameCachesAndInvalidates(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer.Nested"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "Outer.Harness"

	resolved, ok := machine.resolveUniqueNestedTypeName("Nested")
	if !ok || resolved != "Outer.Nested" {
		t.Fatalf("resolve Nested = %q, %v; want Outer.Nested, true", resolved, ok)
	}
	if len(machine.uniqueNestedTypeCache) == 0 {
		t.Fatalf("resolveUniqueNestedTypeName did not populate cache")
	}

	machine.currentClass = "Outer.Harness"
	if resolved, ok := machine.resolveUniqueNestedTypeName("Later"); ok || resolved != "" {
		t.Fatalf("resolve Later before registration = %q, %v; want empty, false", resolved, ok)
	}
	if err := machine.RegisterClass(Class{Name: "Outer.Later"}); err != nil {
		t.Fatal(err)
	}
	resolved, ok = machine.resolveUniqueNestedTypeName("Later")
	if !ok || resolved != "Outer.Later" {
		t.Fatalf("resolve Later after registration = %q, %v; want Outer.Later, true", resolved, ok)
	}
}

func TestResolveUniqueNestedTypeNameFromTopLevelCurrentClass(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer.Domain", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "pkg.Outer"

	resolved, ok := machine.resolveUniqueNestedTypeName("Domain")
	if !ok || resolved != "Outer.Domain" {
		t.Fatalf("resolve Domain = %q, %v; want Outer.Domain, true", resolved, ok)
	}
}

func TestResolveUniqueNestedTypeNameFallsBackToOnlyNestedSuffix(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer.Domain", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "pkg.Harness"

	resolved, ok := machine.resolveUniqueNestedTypeName("Domain")
	if !ok || resolved != "pkg.Outer.Domain" {
		t.Fatalf("resolve Domain = %q, %v; want pkg.Outer.Domain, true", resolved, ok)
	}
}

func TestResolveOnlyNestedTypeNameFindsUnambiguousSuffix(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer.Domain", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}

	resolved, ok := machine.resolveOnlyNestedTypeName("Domain")
	if !ok || resolved != "Outer.Domain" {
		t.Fatalf("resolve only Domain = %q, %v; want Outer.Domain, true", resolved, ok)
	}
}

func TestResolveOnlyNestedTypeNameCachesAndInvalidates(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer.Nested"}); err != nil {
		t.Fatal(err)
	}

	resolved, ok := machine.resolveOnlyNestedTypeName("Nested")
	if !ok || resolved != "Outer.Nested" {
		t.Fatalf("resolve Nested = %q, %v; want Outer.Nested, true", resolved, ok)
	}
	if len(machine.onlyNestedTypeCache) == 0 {
		t.Fatalf("resolveOnlyNestedTypeName did not populate cache")
	}
	if len(machine.classNameSearchCache) == 0 {
		t.Fatalf("resolveOnlyNestedTypeName did not populate class name search cache")
	}

	if resolved, ok := machine.resolveOnlyNestedTypeName("Later"); ok || resolved != "" {
		t.Fatalf("resolve Later before registration = %q, %v; want empty, false", resolved, ok)
	}
	if err := machine.RegisterClass(Class{Name: "Outer.Later"}); err != nil {
		t.Fatal(err)
	}
	if machine.classNameSearchCache != nil {
		t.Fatalf("RegisterClass did not invalidate class name search cache")
	}
	resolved, ok = machine.resolveOnlyNestedTypeName("Later")
	if !ok || resolved != "Outer.Later" {
		t.Fatalf("resolve Later after registration = %q, %v; want Outer.Later, true", resolved, ok)
	}
}

func TestResolveTopLevelClassNameCachesAndInvalidates(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Widget"}); err != nil {
		t.Fatal(err)
	}

	resolved, ok := machine.resolveTopLevelClassName("Widget")
	if !ok || resolved != "Widget" {
		t.Fatalf("resolve Widget = %q, %v; want Widget, true", resolved, ok)
	}
	if len(machine.topLevelTypeCache) == 0 {
		t.Fatalf("resolveTopLevelClassName did not populate cache")
	}
	if resolved, ok := machine.resolveTopLevelClassName("Later"); ok || resolved != "" {
		t.Fatalf("resolve Later before registration = %q, %v; want empty, false", resolved, ok)
	}
	if err := machine.RegisterClass(Class{Name: "Later"}); err != nil {
		t.Fatal(err)
	}
	if machine.topLevelTypeCache != nil {
		t.Fatalf("RegisterClass did not invalidate top-level type cache")
	}
	resolved, ok = machine.resolveTopLevelClassName("Later")
	if !ok || resolved != "Later" {
		t.Fatalf("resolve Later after registration = %q, %v; want Later, true", resolved, ok)
	}
}

func TestConstructBareRuntimeVersionDoesNotResolveToGeneratedNestedType(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Package.Version"}); err != nil {
		t.Fatal(err)
	}

	value, err := machine.constructValue("version", []Value{Int(1), Int(19)}, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Type != "Version" {
		t.Fatalf("construct Version type = %q; want Version", value.Type)
	}
}

func TestConstructBareStandardSObjectDoesNotResolveToGeneratedNestedType(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "ConnectApi.Task"}); err != nil {
		t.Fatal(err)
	}

	value, err := machine.constructValue("Task", nil, map[string]Value{"Subject": String("Call")}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Type != "Task" {
		t.Fatalf("construct Task type = %q; want Task", value.Type)
	}
}

func TestConstructBareTopLevelClassDoesNotResolveToNestedEnumWithSameShortName(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "ParameterSet", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "TransferNpdbRequestModel.ParameterSet",
		Namespace:  "pkg",
		EnumValues: []string{"SSN_PARAMETER_SET"},
	}); err != nil {
		t.Fatal(err)
	}
	machine.currentNamespace = "pkg"

	value, err := machine.constructValue("ParameterSet", nil, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Type != "pkg.ParameterSet" {
		t.Fatalf("construct ParameterSet type = %q; want pkg.ParameterSet", value.Type)
	}
}

func TestLookupClassInNamespacePrefersTopLevelAndRejectsAmbiguousNestedShortName(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "WrapperA.FileModel", Namespace: "pkg"},
		{Name: "FileModel", Namespace: "pkg"},
		{Name: "WrapperB.FileModel", Namespace: "pkg"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}

	class, ok := machine.lookupClassInNamespace("pkg", "FileModel")
	if !ok || class.Name != "FileModel" {
		t.Fatalf("lookup FileModel = %q, %v; want top-level FileModel, true", class.Name, ok)
	}

	machine = New(nil)
	for _, class := range []Class{
		{Name: "WrapperA.FileModel", Namespace: "pkg"},
		{Name: "WrapperB.FileModel", Namespace: "pkg"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if class, ok := machine.lookupClassInNamespace("pkg", "FileModel"); ok {
		t.Fatalf("lookup ambiguous FileModel = %q, true; want false", class.Name)
	}
}

func TestConstructUnqualifiedNestedTypeUsesCurrentMethodOwnerBeforePlatformShortName(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Domain"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "Outer.Domain",
		Namespace: "pkg",
		Constructors: []Method{{
			Name:          "Outer.Domain.<init>",
			ClassName:     "Outer.Domain",
			IsConstructor: true,
			Params:        []Param{{Name: "items", Type: "List<Object>"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	machine.currentMethod = Method{Name: "pkg.Outer.make"}

	value, err := machine.constructValue("Domain", []Value{List()}, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Type != "pkg.Outer.Domain" {
		t.Fatalf("constructed type = %q, want pkg.Outer.Domain", value.Type)
	}
}

func TestExecCustomExceptionConstructorCanSetMessage(t *testing.T) {
	ctor, err := CompileAnonymous(`this.setMessage(message);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String message = '';
try {
	throw new LocalException('blocked');
} catch (Exception e) {
	message = e.getMessage();
}
System.assertEquals('blocked', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "LocalException",
		SuperClass: "Exception",
		Constructors: []Method{{
			Name:          "LocalException.<init>",
			ClassName:     "LocalException",
			Params:        []Param{{Name: "message", Type: "String"}},
			Program:       ctor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomExceptionConstructorCanChainToInheritedMessageConstructor(t *testing.T) {
	ctor, err := CompileAnonymous(`this(message);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
AppException ex = new AppException('blocked', true);
System.assertEquals('blocked', ex.getMessage());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "AppException",
		SuperClass: "Exception",
		Constructors: []Method{{
			Name:          "AppException.<init>",
			ClassName:     "AppException",
			Params:        []Param{{Name: "message", Type: "String"}, {Name: "display", Type: "Boolean"}},
			Program:       ctor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomExceptionMethodCanCallInheritedMessageUnqualified(t *testing.T) {
	userMessage, err := CompileAnonymous(`return getMessage();`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
AppException ex = new AppException('blocked');
System.assertEquals('blocked', ex.userMessage());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "AppException",
		SuperClass: "Exception",
		Methods: map[string]Method{
			"userMessage": {
				Name:       "AppException.userMessage",
				ClassName:  "AppException",
				ReturnType: "String",
				Program:    userMessage,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedEnumStaticValue(t *testing.T) {
	program, err := CompileAnonymous(`
Object direction = TriggerConstants.Direction.ToCustomer;
System.assertEquals('ToCustomer', String.valueOf(direction));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "TriggerConstants"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "TriggerConstants.Direction", EnumValues: []string{"ToPlatform", "ToCustomer", "Both"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticInitializerAssignsNestedEnumField(t *testing.T) {
	staticInit, err := CompileAnonymous(`jobType = Options.Kind.AccountHardCredit;`)
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous(`return jobType;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(Options.Kind.AccountHardCredit, AccountJob.getJobType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Options"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Options.Kind", EnumValues: []string{"AccountHardCredit"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "AccountJob",
		StaticFields: map[string]Field{
			"jobType": {Name: "jobType", Type: "Options.Kind", Static: true},
		},
		StaticInitializers: []Method{{
			Name:      "AccountJob.<static_field_init>.jobType",
			ClassName: "AccountJob",
			IsStatic:  true,
			Program:   staticInit,
		}},
		Methods: map[string]Method{
			"getJobType": {Name: "AccountJob.getJobType", ClassName: "AccountJob", IsStatic: true, ReturnType: "Options.Kind", Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedEnumStaticValueIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Object mode = VerificationMode.ModeName.CALLS;
System.assertEquals('calls', String.valueOf(mode));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VerificationMode"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "VerificationMode.ModeName", EnumValues: []string{"times", "calls"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedShortEnumEqualityUsesExecutingNamespace(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
System.assertEquals(true, TokenType.RIGHT_PAREN == TokenType.RIGHT_PAREN);
return TokenType.RIGHT_PAREN == TokenType.RIGHT_PAREN;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`System.assertEquals(true, pkg.Parser.check());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "pkg.TokenType", Namespace: "pkg", EnumValues: []string{"LEFT_PAREN", "RIGHT_PAREN"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "pkg.Parser", Namespace: "pkg", Access: "global"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "pkg.Parser.check", ClassName: "pkg.Parser", ReturnType: "Boolean", IsStatic: true, Access: "global", Program: methodProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserEnumNameMethodIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Object mode = VerificationMode.ModeName.calls;
System.assertEquals('calls', mode.Name());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VerificationMode.ModeName", EnumValues: []string{"times", "calls"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserEnumNameAndOrdinalFieldAccess(t *testing.T) {
	program, err := CompileAnonymous(`
Object mode = VerificationMode.ModeName.calls;
System.assertEquals('calls', mode.name);
System.assertEquals(1, mode.ordinal);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VerificationMode.ModeName", EnumValues: []string{"times", "calls"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnumNameOnThisFieldWithStringRuntimeValue(t *testing.T) {
	program, err := CompileAnonymous(`
Harness h = new Harness();
h.mode = 'calls';
System.assertEquals('calls', h.modeName());
`)
	if err != nil {
		t.Fatal(err)
	}
	body, err := CompileAnonymous(`return this.mode.name();`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VerificationMode.ModeName", EnumValues: []string{"times", "calls"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Harness",
		Fields: map[string]Field{
			"mode": {Name: "mode", Type: "VerificationMode.ModeName"},
		},
		Methods: map[string]Method{
			"modeName": {Name: "Harness.modeName", ClassName: "Harness", ReturnType: "String", Program: body},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSchemaDisplayType(t *testing.T) {
	program, err := CompileAnonymous(`
Row row = (Row) JSON.deserialize('{"dataType":"STRING"}', Row.class);
System.assertEquals(Schema.DisplayType.STRING, row.dataType);
System.assertEquals(Schema.DisplayType.STRING, DisplayType.valueOf('STRING'));
System.assertEquals('{"dataType":"STRING"}', JSON.serialize(row));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Row",
		Fields: map[string]Field{
			"dataType": {Name: "dataType", Type: "Schema.DisplayType"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnumValueOfInvalidValueIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	Object mode = VerificationMode.ModeName.valueOf('missing');
} catch (Exception e) {
	caught = true;
	System.assertEquals('System.NoSuchElementException', e.getTypeName());
	System.assert(e.getMessage().contains('No enum value found called missing'));
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VerificationMode.ModeName", EnumValues: []string{"times", "calls"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserEnumValueOfIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(BillingMode.AccountingMethod.CASH, BillingMode.AccountingMethod.valueOf('Cash'));
System.assertEquals(BillingMode.AccountingMethod.CASH, BillingMode.AccountingMethod.valueOf('cash'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "BillingMode.AccountingMethod", EnumValues: []string{"CASH", "ACCRUAL"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecChainedAssignmentExpression(t *testing.T) {
	program, err := CompileAnonymous(`
Integer left;
Integer right;
left = right = 3;
System.assertEquals(3, left);
System.assertEquals(3, right);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecMultiCatchAndRethrow(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
	try {
		throw new MyException('boom');
	} catch (Exception e) {
		throw;
	}
} catch (OtherException | MyException e) {
	message = e.getMessage();
}
System.assertEquals('boom', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecFinallyRunsOnReturnAndCanOverrideReturn(t *testing.T) {
	var stdout strings.Builder
	firstProgram, err := CompileAnonymous(`
try {
	return 1;
} finally {
	System.debug('clean');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	secondProgram, err := CompileAnonymous(`
try {
	return 1;
} finally {
	return 3;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(1, Util.first());
System.assertEquals(3, Util.second());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(&stdout)
	if err := machine.RegisterMethod(Method{Name: "Util.first", ReturnType: "Integer", Program: firstProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.second", ReturnType: "Integer", Program: secondProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "clean\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestExecFinallyRunsBeforeUncaughtThrow(t *testing.T) {
	var stdout strings.Builder
	throwProgram, err := CompileAnonymous(`
try {
	throw new MyException('boom');
} finally {
	System.debug('clean');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
	Util.thrower();
} catch (MyException e) {
	System.assertEquals('boom', e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(&stdout)
	if err := machine.RegisterMethod(Method{Name: "Util.thrower", ReturnType: "void", Program: throwProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "clean\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestExecCatchInterfaceExceptionType(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = 'no';
try {
	throw new MyException();
} catch (Marker e) {
	caught = 'yes';
}
System.assertEquals('yes', caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Marker"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "MyException", Interfaces: []string{"Marker"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExceptionHierarchyMultipleCatchAndMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	throw new QueryException('bad query');
} catch (DmlException e) {
	caught = 'wrong';
} catch (System.QueryException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
	System.assert(e.getLineNumber() > 0);
	String trace = e.getStackTraceString();
	System.assert(trace != '');
}
Exception base = new DmlException('blocked');
System.assertEquals('System.DmlException', base.getTypeName());
System.assertEquals('System.QueryException:bad query', caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRethrowPreservesOriginalExceptionStack(t *testing.T) {
	throwProgram, err := CompileAnonymous(`
throw new DmlException('boom');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String stack = '';
try {
	try {
		Util.thrower();
	} catch (Exception e) {
		throw;
	}
} catch (DmlException e) {
	stack = e.getStackTraceString();
}
System.assert(stack.contains('Util.thrower'));
System.assert(!stack.contains('rethrow outside catch block'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.thrower", ReturnType: "void", Program: throwProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCaughtExceptionStackTraceUsesApexFrameFormat(t *testing.T) {
	throwProgram, err := CompileAnonymous(`
throw new DmlException('boom');
	`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String stack = '';
try {
	Thrower.fail();
} catch (DmlException e) {
	stack = e.getStackTraceString();
}
	System.assert(stack.startsWith('Class.NU.Thrower.fail: line '), stack);
	System.assert(!stack.contains('.cls:'), stack);
		`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "NU"
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name:      "Thrower",
		Namespace: "NU",
		Access:    "global",
		Methods: map[string]Method{
			"fail": {ReturnType: "void", Program: throwProgram, IsStatic: true, Access: "global"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLExceptionStackRestoresCallerStatementAfterTriggers(t *testing.T) {
	dmlProgram, err := CompileAnonymous(`insert new Account();`)
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymous(`TriggerHelper.touch();`)
	if err != nil {
		t.Fatal(err)
	}
	helperProgram, err := CompileAnonymous(`String marker = 'trigger';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String stack = '';
try {
	Harness.run();
} catch (DmlException e) {
	stack = e.getStackTraceString();
}
System.assert(stack.startsWith('Class.Harness.run: line '), stack);
System.assert(!stack.contains('TriggerHelper.touch'), stack);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterMethod(Method{Name: "Harness.run", ReturnType: "void", Program: dmlProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "TriggerHelper.touch", ReturnType: "void", Program: helperProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "AccountTrigger", Object: "Account", Timing: triggerTimingBefore, Operation: "insert", Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalArithmetic(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal total = 1.5 + 2;
System.assertEquals('3.5', '' + total);
System.assert(total > 3);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCronNextFireTimeOneShotFixedDate(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)

	// Fully pinned future date with a weekday wildcard ("?") beyond the
	// day-scan window must resolve directly to that datetime.
	got, ok := cronNextFireTime("0 30 9 15 6 ? 2040", now)
	if !ok {
		t.Fatal("expected one-shot future cron to fire")
	}
	if want := "2040-06-15T09:30:00Z"; got != want {
		t.Fatalf("next fire = %q, want %q", got, want)
	}

	// A fully pinned date in the past must not fire.
	if _, ok := cronNextFireTime("0 0 0 1 1 ? 2000", now); ok {
		t.Fatal("expected past one-shot cron to not fire")
	}
}
