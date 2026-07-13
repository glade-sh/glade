package sema

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSemaSObjectFieldProviderPrecedenceAndDeterministicVisit(t *testing.T) {
	provider := newSemaSObjectFieldProvider("pkg", schema.Object{
		Name: "Account",
		Fields: []schema.Field{
			{Name: "Name", Type: "Number"},
			{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"pkg__Queue__c"}, RelationshipName: "pkg__Owner"},
			{Name: "pkg__Flag__c", Type: "Checkbox"},
		},
	})

	name, ok := provider.lookup("nAmE")
	if !ok || name.Type != "Number" {
		t.Fatalf("project Account.Name = %#v, %v; want project Number", name, ok)
	}
	owner, ok := provider.lookup("OwnerId")
	if !ok || owner.Name != "OwnerId" || !reflect.DeepEqual(owner.ReferenceTo, []string{"pkg__Queue__c"}) || owner.RelationshipName != "pkg__Owner" {
		t.Fatalf("project Account.OwnerId authority = %#v, %v", owner, ok)
	}
	flag, ok := provider.lookup("Flag__c")
	if !ok || flag.Name != "pkg__Flag__c" {
		t.Fatalf("project namespace alias = %#v, %v; want canonical pkg__Flag__c", flag, ok)
	}
	createdDate, ok := provider.lookup("CreatedDate")
	if !ok || createdDate.Type == "" {
		t.Fatalf("standard Account.CreatedDate = %#v, %v", createdDate, ok)
	}

	var first []string
	provider.visit(func(field schema.Field) {
		first = append(first, field.Name)
	})
	var second []string
	provider.visit(func(field schema.Field) {
		second = append(second, field.Name)
	})
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("provider visit order changed:\nfirst: %#v\nsecond: %#v", first, second)
	}
	for i := 1; i < len(first); i++ {
		if normalizeName(first[i-1]) > normalizeName(first[i]) {
			t.Fatalf("provider visit is not sorted at %q, %q", first[i-1], first[i])
		}
	}
}

func TestSemaSObjectFieldProviderStandardThenCommonFallback(t *testing.T) {
	standard := newSemaSObjectFieldProvider("", schema.Object{Name: "Account"})
	name, ok := standard.lookup("Name")
	if !ok || !strings.EqualFold(name.Type, "String") {
		t.Fatalf("standard Account.Name = %#v, %v; want String", name, ok)
	}

	common := newSemaSObjectFieldProvider("", schema.Object{Name: "Widget__c"})
	id, ok := common.lookup("ID")
	if !ok || id.Type != "Id" {
		t.Fatalf("common Widget__c.Id = %#v, %v; want Id", id, ok)
	}
}

func TestSemaSObjectFieldProviderKeepsCompatibilityRepairsWithoutMutatingSchema(t *testing.T) {
	references := []string{"ProjectTarget__c"}
	object := schema.Object{
		Name: "Account",
		Fields: []schema.Field{
			{Name: "Name", Type: "Any", ReferenceTo: references},
			{Name: "PersonDoNotCall", Type: "Text"},
		},
	}
	provider := newSemaSObjectFieldProvider("", object)

	name, ok := provider.lookup("Name")
	if !ok || !strings.EqualFold(name.Type, "String") {
		t.Fatalf("FieldAny repair = %#v, %v; want standard String", name, ok)
	}
	if !reflect.DeepEqual(name.ReferenceTo, references) {
		t.Fatalf("project relationship target = %#v, want %#v", name.ReferenceTo, references)
	}
	personDoNotCall, ok := provider.lookup("PersonDoNotCall")
	if !ok || !(strings.EqualFold(personDoNotCall.Type, "Boolean") || strings.EqualFold(personDoNotCall.Type, "Checkbox")) {
		t.Fatalf("PersonDoNotCall repair = %#v, %v; want Boolean", personDoNotCall, ok)
	}

	name.ReferenceTo[0] = "Changed__c"
	provider.visit(func(field schema.Field) {
		if len(field.ReferenceTo) > 0 {
			field.ReferenceTo[0] = "Visited__c"
		}
	})
	if got := object.Fields[0].ReferenceTo[0]; got != "ProjectTarget__c" {
		t.Fatalf("provider mutated schema-owned relationship target to %q", got)
	}
}

func TestSemaSObjectFieldProviderDoesNotMutateStandardDefinitionForFeatures(t *testing.T) {
	before, ok := storage.StandardObjectDefinition("Account")
	if !ok {
		t.Fatal("missing Account standard definition")
	}
	if _, exists := before.Fields["PersonMailingStreet"]; exists {
		t.Fatal("featureless Account unexpectedly has PersonMailingStreet")
	}
	provider := newSemaSObjectFieldProvider("", schema.Object{Name: "Account"})
	if _, ok := provider.lookup("PersonMailingStreet"); !ok {
		t.Fatal("provider omitted PersonMailingStreet feature field")
	}
	after, ok := storage.StandardObjectDefinition("Account")
	if !ok {
		t.Fatal("missing Account standard definition after provider construction")
	}
	if _, exists := after.Fields["PersonMailingStreet"]; exists {
		t.Fatal("provider mutated the featureless Account standard definition")
	}
}

func TestSemaSObjectFieldProviderKeepsKnownStandardAndFallbackTypes(t *testing.T) {
	for _, test := range []struct {
		object string
		field  string
		typ    string
	}{
		{object: "AIInsightAction", field: "Confidence", typ: "Decimal"},
		{object: "EmailTemplate", field: "Subject", typ: "String"},
		{object: "FlowDefinitionView", field: "LastModifiedBy", typ: "Text"},
	} {
		provider := newSemaSObjectFieldProvider("", schema.Object{Name: test.object})
		field, ok := provider.lookup(test.field)
		if !ok || !strings.EqualFold(field.Type, test.typ) {
			t.Errorf("%s.%s = %#v, %v; want %s", test.object, test.field, field, ok, test.typ)
		}
	}
}

func TestTypeMembersKeepMethodCollisionSeparateFromSObjectFields(t *testing.T) {
	model := buildTypeMembers(typesys.Index{
		Types: []typesys.TypeSymbol{{
			Kind: apexast.DeclarationClass,
			Name: "Account",
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationMethod,
				Name: "Name",
				Type: "String",
			}},
		}},
		Objects: []schema.Object{{
			Name:   "Account",
			Fields: []schema.Field{{Name: "Name", Type: "Number"}},
		}},
	})
	account := model.members[normalizeName("Account")]
	if len(account.methods[normalizeName("Name")]) != 1 {
		t.Fatalf("Account.Name method collision = %#v", account.methods)
	}
	if _, exists := account.fields[normalizeName("Name")]; exists {
		t.Fatalf("project class Account unexpectedly gained SObject field: %#v", account.fields)
	}
}

func TestTypeMembersKeepProjectSObjectFieldTypeOverStandard(t *testing.T) {
	model := buildTypeMembers(typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Objects: []schema.Object{{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Name", Type: "Number"},
				{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"pkg__Queue__c"}, RelationshipName: "pkg__Owner"},
				{Name: "pkg__Flag__c", Type: "Checkbox"},
			},
		}},
	})
	account := model.members[normalizeName("Account")]
	name := account.fields[normalizeName("Name")]
	if name.Type != "Decimal" {
		t.Fatalf("Account.Name type = %q, want project Decimal", name.Type)
	}
	owner := account.fields[normalizeName("pkg__Owner")]
	if owner.Type != "pkg__Queue__c" {
		t.Fatalf("Account.pkg__Owner type = %q, want project pkg__Queue__c", owner.Type)
	}
	for _, alias := range []string{"pkg__Flag__c", "Flag__c"} {
		if field := account.fields[normalizeName(alias)]; field.Type != "Boolean" {
			t.Fatalf("Account.%s type = %q, want Boolean", alias, field.Type)
		}
	}
}

func TestTypeMembersKeepStandardRelationshipWhenPartialProjectFieldUsesRelationshipName(t *testing.T) {
	object := schema.Object{
		Name:    "Contact",
		Partial: true,
		Fields: []schema.Field{
			{Name: "AccountId", Type: "Id", ReferenceTo: []string{"Name"}, RelationshipName: "Account"},
			{Name: "HasOptedOutOfEmail", Type: "String"},
			{Name: "pkg__ProjectParent__c", Type: "Lookup", ReferenceTo: []string{"pkg__ProjectTarget__c"}, RelationshipName: "pkg__ProjectParent__r"},
		},
	}
	provider := newSemaSObjectFieldProvider("pkg", object)
	projectParent, ok := provider.lookup("ProjectParent__c")
	if !ok || projectParent.Name != "pkg__ProjectParent__c" || !reflect.DeepEqual(projectParent.ReferenceTo, []string{"pkg__ProjectTarget__c"}) || projectParent.RelationshipName != "pkg__ProjectParent__r" {
		t.Fatalf("partial project relationship authority = %#v, %v", projectParent, ok)
	}

	model := buildTypeMembers(typesys.Index{Project: typesys.ProjectInfo{Namespace: "pkg"}, Objects: []schema.Object{object}})
	contact := model.members[normalizeName("Contact")]
	if field := contact.fields[normalizeName("Account")]; field.Type != "Account" {
		t.Fatalf("Contact.Account type = %q, want standard relationship target Account", field.Type)
	}
	if field := contact.fields[normalizeName("HasOptedOutOfEmail")]; field.Type != "Boolean" {
		t.Fatalf("Contact.HasOptedOutOfEmail type = %q, want Boolean", field.Type)
	}
	for _, name := range []string{"pkg__ProjectParent__c", "ProjectParent__c"} {
		if field := contact.fields[normalizeName(name)]; field.Type != "Id" {
			t.Errorf("Contact.%s type = %q, want Id", name, field.Type)
		}
	}
	for _, name := range []string{"pkg__ProjectParent__r", "ProjectParent__r"} {
		if field := contact.fields[normalizeName(name)]; field.Type != "pkg__ProjectTarget__c" {
			t.Errorf("Contact.%s type = %q, want pkg__ProjectTarget__c", name, field.Type)
		}
	}
}

func TestTypeMembersSchemaPresentChangeEventUsesBaseStandardShape(t *testing.T) {
	model := buildTypeMembers(typesys.Index{Objects: []schema.Object{{
		Name:   "ContactPointAddressChangeEvent",
		Fields: []schema.Field{{Name: "DeclaredOnly__c", Type: "Text"}},
	}}})
	changeEvent := model.members[normalizeName("ContactPointAddressChangeEvent")]
	for _, test := range []struct {
		name string
		typ  string
	}{
		{name: "DeclaredOnly__c", typ: "String"},
		{name: "PreferenceRank", typ: "Integer"},
		{name: "ChangeEventHeader", typ: "EventBus.ChangeEventHeader"},
	} {
		if field := changeEvent.fields[normalizeName(test.name)]; field.Type != test.typ {
			t.Errorf("schema-present %s.%s type = %q, want %q", changeEvent.name, test.name, field.Type, test.typ)
		}
	}
}

func TestTypeMembersOnlyAddOwnerForAuthoritativeOwnerShapes(t *testing.T) {
	model := buildTypeMembers(typesys.Index{Objects: []schema.Object{
		{Name: "Feature__mdt"},
		{Name: "Thing__c"},
		{Name: "Explicit__mdt", Fields: []schema.Field{{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"Name"}, RelationshipName: "Owner"}}},
	}})
	for _, fieldName := range []string{"OwnerId", "Owner"} {
		if _, exists := model.members[normalizeName("Feature__mdt")].fields[normalizeName(fieldName)]; exists {
			t.Errorf("Feature__mdt unexpectedly has %s", fieldName)
		}
	}
	view := newSemaTypeMemberState(model).view()
	for _, objectName := range []string{"Account", "Thing__c", "Explicit__mdt"} {
		key := normalizeName(objectName)
		members := semaEnsureStandardSObjectTypeMembers(view, key, model.members[key])
		for _, fieldName := range []string{"OwnerId", "Owner"} {
			if _, exists := members.fields[normalizeName(fieldName)]; !exists {
				t.Errorf("%s missing %s", objectName, fieldName)
			}
		}
	}
}

func TestTypeMembersSyntheticChangeEventStillUsesBaseStandardShape(t *testing.T) {
	changeEvent := semaBuildStandardSObjectMembers("ContactPointAddressChangeEvent")
	if field := changeEvent.fields[normalizeName("PreferenceRank")]; field.Type != "Integer" {
		t.Fatalf("synthetic PreferenceRank type = %q, want Integer", field.Type)
	}
	if field := changeEvent.fields[normalizeName("ChangeEventHeader")]; field.Type != "EventBus.ChangeEventHeader" {
		t.Fatalf("synthetic ChangeEventHeader type = %q, want EventBus.ChangeEventHeader", field.Type)
	}
}

func TestSemaStandardSObjectMembersCacheIsNamesOnly(t *testing.T) {
	resetSemaStandardSObjectMembersCacheForTest()
	defer resetSemaStandardSObjectMembersCacheForTest()

	names, members := semaStandardSObjectMembers()
	if len(names) == 0 {
		t.Fatalf("semaStandardSObjectMembers returned no names")
	}
	if len(members) != 0 {
		t.Fatalf("semaStandardSObjectMembers eagerly materialized %d member sets", len(members))
	}
	if name, ok := semaStandardSObjectNameForKey(normalizeName("Account")); !ok || name != "Account" {
		t.Fatalf("semaStandardSObjectNameForKey(Account) = %q, %v", name, ok)
	}
	if len(members) != 0 {
		t.Fatalf("semaStandardSObjectNameForKey materialized %d member sets", len(members))
	}
}

func resetSemaStandardSObjectMembersCacheForTest() {
	semaStandardSObjectMembersCache = struct {
		once      sync.Once
		names     []string
		members   map[string]typeMembers
		nameByKey map[string]string
	}{}
}

func TestAddStandardSObjectMembersUsesLazyPlaceholdersForUnreferencedObjects(t *testing.T) {
	resetSemaStandardSObjectMembersCacheForTest()
	defer resetSemaStandardSObjectMembersCacheForTest()

	model := buildTypeMembers(typesys.Index{})
	account, ok := model.members[normalizeName("Account")]
	if !ok {
		t.Fatalf("missing Account placeholder")
	}
	if account.fields != nil {
		t.Fatalf("Account placeholder eagerly materialized %d fields", len(account.fields))
	}
}

func TestTypeMemberCurrentOverlaySharesBase(t *testing.T) {
	first := typesys.TypeSymbol{
		Kind: apexast.DeclarationClass,
		Name: "Duplicate",
		Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationField, Name: "firstField", Type: "String"},
			{Kind: apexast.DeclarationMethod, Name: "firstHelper", Type: "void"},
		},
	}
	second := typesys.TypeSymbol{
		Kind: apexast.DeclarationClass,
		Name: "Duplicate",
		Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationField, Name: "secondField", Type: "Integer"},
			{Kind: apexast.DeclarationMethod, Name: "secondHelper", Type: "void"},
		},
	}
	unrelated := typesys.TypeSymbol{
		Kind: apexast.DeclarationClass,
		Name: "Unrelated",
		Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationField, Name: "sharedField", Type: "Boolean"},
		},
	}

	base := buildTypeMembers(typesys.Index{Types: []typesys.TypeSymbol{first, unrelated}})
	before := base.members[normalizeName(first.Name)]
	state := newSemaTypeMemberState(base)
	view := semaModelWithCurrentType(state.view(), second)

	if got := len(view.current); got > 2 {
		t.Fatalf("current overlay has %d aliases, want at most 2", got)
	}
	current, _, ok := semaLookupTypeMembers(view, second.Name)
	if !ok {
		t.Fatal("current duplicate type is missing from overlay")
	}
	if _, ok := current.fields[normalizeName("secondField")]; !ok {
		t.Fatalf("current duplicate fields = %#v, want secondField", current.fields)
	}
	if _, ok := current.methods[normalizeName("secondHelper")]; !ok {
		t.Fatalf("current duplicate methods = %#v, want secondHelper", current.methods)
	}
	shared, _, ok := semaLookupTypeMembers(view, unrelated.Name)
	if !ok || shared.name != unrelated.Name {
		t.Fatalf("unrelated lookup = %#v, %v; want base member", shared, ok)
	}
	if !reflect.DeepEqual(base.members[normalizeName(first.Name)], before) {
		t.Fatalf("base duplicate members changed\nwant: %#v\n got: %#v", before, base.members[normalizeName(first.Name)])
	}
	if _, ok := base.members[normalizeName(second.Name)].fields[normalizeName("secondField")]; ok {
		t.Fatalf("current duplicate field leaked into base: %#v", base.members[normalizeName(second.Name)].fields)
	}
}

func TestTypeMemberShortCandidateCollisionPreservesIndexOrder(t *testing.T) {
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "First.Shared"},
		{Kind: apexast.DeclarationClass, Name: "Second.Shared"},
	}}
	model := buildTypeMembers(index)
	want := []string{normalizeName("First.Shared"), normalizeName("Second.Shared")}
	if got := model.shortCandidates[normalizeName("Shared")]; !reflect.DeepEqual(got, want) {
		t.Fatalf("Shared candidates = %#v, want %#v", got, want)
	}
}
