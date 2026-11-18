package soql

import "testing"

// api68_native queryoptions: SET OPTIONS :bind follows LIMIT and ends the query.
func TestParseSetOptionsBind(t *testing.T) {
	query, err := Parse("SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt")
	if err != nil || query.OptionsBind != "opt" || query.Limit != 1 {
		t.Fatalf("SET OPTIONS = %+v, %v", query, err)
	}
	for _, text := range []string{
		"SELECT Id FROM Account SET OPTIONS :opt LIMIT 1",
		"SELECT Id FROM Account SET OPTIONS opt",
		"SELECT Id FROM Account SET :opt",
		"SELECT Id FROM Account LIMIT 1 SET OPTIONS :a SET OPTIONS :b",
	} {
		if _, err := Parse(text); err == nil {
			t.Fatalf("%s parsed", text)
		}
	}
}
