package sema

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
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

func TestLayeredModelCutoverFeatureFieldsKeepExactTypesAndObjectIsolation(t *testing.T) {
	account := schema.Object{
		Name: "Account",
		Fields: []schema.Field{
			{Name: "BillingCountryCode", Type: "Text"},
			{Name: "ShippingStateCode", Type: "Text"},
			{Name: "CurrencyIsoCode", Type: "Text"},
		},
	}
	provider := newSemaSObjectFieldProvider("", account)
	for _, test := range []struct {
		name string
		typ  string
	}{
		{name: "PersonContactId", typ: "REFERENCE"},
		{name: "FirstName", typ: "STRING"},
		{name: "PersonDoNotCall", typ: "Boolean"},
		{name: "BillingCountryCode", typ: "Text"},
		{name: "ShippingStateCode", typ: "Text"},
		{name: "CurrencyIsoCode", typ: "Text"},
	} {
		field, ok := provider.lookup(test.name)
		if !ok || field.Name != test.name || !strings.EqualFold(field.Type, test.typ) {
			t.Errorf("Account.%s = %#v, %v; want exact %s shape", test.name, field, ok, test.typ)
		}
	}
	personContact, _ := provider.lookup("PersonContactId")
	if !reflect.DeepEqual(personContact.ReferenceTo, []string{"Contact"}) || personContact.RelationshipName != "PersonContact" {
		t.Fatalf("Account.PersonContactId relationship = %#v", personContact)
	}

	other := newSemaSObjectFieldProvider("", schema.Object{Name: "Group"})
	for _, name := range []string{"PersonContactId", "PersonDoNotCall", "BillingCountryCode", "ShippingStateCode", "CurrencyIsoCode"} {
		if field, ok := other.lookup(name); ok {
			t.Errorf("Account feature field %s leaked to Group as %#v", name, field)
		}
	}
	if !reflect.DeepEqual(account.Fields, []schema.Field{
		{Name: "BillingCountryCode", Type: "Text"},
		{Name: "ShippingStateCode", Type: "Text"},
		{Name: "CurrencyIsoCode", Type: "Text"},
	}) {
		t.Fatalf("layered lookup mutated declared Account fields: %#v", account.Fields)
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

func TestTypeMembersParentRelationshipReplacesSyntheticChildAliasInEitherOrder(t *testing.T) {
	child := schema.Object{
		Name: "pkg__Merchandise__c",
		Fields: []schema.Field{{
			Name:                          "pkg__OrderItemLine__c",
			Type:                          "Lookup",
			ReferenceTo:                   []string{"pkg__OrderItemLine__c"},
			ChildRelationshipName:         "pkg__Merchandise__r",
			ChildRelationshipNameInferred: true,
		}},
	}
	parent := schema.Object{
		Name: "pkg__OrderItemLine__c",
		Fields: []schema.Field{{
			Name:             "pkg__Merchandise__c",
			Type:             "Lookup",
			ReferenceTo:      []string{"pkg__Merchandise__c"},
			RelationshipName: "pkg__Merchandise__r",
		}},
	}

	for _, test := range []struct {
		name    string
		objects []schema.Object
	}{
		{name: "child first", objects: []schema.Object{child, parent}},
		{name: "parent first", objects: []schema.Object{parent, child}},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := buildSemaTypeMemberView(typesys.Index{Project: typesys.ProjectInfo{Namespace: "pkg"}, Objects: test.objects})
			resolved, ok := semaResolveField(model, "pkg__OrderItemLine__c", "pkg__Merchandise__r", map[string]bool{})
			if !ok || resolved.member.Type != "pkg__Merchandise__c" {
				t.Fatalf("parent relationship = %#v, %v; want pkg__Merchandise__c", resolved, ok)
			}
		})
	}
}

func TestTypeMembersKeepCanonicalChildRelationshipWhenItCollidesWithParentName(t *testing.T) {
	child := schema.Object{
		Name: "pkg__Merchandise__c",
		Fields: []schema.Field{{
			Name:                  "pkg__OrderItemLine__c",
			Type:                  "Lookup",
			ReferenceTo:           []string{"pkg__OrderItemLine__c"},
			ChildRelationshipName: "pkg__Merchandise__r",
		}},
	}
	parent := schema.Object{
		Name: "pkg__OrderItemLine__c",
		Fields: []schema.Field{{
			Name:             "pkg__Merchandise__c",
			Type:             "Lookup",
			ReferenceTo:      []string{"pkg__Merchandise__c"},
			RelationshipName: "pkg__Merchandise__r",
		}},
	}

	for _, test := range []struct {
		name    string
		objects []schema.Object
	}{
		{name: "child first", objects: []schema.Object{child, parent}},
		{name: "parent first", objects: []schema.Object{parent, child}},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := buildSemaTypeMemberView(typesys.Index{Project: typesys.ProjectInfo{Namespace: "pkg"}, Objects: test.objects})
			resolved, ok := semaResolveField(model, "pkg__OrderItemLine__c", "pkg__Merchandise__r", map[string]bool{})
			if !ok || resolved.member.Type != "List<pkg__Merchandise__c>" {
				t.Fatalf("canonical child relationship = %#v, %v; want List<pkg__Merchandise__c>", resolved, ok)
			}
		})
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
		members, _, ok := semaLookupTypeMembers(view, objectName)
		if !ok {
			t.Errorf("%s is not resolvable", objectName)
			continue
		}
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

func resetSemaStandardSObjectMembersCacheForTest() {
	semaStandardSObjectMembersCache = struct {
		once      sync.Once
		names     []string
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

func TestBuildTypeMembersDoesNotRetainPerAnalysisStandardPlaceholders(t *testing.T) {
	resetSemaStandardSObjectMembersCacheForTest()
	defer resetSemaStandardSObjectMembersCacheForTest()

	model := buildTypeMembers(typesys.Index{})
	if account, ok := model.members[normalizeName("Account")]; ok {
		t.Fatalf("per-analysis model retained Account placeholder: %#v", account)
	}
	if _, _, ok := semaLookupTypeMembers(newSemaTypeMemberState(model).view(), "Account"); !ok {
		t.Fatal("process-level standard-object lookup no longer resolves Account")
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
		if semaAPI67RejectedPlatformType(name) {
			continue
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
	firstView := first.view()
	secondView := second.view()

	if _, _, ok := semaLookupTypeMembers(firstView, "Account"); !ok {
		t.Fatal("first state could not hydrate Account")
	}
	if len(firstView.hydrated[key].fields) == 0 {
		t.Fatal("first state did not retain local Account hydration")
	}
	if _, exists := secondView.hydrated[key]; exists {
		t.Fatal("Account hydration leaked into a second analysis state")
	}
	if _, exists := base.members[key]; exists {
		t.Fatalf("Account hydration retained a per-analysis base placeholder")
	}
}

func TestTypeMemberViewsKeepHydrationPrivate(t *testing.T) {
	key := normalizeName("Account")
	state := newSemaTypeMemberState(&semaTypeMemberModel{
		members: map[string]typeMembers{key: semaStandardSObjectPlaceholder("Account")},
	})
	first := state.view()
	second := state.view()

	firstAccount, _, ok := semaLookupTypeMembers(first, "Account")
	if !ok {
		t.Fatal("first view could not hydrate Account")
	}
	const sentinel = "FirstViewOnly"
	firstAccount.fields[normalizeName(sentinel)] = typesys.MemberSymbol{Name: sentinel, Type: "String"}
	first.storeHydrated(key, firstAccount)

	secondAccount, _, ok := semaLookupTypeMembers(second, "Account")
	if !ok {
		t.Fatal("second view could not hydrate Account")
	}
	if _, leaked := secondAccount.fields[normalizeName(sentinel)]; leaked {
		t.Fatal("hydrated sentinel leaked between type-member views")
	}
}

func TestFrozenTypeMemberViewsHydrateConcurrently(t *testing.T) {
	state := buildSemaTypeMemberState(typesys.Index{}, nil)
	reference, _, ok := semaLookupTypeMembers(state.view(), "Account")
	if !ok {
		t.Fatal("sequential reference could not hydrate Account")
	}

	const workers = 16
	errors := make(chan string, workers)
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		group.Add(1)
		go func() {
			defer group.Done()
			view := state.view()
			account, _, lookupOK := semaLookupTypeMembers(view, "Account")
			if !lookupOK {
				errors <- fmt.Sprintf("worker %d could not hydrate Account", worker)
				return
			}
			if !reflect.DeepEqual(account, reference) {
				errors <- fmt.Sprintf("worker %d hydration differs from sequential reference", worker)
				return
			}
			sentinel := normalizeName(fmt.Sprintf("Worker%dOnly", worker))
			account.fields[sentinel] = typesys.MemberSymbol{Name: sentinel, Type: "String"}
			view.storeHydrated(normalizeName("Account"), account)
			if _, exists := view.hydrated[normalizeName("Account")].fields[sentinel]; !exists {
				errors <- fmt.Sprintf("worker %d lost its private hydration", worker)
			}
		}()
	}
	group.Wait()
	close(errors)
	for message := range errors {
		t.Error(message)
	}
}

func TestOneWorkerViewPreservesCrossPhaseHydrationOrder(t *testing.T) {
	root := t.TempDir()
	baseFile := filepath.Join(root, "EmailTemplate.cls")
	wrapperFile := filepath.Join(root, "EmailTemplateWrapper.cls")
	writeSemaFile(t, baseFile, `
public abstract class EmailTemplate {
    public abstract String getId();
}
`)
	writeSemaFile(t, wrapperFile, `
public class EmailTemplateWrapper extends EmailTemplate {
    private Schema.EmailTemplate templateRecord;
    public override String getId() {
        return templateRecord.Id;
    }
}
`)
	index, artifacts := typesys.BuildWithArtifacts(project.Project{
		Root:      root,
		ApexFiles: []string{baseFile, wrapperFile},
	}, schema.Schema{})
	result := AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:    true,
		BuildArtifacts: &artifacts,
	})

	count := 0
	for _, item := range result.Diagnostics {
		if item.Code == "GLADESEMA016" && item.File == wrapperFile {
			count++
		}
	}
	if count != 0 {
		t.Fatalf("explicit Schema.EmailTemplate hydration shadowed the project EmailTemplate class: %#v", result.Diagnostics)
	}
}

func TestCurrentTypeOverlayKeepsWorkerHydrationHistory(t *testing.T) {
	state := buildSemaTypeMemberState(typesys.Index{}, nil)
	worker := state.view()
	overlay := semaModelWithCurrentType(worker, typesys.TypeSymbol{
		Kind: apexast.DeclarationClass,
		Name: "Duplicate",
	})
	if _, _, ok := semaLookupTypeMembers(overlay, "Account"); !ok {
		t.Fatal("current-type overlay could not hydrate Account")
	}
	key := normalizeName("Account")
	if len(worker.hydrated[key].fields) == 0 {
		t.Fatal("current-type overlay did not retain hydration in its owning worker view")
	}
}

func TestAnalyzeDuplicateOverlayHydrationPreservesUnknownCallOrder(t *testing.T) {
	root := t.TempDir()
	emailTemplateFile := filepath.Join(root, "EmailTemplate.cls")
	firstHydratorFile := filepath.Join(root, "HydratorOne.cls")
	secondHydratorFile := filepath.Join(root, "HydratorTwo.cls")
	orderedCallsFile := filepath.Join(root, "OrderedCalls.cls")
	orderedCallsSource := `public class OrderedCalls {
    public void run() {
        Integer value = EmailTemplate.known();
        EmailTemplate.missingFirst();
        EmailTemplate.missingSecond();
    }
}
`
	writeSemaFile(t, emailTemplateFile, `public class EmailTemplate {
    public static Integer known() { return 1; }
}
`)
	writeSemaFile(t, firstHydratorFile, `public class Hydrator {
    private Schema.EmailTemplate templateRecord;
    public void hydrate() { String value = templateRecord.Id; }
}
`)
	writeSemaFile(t, secondHydratorFile, `public class Hydrator {
    public void secondCopy() {}
}
`)
	writeSemaFile(t, orderedCallsFile, orderedCallsSource)
	index, artifacts := typesys.BuildWithArtifacts(project.Project{
		Root: root,
		ApexFiles: []string{
			emailTemplateFile,
			firstHydratorFile,
			secondHydratorFile,
			orderedCallsFile,
		},
	}, schema.Schema{})
	result := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts})

	type identity struct {
		code    string
		message string
		range_  diagnostic.Range
	}
	var got []identity
	for _, item := range result.Diagnostics {
		if item.Code != "GLADESEMA008" || item.File != orderedCallsFile || item.Range == nil {
			continue
		}
		got = append(got, identity{code: item.Code, message: item.Message, range_: *item.Range})
	}
	expectedRange := func(callee string) diagnostic.Range {
		start := strings.Index(orderedCallsSource, callee)
		if start < 0 {
			t.Fatalf("ordered-call fixture is missing %q", callee)
		}
		return *semaRange(orderedCallsSource, start, start+len(callee))
	}
	want := []identity{
		{
			code:    "GLADESEMA008",
			message: `method "run" calls unknown method "EmailTemplate.missingFirst"`,
			range_:  expectedRange("EmailTemplate.missingFirst"),
		},
		{
			code:    "GLADESEMA008",
			message: `method "run" calls unknown method "EmailTemplate.missingSecond"`,
			range_:  expectedRange("EmailTemplate.missingSecond"),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("overlay-sensitive GLADESEMA008 identity/order changed\nwant: %#v\n got: %#v", want, got)
	}
}

func TestZeroBodyDuplicateOverlayHydrationPrecedesLaterDiagnostics(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "LaterDiagnostics.cls")
	source := `public class LaterDiagnostics {
    public void run() {
        OverlayOnly = 1;
        missingHelper();
    }
}
`
	writeSemaFile(t, file, source)
	methodStart := strings.Index(source, "public void run()")
	if methodStart < 0 {
		t.Fatal("later-diagnostic fixture is missing run method")
	}
	methodEnd := strings.LastIndex(source, "    }") + len("    }")
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass,
			Name: "DuplicateWithoutBody",
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationField,
				Name: "Seed",
				Type: "OverlaySeed__c",
			}},
		},
		{Kind: apexast.DeclarationClass, Name: "DuplicateWithoutBody"},
		{
			Kind: apexast.DeclarationClass,
			Name: "LaterDiagnostics",
			File: file,
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationMethod,
				Name: "run",
				Type: "void",
				Range: diagnostic.Range{
					Start: diagnostic.Position{Offset: methodStart},
					End:   diagnostic.Position{Offset: methodEnd},
				},
			}},
		},
	}}
	lazyPlatform := newSemaLazyPlatformTypeMemberModel(nil)
	lazyPlatform.lookups.Store(normalizeName("OverlaySeed__c"), semaPlatformTypeMemberLookup{
		ok: true,
		members: typeMembers{
			name: "OverlaySeed__c",
			kind: apexast.DeclarationClass,
			fields: map[string]typesys.MemberSymbol{
				normalizeName("OverlayOnly"): {
					Kind: apexast.DeclarationField,
					Name: "OverlayOnly",
					Type: "Integer",
				},
			},
			methods: make(map[string][]typesys.MemberSymbol),
		},
	})
	base := buildTypeMemberLayerWithSources(index, newSemaSources(nil, nil), nil)
	state := newSemaTypeMemberStateWithPlatform(base, &semaTypeMemberModel{
		members:  make(map[string]typeMembers),
		platform: lazyPlatform,
	})
	analyzer := NewAnalyzer()
	diagnostics := analyzer.checkMethodBodiesWithView(index, state.view(), nil)

	type identity struct {
		code    string
		message string
		range_  diagnostic.Range
	}
	got := make([]identity, 0, len(diagnostics))
	for _, item := range diagnostics {
		if item.Range == nil {
			continue
		}
		got = append(got, identity{code: item.Code, message: item.Message, range_: *item.Range})
	}
	callStart := strings.Index(source, "missingHelper")
	want := []identity{{
		code:    "GLADESEMA008",
		message: `method "run" calls unknown method "missingHelper"`,
		range_:  *semaRange(source, callStart, callStart+len("missingHelper")),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("zero-body overlay changed later diagnostic identity/order\nwant: %#v\n got: %#v", want, got)
	}
}

func TestMethodBodyWorkerScaffoldKeepsViewsPrivate(t *testing.T) {
	root := t.TempDir()
	workerFile := filepath.Join(root, "WorkerOne.cls")
	workerSource := `public class WorkerOne {
    public void run() {
        Account.primaryOnly();
    }
}
`
	writeSemaFile(t, workerFile, workerSource)
	methodStart := strings.Index(workerSource, "public void run()")
	methodEnd := strings.LastIndex(workerSource, "    }") + len("    }")
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "ZeroBody"},
		{
			Kind: apexast.DeclarationClass,
			Name: "WorkerOne",
			File: workerFile,
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationMethod,
				Name: "run",
				Type: "void",
				Range: diagnostic.Range{
					Start: diagnostic.Position{Offset: methodStart},
					End:   diagnostic.Position{Offset: methodEnd},
				},
			}},
		},
	}}
	state := buildSemaTypeMemberState(index, nil)
	primary := state.view()
	account, _, ok := semaLookupTypeMembers(primary, "Account")
	if !ok {
		t.Fatal("primary view could not hydrate Account")
	}
	methodKey := normalizeName("primaryOnly")
	account.methods[methodKey] = []typesys.MemberSymbol{{
		Kind: apexast.DeclarationMethod,
		Name: "primaryOnly",
		Type: "void",
	}}
	primary.storeHydrated(normalizeName("Account"), account)

	analyzer := NewAnalyzer()
	diagnostics := analyzer.checkMethodBodiesWithViewWorkers(index, primary, nil, 2)
	if len(diagnostics) != 1 {
		t.Fatalf("private-worker diagnostics = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	callee := "Account.primaryOnly"
	callStart := strings.Index(workerSource, callee)
	wantRange := semaRange(workerSource, callStart, callStart+len(callee))
	want := diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA008",
		Message:  `method "run" calls unknown method "Account.primaryOnly"`,
		File:     workerFile,
		Range:    wantRange,
	}
	if !reflect.DeepEqual(diagnostics[0], want) {
		t.Fatalf("private-worker diagnostic\nwant: %#v\n got: %#v", want, diagnostics[0])
	}
}

func TestMethodBodyWorkerScaffoldMergesIndexedOrder(t *testing.T) {
	root := t.TempDir()
	index := typesys.Index{Types: make([]typesys.TypeSymbol, 4)}
	type expectedDiagnostic struct {
		code    string
		message string
		file    string
		range_  diagnostic.Range
	}
	want := make([]expectedDiagnostic, 0, len(index.Types))
	for i := range index.Types {
		typeName := fmt.Sprintf("WorkerType%d", i)
		callee := fmt.Sprintf("missing%d", i)
		file := filepath.Join(root, typeName+".cls")
		source := fmt.Sprintf("public class %s {\n    public void run() {\n        %s();\n    }\n}\n", typeName, callee)
		writeSemaFile(t, file, source)
		methodStart := strings.Index(source, "public void run()")
		methodEnd := strings.LastIndex(source, "    }") + len("    }")
		index.Types[i] = typesys.TypeSymbol{
			Kind: apexast.DeclarationClass,
			Name: typeName,
			File: file,
			Members: []typesys.MemberSymbol{{
				Kind: apexast.DeclarationMethod,
				Name: "run",
				Type: "void",
				Range: diagnostic.Range{
					Start: diagnostic.Position{Offset: methodStart},
					End:   diagnostic.Position{Offset: methodEnd},
				},
			}},
		}
		callStart := strings.Index(source, callee)
		want = append(want, expectedDiagnostic{
			code:    "GLADESEMA008",
			message: fmt.Sprintf(`method "run" calls unknown method %q`, callee),
			file:    file,
			range_:  *semaRange(source, callStart, callStart+len(callee)),
		})
	}

	analyzer := NewAnalyzer()
	state := buildSemaTypeMemberState(index, nil)
	diagnostics := analyzer.checkMethodBodiesWithViewWorkers(index, state.view(), nil, 2)
	if len(diagnostics) != len(want) {
		t.Fatalf("merged diagnostics = %d, want %d: %#v", len(diagnostics), len(want), diagnostics)
	}
	for i, item := range diagnostics {
		if item.Range == nil {
			t.Fatalf("merged diagnostic %d has no range: %#v", i, item)
		}
		got := expectedDiagnostic{code: item.Code, message: item.Message, file: item.File, range_: *item.Range}
		if !reflect.DeepEqual(got, want[i]) {
			t.Fatalf("merged diagnostic %d\nwant: %#v\n got: %#v", i, want[i], got)
		}
	}
}

func TestMethodBodyWorkerScaffoldDefaultPreservesOneWorkerHistory(t *testing.T) {
	root := t.TempDir()
	duplicateFirstFile := filepath.Join(root, "DuplicateFirst.cls")
	duplicateSecondFile := filepath.Join(root, "DuplicateSecond.cls")
	baseFile := filepath.Join(root, "EmailTemplate.cls")
	wrapperFile := filepath.Join(root, "EmailTemplateWrapper.cls")
	writeSemaFile(t, duplicateFirstFile, `public class Duplicate {
    private HistorySeed__c seed;
}
`)
	writeSemaFile(t, duplicateSecondFile, "public class Duplicate {}\n")
	writeSemaFile(t, baseFile, `
public abstract class EmailTemplate {
    public abstract String getId();
}
`)
	writeSemaFile(t, wrapperFile, `
public class EmailTemplateWrapper extends EmailTemplate {
    private Schema.EmailTemplate templateRecord;
    public override String getId() {
        return templateRecord.Id;
    }
}
`)
	index, artifacts := typesys.BuildWithArtifacts(project.Project{
		Root: root,
		ApexFiles: []string{
			duplicateFirstFile,
			duplicateSecondFile,
			baseFile,
			wrapperFile,
		},
	}, schema.Schema{})
	analyzer := NewAnalyzer()
	analyzer.prepareAnalysisContext(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts})
	index = prepareAnalysisIndexWithSources(index, analyzer.sources)
	analyzer.prepareAnalysisModel(index)
	historyKey := normalizeName("HistorySeed__c")
	state := buildSemaTypeMemberState(index, nil, analyzer.sources)
	primary := state.view()
	wantHistory := typeMembers{
		name:    "HistorySeed__c",
		kind:    apexast.DeclarationClass,
		fields:  map[string]typesys.MemberSymbol{normalizeName("HistoryOnly"): {Kind: apexast.DeclarationField, Name: "HistoryOnly", Type: "String"}},
		methods: make(map[string][]typesys.MemberSymbol),
	}
	primary.storeHydrated(historyKey, wantHistory)

	bodyDiagnostics := analyzer.checkMethodBodiesWithView(index, primary, nil)
	if len(bodyDiagnostics) != 0 {
		t.Fatalf("default method-body diagnostics = %d, want 0: %#v", len(bodyDiagnostics), bodyDiagnostics)
	}
	if history, ok := primary.hydrated[historyKey]; !ok || !reflect.DeepEqual(history, wantHistory) {
		t.Fatalf("default worker lost zero-body platform hydration: %#v, %v", history, ok)
	}

	inheritanceDiagnostics := analyzer.checkInheritanceContractsWithView(index, primary, nil)
	if len(inheritanceDiagnostics) != 0 {
		t.Fatalf("explicit Schema.EmailTemplate hydration shadowed the project superclass: %#v", inheritanceDiagnostics)
	}
}

func TestMethodBodyWorkItemsPreserveOriginalOrder(t *testing.T) {
	root := t.TempDir()
	firstFile := filepath.Join(root, "FirstShared.cls")
	secondFile := filepath.Join(root, "SecondShared.cls")
	firstSource := "firstMethod{} constructor{} initializer{} getter{} setter{}"
	secondSource := "secondMethod{}"
	writeSemaFile(t, firstFile, firstSource)
	writeSemaFile(t, secondFile, secondSource)
	rangeFor := func(source, marker string) diagnostic.Range {
		start := strings.Index(source, marker)
		if start < 0 {
			t.Fatalf("fixture marker %q is missing", marker)
		}
		return diagnostic.Range{
			Start: diagnostic.Position{Offset: start},
			End:   diagnostic.Position{Offset: start + len(marker)},
		}
	}
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass,
			Name: "Shared",
			File: firstFile,
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationMethod, Name: "firstMethod", Type: "void", Range: rangeFor(firstSource, "firstMethod{}")},
				{Kind: apexast.DeclarationConstructor, Name: "Shared", Range: rangeFor(firstSource, "constructor{}")},
				{Kind: apexast.DeclarationInitializer, Name: "initializer", Range: rangeFor(firstSource, "initializer{}")},
				{
					Kind: apexast.DeclarationProperty,
					Name: "Value",
					Type: "String",
					Accessors: []apexast.Accessor{
						{Kind: "get", HasBody: true, Range: rangeFor(firstSource, "getter{}")},
						{Kind: "set", HasBody: true, Range: rangeFor(firstSource, "setter{}")},
					},
				},
			},
		},
		{
			Kind: apexast.DeclarationClass,
			Name: "Shared",
			File: secondFile,
			Members: []typesys.MemberSymbol{{
				Kind:  apexast.DeclarationMethod,
				Name:  "secondMethod",
				Type:  "void",
				Range: rangeFor(secondSource, "secondMethod{}"),
			}},
		},
	}}
	counters := PerfCounters{Enabled: true}
	recorder := newPerfRecorder(&counters)
	items := buildSemaMethodBodyWorkItems(index, newSemaSources(nil, &recorder))
	recorder.finish()

	type identity struct {
		file        string
		kind        apexast.DeclarationKind
		name        string
		typeName    string
		parameters  []apexast.Parameter
		memberRange diagnostic.Range
		body        string
		bodyOffset  int
		source      string
	}
	got := make([]identity, 0, len(items))
	for _, item := range items {
		typ, member, ok := resolveSemaMethodBodyWorkItem(index, item)
		if !ok {
			t.Fatalf("method-body descriptor did not resolve: %#v", item)
		}
		got = append(got, identity{
			file:        filepath.Base(typ.File),
			kind:        member.Kind,
			name:        member.Name,
			typeName:    member.Type,
			parameters:  member.Parameters,
			memberRange: member.Range,
			body:        item.body,
			bodyOffset:  item.bodyOffset,
			source:      item.source,
		})
	}
	want := []identity{
		{file: "FirstShared.cls", kind: apexast.DeclarationMethod, name: "firstMethod", typeName: "void", memberRange: rangeFor(firstSource, "firstMethod{}"), bodyOffset: strings.Index(firstSource, "firstMethod{}") + len("firstMethod{"), source: firstSource},
		{file: "FirstShared.cls", kind: apexast.DeclarationConstructor, name: "Shared", memberRange: rangeFor(firstSource, "constructor{}"), bodyOffset: strings.Index(firstSource, "constructor{}") + len("constructor{"), source: firstSource},
		{file: "FirstShared.cls", kind: apexast.DeclarationInitializer, name: "initializer", memberRange: rangeFor(firstSource, "initializer{}"), bodyOffset: strings.Index(firstSource, "initializer{}") + len("initializer{"), source: firstSource},
		{file: "FirstShared.cls", kind: apexast.DeclarationMethod, name: "Value.get", typeName: "String", bodyOffset: strings.Index(firstSource, "getter{}") + len("getter{"), source: firstSource},
		{file: "FirstShared.cls", kind: apexast.DeclarationMethod, name: "Value.set", typeName: "void", parameters: []apexast.Parameter{{Name: "value", Type: "String"}}, bodyOffset: strings.Index(firstSource, "setter{}") + len("setter{"), source: firstSource},
		{file: "SecondShared.cls", kind: apexast.DeclarationMethod, name: "secondMethod", typeName: "void", memberRange: rangeFor(secondSource, "secondMethod{}"), bodyOffset: strings.Index(secondSource, "secondMethod{}") + len("secondMethod{"), source: secondSource},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("method-body work order = %#v, want %#v", got, want)
	}
	if cap(items) != len(items) {
		t.Fatalf("method-body work capacity = %d, want exact length %d", cap(items), len(items))
	}
	if got := counters.SourceArenaHits + counters.SourceArenaMisses; got != 2 {
		t.Fatalf("source resolutions = %d, want one per eligible type", got)
	}
}

func TestMethodBodyWorkItemsDoNotReserveInvalidBodies(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "MixedBodies.cls")
	source := "valid{} abstract; property;"
	writeSemaFile(t, file, source)
	rangeFor := func(marker string) diagnostic.Range {
		start := strings.Index(source, marker)
		if start < 0 {
			t.Fatalf("fixture marker %q is missing", marker)
		}
		return diagnostic.Range{
			Start: diagnostic.Position{Offset: start},
			End:   diagnostic.Position{Offset: start + len(marker)},
		}
	}
	index := typesys.Index{Types: []typesys.TypeSymbol{{
		Kind: apexast.DeclarationClass,
		Name: "MixedBodies",
		File: file,
		Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationMethod, Name: "valid", Type: "void", Range: rangeFor("valid{}")},
			{Kind: apexast.DeclarationMethod, Name: "abstractMethod", Type: "void", Range: rangeFor("abstract;")},
			{
				Kind: apexast.DeclarationProperty,
				Name: "Value",
				Type: "String",
				Accessors: []apexast.Accessor{{
					Kind:    "get",
					HasBody: true,
					Range:   rangeFor("property;"),
				}},
			},
		},
	}}}
	items := buildSemaMethodBodyWorkItems(index, newSemaSources(nil, nil))
	if len(items) != 1 {
		t.Fatalf("extractable method-body items = %#v, want only valid", items)
	}
	_, member, ok := resolveSemaMethodBodyWorkItem(index, items[0])
	if !ok || member.Name != "valid" {
		t.Fatalf("extractable method-body descriptor resolves to %#v, %v; want valid", member, ok)
	}
	if cap(items) != len(items) {
		t.Fatalf("method-body work capacity = %d, want exact extracted length %d", cap(items), len(items))
	}
}

func TestMethodBodyWorkItemDescriptorIsCompact(t *testing.T) {
	const maxDescriptorBytes = 64
	if size := unsafe.Sizeof(semaMethodBodyWorkItem{}); size > maxDescriptorBytes {
		t.Fatalf("method-body work descriptor size = %d bytes, want at most %d", size, maxDescriptorBytes)
	}
}

func TestResolveMethodBodyWorkItemRejectsInvalidIndexes(t *testing.T) {
	index := typesys.Index{Types: []typesys.TypeSymbol{{
		Kind: apexast.DeclarationClass,
		Name: "Indexed",
		Members: []typesys.MemberSymbol{
			{Kind: apexast.DeclarationMethod, Name: "run"},
			{
				Kind: apexast.DeclarationProperty,
				Name: "Value",
				Accessors: []apexast.Accessor{
					{Kind: "get", HasBody: true},
					{Kind: "set", HasBody: false},
				},
			},
		},
	}}}
	tests := []semaMethodBodyWorkItem{
		{typeIndex: -1, memberIndex: 0, accessorIndex: semaMethodBodyNoAccessor},
		{typeIndex: 1, memberIndex: 0, accessorIndex: semaMethodBodyNoAccessor},
		{typeIndex: 0, memberIndex: -1, accessorIndex: semaMethodBodyNoAccessor},
		{typeIndex: 0, memberIndex: 2, accessorIndex: semaMethodBodyNoAccessor},
		{typeIndex: 0, memberIndex: 0, accessorIndex: 0},
		{typeIndex: 0, memberIndex: 1, accessorIndex: semaMethodBodyNoAccessor},
		{typeIndex: 0, memberIndex: 1, accessorIndex: -2},
		{typeIndex: 0, memberIndex: 1, accessorIndex: 2},
		{typeIndex: 0, memberIndex: 1, accessorIndex: 1},
	}
	for _, item := range tests {
		if typ, member, ok := resolveSemaMethodBodyWorkItem(index, item); ok {
			t.Fatalf("invalid descriptor %#v resolved to %#v, %#v", item, typ, member)
		}
	}
}

func TestCountExtractableMethodBodyRangesDoesNotAllocatePerAccessor(t *testing.T) {
	const propertyCount = 128
	source := "getter{} setter{}"
	rangeFor := func(marker string) diagnostic.Range {
		start := strings.Index(source, marker)
		if start < 0 {
			t.Fatalf("fixture marker %q is missing", marker)
		}
		return diagnostic.Range{
			Start: diagnostic.Position{Offset: start},
			End:   diagnostic.Position{Offset: start + len(marker)},
		}
	}
	typ := typesys.TypeSymbol{
		Kind:    apexast.DeclarationClass,
		Name:    "PropertyHeavy",
		Members: make([]typesys.MemberSymbol, propertyCount),
	}
	for i := range typ.Members {
		typ.Members[i] = typesys.MemberSymbol{
			Kind: apexast.DeclarationProperty,
			Name: "Value",
			Type: "String",
			Accessors: []apexast.Accessor{
				{Kind: "get", HasBody: true, Range: rangeFor("getter{}")},
				{Kind: "set", HasBody: true, Range: rangeFor("setter{}")},
			},
		}
	}

	if got := countExtractableSemaMethodBodyRanges(typ, source); got != propertyCount*2 {
		t.Fatalf("extractable accessor bodies = %d, want %d", got, propertyCount*2)
	}
	if allocations := testing.AllocsPerRun(100, func() {
		_ = countExtractableSemaMethodBodyRanges(typ, source)
	}); allocations != 0 {
		t.Fatalf("count-only accessor allocations = %.2f, want 0", allocations)
	}
}

func TestTypeMemberResolutionPrefersTopLevelTypeOverUnrelatedNestedShortName(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"ExternalProfile.cls": `
public class ExternalProfile {
  public class LicenseRecord {}
}
`,
		"LicenseRecord.cls": `
public class LicenseRecord {
  public static List<LicenseRecord> parseList(List<Object> values) {
    return new List<LicenseRecord>();
  }
}
`,
		"ProviderIdentifiers.cls": `
public class ProviderIdentifiers {
  public List<LicenseRecord> licenses;
  public static List<LicenseRecord> parse(List<Object> values) {
    return LicenseRecord.parseList(values);
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("unrelated nested short name shadowed top-level type: %#v", result.Diagnostics)
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
	if !ok || len(exception.constructors) != 0 || !exception.constructorsAuthoritative || !hasModifier(exception.modifiers, "abstract") {
		t.Fatalf("Exception platform shape = %#v, %v; want abstract with no source constructors", exception, ok)
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
