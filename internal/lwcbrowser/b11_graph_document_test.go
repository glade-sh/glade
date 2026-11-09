package lwcbrowser

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestB11MutationDocumentNative(t *testing.T) {
	f, err := os.Open("testdata/b11_graphql/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[1], func(t *testing.T) {
			input, err := json.Marshal(map[string]string{"expected": row[2]})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("node", "testdata/b11_graphql/document.mjs")
			cmd.Stdin = bytes.NewReader(input)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("GraphQL document: %v\n%s", err, out)
			}
		})
	}
}
