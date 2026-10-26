package apextest

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestBlobPDFRenderingNativeConformance(t *testing.T) {
	fixture := "testdata/conformance/blob_pdf_rendering"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 119 {
		t.Fatalf("native rows=%d", len(rows)-1)
	}
	// Reuse the captured synthetic inputs. Rich rendering facets and native
	// nondeterminism remain outside the local text writer's contract.
	source, err := os.ReadFile(filepath.Join(fixture, "project/main/default/classes/PdfProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	inputs := map[string]string{}
	for _, m := range regexp.MustCompile(`put\(out, '([^']+)', '([^']*)'\);`).FindAllStringSubmatch(string(source), -1) {
		inputs[m[1]] = m[2]
	}
	inputs["one_mb_text_run"] = "<html><body>" + strings.Repeat("x", 1000000) + "</body></html>"
	selected := map[string]bool{}
	for _, name := range []string{"plain_text", "simple_html", "css", "table", "img_data_uri", "img_external", "img_relative", "non_ascii", "empty", "whitespace", "not_html", "two_pages", "landscape_css", "a4_css", "script_tag", "malformed_html", "one_mb_text_run", "null", "toString_on_pdf"} {
		selected[name] = true
	}
	seen := map[string]bool{}
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
		// The named route uses the project's test discovery and method compiler;
		// observation transport leaves the binary bytes intact for Go assertions.
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/PDFLayoutTest.cls"), `@IsTest private class PDFLayoutTest { public static String input; public static Boolean instanceRoute; public static Boolean stringify; @IsTest static void observe() { String observed; try { Blob b; if(instanceRoute) { b = Blob.valueOf('receiver').toPdf(input); } else { b = Blob.toPdf(input); } if(stringify) { b.toString(); observed='ok'; } else { observed = 'PDF_BYTES|' + EncodingUtil.base64Encode(b); } } catch(Exception e) { observed = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); } System.assert(false,observed); } }`)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/PDFLayoutTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
		index := loadTestIndex(t, root)
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		cases := Discover(index, Options{SelectedClasses: []string{"PDFLayoutTest"}, SelectedMethod: "observe"})
		if len(cases) != 1 {
			t.Fatal("missing named test")
		}
		methods, methodErrors := compileTestMethodsForIndex(&index, cases)
		invokes, invokeErrors := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrors[key] != nil || invokeErrors[key] != nil {
			t.Fatal(methodErrors[key], invokeErrors[key])
		}
		for _, row := range rows[1:] {
			if row[0] != api || !selected[row[2]] {
				continue
			}
			for _, entry := range []string{"static", "instance"} {
				t.Run(api+"/"+row[1]+"/"+entry+"/"+row[2], func(t *testing.T) {
					seen[api+"/"+row[1]+"/"+entry] = true
					var native map[string]any
					if row[2] == "toString_on_pdf" {
						native = map[string]any{"exc": row[3]}
					} else if json.Unmarshal([]byte(row[3]), &native) != nil {
						t.Fatal("invalid oracle")
					}
					call := "Blob.toPdf"
					if entry == "instance" {
						call = "Blob.valueOf('receiver').toPdf"
					}
					input, ok := inputs[row[2]]
					if !ok && row[2] != "null" && row[2] != "toString_on_pdf" {
						t.Fatal("missing captured input")
					}
					arg := "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n").Replace(input) + "'"
					if row[2] == "null" {
						arg = "null"
					}
					expr := call + "(" + arg + ")"
					if row[2] == "toString_on_pdf" {
						expr = call + "('Hello').toString()"
					}
					code := "String observed; try { observed = 'PDF_BYTES|' + EncodingUtil.base64Encode(" + expr + "); } catch(Exception e) { observed = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); } System.assert(false,observed);"
					if row[2] == "toString_on_pdf" {
						code = "String observed; try { " + expr + "; observed='ok'; } catch(Exception e) { observed='EXC|' + e.getTypeName() + '|' + e.getMessage(); } System.assert(false,observed);"
					}
					machine := runner.newMachine()
					if row[1] == "isTest" {
						setup := "PDFLayoutTest.input = " + arg + "; PDFLayoutTest.instanceRoute = " + fmt.Sprint(entry == "instance") + "; PDFLayoutTest.stringify = " + fmt.Sprint(row[2] == "toString_on_pdf") + ";"
						program, compileErr := vm.CompileAnonymousWithOptions(setup, vm.CompileOptions{APIVersion: api})
						if compileErr != nil {
							t.Fatal(compileErr)
						}
						if _, setupErr := machine.Execute(program); setupErr != nil {
							t.Fatal(setupErr)
						}
						if registerErr := machine.RegisterMethod(methods[key]); registerErr != nil {
							t.Fatal(registerErr)
						}
						machine.EnableTestContext()
						_, err = machine.ExecuteInClass(invokes[key], cases[0].ClassName)
					} else {
						program, compileErr := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
						if compileErr != nil {
							t.Fatal(compileErr)
						}
						_, err = machine.Execute(program)
					}
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("observation: %v", err)
					}
					msg := fault.ExceptionMessage()
					if exception, ok := native["exc"]; ok {
						if !strings.HasSuffix(msg, "EXC|"+fmt.Sprint(exception)) {
							t.Fatalf("native=%s; local=%s", exception, msg)
						}
						return
					}
					_, encoded, ok := strings.Cut(msg, "PDF_BYTES|")
					if !ok {
						t.Fatalf("native PDF; local=%s", msg)
					}
					data, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						t.Fatal(err)
					}
					if len(data) == 0 {
						t.Fatal("empty PDF")
					}
					header, _ := native["head_hex"].(string)
					if header == "" {
						header, _ = native["hx"].(string)
					}
					// The next object number after the binary marker may differ.
					if hex.EncodeToString(data[:min(15, len(data))]) != header[:30] {
						t.Fatal("PDF header differs")
					}
					if want, ok := native["b64_8"]; ok && base64.StdEncoding.EncodeToString(data)[:8] != want {
						t.Fatal("base64 prefix differs")
					}
					info, ok := native["info"].(map[string]any)
					if !ok {
						info = map[string]any{"mediaBox": native["mb"], "pageCount": native["pc"], "fonts": native["fo"]}
					}
					if !strings.Contains(string(data), "/MediaBox["+fmt.Sprint(info["mediaBox"])+"]") {
						t.Fatalf("MediaBox differs: %v", info["mediaBox"])
					}
					count := len(regexp.MustCompile(`/Type/Page(?:/|\s)`).FindAll(data, -1))
					if fmt.Sprint(count) != fmt.Sprint(info["pageCount"]) {
						t.Fatalf("pageCount=%d native=%v", count, info["pageCount"])
					}
					fonts := []any{}
					for _, m := range regexp.MustCompile(`/BaseFont/([^/>\s]+)`).FindAllSubmatch(data, -1) {
						fonts = append(fonts, string(m[1]))
					}
					wantFonts := info["fonts"]
					if reflect.DeepEqual(wantFonts, []any{}) || reflect.DeepEqual(wantFonts, []any{"Times-Roman"}) {
						if !reflect.DeepEqual(fonts, wantFonts) {
							t.Fatalf("fonts=%v native=%v", fonts, wantFonts)
						}
					}
				})
			}
		}
	}
	if len(seen) != 8 {
		t.Fatalf("route matrix=%v", seen)
	}
}

// This pins the local text writer's pre-existing option spacing contract; it
// does not claim native select-control rendering fidelity.
func TestBlobPDFOptionSpacingProjectRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, entry := range []string{"static", "instance"} {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			call := "Blob.toPdf"
			if entry == "instance" {
				call = "Blob.valueOf('receiver').toPdf"
			}
			code := `Blob pdf = ` + call + `('<select><option>United States</option><option>Canada</option></select>'); System.assert(false, 'PDF_BYTES|' + EncodingUtil.base64Encode(pdf));`
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/PDFOptionsTest.cls"), `@IsTest private class PDFOptionsTest { @IsTest static void observe() { `+code+` } }`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/PDFOptionsTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{"PDFOptionsTest"}, SelectedMethod: "observe"})
			if len(cases) != 1 {
				t.Fatal("missing named test")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			invokes, invokeErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || invokeErrors[key] != nil {
				t.Fatal(methodErrors[key], invokeErrors[key])
			}
			for _, route := range []string{"anonymous", "isTest"} {
				t.Run(api+"/"+entry+"/"+route, func(t *testing.T) {
					machine := runner.newMachine()
					var err error
					if route == "isTest" {
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, err = machine.ExecuteInClass(invokes[key], cases[0].ClassName)
					} else {
						program, compileErr := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
						if compileErr != nil {
							t.Fatal(compileErr)
						}
						_, err = machine.Execute(program)
					}
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("observation: %v", err)
					}
					_, encoded, ok := strings.Cut(fault.ExceptionMessage(), "PDF_BYTES|")
					if !ok {
						t.Fatal("missing PDF observation")
					}
					data, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						t.Fatal(err)
					}
					var text strings.Builder
					for _, m := range regexp.MustCompile(`/Length (\d+)>>stream\n`).FindAllSubmatchIndex(data, -1) {
						n, _ := strconv.Atoi(string(data[m[2]:m[3]]))
						if n > len(data)-m[1] {
							t.Fatal("invalid stream length")
						}
						r, err := zlib.NewReader(bytes.NewReader(data[m[1] : m[1]+n]))
						if err != nil {
							t.Fatal(err)
						}
						content, err := io.ReadAll(r)
						r.Close()
						if err != nil {
							t.Fatal(err)
						}
						text.Write(content)
					}
					if !strings.Contains(text.String(), "(United States Canada)Tj") {
						t.Fatalf("project PDF option text=%q", text.String())
					}
				})
			}
		}
	}
}
