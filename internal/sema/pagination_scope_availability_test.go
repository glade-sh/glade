package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Owned source API 62 captures compile these signatures on both Apex routes.
// Scope/ownership behavior and the bound SYSTEM_MODE pagination form are replayed
// by the scope_mine_pagination conformance fixture.
const observedScopePaginationBody = `
Set<Id> ids=new Set<Id>();
Database.PaginationCursor page=Database.getPaginationCursorWithBinds('SELECT Id FROM Account USING SCOPE Mine WHERE Id IN :ids ORDER BY Name',new Map<String,Object>{'ids'=>ids},AccessLevel.SYSTEM_MODE);
Integer count=page.getNumRecords();
List<SObject> records=count==0 ? new List<SObject>() : page.fetchPage(0,count).getRecords();
`

func TestPaginationScopeObservedAPI62Signatures(t *testing.T) {
	for _, anonymous := range []bool{false, true} {
		t.Run(map[bool]string{false: "named", true: "anonymous"}[anonymous], func(t *testing.T) {
			var result Result
			if anonymous {
				result = AnalyzeAnonymous(typesys.Index{}, observedScopePaginationBody, "62.0")
			} else {
				result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"OwnedScopeReader.cls": "public class OwnedScopeReader { public static void read() { " + observedScopePaginationBody + " } }"}, "62.0")
			}
			if result.HasErrors() {
				t.Fatalf("native source API 62 accepted; local diagnostics=%v", result.Diagnostics)
			}
		})
	}
}

// Intermediate source profiles and other cursor surfaces have no new oracle.
func TestPaginationScopeAvailabilityPreservesOtherProfiles(t *testing.T) {
	for _, api := range []string{"63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"OwnedScopeReader.cls": "public class OwnedScopeReader { public static void read() { " + observedScopePaginationBody + " } }"}, api)
			wantBlocked := api == "63.0" || api == "64.0" || api == "65.0"
			if got := hasDiagnosticCode(result.Diagnostics, "GLADESEMA028"); got != wantBlocked {
				t.Fatalf("API %s unavailable=%v want %v: %v", api, got, wantBlocked, result.Diagnostics)
			}
			if !wantBlocked && result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
		})
	}
}

func TestPaginationScopeAvailabilityKeepsOtherShapes(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"plain factory", `Database.PaginationCursor page=Database.getPaginationCursor('SELECT Id FROM Account USING SCOPE Mine');`},
		{"unknown mode type", `Database.PaginationCursor page=Database.getPaginationCursorWithBinds('SELECT Id FROM Account USING SCOPE Mine',new Map<String,Object>(),(Object)null);`},
		{"unknown bind type", `Database.PaginationCursor page=Database.getPaginationCursorWithBinds('SELECT Id FROM Account USING SCOPE Mine',new Map<Id,Account>(),AccessLevel.SYSTEM_MODE);`},
		{"unknown arity", `Database.PaginationCursor page=Database.getPaginationCursorWithBinds('SELECT Id FROM Account USING SCOPE Mine',new Map<String,Object>());`},
		{"unknown reader arity", `Database.PaginationCursor page; Integer count=page.getNumRecords(1);`},
		{"unknown fetch types", `Database.PaginationCursor page; Object records=page.fetchPage(0,'unknown');`},
		{"uncaptured deleted fetch", `Database.PaginationCursor page; Object records=page.fetchDeleted(0,1);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"OwnedScopeReader.cls": "public class OwnedScopeReader { public static void read() { " + tc.body + " } }"}, "62.0")
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA028") {
				t.Fatalf("unobserved signature lost existing gate: %v", result.Diagnostics)
			}
		})
	}
}

func TestPaginationScopeAvailabilityPreservesProjectShadow(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Database.cls":         `public class Database { public static Integer getPaginationCursorWithBinds(String query, Map<String,Object> binds, AccessLevel mode) { return 7; } }`,
		"OwnedScopeReader.cls": `public class OwnedScopeReader { public static Integer read() { return Database.getPaginationCursorWithBinds('owned',new Map<String,Object>(),AccessLevel.SYSTEM_MODE); } }`,
	}, "62.0")
	if result.HasErrors() {
		t.Fatal(result.Diagnostics)
	}
}

// A project argument named like the platform enum is outside the observed typed
// signature. Preserve its pre-existing availability rejection.
func TestPaginationScopeAvailabilityKeepsArgumentTypeShadowBoundary(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"AccessLevel.cls":      `public class AccessLevel {}`,
		"OwnedScopeReader.cls": `public class OwnedScopeReader { public static void read() { Database.PaginationCursor page=Database.getPaginationCursorWithBinds('SELECT Id FROM Account USING SCOPE Mine',new Map<String,Object>(),new AccessLevel()); } }`,
	}, "62.0")
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA028") {
		t.Fatalf("project argument type acquired platform availability: %v", result.Diagnostics)
	}
}

// The capture consumes an inferred fetch result's records. Other result members
// and explicit references to that type retain their previous source gate.
func TestPaginationScopeAvailabilityKeepsOtherResultSurfaces(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"explicit result type", `Database.CursorFetchResult result;`},
		{"next index", observedScopePaginationBody + `Integer next=page.fetchPage(0,1).getNextIndex();`},
		{"deleted count", observedScopePaginationBody + `Integer deleted=page.fetchPage(0,1).getNumDeletedRecords();`},
		{"done flag", observedScopePaginationBody + `Boolean done=page.fetchPage(0,1).isDone();`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"OwnedScopeReader.cls": "public class OwnedScopeReader { public static void read() { " + tc.body + " } }"}, "62.0")
			if !result.HasErrors() {
				t.Fatalf("unobserved result surface acquired availability: %s", tc.name)
			}
		})
	}
}
