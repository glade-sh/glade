package compile

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// legacySourcePartitions is the existing CompileBatch contract: every source
// must belong to exactly one root, and each root keeps the original order.
func legacySourcePartitions(root string, order, js, html []string) (map[string][]string, map[string][]string, error) {
	for _, source := range append(append([]string{}, js...), html...) {
		matches := 0
		for _, key := range order {
			if len(filesInBundle(filepath.Join(root, filepath.FromSlash(key)), []string{source})) != 0 {
				matches++
			}
		}
		if matches != 1 {
			return nil, nil, fmt.Errorf("LWC source %s belongs to %d metadata bundles", filepath.ToSlash(source), matches)
		}
	}
	javascript, templates := map[string][]string{}, map[string][]string{}
	for _, key := range order {
		dir := filepath.Join(root, filepath.FromSlash(key))
		javascript[key] = filesInBundle(dir, js)
		templates[key] = filesInBundle(dir, html)
	}
	return javascript, templates, nil
}

func TestPartitionBundleSourcesMatchesLegacy(t *testing.T) {
	root := t.TempDir()
	p := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	for _, tc := range []struct {
		name            string
		order, js, html []string
	}{
		{"ordered-duplicates", []string{"alpha", "beta"}, []string{p("beta", "b.js"), p("alpha", "a.js"), p("alpha", "a.js")}, []string{p("alpha", "a.html"), p("beta", "b.html")}},
		{"nested-overlap", []string{"alpha", "alpha/nested"}, []string{p("alpha", "nested", "x.js")}, nil},
		{"outside-first", []string{"alpha", "beta"}, []string{p("outside", "x.js"), p("alpha", "a.js")}, []string{p("outside", "x.html")}},
		{"relative-source", []string{"alpha"}, []string{filepath.Join("alpha", "x.js")}, nil},
		{"lexical-alias", []string{"alpha"}, []string{p("alias", "x.js")}, nil},
		{"root-itself", []string{"alpha"}, []string{p("alpha")}, nil},
		{"duplicate-roots", []string{"alpha", "alpha"}, []string{p("alpha", "x.js")}, nil},
		{"empty", nil, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantJS, wantHTML, wantErr := legacySourcePartitions(root, tc.order, tc.js, tc.html)
			gotJS, gotHTML, gotErr := partitionBundleSources(root, tc.order, tc.js, tc.html)
			if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
				t.Fatalf("error = %v, legacy %v", gotErr, wantErr)
			}
			if gotErr == nil && (!reflect.DeepEqual(gotJS, wantJS) || !reflect.DeepEqual(gotHTML, wantHTML)) {
				t.Fatalf("partitions differ: js=%v legacy=%v html=%v legacy=%v", gotJS, wantJS, gotHTML, wantHTML)
			}
		})
	}
}

func TestPartitionBundleSourcesWideAndSymlink(t *testing.T) {
	root := t.TempDir()
	var order, javascript, templates []string
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("bundles/b%04d", i)
		order = append(order, key)
		javascript = append(javascript, filepath.Join(root, key, "main.js"), filepath.Join(root, key, "nested", "helper.js"))
		templates = append(templates, filepath.Join(root, key, "main.html"))
	}
	wantJS, wantHTML, wantErr := legacySourcePartitions(root, order, javascript, templates)
	gotJS, gotHTML, gotErr := partitionBundleSources(root, order, javascript, templates)
	if wantErr != nil || gotErr != nil || !reflect.DeepEqual(gotJS, wantJS) || !reflect.DeepEqual(gotHTML, wantHTML) {
		t.Fatalf("wide partitions differ: legacy error %v, indexed error %v", wantErr, gotErr)
	}

	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(target, alias); err != nil && filepath.Separator != '\\' {
		t.Fatal(err)
	}
	path := filepath.Join(alias, "source.js")
	_, _, wantErr = legacySourcePartitions(root, []string{"target"}, []string{path}, nil)
	_, _, gotErr = partitionBundleSources(root, []string{"target"}, []string{path}, nil)
	if wantErr == nil || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
		t.Fatalf("symlink ownership error = %v, legacy %v", gotErr, wantErr)
	}
}

func BenchmarkPartitionBundleSources(b *testing.B) {
	root := filepath.Join(string(filepath.Separator), "synthetic", "project")
	var order, js, html []string
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("bundles/b%04d", i)
		order = append(order, key)
		js = append(js, filepath.Join(root, key, "main.js"), filepath.Join(root, key, "nested", "helper.js"))
		html = append(html, filepath.Join(root, key, "main.html"))
	}
	for _, tc := range []struct {
		name string
		fn   func(string, []string, []string, []string) (map[string][]string, map[string][]string, error)
	}{{"legacy", legacySourcePartitions}, {"indexed", partitionBundleSources}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, err := tc.fn(root, order, js, html)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
