package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A page the Visualforce loader rejects is reported on stderr with its file
// and message. The run's results and exit code stay what they were without it.
func TestRunTestReportsRejectedVisualforcePages(t *testing.T) {
	root := t.TempDir()
	if err := copySelectionFixture(filepath.Join("..", "..", "testdata", "local-tests", "basic"), root); err != nil {
		t.Fatal(err)
	}
	args := []string{"test", "--project", root, "--class", "PassingTest", "--progress", "--no-cache", "--no-serve"}
	var baseOut, baseErr bytes.Buffer
	baseCode := Run(context.Background(), args, &baseOut, &baseErr)
	if baseCode != 0 {
		t.Fatalf("baseline exit = %d stderr=%q stdout=%q", baseCode, baseErr.String(), baseOut.String())
	}
	if strings.Contains(baseErr.String(), "visualforce:") {
		t.Fatalf("baseline stderr reports a page: %q", baseErr.String())
	}
	pages := filepath.Join(root, "force-app", "main", "default", "pages")
	if err := os.MkdirAll(pages, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>L</label></ApexPage>` + "\n"
	files := map[string]string{
		"Good.page":          `<apex:page>ok</apex:page>`,
		"Bad.page":           `<apex:page>{!ISPICKVAL($User.FirstName, 'x')}</apex:page>`,
		"Good.page-meta.xml": meta,
		"Bad.page-meta.xml":  meta,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(pages, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	code := Run(context.Background(), args, &out, &errOut)
	if code != baseCode {
		t.Fatalf("exit with a rejected page = %d, want %d; stderr=%q", code, baseCode, errOut.String())
	}
	want := filepath.Join("force-app", "main", "default", "pages", "Bad.page")
	if !strings.Contains(errOut.String(), "visualforce: "+want+": ") {
		t.Fatalf("stderr does not name the rejected page %s: %q", want, errOut.String())
	}
	if strings.Contains(errOut.String(), "Good.page") {
		t.Fatalf("stderr reports the page that loads: %q", errOut.String())
	}
}
