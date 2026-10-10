package sema

import (
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

// SF177 exact API65 query and comment declarations.
func TestQualifiedQueryAndCommentAPI65(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"GladeQualifiedQuery65Proof.cls": `@IsTest private class GladeQualifiedQuery65Proof {
 private static SObject queryOne(String expectedName) {
  return System.Database.query('SELECT Id,Name FROM Account WHERE Name=:expectedName');
 }
 @IsTest static void qualifiedQueryReturnsSingleSObject() {
  Account expected=new Account(Name='owned-qualified65-one'); insert expected;
  SObject actual=queryOne(expected.Name);
  System.assertEquals(expected.Id,actual.Id);
  System.assertEquals(expected.Name,actual.get('Name'));
 }
 @IsTest static void qualifiedQueryCastsToConcreteSObject() {
  Account expected=new Account(Name='owned-qualified65-cast'); insert expected;
  Id expectedId=expected.Id;
  Account actual=(Account)System.Database.query('SELECT Id,Name FROM Account WHERE Id=:expectedId');
  System.assertEquals(expected.Id,actual.Id);
  System.assertEquals(expected.Name,actual.Name);
 }
 @IsTest static void inlineQueryCommentPreservesQuoteAndBracketBoundaries() {
  Account expected=new Account(Name='owned-comment65',Description='observed'); insert expected;
  Id expectedId=expected.Id;
  Account actual=[SELECT Id,
   Name, // This is a text field of the person's name - it's NOT a lookup :'(
   Description FROM Account WHERE Id=:expectedId];
  System.assertEquals(expected.Id,actual.Id);
  System.assertEquals('observed',actual.Description);
 }
}
`}, "65.0")
	if result.HasErrors() {
		t.Fatalf("query proof rejected: %#v", result.Diagnostics)
	}
}

func TestQueryLiteralCommentPunctuationStopsAtClosingBracket(t *testing.T) {
	source := "[SELECT Id, Name, // person's name - it's NOT a lookup :'(\n Description FROM Account]; Integer later=1;"
	end := semaMatchingBracket(source, 0)
	if end < 0 || source[end:] != "]; Integer later=1;" {
		t.Fatalf("closing bracket = %d", end)
	}
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("Comment.cls", "public class Comment { void run() { Account a="+source+" } }")
	for _, d := range diagnostics {
		if d.Code == "GLADESEMA_QUERY_PARSE" {
			t.Fatalf("comment swallowed query boundary: %#v", d)
		}
	}
}

func TestQualifiedQueryPlatformMemberIdentity(t *testing.T) {
	view := buildSemaTypeMemberState(typesys.Index{}, nil).view()
	candidates := resolveMemberMethods(view, "System.Database", "query")
	if !semaResolvedMembersAllPlatformBacked(view, candidates) {
		t.Fatal("qualified Database query lost platform identity")
	}
}

func TestQueryLiteralBracketInsideStringOrBlockComment(t *testing.T) {
	for _, source := range []string{
		"[SELECT Id /* ] person's */ FROM Account];",
		"[SELECT Id FROM Account WHERE Name='http://example/]'];",
	} {
		if end := semaMatchingBracket(source, 0); end != len(source)-2 {
			t.Errorf("closing bracket %d for %s", end, source)
		}
	}
}
