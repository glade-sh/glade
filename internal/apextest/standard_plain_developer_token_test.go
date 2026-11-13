package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func plainDeveloperTokenRows(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "standard_plain_developer_token", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 13 {
		t.Fatalf("native rows: %d", len(rows)-1)
	}
	result := make(map[string]string)
	for _, row := range rows[1:] {
		result[row[0]+"/"+row[1]] = row[3]
	}
	return result
}

// NATIVE testdata/standard_plain_developer_token/org.tsv:2-3 anonymous 62.0 and 67.0.
func TestPlainDeveloperTokenCrossesContext(t *testing.T) {
	rows := plainDeveloperTokenRows(t)
	source, err := os.ReadFile(filepath.Join("testdata", "standard_plain_developer_token", "classes", "DescribeProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := standardDescribeDeclaredBoundaryIndex(t, api, map[string]string{"DescribeProbe": string(source)}, nil)
			org := orgFromIndex(index)
			org.APIVersion = api
			machine := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).newMachine()
			program, err := vm.CompileAnonymousWithOptions(`Schema.DescribeFieldResult d=DescribeProbe.token().getDescribe(); String result=d.getName()+':'+d.getType()+':'+d.getLength()+':'+d.isAggregatable()+':'+d.getPrecision()+':'+d.getScale()+':'+d.getSoapType()+':'+d.isAutoNumber()+':'+d.isCascadeDelete()+':'+d.isCaseSensitive()+':'+d.isCustom()+':'+d.isDefaultedOnCreate()+':'+d.isDependentPicklist()+':'+d.isExternalId()+':'+d.isIdLookup()+':'+d.isNameField()+':'+d.isUnique();`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			want := rows["cross/"+api]
			if got := machine.Globals["result"].Text; got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

// NATIVE testdata/standard_plain_developer_token/org.tsv:4-9 anonymous 62.0 and 67.0.
func TestPlainDeveloperTokenAdditionalFacets(t *testing.T) {
	rows := plainDeveloperTokenRows(t)
	source, err := os.ReadFile(filepath.Join("testdata", "standard_plain_developer_token", "classes", "DescribeProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := standardDescribeDeclaredBoundaryIndex(t, api, map[string]string{"DescribeProbe": string(source)}, nil)
			org := orgFromIndex(index)
			org.APIVersion = api
			machine := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).newMachine()
			program, err := vm.CompileAnonymousWithOptions(`Schema.DescribeFieldResult d=DescribeProbe.token().getDescribe(); String a=d.getByteLength()+':'+d.getDigits()+':'+d.getCalculatedFormula()+':'+d.isHtmlFormatted()+':'+d.isEncrypted()+':'+d.getRelationshipName()+':'+d.getReferenceTo().size()+':'+d.getPicklistValues().size(); String b=d.getInlineHelpText()+':'+d.getController()+':'+d.getRelationshipOrder()+':'+d.getDefaultValue()+':'+d.getDefaultValueFormula(); String e=d.getLocalName()+':'+d.getCompoundFieldName()+':'+d.getReferenceTargetField()+':'+d.isRestrictedPicklist(); String f=d.isAiPredictionField()+':'+d.isDisplayLocationInDecimal()+':'+d.isFormulaTreatNullNumberAsZero(); String c=d.isCascadeDelete()+':'+d.isDeprecatedAndHidden()+':'+d.isHighScaleNumber()+':'+d.isNamePointing()+':'+d.isQueryByDistance()+':'+d.isRestrictedDelete()+':'+d.isSearchPrefilterable()+':'+d.isWriteRequiresMasterRead();`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			for name, id := range map[string]string{"a": "facets_a", "b": "facets_b", "c": "facets_c", "e": "facets_d", "f": "facets_e"} {
				want := rows[id+"/"+api]
				if got := machine.Globals[name].Text; got != want {
					t.Errorf("%s got %q want %q", name, got, want)
				}
			}
		})
	}
}
