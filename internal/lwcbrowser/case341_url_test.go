package lwcbrowser

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCase341FormattedURLNative(t *testing.T) {
	f, err := os.Open("testdata/case341_url/org.tsv")
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
	source, err := filepath.Abs("../../lwcruntime/src/lightning/formattedUrl.mjs")
	if err != nil {
		t.Fatal(err)
	}
	script := `import {readFile} from 'node:fs/promises';
const source=await readFile(process.argv[1], 'utf8');
const transformed=source.replace('import { createBaseComponent } from "./base.mjs";', '').replace('export default createBaseComponent("lightning-formatted-url", renderFormattedUrl);', 'export { renderFormattedUrl };');
const {renderFormattedUrl}=await import('data:text/javascript;base64,'+Buffer.from(transformed).toString('base64'));
const [value,want]=JSON.parse(process.argv[2]);
const api={h:(tag,data,children)=>({tag,data,children}),t:x=>x};
const result=renderFormattedUrl(api,{value});
const got=result.length ? result[0].data.attrs.href : '';
if(got!==want) throw Error(JSON.stringify({value,want,got}));`
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			input, err := json.Marshal(row[2:4])
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("node", "--input-type=module", "-e", script, source, string(input))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("native URL result: %v\n%s", err, out)
			}
		})
	}
}
