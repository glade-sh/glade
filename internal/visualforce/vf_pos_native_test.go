package visualforce_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// The controller-less ISPICKVAL and $Label.site.* families were captured with
// each family's gen.py and run.sh. The test writes the same pages and
// components from cases.tsv, so no generated project is checked in.
type posCase struct{ name, kind, markup string }

type posRow struct{ api, name, outcome, value string }

func posCases(t *testing.T, family string) []posCase {
	t.Helper()
	f, err := os.Open(filepath.Join(family, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	cases := make([]posCase, 0, len(records)-1)
	for _, r := range records[1:] {
		cases = append(cases, posCase{r[0], r[1], r[2]})
	}
	return cases
}

func posRows(t *testing.T, family, api string) []posRow {
	t.Helper()
	f, err := os.Open(filepath.Join(family, "api"+api[:2], "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]posRow, 0, len(records)-1)
	for _, r := range records[1:] {
		if r[0] != api {
			t.Fatalf("native API=%q, want %q", r[0], api)
		}
		rows = append(rows, posRow{r[0], r[1], r[2], r[3]})
	}
	return rows
}

// posProject writes the pages (and components) for the selected cases.
func posProject(t *testing.T, family, pagePrefix, compPrefix, api string, keep map[string]bool) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		target := filepath.Join(root, "force-app/main/default", rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"%s"}`, api)), 0644); err != nil {
		t.Fatal(err)
	}
	meta := func(kind, name string) string {
		return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+`<%s xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>%s</label></%s>`+"\n", kind, api, name, kind)
	}
	for i, c := range posCases(t, family) {
		if !keep[c.name] {
			continue
		}
		page := posPageName(pagePrefix, c.name, i)
		body := c.markup
		if c.kind == "comp" {
			comp := compPrefix + strings.ReplaceAll(c.name, "-", "_")
			write("components/"+comp+".component", "<apex:component>\n"+c.markup+"\n</apex:component>\n")
			write("components/"+comp+".component-meta.xml", meta("ApexComponent", comp))
			body = "<c:" + comp + "/>"
		}
		if c.kind == "raw" {
			// The case is the whole page (gen.py writes it as given).
			write("pages/"+page+".page", c.markup+"\n")
			write("pages/"+page+".page-meta.xml", meta("ApexPage", page))
			continue
		}
		pageAttrs := ""
		if c.kind == "post" {
			write("classes/PostCtl.cls", posPostController)
			write("classes/PostCtl.cls-meta.xml", fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`+"\n", api))
			pageAttrs = ` controller="PostCtl"`
		}
		if c.kind == "res" {
			// gen.py writes the same static resource for every "res" case.
			write("staticresources/vfr4res.resource", "x\n")
			write("staticresources/vfr4res.resource-meta.xml", `<?xml version="1.0" encoding="UTF-8"?>`+"\n"+`<StaticResource xmlns="http://soap.sforce.com/2006/04/metadata"><cacheControl>Public</cacheControl><contentType>text/plain</contentType></StaticResource>`+"\n")
		}
		if c.kind == "ctl" {
			// gen.py writes the same controller for every controller case.
			write("classes/EntCtl.cls", "public class EntCtl { public String s { get; set; } public String t { get; set; } public Boolean b { get; set; } public List<String> lst { get; set; } public List<SelectOption> opts { get; set; } public Account acc { get; set; } public Boolean fld; public String fs; public Boolean getFlag2() { return true; } public PageReference go() { return null; } }\n")
			write("classes/EntCtl.cls-meta.xml", fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`+"\n", api))
			pageAttrs = ` controller="EntCtl"`
		}
		write("pages/"+page+".page", `<apex:page`+pageAttrs+` showHeader="false" sidebar="false" standardStylesheets="false">`+"\n"+`<div data-case="`+c.name+`">`+body+"</div>\n</apex:page>\n")
		write("pages/"+page+".page-meta.xml", meta("ApexPage", page))
	}
	return root
}

// posPostController is the controller of the "post" cases; gen.py writes the same text.
const posPostController = "public class PostCtl { public class Row { public String v { get; set; } public Boolean c { get; set; } public String sel { get; set; } } public String hid { get; set; } public String s { get; set; } public String t { get; set; } public Boolean b { get; set; } public String sel { get; set; } public String mode { get; set; } public List<Row> rows { get; set; } public List<Contact> cons { get; set; } public String msg { get; set; } public List<SelectOption> opts { get; set; } public PostCtl() { rows = new List<Row>{ new Row(), new Row() }; cons = new List<Contact>{ new Contact(), new Contact() }; opts = new List<SelectOption>{ new SelectOption('a', 'A'), new SelectOption('b', 'B') }; } public PageReference save() { String r = ''; for (Row x : rows) { r += x.v + ',' + x.c + ',' + x.sel + ';'; } msg = 'hid=' + hid + '|s=' + s + '|t=' + t + '|b=' + b + '|sel=' + sel + '|mode=' + mode + '|rows=' + r; return null; } public PageReference save2() { String r = ''; for (Contact x : cons) { r += x.LastName + ',' + x.DoNotCall + ',' + x.HasOptedOutOfEmail + ',' + x.LeadSource + ',' + x.Description + ';'; } msg = 'hid=' + hid + '|cons=' + r + '|rows=' + rows[0].v + ',' + rows[1].v; return null; } }\n"

// posPageName follows each family's gen.py: names from the case, or the case
// index when pagePrefix ends in an underscore-free sequence marker.
func posPageName(prefix, name string, index int) string {
	if strings.HasPrefix(prefix, "SiteLbl_") {
		return fmt.Sprintf("%s%03d", prefix, index)
	}
	return prefix + strings.ReplaceAll(name, "-", "_")
}

var posTag = regexp.MustCompile(`<[^>]*>`)

func posText(s string) string { return strings.TrimSpace(posTag.ReplaceAllString(s, "")) }

func runPosFamily(t *testing.T, family, pagePrefix, compPrefix string, apis []string, gaps map[string]bool) {
	runPosFamilyView(t, family, pagePrefix, compPrefix, apis, gaps, nil)
}

// runPosFamilyView is runPosFamily with a family-specific reduction of Glade's
// markup for the "extract" rows; a nil view uses posExtract.
func runPosFamilyView(t *testing.T, family, pagePrefix, compPrefix string, apis []string, gaps map[string]bool, view func(name, rendered string) string) {
	for _, api := range apis {
		t.Run(api, func(t *testing.T) {
			rows := posRows(t, family, api)
			if want := len(posCases(t, family)); len(rows) != want {
				t.Fatalf("native rows=%d, cases=%d", len(rows), want)
			}
			index := map[string]int{}
			for i, c := range posCases(t, family) {
				index[c.name] = i
			}
			render := map[string]bool{}
			for _, row := range rows {
				if posRenderOutcome(row.outcome) {
					render[row.name] = true
				}
			}
			machine, _, vfIndex := ispickvalMachine(t, posProject(t, family, pagePrefix, compPrefix, api, render))
			for _, row := range rows {
				if !posRenderOutcome(row.outcome) || gaps[row.name] {
					continue
				}
				page := posPageName(pagePrefix, row.name, index[row.name])
				t.Run("render/"+row.name, func(t *testing.T) {
					for _, route := range []string{"RenderPage", "AnonymousGetContent"} {
						var rendered string
						if route == "RenderPage" {
							result, err := visualforce.RenderPage(visualforce.PageRenderRequest{VFIndex: vfIndex, Machine: machine, PageName: page})
							if err != nil {
								t.Fatalf("%s: %v", route, err)
							}
							if result.Error != nil {
								t.Fatalf("%s: %v", route, result.Error.Message)
							}
							rendered = result.HTML
						} else {
							program, err := vm.CompileAnonymous(fmt.Sprintf("System.debug(new PageReference('/apex/%s').getContent().toString());", page))
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.Execute(program)
							if err != nil {
								t.Fatalf("%s: %v", route, err)
							}
							if len(result.Debug) != 1 {
								t.Fatalf("%s: debug rows=%d", route, len(result.Debug))
							}
							rendered = result.Debug[0]
						}
						if view != nil {
							// Byte-exact family: compare the raw markup, not a re-serialized parse.
							inner, ok := posRawCaseHTML(rendered, row.name)
							if !ok {
								t.Fatalf("%s: no data-case %s in %q", route, row.name, rendered)
							}
							switch row.outcome {
							case "extract":
								inner = view(row.name, inner)
							case "form":
								inner = posConvertFormBody(inner)
							}
							if inner != row.value {
								t.Fatalf("%s: HTML=%q, native=%q", route, inner, row.value)
							}
							continue
						}
						got := outputTextCaseHTML(t, rendered)
						value, present := got[row.name]
						if !present || len(got) != 1 {
							t.Fatalf("%s: HTML=%q", route, got)
						}
						want := outputTextCaseHTML(t, `<div data-case="expected">`+row.value+`</div>`)["expected"]
						value = posOutcomeView(row.outcome, row.name, value)
						if row.outcome == "extract" {
							// Both sides compare as parsed text, like every other row.
							value = outputTextCaseHTML(t, `<div data-case="expected">`+value+`</div>`)["expected"]
						}
						if strings.HasPrefix(row.name, "comp-") {
							// Component output sits in a generated span with an id and
							// whitespace; the family checks the text, not the wrapper.
							value, want = posText(value), posText(want)
						}
						if value != want {
							t.Fatalf("%s: HTML=%q, native=%q", route, value, want)
						}
					}
				})
			}
			for _, row := range rows {
				if row.outcome != "error" || gaps[row.name] {
					continue
				}
				t.Run("error/"+row.name, func(t *testing.T) {
					root := posProject(t, family, pagePrefix, compPrefix, api, map[string]bool{row.name: true})
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					_, err = visualforce.LoadProject(p)
					if err == nil {
						t.Fatalf("LoadProject succeeded, native error=%q", row.value)
					}
					if !strings.Contains(err.Error(), row.value) {
						t.Fatalf("error=%q, native=%q", err.Error(), row.value)
					}
				})
			}
		})
	}
}

func TestVisualforceControllerlessISPICKVALNativeRows(t *testing.T) {
	runPosFamily(t, "testdata/vf_ispickval_nocontroller", "PickNC_", "PickNCComp_", []string{"59.0", "62.0", "67.0", "68.0"}, nil)
}

func TestVisualforceSiteLabelsNativeRows(t *testing.T) {
	runPosFamily(t, "testdata/vf_site_labels", "SiteLbl_", "", []string{"59.0", "62.0", "67.0", "68.0"}, nil)
}

// posRenderOutcome reports whether a native row records rendered output. The
// "form" rows hold a page's controls without the form wrapper, hidden field,
// script and view state; the "extract" rows hold only the text named in
// posExtract.
func posRenderOutcome(outcome string) bool {
	return outcome == "render" || outcome == "form" || outcome == "extract"
}

var (
	posFormBody    = regexp.MustCompile(`(?s)<form[^>]*>(.*)</form>`)
	posHiddenInput = regexp.MustCompile(`<input type="hidden"[^>]*>\s*`)
	posScriptTag   = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>\s*`)
	posTrailingDiv = regexp.MustCompile(`<div id="[^"]*"></div>\s*$`)
	// posExtract mirrors EXTRACT in testdata/vf_entity_expressions/convert.py.
	posExtractAttrs = []*regexp.Regexp{regexp.MustCompile(`(?s)\stitle="([^"]*)"`), regexp.MustCompile(`(?s)\sstyle="([^"]*)"`), regexp.MustCompile(`(?s)\sclass="([^"]*)"`)}
	posExtractLabel = []*regexp.Regexp{regexp.MustCompile(`(?s)<label[^>]*>\s*(.*?)</label>`)}
	posExtractBtn   = `<input type="submit"[^>]*?\s`
	posExtractLink  = `<a href="#"[^>]*?\s`
	posExtractOpts  = []*regexp.Regexp{regexp.MustCompile(`(?s)<option value="([^"]*)"`), regexp.MustCompile(`(?s)<option[^>]*>(.*?)</option>`)}
	posExtract      = map[string][]*regexp.Regexp{
		"PageBlockTitle": {regexp.MustCompile(`(?s)<h2 class="mainTitle">(.*?)</h2>`)},
		"LabelAmp":       posExtractLabel, "LabelAmpEscF": posExtractLabel, "LabelLit": posExtractLabel,
		"ButtonValue": {regexp.MustCompile(`(?s)` + posExtractBtn + `value="([^"]*)"`)},
		"ButtonNbsp":  {regexp.MustCompile(`(?s)` + posExtractBtn + `value="([^"]*)"`)},
		"ButtonTitle": {regexp.MustCompile(`(?s)` + posExtractBtn + `value="([^"]*)"`), regexp.MustCompile(`(?s)` + posExtractBtn + `title="([^"]*)"`)},
		"ButtonLit":   {regexp.MustCompile(`(?s)` + posExtractBtn + `value="([^"]*)"`), regexp.MustCompile(`(?s)` + posExtractBtn + `title="([^"]*)"`)},
		"LinkValue":   {regexp.MustCompile(`(?s)<a href="#"[^>]*>(.*?)</a>`)},
		"LinkTitle":   {regexp.MustCompile(`(?s)<a href="#"[^>]*>(.*?)</a>`), regexp.MustCompile(`(?s)` + posExtractLink + `title="([^"]*)"`), regexp.MustCompile(`(?s)` + posExtractLink + `style="([^"]*)"`)},
		"CtlSelect":   posExtractOpts, "CtlSelectLit": posExtractOpts,
		"StyleTitleOT": posExtractAttrs, "PanelTitle": posExtractAttrs, "PanelLit": posExtractAttrs[:1],
	}
)

// posOutcomeView reduces Glade's rendered div to the part a native row holds.
func posOutcomeView(outcome, name, rendered string) string {
	switch outcome {
	case "form":
		m := posFormBody.FindStringSubmatch(rendered)
		if m == nil {
			return rendered
		}
		body := posHiddenInput.ReplaceAllString(m[1], "")
		body = posScriptTag.ReplaceAllString(body, "")
		return strings.TrimSpace(posTrailingDiv.ReplaceAllString(body, ""))
	case "extract":
		var found []string
		for _, pattern := range posExtract[name] {
			for _, m := range pattern.FindAllStringSubmatch(rendered, -1) {
				found = append(found, m[1])
			}
		}
		return strings.Join(found, "|")
	}
	return rendered
}
