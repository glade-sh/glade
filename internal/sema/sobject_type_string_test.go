package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzePermissionSObjectTypeStringField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "PermissionSObjectType.cls")
	writeSemaFile(t, path, `
public class PermissionSObjectType {
  public static void run() {
    FieldPermissions fieldPermission = new FieldPermissions();
    fieldPermission.SobjectType = 'Account';
    String fieldObjectName = fieldPermission.SobjectType;
    ObjectPermissions objectPermission = new ObjectPermissions();
    objectPermission.SobjectType = 'Account';
    String objectName = objectPermission.SobjectType;
  }
}
`)

	result := Analyze(typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{}))
	if len(result.Diagnostics) != 0 {
		t.Fatalf("permission SObjectType fields should support instance String assignment and static tokens: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedTypeUsesEnclosingStaticQueryLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "NestedStaticLimit.cls")
	writeSemaFile(t, path, `
@IsTest
private class NestedStaticLimit {
  public static Integer QUERY_RECORD_LIMIT = 1;
  private class Loader {
    List<Account> load() {
      return [SELECT Id FROM Account LIMIT :QUERY_RECORD_LIMIT];
    }
  }
  @IsTest static void run() {
    System.assertNotEquals(null, new Loader().load());
  }
}
`)

	result := Analyze(typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{}))
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA_QUERY_BIND" {
			t.Fatalf("nested type should resolve enclosing static query limit: %#v", result.Diagnostics)
		}
	}
}
