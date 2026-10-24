package vm

import "testing"

func TestExecPageReferenceContentAnonymousAndInTestContext(t *testing.T) {
	saved := pageContentRenderer
	defer SetPageContentRenderer(saved)
	for _, api := range []string{"62.0", "67.0"} {
		for _, method := range []string{"getContent", "getContentAsPDF"} {
			for _, route := range []string{"anonymous", "isTest"} {
				t.Run(api+"/"+method+"/"+route, func(t *testing.T) {
					calls := 0
					SetPageContentRenderer(func(_ *VM, url string, pdf bool) (Value, error) {
						calls++
						if url != "/apex/ContentProbe" || pdf != (method == "getContentAsPDF") {
							t.Fatal("incorrect render request")
						}
						return NewBlobValue("content"), nil
					})
					machine := New(nil)
					if route == "isTest" {
						machine.EnableTestContext()
					}
					code := `Blob body = new PageReference('/apex/ContentProbe').` + method + `(); System.assertEquals('content', body.toString());`
					if route == "isTest" {
						code = `Boolean caught = false; try { new PageReference('/apex/ContentProbe').` + method + `(); } catch (VisualforceException e) { caught = true; System.assertEquals('System.VisualforceException', e.getTypeName()); System.assertEquals('Methods defined as TestMethod do not support getContent call', e.getMessage()); } System.assertEquals(true, caught);`
					}
					program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					want := 1
					if route == "isTest" {
						want = 0
					}
					if calls != want {
						t.Fatalf("render calls=%d, want %d", calls, want)
					}
				})
			}
		}
	}
}
