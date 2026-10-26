package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStaticObjectChainReproducerNamedAndAnonymousRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "orchard"} {
			for _, route := range []string{"named", "anonymous"} {
				t.Run(api+"/"+namespace+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					receiver := "ReceiverValues.Instance.Text"
					if namespace != "" {
						receiver = namespace + "." + receiver
					}
					statement := "System.assertEquals('parcel', " + receiver + ".toLowerCase());"
					files := map[string]string{
						"Holder.cls":             "public class Holder { public String Text = 'PARCEL'; }",
						"ReceiverValues.cls":     "public class ReceiverValues { public static Holder Instance = new Holder(); }",
						"ReceiverChainProbe.cls": "public class ReceiverChainProbe { public static void observe() { " + statement + " } }",
					}
					if route == "anonymous" {
						files["ReceiverChainProbe.cls"] = "public class ReceiverChainProbe {}"
					}
					var paths []string
					for name, source := range files {
						path := filepath.Join(root, name)
						writeSemaFile(t, path, source)
						paths = append(paths, path)
					}
					index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
					if index.HasErrors() {
						t.Fatalf("index errors: %v", index.Diagnostics)
					}
					result := Analyze(index)
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, statement, api)
					}
					if result.HasErrors() {
						t.Fatalf("static object field chain rejected: %v", result.Diagnostics)
					}
					for _, d := range result.Diagnostics {
						if strings.Contains(d.NativeMessage, "Variable does not exist") {
							t.Fatal(d)
						}
					}
				})
			}
		}
	}
}
