package dataweave

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOfficialEngineQueryRelationshipWriterBoundary(t *testing.T) {
	cp, javaHome := os.Getenv("GLADE_DATAWEAVE_TEST_CLASSPATH"), os.Getenv("GLADE_DATAWEAVE_TEST_JAVA_HOME")
	if cp == "" || javaHome == "" {
		t.Skip("query relationship contract requires explicit real DataWeave toolchain; configured acceptance must execute this test")
	}
	rt := Runtime{JavaPath: filepath.Join(javaHome, "bin", "java"), ClassPath: filepath.SplitList(cp), AdapterDirectory: t.TempDir()}
	ctx := context.Background()
	if err := CompileAdapter(ctx, filepath.Join(javaHome, "bin", "javac"), rt.ClassPath, rt.AdapterDirectory); err != nil {
		t.Fatal(err)
	}
	records := TypedValue{Kind: "list", Type: "List<Account>", Elements: []TypedValue{{Kind: "object", Type: "Account", Fields: []TypedField{
		{Name: "Id", Value: TypedValue{Kind: "string", Text: "001000000000001"}},
		{Name: "Contacts", Value: TypedValue{Kind: "query-list", Type: "List<Contact>", Elements: []TypedValue{{Kind: "object", Type: "Contact", Fields: []TypedField{{Name: "Id", Value: TypedValue{Kind: "string", Text: "003000000000001"}}}}}}},
	}}}}
	request := Request{Name: "ownedQuery", APIVersion: "66.0", Source: "%dw 2.0\ninput records application/java\noutput application/apex\n---\nrecords\n", Inputs: map[string]Input{"records": {Typed: &records}}}
	_, err := rt.Execute(ctx, request)
	var engine *EngineError
	if !errors.As(err, &engine) || engine.Phase != "execute" || engine.Message != `Invalid type: "Map_Wrapper", while writing application/apex at records.` {
		t.Fatalf("query writer error=%#v", err)
	}
	// Local regression control: carrying an input is not itself a writer error.
	// These source-change controls do not constitute additional Salesforce proof.
	request.Source = "%dw 2.0\ninput records application/java\noutput application/apex\n---\n7\n"
	output, err := rt.Execute(ctx, request)
	if err != nil || output.Typed == nil || output.Typed.Kind != "decimal" || output.Typed.Text != "7" {
		t.Fatalf("ignored query input output=%+v err=%v", output, err)
	}
	request.Source = "%dw 2.0\ninput records application/java\noutput application/apex\n---\nrecords map ((record)->record.Id)\n"
	output, err = rt.Execute(ctx, request)
	if err != nil || output.Typed == nil || output.Typed.Kind != "list" || len(output.Typed.Elements) != 1 || output.Typed.Elements[0].Text != "001000000000001" {
		t.Fatalf("transformed query output=%+v err=%v", output, err)
	}
}
