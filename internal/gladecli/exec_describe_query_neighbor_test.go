package gladecli

import (
	"path/filepath"
	"testing"
)

// This uncaptured neighbor is also run on origin/main; its JSON is compared
// byte for byte. The eager and lazy orgs must give the same answer.
func TestExecDescribeQueryNeighbor(t *testing.T) {
	root := execStandardOrgProject(t, "")
	writeTestFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":[],"settings":{}}`)
	source := `Schema.getGlobalDescribe(); System.debug(Database.query('SELECT Id FROM Account LIMIT 1').size()); try { Database.query('SELECT Id FROM MissingNeighbor__c LIMIT 1'); } catch (QueryException e) { System.debug(e.getMessage()); }`
	eager, eagerErr := execJSONData(t, root, source, eagerExecProjectLoader)
	lazy, lazyErr := execJSONData(t, root, source, loadExecProject)
	if eagerErr != nil || lazyErr != nil {
		t.Fatalf("eager: %v; lazy: %v", eagerErr, lazyErr)
	}
	if eager != lazy {
		t.Fatalf("eager: %s\nlazy: %s", eager, lazy)
	}
	t.Log(lazy)
}
