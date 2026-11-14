package repoguard

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestInputViewGuardRejectsFilesystemBypasses(t *testing.T) {
	sources := []string{
		`package p; import "os"; var _ = os.OpenFile`,
		`package p; import "os"; var _ = os.Readlink`,
		`package p; import "os"; var _ = os.OpenRoot`,
		`package p; import "os"; import "io/fs"; var _ = fs.ReadFile(os.DirFS("."), "x")`,
		`package p; import "io/fs"; var _ = fs.ReadDir`,
		`package p; import "io/fs"; var _ = fs.Stat`,
		`package p; import "io/fs"; var _ = fs.WalkDir`,
		`package p; import "io/fs"; var _ = fs.Glob`,
		`package p; import "io/ioutil"; var _ = ioutil.ReadFile`,
	}
	for _, source := range sources {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "input.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(inputViewBypasses(fset, file)) == 0 {
			t.Errorf("missed %s", source)
		}
	}
}
