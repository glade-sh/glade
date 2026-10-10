package compile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/project"
)

// Keep each fixture's original bundle identity (and auxiliary bundles), while
// loading all cases as one project. Output directories are isolated by path.
func compileConformanceBatch(t *testing.T, api string, count int, write func(string, int)) []BundleResult {
	t.Helper()
	root := t.TempDir()
	for index := 0; index < count; index++ {
		write(filepath.Join(root, "cases", fmt.Sprintf("%04d", index)), index)
	}
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"cases","default":true}],"sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	bundles, err := CompileBatch(p, Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		t.Fatal(err)
	}
	results := make([]BundleResult, count)
	seen := make([]bool, count)
	// Preserve Compile's first error: metadata validation precedes Node, and
	// Node visits bundle metadata in project.Load's sorted order.
	for _, meta := range p.LWCMetaFiles {
		key, err := filepath.Rel(root, filepath.Dir(meta))
		if err != nil {
			t.Fatal(err)
		}
		key = filepath.ToSlash(key)
		index, scanErr := strconv.Atoi(strings.Split(key, "/")[1])
		if scanErr != nil || index < 0 || index >= count {
			t.Fatalf("unexpected bundle path %s", key)
		}
		result, ok := bundles[key]
		if !ok {
			t.Fatalf("missing batch bundle %s", key)
		}
		seen[index] = true
		previous := results[index]
		var metadataError, previousMetadataError *lwc.MetadataValidationError
		metadataFailure := result.Err != nil && (len(result.Diagnostics) == 0 || errors.As(result.Err, &metadataError))
		previousMetadataFailure := previous.Err != nil && (len(previous.Diagnostics) == 0 || errors.As(previous.Err, &previousMetadataError))
		if previous.Err == nil || (metadataFailure && !previousMetadataFailure) {
			results[index] = result
		}
	}
	for index, ok := range seen {
		if !ok {
			t.Fatalf("case %d has no batch result", index)
		}
	}
	return results
}

// Restrict a loaded project to a fixture without changing its source paths.
func compileCaseProject(p project.Project, root string) project.Project {
	p.LWCFiles = filesInBundle(root, p.LWCFiles)
	p.LWCHTMLFiles = filesInBundle(root, p.LWCHTMLFiles)
	p.LWCMetaFiles = filesInBundle(root, p.LWCMetaFiles)
	return p
}

func compileErrorDiagnostics(t *testing.T, err error) []Diagnostic {
	t.Helper()
	diagnostics := []Diagnostic{}
	// Metadata validation runs in Go before the Node observer can emit markers.
	// Unwrap the same ordered native messages exposed by the batch result.
	var metadataError *lwc.MetadataValidationError
	if errors.As(err, &metadataError) {
		for _, message := range metadataError.Messages {
			diagnostics = append(diagnostics, Diagnostic{Message: message})
		}
		return diagnostics
	}
	if err != nil {
		for _, line := range strings.Split(err.Error(), "\n") {
			const marker = "GLADE_L03_DIAGNOSTIC|"
			if strings.HasPrefix(line, marker) {
				var diagnostic Diagnostic
				if decodeErr := json.Unmarshal([]byte(strings.TrimPrefix(line, marker)), &diagnostic); decodeErr != nil {
					t.Fatal(decodeErr)
				}
				diagnostics = append(diagnostics, diagnostic)
			}
		}
	}
	return diagnostics
}

// TestCompileBatchEquivalence compares original diagnostics and every output
// byte (including partial outputs on error) using identical source/output paths.
// The owned sample spans duplicate bundle names, version gates, metadata errors,
// preflight errors, auxiliary templates, and multi-bundle lifecycle fixtures.
func TestCompileBatchEquivalence(t *testing.T) {
	var l01 []l01Case
	var l03 []l03Case
	var l04 []l04Case
	var l06 []l06Case
	for _, table := range []struct {
		path string
		rows any
	}{
		{"testdata/l01_salesforce.json", &l01},
		{"testdata/l03_salesforce.json", &l03},
		{"testdata/l04_salesforce.json", &l04},
		{"testdata/l06_salesforce.json", &l06},
	} {
		data, err := os.ReadFile(table.path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, table.rows); err != nil {
			t.Fatal(err)
		}
	}
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			index := 0
			for _, id := range []string{"grammar_span_text", "meta_lightning__Bogus_true", "meta_property_missing", "meta_no_version"} {
				found := false
				for _, c := range l01 {
					if c.ID == id {
						l01WriteBundle(t, filepath.Join(root, "cases", fmt.Sprintf("%04d", index)), c, api)
						index++
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("missing owned L01 sample %s", id)
				}
			}
			for _, id := range []string{"each_span_key_id", "each_span_each_call", "each_span_each_quoted_call"} {
				found := false
				for _, c := range l03 {
					if c.ID == id {
						l03WriteCompileBundle(t, filepath.Join(root, "cases", fmt.Sprintf("%04d", index)), c, api)
						index++
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("missing owned L03 sample %s", id)
				}
			}
			for _, id := range []string{"compile_track_static_text", "compile_render_malformed_alternate", "compile_render_switch_templates", "dom_order_initial", "compile_field_string_text"} {
				found := false
				for _, c := range l04 {
					if c.ID == id {
						l04WriteBundle(t, filepath.Join(root, "cases", fmt.Sprintf("%04d", index)), "familyL04Sample", c, api)
						index++
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("missing owned L04 sample %s", id)
				}
			}
			// Exercise the complete native wire-conflict deployment diagnostics
			// through both entry points with the original source/bundle identities.
			var l14 []struct {
				ID    string `json:"id"`
				Input struct {
					JS           string `json:"js"`
					Template     string `json:"template"`
					MetaFragment string `json:"meta_fragment"`
				} `json:"input"`
			}
			data, err := os.ReadFile("testdata/l14_salesforce.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &l14); err != nil {
				t.Fatal(err)
			}
			wireSamples := 0
			for originalIndex, c := range l14 {
				if c.ID != "compile_current_api_wire" && c.ID != "compile_current_duplicate_wire" {
					continue
				}
				name := fmt.Sprintf("familyL14%03d", originalIndex)
				bundle := filepath.Join(root, "cases", fmt.Sprintf("%04d", index), "force-app", "main", "default", "lwc", name)
				js := strings.ReplaceAll(c.Input.JS, "FamilyNavigation", "FamilyL14"+fmt.Sprintf("%03d", originalIndex))
				writeCompileFixtureFile(t, filepath.Join(bundle, name+".js"), js)
				writeCompileFixtureFile(t, filepath.Join(bundle, name+".html"), c.Input.Template)
				fragment := c.Input.MetaFragment
				if fragment == "" {
					fragment = "<isExposed>false</isExposed>"
				}
				writeCompileFixtureFile(t, filepath.Join(bundle, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`)
				index++
				wireSamples++
			}
			if wireSamples != 2 {
				t.Fatalf("missing L14 native wire-conflict samples: got %d", wireSamples)
			}
			for _, id := range []string{"c_dynamic_component_ctor", "c_dynamic_component_literal", "c_dynamic_legacy", "c_metadata_shadow_duplicate", "c_metadata_shadow_unknown"} {
				found := false
				for _, c := range l06 {
					if c.ID == id {
						l06PrepareCase(t, filepath.Join(root, "cases", fmt.Sprintf("%04d", index)), c.BundleName, c, api)
						index++
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("missing owned L06 sample %s", id)
				}
			}
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"cases","default":true}],"sourceApiVersion":"`+api+`"}`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			observer := filepath.Join(root, "diagnostics.cjs")
			writeCompileFixtureFile(t, observer, l03DiagnosticObserverJS)
			t.Setenv("NODE_OPTIONS", os.Getenv("NODE_OPTIONS")+" --require "+l03JSON(t, observer))
			started := time.Now()
			batch, err := CompileBatch(p, Options{OutDir: filepath.Join(root, "dist")})
			batchTime := time.Since(started)
			if err != nil {
				t.Fatal(err)
			}
			if len(batch) != len(p.LWCMetaFiles) {
				t.Fatalf("batch returned %d of %d bundles", len(batch), len(p.LWCMetaFiles))
			}
			var singleTime time.Duration
			failures, successes := 0, 0
			for _, meta := range p.LWCMetaFiles {
				bundleRoot := filepath.Dir(meta)
				key, err := filepath.Rel(root, bundleRoot)
				if err != nil {
					t.Fatal(err)
				}
				result := batch[filepath.ToSlash(key)]
				batchFiles := compileOutputBytes(t, result.Manifest.OutDir)
				// Only remove this test's own generated output, so the old Compile
				// call writes into the same destination as the batch comparison.
				if err := os.RemoveAll(result.Manifest.OutDir); err != nil {
					t.Fatal(err)
				}
				started := time.Now()
				single, singleErr := Compile(compileCaseProject(p, bundleRoot), Options{OutDir: result.Manifest.OutDir})
				singleTime += time.Since(started)
				if (singleErr == nil) != (result.Err == nil) {
					t.Fatalf("%s batch error %v; single error %v", key, result.Err, singleErr)
				}
				if !reflect.DeepEqual(batchFiles, compileOutputBytes(t, result.Manifest.OutDir)) {
					t.Errorf("%s output bytes differ", key)
				}
				diagnostics := compileErrorDiagnostics(t, singleErr)
				if l03JSON(t, result.Diagnostics) != l03JSON(t, diagnostics) {
					t.Errorf("%s diagnostic bytes differ: batch %s single %s", key, l03JSON(t, result.Diagnostics), l03JSON(t, diagnostics))
				}
				if singleErr == nil {
					successes++
					if !reflect.DeepEqual(result.Manifest, single) {
						t.Errorf("%s manifest differs: batch %+v single %+v", key, result.Manifest, single)
					}
				} else {
					failures++
					if len(diagnostics) == 0 && result.Err.Error() != singleErr.Error() {
						t.Errorf("%s metadata diagnostic bytes differ", key)
					}
					if l04CompileDiagnostic(result.Err) != l04CompileDiagnostic(singleErr) {
						t.Errorf("%s lifecycle diagnostic signature differs", key)
					}
					if len(diagnostics) != 0 && diagnostics[0].Code != 0 {
						expected := BundleResult{Err: singleErr, Diagnostics: diagnostics}
						if l03JSON(t, l03BatchSignatures(t, result)) != l03JSON(t, l03BatchSignatures(t, expected)) {
							t.Errorf("%s iteration diagnostic signature bytes differ", key)
						}
					}
				}
			}
			if failures == 0 || successes == 0 {
				t.Fatalf("sample must include successes and failures: %d/%d", successes, failures)
			}
			t.Logf("API %s: %d bundles byte-identical; per-case %.3fs batch %.3fs", api, len(batch), singleTime.Seconds(), batchTime.Seconds())
		})
	}
}

func compileOutputBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return files
	}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
