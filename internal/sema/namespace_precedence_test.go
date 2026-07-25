package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeExpressionPrecedenceLocalThenClassThenNamespace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Database.cls"), `
public class Database {
  public String query(String soql) {
    return 'local';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesPrecedence.cls"), `
public class UsesPrecedence {
  public void run() {
    Database Database = new Database();
    String value = Database.query('SELECT Name FROM Account');
  }
}
`)
	result := analyzeFiles(t, root, "Database.cls", "UsesPrecedence.cls")
	if result.HasErrors() {
		t.Fatalf("unexpected local-variable precedence diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemQualifierDisambiguatesShadowedDatabase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Database.cls"), `
public class Database {
  public static String query() {
    return 'local';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesSystemDatabase.cls"), `
public class UsesSystemDatabase {
  public void run() {
    List<SObject> rows = System.Database.query('SELECT Name FROM Account');
  }
}
`)
	result := analyzeFiles(t, root, "Database.cls", "UsesSystemDatabase.cls")
	if result.HasErrors() {
		t.Fatalf("unexpected System.Database diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaQualifierDisambiguatesShadowedSObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Account.cls"), `
public class Account {
  public Integer myInteger;
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesSchemaAccount.cls"), `
public class UsesSchemaAccount {
  public void run() {
    Schema.Account myAccountSObject = new Schema.Account();
    Account accountClassInstance = new Account();
    accountClassInstance.myInteger = 1;
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Account.cls"),
			filepath.Join(root, "UsesSchemaAccount.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Schema.Account diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInnerTypeWinsBeforeNamespaceType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Container.cls"), `
public class Container {
  public class Nested {
    public String localValue;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "A.cls"), `
public class A {
  public class B {
    public String value;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesInnerPrecedence.cls"), `
public class UsesInnerPrecedence {
  public class A {
    public class B {
      public Integer value;
    }
  }
  public void run(Object obj) {
    Integer value = ((A.B)obj).value;
  }
}
`)
	result := analyzeFiles(t, root, "Container.cls", "A.cls", "UsesInnerPrecedence.cls")
	if result.HasErrors() {
		t.Fatalf("unexpected inner-before-namespace diagnostics: %#v", result.Diagnostics)
	}
}

func analyzeFiles(t *testing.T, root string, files ...string) Result {
	t.Helper()
	apexFiles := make([]string, 0, len(files))
	for _, file := range files {
		apexFiles = append(apexFiles, filepath.Join(root, file))
	}
	return Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: apexFiles,
	}, schema.Schema{}))
}
