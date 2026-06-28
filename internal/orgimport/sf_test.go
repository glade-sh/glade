package orgimport

import (
	"context"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type fakeSFRunner struct {
	output string
	args   []string
}

func (r *fakeSFRunner) RunSF(_ context.Context, args []string) ([]byte, error) {
	r.args = append([]string{}, args...)
	return []byte(r.output), nil
}

func TestListObjectsReadsSFCLIJSON(t *testing.T) {
	runner := &fakeSFRunner{output: `{"status":0,"result":["Account","Invoice__c"]}`}
	objects, err := ListObjects(context.Background(), runner, ListObjectsOptions{TargetOrg: "devhub", Category: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(objects, ","); got != "Account,Invoice__c" {
		t.Fatalf("objects = %s", got)
	}
	if got := strings.Join(runner.args, " "); !strings.Contains(got, "sobject list") || !strings.Contains(got, "--target-org devhub") {
		t.Fatalf("sf args = %#v", runner.args)
	}
}

func TestImportBuildsFixtureFromSFRows(t *testing.T) {
	runner := &fakeSFRunner{output: `{
  "status":0,
  "result":{
    "totalSize":1,
    "done":true,
    "records":[{
      "attributes":{"type":"Account"},
      "Id":"001000000000123AAA",
      "Name":"Acme",
      "NumberOfEmployees":7,
      "AnnualRevenue":12.50,
      "IsDeleted":false,
      "CreatedDate":"2026-06-28T12:00:00.000+0000",
      "Description":null
    }]
  }
}`}
	result, err := Import(context.Background(), runner, ImportOptions{
		TargetOrg: "devhub",
		Objects:   []string{"Account"},
		Fields:    []string{"Id", "Name", "NumberOfEmployees", "AnnualRevenue", "IsDeleted", "CreatedDate", "Description"},
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fixture.Objects) != 1 || result.Fixture.Objects[0].Name != "Account" {
		t.Fatalf("fixture objects = %#v", result.Fixture.Objects)
	}
	record := result.Fixture.Objects[0].Records[0]
	if record.ID != storage.ID("001000000000123AAA") {
		t.Fatalf("record id = %s", record.ID)
	}
	if got := record.Fields["Name"]; got.Kind != storage.ValueString || got.String != "Acme" {
		t.Fatalf("Name field = %#v", got)
	}
	if got := record.Fields["NumberOfEmployees"]; got.Kind != storage.ValueInteger || got.Integer != 7 {
		t.Fatalf("NumberOfEmployees field = %#v", got)
	}
	if got := record.Fields["AnnualRevenue"]; got.Kind != storage.ValueDecimal || got.Decimal != "12.50" {
		t.Fatalf("AnnualRevenue field = %#v", got)
	}
	if got := record.Fields["IsDeleted"]; got.Kind != storage.ValueBoolean || got.Boolean {
		t.Fatalf("IsDeleted field = %#v", got)
	}
	if got := record.Fields["CreatedDate"]; got.Kind != storage.ValueDateTime {
		t.Fatalf("CreatedDate field = %#v", got)
	}
	if got := strings.Join(record.ExplicitNulls, ","); got != "Description" {
		t.Fatalf("explicit nulls = %q", got)
	}
	if !strings.Contains(strings.Join(runner.args, " "), "SELECT Id, Name, NumberOfEmployees, AnnualRevenue, IsDeleted, CreatedDate, Description FROM Account LIMIT 10") {
		t.Fatalf("sf args = %#v", runner.args)
	}
}

func TestImportDefaultsGeneratedObjectQueriesToTwentyFiveRows(t *testing.T) {
	runner := &fakeSFRunner{output: `{"status":0,"result":{"totalSize":0,"done":true,"records":[]}}`}
	_, err := Import(context.Background(), runner, ImportOptions{Objects: []string{"Account"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(runner.args, " "), "SELECT Id, Name FROM Account LIMIT 25") {
		t.Fatalf("sf args = %#v", runner.args)
	}
}
