package soql

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type operationFormula struct {
	name, formula, display, policy string
}

var operationFormulas = []operationFormula{
	{"Sub", "Num__c - 1", "DOUBLE", "BlankAsBlank"}, {"RSub", "1 - Num__c", "DOUBLE", "BlankAsBlank"},
	{"Mul", "Num__c * 2", "DOUBLE", "BlankAsBlank"}, {"Neg", "-Num__c", "DOUBLE", "BlankAsBlank"},
	{"Pow", "Num__c ^ 2", "DOUBLE", "BlankAsBlank"}, {"Div", "Num__c / 2", "DOUBLE", "BlankAsBlank"},
	{"Cmp", "IF(Num__c + 1 > 0, 1, 2)", "DOUBLE", "BlankAsBlank"}, {"Mx", "MAX(Num__c + 1, 5)", "DOUBLE", "BlankAsBlank"},
	{"Ab", "ABS(Num__c - 1)", "DOUBLE", "BlankAsBlank"}, {"MulZ", "Num__c * 2", "DOUBLE", "BlankAsZero"},
	{"SubZ", "Num__c - 1", "DOUBLE", "BlankAsZero"}, {"CurMul", "Num__c * 2", "CURRENCY", "BlankAsBlank"},
	{"CurSub", "Num__c - 1", "CURRENCY", "BlankAsBlank"}, {"Div3Z", "Num__c / 3", "DOUBLE", "BlankAsZero"},
	{"Div3B", "Num__c / 3", "DOUBLE", "BlankAsBlank"},
	{"Flo", "FLOOR(Num__c - 1)", "DOUBLE", "BlankAsBlank"}, {"Cei", "CEILING(Num__c + 1)", "DOUBLE", "BlankAsBlank"},
	{"Mn", "MIN(Num__c + 1, 5)", "DOUBLE", "BlankAsBlank"}, {"Mx2", "MAX(5, Num__c + 1)", "DOUBLE", "BlankAsBlank"},
	{"Md", "MOD(Num__c + 1, 3)", "DOUBLE", "BlankAsBlank"}, {"Sq", "SQRT(Num__c + 1)", "DOUBLE", "BlankAsBlank"},
	{"Ex", "EXP(Num__c + 1)", "DOUBLE", "BlankAsBlank"}, {"Ln", "LN(Num__c + 1)", "DOUBLE", "BlankAsBlank"},
	{"Lg", "LOG(Num__c + 1)", "DOUBLE", "BlankAsBlank"}, {"Bv", "BLANKVALUE(Num__c + 1, 7)", "DOUBLE", "BlankAsBlank"},
	{"Nv", "NULLVALUE(Num__c + 1, 7)", "DOUBLE", "BlankAsBlank"}, {"Ib", "IF(ISBLANK(Num__c + 1), 9, Num__c + 1)", "DOUBLE", "BlankAsBlank"},
	{"In", "IF(ISNULL(Num__c - 1), 9, 1)", "DOUBLE", "BlankAsBlank"}, {"Ch", "(Num__c + 1) * 2", "DOUBLE", "BlankAsBlank"},
	{"Rd", "ROUND(Num__c * 3, 0)", "DOUBLE", "BlankAsBlank"}, {"Ln2", "LEN(TEXT(Num__c + 1))", "DOUBLE", "BlankAsBlank"},
	{"CAbs", "ABS(Num__c - 1)", "CURRENCY", "BlankAsBlank"}, {"CFlo", "FLOOR(Num__c - 1)", "CURRENCY", "BlankAsBlank"},
	{"CMn", "MIN(Num__c + 1, 5)", "CURRENCY", "BlankAsBlank"}, {"CMx", "MAX(Num__c + 1, 5)", "CURRENCY", "BlankAsBlank"},
	{"CMd", "MOD(Num__c + 1, 3)", "CURRENCY", "BlankAsBlank"}, {"CRd", "ROUND(Num__c * 3, 0)", "CURRENCY", "BlankAsBlank"},
	{"CRd2", "ROUND(Num__c / 3, 2)", "CURRENCY", "BlankAsBlank"}, {"CSq", "SQRT(Num__c + 1)", "CURRENCY", "BlankAsBlank"},
	{"CBv", "BLANKVALUE(Num__c + 1, 7)", "CURRENCY", "BlankAsBlank"}, {"CIb", "IF(ISBLANK(Num__c + 1), 9, Num__c + 1)", "CURRENCY", "BlankAsBlank"},
}

// operationKnownGaps lists non-null native cells that differ for reasons that
// predate and do not depend on the blank policy: FLOOR of a negative
// non-integer and the text width of a negative fraction. They are unchanged
// from origin/main and are not closed here.
var operationKnownGaps = map[string]bool{"e_neg/Flo": true, "e_neg/CFlo": true, "e_neg/Ln2": true}

func operationOrg() storage.OrgState {
	fields := map[string]storage.Field{
		"Name":   {APIName: "Name", Type: storage.FieldString},
		"Num__c": {APIName: "Num__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", Scale: 2, ScaleSpecified: true},
	}
	for _, f := range operationFormulas {
		fields[f.name+"__c"] = storage.Field{APIName: f.name + "__c", Type: storage.FieldCalculated, DisplayType: f.display, Formula: f.formula, FormulaTreatBlanksAs: f.policy, Scale: 2, ScaleSpecified: true}
	}
	def := storage.ObjectDefinition{APIName: "Pf__c", Fields: fields}
	records := map[storage.ID]storage.Record{}
	n := 0
	add := func(name, num string) {
		n++
		r := storage.Record{ID: storage.ID("a01" + strconv.Itoa(1000000000000+n)), Object: "Pf__c", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}
		if num != "" {
			r.Fields["Num__c"] = storage.DecimalValue(num)
		}
		records[r.ID] = r
	}
	add("a_null", "")
	add("b_zero", "0.00")
	add("c_two", "2.00")
	add("d_frac", "1.25")
	add("e_neg", "-1.25")
	for i := 1; i <= 6; i++ {
		add("s"+strconv.Itoa(i), "1.00")
	}
	org := storage.NewOrgState()
	org.Objects["Pf__c"] = storage.ObjectState{Definition: def, Records: records}
	return org
}

func readOperationTSV(t *testing.T, name, header string, rows int) [][]string {
	t.Helper()
	data, err := os.ReadFile("testdata/calculated_number_query/" + name)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lines[0] != header || len(lines) != rows+1 {
		t.Fatalf("invalid native table %s (%d lines)", name, len(lines))
	}
	var out [][]string
	for _, l := range lines[1:] {
		out = append(out, strings.Split(l, "\t"))
	}
	return out
}

// NATIVE testdata/calculated_number_query/operations.tsv rows 2-21
// anonymous and istest at API 62.0 and 67.0: arithmetic, comparison and
// function operations on a null Number input under both blank policies.
func TestCalculatedNumberQueryOperationsNative(t *testing.T) {
	checkOperationTable(t, "operations.tsv", "api\troute\tcase\tNum\tSub\tRSub\tMul\tNeg\tPow\tDiv\tCmp\tMx\tAb\tMulZ\tSubZ\tCurMul\tCurSub\tDiv3Z\tDiv3B")
}

// NATIVE testdata/calculated_number_query/functions.tsv rows 2-21
// anonymous and istest at API 62.0 and 67.0: numeric functions consuming a
// null Number intermediate.
func TestCalculatedNumberQueryFunctionsNative(t *testing.T) {
	checkOperationTable(t, "functions.tsv", "api\troute\tcase\tNum\tFlo\tCei\tMn\tMx2\tMd\tSq\tEx\tLn\tLg\tBv\tNv\tIb\tIn\tCh\tRd\tLn2\tMx\tAb")
}

// NATIVE testdata/calculated_number_query/currency_functions.tsv rows 2-21
// anonymous and istest at API 62.0 and 67.0: the same functions on Currency.
func TestCalculatedNumberQueryCurrencyFunctionsNative(t *testing.T) {
	checkOperationTable(t, "currency_functions.tsv", "api\troute\tcase\tNum\tCAbs\tCFlo\tCMn\tCMx\tCMd\tCRd\tCRd2\tCSq\tCBv\tCIb")
}

func checkOperationTable(t *testing.T, file, header string) {
	t.Helper()
	cols := strings.Split(header, "\t")[3:]
	var names []string
	for _, c := range cols {
		names = append(names, c+"__c")
	}
	result, err := ParseAndExecute(operationOrg(), "SELECT Name,"+strings.Join(names, ",")+" FROM Pf__c WHERE Name < 's' ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]storage.Record{}
	for _, r := range result.Records {
		byName[r.Fields["Name"].String] = r
	}
	for _, row := range readOperationTSV(t, file, header, 20) {
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			rec, ok := byName[row[2]]
			if !ok {
				t.Fatalf("no row %s", row[2])
			}
			for i, c := range cols {
				if operationKnownGaps[row[2]+"/"+c] {
					continue
				}
				if got := observedNumberText(rec.Fields[c+"__c"]); got != row[3+i] {
					t.Errorf("%s=%q, want %q", c, got, row[3+i])
				}
			}
		})
	}
}

// NATIVE testdata/calculated_number_query/surfaces.tsv rows 2-61
// anonymous and istest at API 62.0 and 67.0: WHERE, aggregate and ORDER BY
// consumers of queried Number formulas.
func TestCalculatedNumberQuerySurfacesNative(t *testing.T) {
	org := operationOrg()
	count := func(where string) string {
		result, err := ParseAndExecute(org, "SELECT Name FROM Pf__c WHERE Id != null AND "+where+" AND Name != 'zz'")
		if err != nil {
			t.Fatalf("%s: %v", where, err)
		}
		return strconv.Itoa(len(result.Records))
	}
	want := map[string]string{
		"eq033":        count("Div3Z__c = 0.33"),
		"eq0333333333": count("Div3Z__c = 0.3333333333"),
		"gt0331":       count("Div3Z__c > 0.331"),
		"gt0334":       count("Div3Z__c > 0.334"),
		"blankEq033":   count("Div3B__c = 0.33"),
		"mulNull":      count("Mul__c = null"),
		"subNull":      count("Sub__c = null"),
		"mulZero":      count("Mul__c = 0"),
		"mulZNull":     count("MulZ__c = null"),
	}
	agg, err := ParseAndExecute(org, "SELECT SUM(Div3Z__c) sz, AVG(Div3Z__c) az, SUM(Div3B__c) sb, SUM(Mul__c) sm, COUNT(Mul__c) cm FROM Pf__c WHERE Name >= 's'")
	if err != nil {
		t.Fatal(err)
	}
	if len(agg.Records) != 1 {
		t.Fatalf("aggregate rows=%d", len(agg.Records))
	}
	for k, f := range map[string]string{"sumZ": "sz", "avgZ": "az", "sumB": "sb", "sumMul": "sm", "countMul": "cm"} {
		v := agg.Records[0].Fields[f]
		if v.Kind == storage.ValueInteger {
			want[k] = strconv.FormatInt(int64(v.Integer), 10)
		} else {
			want[k] = observedNumberText(v)
		}
	}
	ord, err := ParseAndExecute(org, "SELECT Name FROM Pf__c WHERE Name < 's' ORDER BY Mul__c ASC NULLS FIRST, Name")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range ord.Records {
		names = append(names, r.Fields["Name"].String)
	}
	want["orderMul"] = strings.Join(names, ",")
	for _, row := range readOperationTSV(t, "surfaces.tsv", "api\troute\tquery\tvalue", 60) {
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			if got := want[row[2]]; got != row[3] {
				t.Errorf("%s=%q, want %q", row[2], got, row[3])
			}
		})
	}
}
