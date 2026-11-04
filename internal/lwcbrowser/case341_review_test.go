package lwcbrowser

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// These rows preserve the pre- implementation; they are not claims
// about Salesforce's handling of schemes that have not been captured.
func TestCase341PreservesOtherURLSchemes(t *testing.T) {
	source, err := filepath.Abs("../../lwcruntime/src/lightning/formattedUrl.mjs")
	if err != nil {
		t.Fatal(err)
	}
	script := `import {readFile} from 'node:fs/promises';
const src=await readFile(process.argv[1],'utf8');
const js=src.replace('import { createBaseComponent } from "./base.mjs";', '').replace('export default createBaseComponent("lightning-formatted-url", renderFormattedUrl);','export { renderFormattedUrl };');
const {renderFormattedUrl}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
const api={h:(tag,data,children)=>({data}),t:value=>value};
for(const value of ['sftp://example.invalid/a','ws://example.invalid/a','wss://example.invalid/a','file:///local','custom+app://example.invalid/a']){
const got=renderFormattedUrl(api,{value})[0].data.attrs.href;if(got!==value)throw Error(JSON.stringify({value,got}));}`
	cmd := exec.Command("node", "--input-type=module", "-e", script, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("previous URL behavior: %v\n%s", err, out)
	}
}

// Browser comparisons preserve the exact two-pass parser behavior on the
// pre-fix tree, including newline-sensitive and raw-text HTML elements.
func TestCase341RichTextParserBaseline(t *testing.T) {
	module := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if module == "" {
		module = filepath.Join("..", "..", "lwcruntime", "node_modules", "playwright")
	}
	module, err := filepath.Abs(module)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(module); err != nil {
		t.Skipf("Playwright unavailable: %v", err)
	}
	source, err := filepath.Abs("../../lwcruntime/src/lightning/formattedRichText.mjs")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "testdata/case341_review/richtext.mjs", module, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("previous rich text behavior: %v\n%s", err, out)
	}
}
