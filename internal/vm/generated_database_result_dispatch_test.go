package vm

import "testing"

// Owned dml_transactions R025-R027, R035-R037, R124 and R144 observe these
// SaveResult getters on successful inserts/updates and failed Name updates.
// Loading a passive catalog declaration must preserve the same result data.
func TestExecGeneratedDatabaseResultDispatch(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, catalog := range []string{"none", "saveResult", "error", "both"} {
			t.Run(api+"/"+catalog, func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				account := org.Objects["Account"]
				name := account.Definition.Fields["Name"]
				name.Required = true
				account.Definition.Fields["Name"] = name
				org.Objects["Account"] = account
				machine.SetOrg(&org)
				for _, declaration := range []struct {
					owner, method, returns string
				}{
					{"Database.SaveResult", "isSuccess", "Boolean"},
					{"Database.SaveResult", "getId", "Id"},
					{"Database.SaveResult", "getErrors", "List<Database.Error>"},
					{"Database.Error", "getStatusCode", "System.StatusCode"},
					{"Database.Error", "getMessage", "String"},
					{"Database.Error", "getFields", "List<String>"},
				} {
					if catalog == "none" ||
						(catalog == "saveResult" && declaration.owner != "Database.SaveResult") ||
						(catalog == "error" && declaration.owner != "Database.Error") {
						continue
					}
					if _, exists := machine.lookupClass(declaration.owner); !exists {
						if err := machine.RegisterClass(Class{Name: declaration.owner}); err != nil {
							t.Fatal(err)
						}
					}
					if err := machine.RegisterMethod(Method{
						Name: declaration.owner + "." + declaration.method, ClassName: declaration.owner,
						ReturnType: declaration.returns, Modifiers: []string{"passive-generated"},
					}); err != nil {
						t.Fatal(err)
					}
				}
				program, err := CompileAnonymousWithOptions(`
Account parent = new Account(Name = 'Synthetic result');
Database.SaveResult inserted = Database.insert(parent, false);
System.assertEquals(true, inserted.isSuccess(), 'insert result');
System.assertEquals(parent.Id, inserted.getId());
System.assertEquals(0, inserted.getErrors().size());
parent.Name = 'Synthetic changed';
Database.SaveResult updated = Database.update(parent, false);
System.assertEquals(true, updated.isSuccess(), 'update result');
System.assertEquals(parent.Id, updated.getId());
System.assertEquals(0, updated.getErrors().size());
parent.Name = null;
Database.SaveResult failed = Database.update(parent, false);
System.assertEquals(false, failed.isSuccess(), 'failed update');
System.assertEquals(parent.Id, failed.getId());
System.assertEquals(1, failed.getErrors().size());
Database.Error issue = failed.getErrors()[0];
System.assertEquals(System.StatusCode.REQUIRED_FIELD_MISSING, issue.getStatusCode());
System.assertEquals('Name', issue.getFields()[0]);
System.assert(issue.getMessage().contains('Name'));
System.assertEquals('Synthetic changed', [SELECT Name FROM Account WHERE Id=:parent.Id].Name);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
