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

func TestAnalyzeProjectDatabaseDoesNotFallBackToSystemDatabase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Database.cls"), `
public class Database {}
`)
	writeSemaFile(t, filepath.Join(root, "UsesDatabase.cls"), `
public class UsesDatabase {
  public void run() {
    Database.query('SELECT Id FROM Account');
  }
}
`)
	result := analyzeFiles(t, root, "Database.cls", "UsesDatabase.cls")
	if !result.HasErrors() {
		t.Fatalf("expected project Database to prevent System.Database fallback, got %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedProjectTypeDoesNotShadowSystemLimits(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "QPlugin.cls"), `
public class QPlugin {
  public virtual class Limits {
    public Integer getLimits() { return 0; }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesLimits.cls"), `
public class UsesLimits {
  public Integer run() {
    return Limits.getLimitCallouts();
  }
}
`)
	result := analyzeFiles(t, root, "QPlugin.cls", "UsesLimits.cls")
	if result.HasErrors() {
		t.Fatalf("nested project type shadowed System.Limits: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTopLevelProjectTypeShadowsSystemLimits(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Limits.cls"), `
public class Limits {
  public Integer getLimits() { return 0; }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesLimits.cls"), `
public class UsesLimits {
  public Integer run() {
    return Limits.getLimitCallouts();
  }
}
`)
	result := analyzeFiles(t, root, "Limits.cls", "UsesLimits.cls")
	if !result.HasErrors() {
		t.Fatalf("top-level project type did not shadow System.Limits: %#v", result.Diagnostics)
	}
}

func TestAnalyzePlatformReceiverSpellingIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		method     string
		wantErrors bool
	}{
		{name: "known method", method: "getLimitCallouts", wantErrors: false},
		{name: "unknown method", method: "noSuchMethod", wantErrors: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": `public class Probe { public static Integer run() { return limits.` + test.method + `(); } }`,
			})
			if result.HasErrors() != test.wantErrors {
				t.Fatalf("lowercase Limits.%s errors = %v diagnostics=%#v", test.method, result.HasErrors(), result.Diagnostics)
			}
		})
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
	writeSemaFile(t, filepath.Join(root, "Shared.cls"), `
public class Shared {
  public String value;
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesInnerPrecedence.cls"), `
public class UsesInnerPrecedence {
  public class Shared {
    public Integer value;
  }
  public void run(Object obj) {
    Integer value = ((Shared)obj).value;
  }
}
`)
	result := analyzeFiles(t, root, "Shared.cls", "UsesInnerPrecedence.cls")
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
