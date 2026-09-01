package gladecli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// preparedAnonymousSource separates transient type declarations from the
// execute-anonymous body. Transient types are compiled through the same
// project compiler used for Apex classes, then registered only on this VM.
// The body keeps its original byte offsets so diagnostics remain useful.
type preparedAnonymousSource struct {
	body       string
	apiVersion string
	index      typesys.Index
	runtime    apextest.CompiledProjectRuntime
	cleanup    func()
}

func (p preparedAnonymousSource) close() {
	if p.cleanup != nil {
		p.cleanup()
	}
}

func prepareAnonymousSource(source, apiVersion string) (preparedAnonymousSource, error) {
	apiVersion, err := apexversion.ResolveSource(apiVersion)
	if err != nil {
		return preparedAnonymousSource{}, err
	}
	prepared := preparedAnonymousSource{body: source, apiVersion: apiVersion}
	parsed := apexast.NewParser().ParseSource("__glade_anonymous.cls", source)
	var declarations []struct {
		start int
		end   int
		text  string
	}
	for _, declaration := range parsed.Declarations {
		switch declaration.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
		default:
			continue
		}
		start, end := declaration.Range.Start.Offset, declaration.Range.End.Offset
		if start < 0 || end <= start || end > len(source) {
			return prepared, fmt.Errorf("anonymous declaration %q has invalid source range", declaration.Name)
		}
		declarations = append(declarations, struct {
			start int
			end   int
			text  string
		}{start: start, end: end, text: source[start:end]})
	}
	if len(declarations) == 0 {
		return prepared, nil
	}
	if len(parsed.Diagnostics) > 0 {
		return prepared, fmt.Errorf("%s", parsed.Diagnostics[0].Message)
	}

	tempRoot, err := os.MkdirTemp("", "glade-anonymous-runtime-")
	if err != nil {
		return prepared, fmt.Errorf("create anonymous runtime: %w", err)
	}
	prepared.cleanup = func() { _ = os.RemoveAll(tempRoot) }

	classDir := filepath.Join(tempRoot, "force-app", "main", "default", "classes")
	if err := os.MkdirAll(classDir, 0o700); err != nil {
		prepared.close()
		return prepared, fmt.Errorf("create anonymous class directory: %w", err)
	}
	paths := make([]string, 0, len(declarations))
	for i, declaration := range declarations {
		path := filepath.Join(classDir, fmt.Sprintf("GladeAnonymous%d.cls", i))
		if err := os.WriteFile(path, []byte(declaration.text), 0o600); err != nil {
			prepared.close()
			return prepared, fmt.Errorf("write anonymous declaration: %w", err)
		}
		paths = append(paths, path)
	}

	body := []byte(source)
	sort.Slice(declarations, func(i, j int) bool { return declarations[i].start < declarations[j].start })
	for _, declaration := range declarations {
		for i := declaration.start; i < declaration.end; i++ {
			if body[i] != '\n' && body[i] != '\r' {
				body[i] = ' '
			}
		}
	}
	prepared.body = string(body)
	index := typesys.Build(project.Project{
		Root:             tempRoot,
		SourceAPIVersion: apiVersion,
		ApexFiles:        paths,
	}, gladeschema.Schema{})
	for _, item := range index.Diagnostics {
		if item.Severity == diagnostic.Error {
			prepared.close()
			return prepared, fmt.Errorf("anonymous declaration index: %s", item.Message)
		}
	}
	runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	if err != nil {
		prepared.close()
		return prepared, fmt.Errorf("compile anonymous declarations: %w", err)
	}
	prepared.index = index
	prepared.runtime = runtime
	return prepared, nil
}

func mergeAnonymousIndex(base, transient typesys.Index) typesys.Index {
	if len(transient.Types) == 0 {
		return base
	}
	merged := base
	merged.Types = append(append([]typesys.TypeSymbol(nil), base.Types...), transient.Types...)
	if merged.Project.Root == "" {
		merged.Project = transient.Project
	}
	return merged
}

func registerAnonymousRuntime(machine *vm.VM, runtime apextest.CompiledProjectRuntime) error {
	return apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime)
}
