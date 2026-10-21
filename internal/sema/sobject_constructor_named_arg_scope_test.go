package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native rows: a class whose method declares the parameter userRoleId and
// builds `User u = new User(Alias = s.length() > 8 ? ... : s, UserRoleId =
// userRoleId)` deployed and compiled at API 62.0 and 67.0. The `>` comparison
// must not end the constructor argument list, or the later named arguments
// read as extra local declarators and collide with the parameter.
func TestSObjectConstructorNamedArgsAfterComparisonAreNotLocals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "UserBuilder.cls")
	writeSemaFile(t, classPath, `
public class UserBuilder {
  public static User build(String uniqueName, Id userRoleId) {
    User testUser = new User(
      Alias = uniqueName.length() > 8
                ? uniqueName.substring(0, 8)
                : uniqueName,
      LastName = 'x',
      UserRoleId = userRoleId
    );
    return testUser;
  }
  public static Map<String, List<Id>> generics(Integer lastName, Integer limit) {
    Map<String, List<Id>> ids = new Map<String, List<Id>>(), other = new Map<String, List<Id>>();
    Boolean ok = lastName < limit;
    return ids;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA014" {
			t.Fatalf("named constructor argument read as a local declaration: %#v", diag)
		}
	}
}
