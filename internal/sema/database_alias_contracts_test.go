package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestDatabaseImmediateAliasesUseDMLOverloads(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `
Database.DMLOptions opts = new Database.DMLOptions();
Account account = new Account(Name = 'alias');
List<Account> accounts = new List<Account>{account};

Database.insertImmediate(account);
Database.insertImmediate(account, false);
Database.insertImmediate(account, opts);
Database.insertImmediate(account, AccessLevel.USER_MODE);
Database.insertImmediate(accounts);
Database.insertImmediate(accounts, false);
Database.insertImmediate(accounts, opts);
Database.insertImmediate(accounts, AccessLevel.USER_MODE);

Database.updateImmediate(account);
Database.updateImmediate(account, false);
Database.updateImmediate(account, opts);
Database.updateImmediate(account, AccessLevel.USER_MODE);
Database.updateImmediate(accounts);
Database.updateImmediate(accounts, false);
Database.updateImmediate(accounts, opts);
Database.updateImmediate(accounts, AccessLevel.USER_MODE);

Database.deleteImmediate(account);
Database.deleteImmediate(account, false);
Database.deleteImmediate(account, AccessLevel.USER_MODE);
	Database.deleteImmediate(accounts);
	Database.deleteImmediate(accounts, false);
	Database.deleteImmediate(accounts, AccessLevel.USER_MODE);
`)
	if result.HasErrors() {
		t.Fatalf("rejected immediate DML alias overloads: %#v", result.Diagnostics)
	}
}

func TestDatabaseImmediateAliasesInferDMLResultTypes(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `
Account account = new Account(Name = 'alias');
List<Account> accounts = new List<Account>{account};
Database.SaveResult oneInsert = Database.insertImmediate(account, false);
List<Database.SaveResult> manyInserts = Database.insertImmediate(accounts, false);
Database.SaveResult oneUpdate = Database.updateImmediate(account, false);
List<Database.SaveResult> manyUpdates = Database.updateImmediate(accounts, false);
Database.DeleteResult oneDelete = Database.deleteImmediate(account, false);
List<Database.DeleteResult> manyDeletes = Database.deleteImmediate(accounts, false);
`)
	if result.HasErrors() {
		t.Fatalf("rejected immediate DML result types: %#v", result.Diagnostics)
	}
}
