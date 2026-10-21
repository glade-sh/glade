package sema

import "testing"

func TestListSObjectDirectQueryCastContracts(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"DirectQueryCastProbe.cls": `public class DirectQueryCastProbe {
  public void run(Set<Id> ids, String q, Map<String,Object> binds) {
    Account dynamicRow = (Account)Database.query(q);
    SObject genericRow = (SObject)Database.query(q);
    Account inlineRow = (Account)[SELECT Id FROM Account WHERE Id IN :ids];
    SObject inlineGeneric = (SObject)[SELECT Id FROM Account WHERE Id IN :ids];
    Account boundRow = (Account)Database.queryWithBinds(q, binds, AccessLevel.SYSTEM_MODE);
    Account userRow = (Account)Database.query(q, AccessLevel.USER_MODE);
  }
}`,
			}, api)
			if result.HasErrors() {
				t.Fatalf("direct query cast contracts: %#v", result.Diagnostics)
			}
		})
	}
}
