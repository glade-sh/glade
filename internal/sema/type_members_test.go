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
	originalName := names[0]
	names[0] = "CallerCorruption"
	members[normalizeName("CallerCorruption")] = typeMembers{name: "CallerCorruption"}
	namesAgain, membersAgain := semaStandardSObjectMembers()
	if namesAgain[0] != originalName {
		t.Fatalf("caller mutation changed cached standard object name from %q to %q", originalName, namesAgain[0])
	}
	if _, leaked := membersAgain[normalizeName("CallerCorruption")]; leaked {
		t.Fatal("caller mutation changed cached standard object members")
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

func TestStandardChildRelationshipMembersDoNotExposeSharedCache(t *testing.T) {
	relationships := semaStandardChildRelationshipMembers("Account")
	want := append([]semaStandardChildRelationshipMember(nil), relationships...)
	if len(want) == 0 {
		t.Fatal("Account has no standard child relationships for cache isolation test")
	}
	relationships[0].name = "CallerCorruption"
	if got := semaStandardChildRelationshipMembers("Account"); !reflect.DeepEqual(got, want) {
		t.Fatalf("caller mutation changed standard child relationships: got %#v, want %#v", got, want)
	}
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

func TestPrepareAnalysisIndexKeepsPlatformSymbolsOutOfWorkspaceIndex(t *testing.T) {
	projectType := typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: "ProjectOnly"}
	index := typesys.Index{Types: []typesys.TypeSymbol{projectType}}

	prepared := prepareAnalysisIndex(index)
	if !reflect.DeepEqual(prepared.Types, []typesys.TypeSymbol{projectType}) {
		t.Fatalf("prepared workspace types = %#v, want project-only index", prepared.Types)
	}

	analyzer := NewAnalyzer()
	analyzer.prepareAnalysisContext(prepared, AnalyzeOptions{})
	analyzer.prepareAnalysisModel(prepared)
	for _, symbol := range typesys.StandardPlatformSymbolView() {
		name := symbol.Name
		if symbol.Namespace != "" {
			name = symbol.Namespace + "." + symbol.Name
		}
		if !analyzer.hasKnown(name) {
			t.Fatalf("platform type %q is not known without index hydration", name)
		}
	}
}

func TestTypeMemberStateResolvesPlatformOutsideWorkspaceIndex(t *testing.T) {
	state := buildSemaTypeMemberState(typesys.Index{}, nil)
	if _, _, ok := semaLookupTypeMembers(state.view(), "Math"); !ok {
		t.Fatal("analysis state cannot resolve platform Math outside the workspace index")
	}
}

func TestTypeMemberStateSharesImmutablePlatformModel(t *testing.T) {
	first := buildSemaTypeMemberState(typesys.Index{}, nil)
	second := buildSemaTypeMemberState(typesys.Index{}, nil)
	if first.platform == nil || first.platform.platform == nil || len(first.platform.platform.symbols) == 0 {
		t.Fatal("analysis state has no platform member model")
	}
	if first.platform != second.platform {
		t.Fatal("analysis states did not share the process platform model")
	}
}

func TestLazyPlatformTypeMemberModelDoesNotCacheUnknownLookups(t *testing.T) {
	model := newSemaLazyPlatformTypeMemberModel(nil)
	cacheSize := func() int {
		size := 0
		model.lookups.Range(func(_, _ any) bool {
			size++
			return true
		})
		return size
	}

	for i := 0; i < 256; i++ {
		key := "projectonlytype" + strings.Repeat("x", i+1)
		if _, ok := model.lookup(key); ok {
			t.Fatalf("unknown lookup %q unexpectedly resolved", key)
		}
	}
	if size := cacheSize(); size != 0 {
		t.Fatalf("unknown lookups grew shared platform cache to %d entries", size)
	}
}

func TestLazyPlatformTypeMemberModelDoesNotExposeCachedMathMembers(t *testing.T) {
	model := buildSemaPlatformTypeMemberModel().platform
	key := normalizeName("Math")
	fieldKey := normalizeName("PI")
	first, ok := model.lookup(key)
	if !ok {
		t.Fatal("shared platform model omitted Math")
	}
	pi, ok := first.fields[fieldKey]
	if !ok || len(pi.Modifiers) == 0 {
		t.Fatalf("Math.PI = %#v, %v; want a field with modifiers", pi, ok)
	}
	wantModifier := pi.Modifiers[0]
	first.fields[normalizeName("CallerOnly")] = typesys.MemberSymbol{Name: "CallerOnly"}
	pi.Modifiers[0] = "caller-mutated"
	first.fields[fieldKey] = pi

	second, ok := model.lookup(key)
	if !ok {
		t.Fatal("shared platform model lost Math after caller mutation")
	}
	if _, leaked := second.fields[normalizeName("CallerOnly")]; leaked {
		t.Fatal("caller field mutation leaked into the shared Math cache")
	}
	if got := second.fields[fieldKey].Modifiers[0]; got != wantModifier {
		t.Fatalf("caller modifier mutation leaked into Math.PI: got %q, want %q", got, wantModifier)
	}

	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			members, lookupOK := model.lookup(key)
			if !lookupOK {
				return
			}
			field := members.fields[fieldKey]
			field.Modifiers[0] = "concurrent-caller-mutation"
			members.fields[fieldKey] = field
		}()
	}
	workers.Wait()
	final, ok := model.lookup(key)
	if !ok || final.fields[fieldKey].Modifiers[0] != wantModifier {
		t.Fatalf("concurrent caller mutation changed shared Math.PI: %#v, %v", final.fields[fieldKey], ok)
	}
}

func TestLazyPlatformTypeMemberModelDeepClonesSourceAndLookupSlices(t *testing.T) {
	newSymbol := func() typesys.TypeSymbol {
		return typesys.TypeSymbol{
			Kind:       apexast.DeclarationClass,
			Name:       "SharedPlatformType",
			Interfaces: []string{"Comparable"},
			Members: []typesys.MemberSymbol{
				{
					Kind:       apexast.DeclarationMethod,
					Name:       "run",
					Type:       "void",
					Modifiers:  []string{"public"},
					Parameters: []apexast.Parameter{{Name: "value", Type: "String", Modifiers: []string{"final"}}},
					Accessors:  []apexast.Accessor{{Kind: "get", Modifiers: []string{"public"}}},
				},
				{
					Kind:       apexast.DeclarationConstructor,
					Name:       "SharedPlatformType",
					Modifiers:  []string{"public"},
					Parameters: []apexast.Parameter{{Name: "value", Type: "String", Modifiers: []string{"final"}}},
				},
				{
					Kind:      apexast.DeclarationProperty,
					Name:      "Value",
					Type:      "String",
					Modifiers: []string{"public"},
					Accessors: []apexast.Accessor{{Kind: "get", Modifiers: []string{"public"}}},
				},
			},
		}
	}

	symbols := []typesys.TypeSymbol{newSymbol()}
	want := semaTypeMembersFromPlatformSymbol(newSymbol())
	model := newSemaLazyPlatformTypeMemberModel(symbols)
	key := normalizeName(symbols[0].Name)
	first, ok := model.lookup(key)
	if !ok {
		t.Fatal("synthetic platform symbol did not resolve")
	}
	first.interfaces[0] = "CallerMutation"
	first.methods[normalizeName("run")][0].Modifiers[0] = "caller-mutated"
	first.methods[normalizeName("run")][0].Parameters[0].Type = "CallerMutation"
	first.methods[normalizeName("run")][0].Parameters[0].Modifiers[0] = "caller-mutated"
	first.methods[normalizeName("run")][0].Accessors[0].Kind = "caller-mutated"
	first.methods[normalizeName("run")][0].Accessors[0].Modifiers[0] = "caller-mutated"
	first.constructors[0].Parameters[0].Type = "CallerMutation"
	property := first.fields[normalizeName("Value")]
	property.Modifiers[0] = "caller-mutated"
	property.Accessors[0].Kind = "caller-mutated"
	property.Accessors[0].Modifiers[0] = "caller-mutated"
	first.fields[normalizeName("Value")] = property

	symbols[0].Interfaces[0] = "SourceMutation"
	symbols[0].Members[0].Modifiers[0] = "source-mutated"
	symbols[0].Members[0].Parameters[0].Type = "SourceMutation"
	symbols[0].Members[0].Parameters[0].Modifiers[0] = "source-mutated"
	symbols[0].Members[0].Accessors[0].Kind = "source-mutated"
	symbols[0].Members[0].Accessors[0].Modifiers[0] = "source-mutated"

	second, ok := model.lookup(key)
	if !ok || !reflect.DeepEqual(second, want) {
		t.Fatalf("shared platform members changed after caller/source mutation:\ngot:  %#v\nwant: %#v", second, want)
	}
}

func TestTypeMemberViewDoesNotExposeSharedPlatformCandidateSlices(t *testing.T) {
	lazy := newSemaLazyPlatformTypeMemberModel([]typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "First.Shared"},
		{Kind: apexast.DeclarationClass, Name: "Second.Shared"},
	})
	platform := &semaTypeMemberModel{members: map[string]typeMembers{}, platform: lazy}
	view := newSemaTypeMemberStateWithPlatform(nil, platform).view()
	short := normalizeName("Shared")
	candidates := view.shortCandidateKeys(short)
	want := append([]string(nil), candidates...)
	if len(want) < 2 {
		t.Fatalf("shared candidate list = %#v, want at least two entries", want)
	}
	candidates[0] = "caller-corruption"
	if got := view.shortCandidateKeys(short); !reflect.DeepEqual(got, want) {
		t.Fatalf("caller mutation changed shared candidates: got %#v, want %#v", got, want)
	}

	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			got := view.shortCandidateKeys(short)
			got[0] = "concurrent-caller-corruption"
		}()
	}
	workers.Wait()
	if got := view.shortCandidateKeys(short); !reflect.DeepEqual(got, want) {
		t.Fatalf("concurrent caller mutation changed shared candidates: got %#v, want %#v", got, want)
	}
}

func TestTypeMemberHydrationStaysAnalysisLocal(t *testing.T) {
	base := buildTypeMembers(typesys.Index{})
	first := newSemaTypeMemberState(base)
	second := newSemaTypeMemberState(base)
	key := normalizeName("Account")

	if _, _, ok := semaLookupTypeMembers(first.view(), "Account"); !ok {
		t.Fatal("first state could not hydrate Account")
	}
	if len(first.hydrated[key].fields) == 0 {
		t.Fatal("first state did not retain local Account hydration")
	}
	if _, exists := second.hydrated[key]; exists {
		t.Fatal("Account hydration leaked into a second analysis state")
	}
	if placeholder := base.members[key]; placeholder.fields != nil || placeholder.standardSObjectFieldsLoaded {
		t.Fatalf("Account hydration mutated shared base placeholder: %#v", placeholder)
	}
}

func TestTypeMemberProjectOverlayShadowsPlatformWithoutMutation(t *testing.T) {
	const sentinel = "projectSentinel"
	projectMath := typesys.TypeSymbol{
		Kind: apexast.DeclarationClass,
		Name: "Math",
		Members: []typesys.MemberSymbol{{
			Kind: apexast.DeclarationField,
			Name: sentinel,
			Type: "String",
		}},
	}
	platform := semaPlatformTypeMemberModel()
	before, ok := platform.lookup(normalizeName(projectMath.Name))
	if !ok {
		t.Fatal("shared platform model omitted Math")
	}
	state := buildSemaTypeMemberState(typesys.Index{Types: []typesys.TypeSymbol{projectMath}}, nil)

	members, _, ok := semaLookupTypeMembers(state.view(), projectMath.Name)
	if !ok {
		t.Fatal("project Math overlay is missing")
	}
	if _, ok := members.fields[normalizeName(sentinel)]; !ok {
		t.Fatalf("project Math fields = %#v, want %s", members.fields, sentinel)
	}
	after, ok := platform.lookup(normalizeName(projectMath.Name))
	if !ok || !reflect.DeepEqual(after, before) {
		t.Fatal("project overlay mutated the shared platform Math members")
	}
	if _, ok := after.fields[normalizeName(sentinel)]; ok {
		t.Fatal("project field leaked into the shared platform model")
	}
}

func TestTypeMemberDependencyOverlayKeepsQualifiedAndShortResolution(t *testing.T) {
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind:       apexast.DeclarationClass,
			Name:       "Outer.Inner",
			Namespace:  "pkg",
			Dependency: true,
			Artifact:   true,
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationField,
				Name: "dependencyField",
				Type: "String",
			}},
		},
		{Kind: apexast.DeclarationClass, Name: "Local.Inner"},
	}}
	state := buildSemaTypeMemberState(index, nil)
	view := state.view()

	if _, _, ok := semaLookupTypeMembers(view, "Outer.Inner"); ok {
		t.Fatal("artifact dependency resolved without its namespace")
	}
	dependency, _, ok := semaLookupTypeMembers(view, "pkg.Outer.Inner")
	if !ok || dependency.name != "pkg.Outer.Inner" {
		t.Fatalf("qualified dependency lookup = %#v, %v", dependency, ok)
	}
	candidates := view.shortCandidateKeys(normalizeName("Inner"))
	wantPrefix := []string{normalizeName("pkg.Outer.Inner"), normalizeName("Local.Inner")}
	if len(candidates) < len(wantPrefix) || !reflect.DeepEqual(candidates[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("short candidates = %#v, want workspace index order prefix %#v", candidates, wantPrefix)
	}
}

func TestTypeMemberPlatformLayerPreservesConstructorsInterfacesEnumsAndStaticFields(t *testing.T) {
	view := buildSemaTypeMemberState(typesys.Index{}, nil).view()

	exception, _, ok := semaLookupTypeMembers(view, "Exception")
	if !ok || len(exception.constructors) != 5 {
		t.Fatalf("Exception constructors = %#v, %v; want five platform overloads", exception.constructors, ok)
	}
	iterator, _, ok := semaLookupTypeMembers(view, "Iterator")
	if !ok || iterator.kind != apexast.DeclarationInterface || len(iterator.methods[normalizeName("hasNext")]) == 0 {
		t.Fatalf("Iterator platform shape = %#v, %v", iterator, ok)
	}
	accessLevel, _, ok := semaLookupTypeMembers(view, "AccessLevel")
	if !ok || accessLevel.kind != apexast.DeclarationEnum || len(accessLevel.methods[normalizeName("withPermissionSetId")]) == 0 {
		t.Fatalf("AccessLevel platform shape = %#v, %v", accessLevel, ok)
	}
	math, _, ok := semaLookupTypeMembers(view, "Math")
	pi, hasPI := math.fields[normalizeName("PI")]
	if !ok || !hasPI || !hasModifier(pi.Modifiers, "static") {
		t.Fatalf("Math.PI platform shape = %#v, %v, %v", pi, ok, hasPI)
	}
}

func TestExportTypesWorkspaceSymbolsShadowPlatformWithoutMutatingPlatformModel(t *testing.T) {
	const (
		mathFile     = "project/Math.cls"
		databaseFile = "project/Database.cls"
		sentinel     = "projectSentinel"
	)
	platform := semaPlatformTypeMemberModel()
	mathBefore, mathOK := platform.lookup(normalizeName("Math"))
	databaseBefore, databaseOK := platform.lookup(normalizeName("Database"))
	if !mathOK || !databaseOK {
		t.Fatalf("shared platform lookup Math=%v Database=%v, want both", mathOK, databaseOK)
	}

	result := AnalyzeWithOptions(typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass,
			Name: "Math",
			File: mathFile,
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationField,
				Name: sentinel,
				Type: "String",
			}},
		},
		{
			Kind: apexast.DeclarationClass,
			Name: "Database",
			File: databaseFile,
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationClass,
				Name: "QueryLocator",
			}},
		},
	}}, AnalyzeOptions{ExportTypes: true})

	for name, want := range map[string]TypeReference{
		"Math":                  {Name: "Math", Kind: TypeApex, Source: mathFile},
		"Database.QueryLocator": {Name: "Database.QueryLocator", Kind: TypeApex, Source: databaseFile},
	} {
		if got := result.Types[name]; got != want {
			t.Errorf("exported %s = %#v, want %#v", name, got, want)
		}
	}
	mathAfter, mathOK := platform.lookup(normalizeName("Math"))
	if !mathOK || !reflect.DeepEqual(mathAfter, mathBefore) {
		t.Fatal("exporting project Math mutated the shared platform Math model")
	}
	databaseAfter, databaseOK := platform.lookup(normalizeName("Database"))
	if !databaseOK || !reflect.DeepEqual(databaseAfter, databaseBefore) {
		t.Fatal("exporting project Database mutated the shared platform Database model")
	}
}

func TestExportTypesDependencySymbolShadowsPlatformSource(t *testing.T) {
	const dependencyFile = "dependencies/base/Math.cls"
	result := AnalyzeWithOptions(typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind:       apexast.DeclarationClass,
			Name:       "Math",
			File:       dependencyFile,
			Dependency: true,
			Artifact:   true,
		},
		{
			Kind:       apexast.DeclarationClass,
			Name:       "InboundEmail",
			Namespace:  "Messaging",
			File:       dependencyFile,
			Dependency: true,
			Artifact:   true,
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationClass,
				Name: "Header",
			}},
		},
	}}, AnalyzeOptions{ExportTypes: true})

	for name, want := range map[string]TypeReference{
		"Math":                          {Name: "Math", Kind: TypePlatform, Source: dependencyFile},
		"Messaging.InboundEmail.Header": {Name: "Messaging.InboundEmail.Header", Kind: TypePlatform, Source: dependencyFile},
	} {
		if got := result.Types[name]; got != want {
			t.Errorf("exported dependency %s = %#v, want %#v", name, got, want)
		}
	}
}
