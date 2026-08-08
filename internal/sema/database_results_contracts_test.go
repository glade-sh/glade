package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestDatabaseImmediateBareOverloadCompiles(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `Database.SaveResult result = Database.insertImmediate(new Account(Name = 'contract'));`)
	if result.HasErrors() {
		t.Fatalf("rejected Database.insertImmediate(SObject): %#v", result.Diagnostics)
	}
}

func TestDatabaseImmediateBooleanOverloadCompiles(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `Object result = Database.insertImmediate(new Account(Name = 'contract'), false);`)
	if result.HasErrors() {
		t.Fatalf("rejected Database.insertImmediate(SObject, Boolean): %#v", result.Diagnostics)
	}
}

func TestDatabaseAsyncResultAndLocatorContractsCompile(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `Object result = Database.insertAsync(new Account(Name = 'contract')); Database.SaveResult saved = Database.getAsyncSaveResult(result); String locator = Database.getAsyncLocator(saved);`)
	if result.HasErrors() {
		t.Fatalf("rejected async result/locator contract: %#v", result.Diagnostics)
	}
}

func TestDatabaseLockResultContractsCompile(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `List<Database.LockResult> results = Database.lock(new List<Account>{new Account(Name = 'contract')});`)
	if result.HasErrors() {
		t.Fatalf("rejected Database.LockResult contract: %#v", result.Diagnostics)
	}
}

func TestDatabaseUnlockAndRecycleResultContractsCompile(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `Database.UnlockResult unlocked = Database.unlock(new Account(Name = 'contract'), false); Database.EmptyRecycleBinResult emptied = Database.emptyRecycleBin(new Account(Name = 'contract'), false);`)
	if result.HasErrors() {
		t.Fatalf("rejected Database record-action result contracts: %#v", result.Diagnostics)
	}
}
