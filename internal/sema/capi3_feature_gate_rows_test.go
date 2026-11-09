package sema

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// capi3_feature_gates/org.tsv: for each scratch shape and API, a gated service
// is hidden exactly when the native compiler hid it, and no service the
// compiler accepted is hidden. Services gated by licenses a scratch org cannot
// enable stay open (the rows record them as "no" everywhere).
func TestCapi3FeatureGateRows(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", "capi3_feature_gates", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type key struct{ api, shape string }
	rows := map[key]map[string]string{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 4 || p[0] == "api" {
			continue
		}
		k := key{p[0], p[1]}
		if rows[k] == nil {
			rows[k] = map[string]string{}
		}
		rows[k][strings.TrimPrefix(p[2], "ConnectApi.")] = p[3]
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) < 25 {
		t.Fatalf("native shape groups = %d", len(rows))
	}
	for k, services := range rows {
		k, services := k, services
		t.Run(k.api+"/"+k.shape, func(t *testing.T) {
			def := scratchDefFromShape(t, k.shape)
			root := t.TempDir()
			var names []string
			for name := range services {
				names = append(names, name)
			}
			sort.Strings(names)
			var b strings.Builder
			b.WriteString("public class Probe { public static void run() {\n")
			for i, name := range names {
				fmt.Fprintf(&b, "ConnectApi.%s v%d;\n", name, i)
			}
			b.WriteString("} }")
			path := filepath.Join(root, "Probe.cls")
			writeSemaFile(t, path, b.String())
			if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), def)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: k.api, ApexFiles: []string{path}}, schema.Schema{})
			hidden := map[string]bool{}
			for _, d := range Analyze(index).Diagnostics {
				if name, ok := strings.CutPrefix(d.Message, "Type is not visible: ConnectApi."); ok {
					hidden[name] = true
				}
			}
			for _, name := range names {
				_, gated := semaConnectAPIFeatureGates[strings.ToLower("ConnectApi."+name)]
				switch services[name] {
				case "yes":
					if hidden[name] {
						t.Errorf("%s compiles natively, glade hides it", name)
					}
				case "no":
					if gated && !hidden[name] {
						t.Errorf("%s is hidden natively, glade shows it", name)
					}
				}
			}
		})
	}
}

// scratchDefFromShape renders "features=A,B; settings=block.key,block.key"
// as a scratch definition with every listed setting true.
func scratchDefFromShape(t *testing.T, shape string) string {
	t.Helper()
	var features []string
	settings := map[string]map[string]bool{}
	for _, part := range strings.Split(shape, ";") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "features="):
			features = strings.Split(strings.TrimPrefix(part, "features="), ",")
		case strings.HasPrefix(part, "settings="):
			for _, item := range strings.Split(strings.TrimPrefix(part, "settings="), ",") {
				block, key, ok := strings.Cut(item, ".")
				if !ok {
					t.Fatalf("bad setting %q", item)
				}
				if settings[block] == nil {
					settings[block] = map[string]bool{}
				}
				settings[block][key] = true
			}
		default:
			t.Fatalf("bad shape %q", shape)
		}
	}
	var blocks []string
	for block, keys := range settings {
		var items []string
		for key := range keys {
			items = append(items, fmt.Sprintf("%q:true", key))
		}
		sort.Strings(items)
		blocks = append(blocks, fmt.Sprintf("%q:{%s}", block, strings.Join(items, ",")))
	}
	sort.Strings(blocks)
	quoted := make([]string, len(features))
	for i, f := range features {
		quoted[i] = fmt.Sprintf("%q", f)
	}
	def := `{"edition":"Developer"`
	if len(features) > 0 {
		def += `,"features":[` + strings.Join(quoted, ",") + `]`
	}
	if len(blocks) > 0 {
		def += `,"settings":{` + strings.Join(blocks, ",") + `}`
	}
	return def + `}`
}
