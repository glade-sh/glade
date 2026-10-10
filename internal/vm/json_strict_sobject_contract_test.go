package vm

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API67 Salesforce observations replace the native compatibility contract's contrary docs-only fixture.
// R004/R006/R008/R032 reject unknown Account fields through both strict routes;
// lenient/known-field controls remain successful. R048/R049 map blank/space
// Decimal fields to zero; R050/R056 preserve explicit-null/absent values.
func TestExecJSONSObjectNativeBlockersAPI67(t *testing.T) {
	var data struct {
		APIVersion string `json:"apiVersion"`
		Cases      []struct {
			ID       string `json:"id"`
			Code     string `json:"code"`
			Expected string `json:"expected"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/conformance/sobject_json_blockers_api67.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string {
		return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
	}
	legacyMatches, legacyTotal, partialMatches, partialTotal := 0, 0, 0, 0
	defer func() {
		t.Logf("JSON API %s anonymous exact 0/0; category-only 0; carried 0; partial %d; legacy %d; subset strict-sobject; partial matched %d/%d; legacy matched %d/%d", data.APIVersion, partialTotal, legacyTotal, partialMatches, partialTotal, legacyMatches, legacyTotal)
	}()
	for _, row := range data.Cases {
		if row.Expected == "0.0" {
			partialTotal++
		} else {
			legacyTotal++
		}
		if t.Run(row.ID, func(t *testing.T) {
			body := "Object r; String observed; try {" + row.Code + " observed=''+String.valueOf(r); } catch(Exception e) { observed='EXC|'+e.getTypeName()+'|'+e.getMessage(); } "
			if row.Expected == "0.0" {
				// Decimal zero formatting is owned by the numeric family; assert
				// the conversion value and successful deserialization here.
				body += "System.assert(r instanceof Decimal); System.assertEquals(0.0,r);"
			} else {
				body += "System.assertEquals(" + quote(row.Expected) + ",observed);"
			}
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: data.APIVersion})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		}) {
			if row.Expected == "0.0" {
				partialMatches++
			} else {
				legacyMatches++
			}
		}
	}
}
