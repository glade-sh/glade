package vm

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned API62/67 terminal observations accept Language on article insertion
// and expose createable=true/updateable=false. They are not passing native tests.
func TestKnowledgeLanguageCreationNative(t *testing.T) {
	var data struct {
		Observations []struct {
			API, Terminal               string
			ComponentErrors, TestErrors int
			Observed                    struct {
				Inserted, Createable, Updateable bool
				Language                         string
			}
		}
	}
	raw, err := os.ReadFile("testdata/knowledge_language_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Observations) != 2 {
		t.Fatal("missing Knowledge observations")
	}
	for _, row := range data.Observations {
		t.Run(row.API, func(t *testing.T) {
			if row.Terminal != "observation_assertion" || row.ComponentErrors != 0 || row.TestErrors != 1 || !row.Observed.Inserted || !row.Observed.Createable || row.Observed.Updateable || row.Observed.Language != "en_US" {
				t.Fatal("native creation observation differs")
			}
			p, err := CompileAnonymousWithOptions(`
Knowledge__kav article=new Knowledge__kav(Title='Owned Knowledge',UrlName='owned-knowledge',Language='en_US');
Database.insert(article,AccessLevel.SYSTEM_MODE);
Knowledge__kav loaded=[SELECT Language FROM Knowledge__kav WHERE Id=:article.Id];
System.assertEquals('en_US',loaded.Language);
System.assertEquals(true,Knowledge__kav.Language.getDescribe().isCreateable());
System.assertEquals(false,Knowledge__kav.Language.getDescribe().isUpdateable());
System.assertEquals(false,Knowledge__kav.VersionNumber.getDescribe().isCreateable());
`, CompileOptions{APIVersion: row.API})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Knowledge__kav")
			m := New(nil)
			m.SetOrg(&org)
			m.EnableTestContext()
			if _, err = m.Execute(p); err != nil {
				t.Fatal(err)
			}
		})
	}
}
