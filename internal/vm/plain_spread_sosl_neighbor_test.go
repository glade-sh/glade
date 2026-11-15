package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPlainDeveloperSOSLNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/plain_developer_route_gaps/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObjectSpread(&org)
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	machine := New(nil)
	machine.SetOrg(&org)
	for _, row := range rows[1:] {
		if row[1] != "sosl" {
			continue
		}
		source := fmt.Sprintf(`String result = 'ok'; try { Search.query('FIND {candidate} RETURNING %s(Id)'); } catch (Exception e) { result = e.getTypeName(); } System.assertEquals('%s', result);`, row[2], row[4])
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			t.Errorf("%s %s: %v", row[0], row[2], err)
		}
	}
}

// SOSL at API 62 was not captured. Its local outcome stays at main.
func TestPlainDeveloperSpreadPreservesUnprobedSOSL(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObjectSpread(&org)
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	program, err := CompileAnonymousWithOptions(`Search.query('FIND {candidate} RETURNING AIFeatureExtractor(Id)');`, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("origin/main SOSL outcome changed: %v", err)
	}
}

func TestPlainDeveloperSpreadPreservesUnprobedSOSLAfterSerialization(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObjectSpread(&org)
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymousWithOptions(`Search.query('FIND {candidate} RETURNING AIFeatureExtractor(Id)');`, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&restored)
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("serialized origin/main SOSL outcome changed: %v", err)
	}
}

// NATIVE: plain_developer_route_gaps/org.tsv:4.
func TestPlainDeveloperSpreadPreservesUnprobedAPIVersion(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObjectSpread(&org)
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	program, err := CompileAnonymousWithOptions(`System.assertEquals(null, Type.forName('AIFeatureExtractor')); String result = 'ok'; try { Database.query('SELECT Id FROM AIFeatureExtractor LIMIT 1'); } catch (Exception e) { result = e.getTypeName(); } System.assertEquals('System.QueryException', result);`, CompileOptions{APIVersion: "64.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("native API 64 outcome changed: %v", err)
	}
}
