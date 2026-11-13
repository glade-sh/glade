package scripts

import (
	"os/exec"
	"testing"
)

func TestCICodeQLTestPathFilter(t *testing.T) {
	code := `import importlib.util,json,os,pathlib,tempfile
s=importlib.util.spec_from_file_location('ci','ci-codeql.py'); m=importlib.util.module_from_spec(s); s.loader.exec_module(m)
with tempfile.TemporaryDirectory() as d:
 os.environ.update(CODEQL_ACTION_TEMP=d,GITHUB_WORKSPACE=d)
 rows=[{'path':p,'startLine':1,'endLine':2} for p in ['a_test.go','nested/b_test.go','testdata/x.go','nested/testdata/y.go','nested/testdata_prod.go','source.go']]
 pathlib.Path(d,'pr-diff-range.json').write_text(json.dumps(rows))
 data=json.loads((m.diff_pack('fast')/'diff-ranges.yml').read_text())['extensions'][0]['data']
 assert [pathlib.Path(r[0]).relative_to(d).as_posix() for r in data] == ['nested/testdata_prod.go','source.go'], data
`
	if out, err := exec.Command("python3", "-c", code).CombinedOutput(); err != nil {
		t.Fatalf("filter: %v\n%s", err, out)
	}
}
