package visualforce_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

// Synthetic pages for component markup captured with gen.py/run.sh: input
// controls with title, styleClass and style, literal values on input
// components, outputLabel, pageMessage, sectionHeader, outputLink and image URL
// attributes, and actionStatus text. Rows compare raw markup: the "render" rows
// are byte-exact, the "extract" rows hold the values convert.py names.
func TestVisualforceRenderMarkupNativeRows(t *testing.T) {
	runPosFamilyView(t, "testdata/vf_render_markup", "", "", []string{"59.0", "62.0", "67.0", "68.0"}, posRenderMarkupKnownDifferences, posRenderMarkupView)
}

// posRenderMarkupKnownDifferences names the captured rows this test does not
// assert, each with the difference that keeps it out. The native values stay in
// org.tsv.
var posRenderMarkupKnownDifferences = map[string]bool{
	// A commandButton and an actionFunction with a literal id: Glade's own
	// postback markup (data-action, onclick, its submit script) stands where
	// the org writes jsfcljs and the SfdcApp form scripts.
	"IdLCommandButton": true, "IdLActionFunction": true,
	// A dataTable: Glade writes a reduced table (class, thead, no colgroup or
	// tbody id); the dataTable markup has no capture beyond this empty table.
	"IdLDataTable": true,
	// A supplementary character in a static resource path: the org writes "&"
	// and drops the rest of the path; one sample gives no rule.
	"LinkUniResourceEmoji": true,
	// An inputCheckbox bound to a Contact checkbox field (DoNotCall,
	// HasOptedOutOfEmail) inside apex:repeat: the org writes no control at all;
	// Glade writes the checkbox. The cause is not isolated.
	"RepConCheck": true, "RepConCheckId": true, "RepConAll": true, "RepConCheckEmail": true, "RepConCheckEmailId": true,
	// The diagnostic column counts each & of the line as five characters
	// (column 108 where the tag ends at 100); one sample does not say whether
	// the count runs over the tag or the line.
	"IdLInputHiddenQuote": true,
}

// posRenderMarkupExceptionRows are the "exception" rows this family asserts:
// pages that compile and fail when they render. Both routes must raise the
// native message.
var posRenderMarkupExceptionRows = map[string]bool{
	"PageMsgStrengthEmpty": true, "PageMsgStrengthEmptyDet": true, "SeqPageMsgStrengthEmpty": true,
}

func TestVisualforceRenderMarkupNativeExceptions(t *testing.T) {
	const family = "testdata/vf_render_markup"
	for _, api := range []string{"59.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			keep := map[string]bool{}
			var rows []posRow
			for _, row := range posRows(t, family, api) {
				if row.outcome == "exception" && posRenderMarkupExceptionRows[row.name] {
					keep[row.name] = true
					rows = append(rows, row)
				}
			}
			if len(rows) != len(posRenderMarkupExceptionRows) {
				t.Fatalf("exception rows=%d, want %d", len(rows), len(posRenderMarkupExceptionRows))
			}
			machine, _, vfIndex := ispickvalMachine(t, posProject(t, family, "", "", api, keep))
			for _, row := range rows {
				want := strings.TrimPrefix(row.value, "System.VisualforceException: ")
				t.Run(row.name, func(t *testing.T) {
					result, err := visualforce.RenderPage(visualforce.PageRenderRequest{VFIndex: vfIndex, Machine: machine, PageName: row.name})
					switch {
					case err != nil:
						if !strings.Contains(err.Error(), want) {
							t.Fatalf("RenderPage error=%q, native=%q", err, want)
						}
					case result.Error != nil:
						if !strings.Contains(result.Error.Message, want) {
							t.Fatalf("RenderPage error=%q, native=%q", result.Error.Message, want)
						}
					default:
						t.Fatalf("RenderPage rendered, native raises %q", want)
					}
					program, err := vm.CompileAnonymous(fmt.Sprintf("System.debug(new PageReference('/apex/%s').getContent().toString());", row.name))
					if err != nil {
						t.Fatal(err)
					}
					if _, err = machine.Execute(program); err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("getContent error=%v, native=%q", err, want)
					}
				})
			}
		})
	}
}

var posViewControlName = regexp.MustCompile(`<(input|textarea|select)\b([^>]*)>`)

// posPostbackKnownDifferences are postback rows this test does not assert.
var posPostbackKnownDifferences = map[string]string{
	"RepConCheck": posNoCheckboxControl, "RepConCheckId": posNoCheckboxControl, "RepConAll": posNoCheckboxControl,
	"RepConCheckEmail": posNoCheckboxControl, "RepConCheckEmailId": posNoCheckboxControl,
}

const posNoCheckboxControl = "the org writes no control for an inputCheckbox bound to a Contact checkbox field inside apex:repeat; Glade writes one"

// TestVisualforceRenderMarkupNativePostback posts the control names the org
// rendered (postback.tsv: every control set, the Save button pressed, the
// controller action's message read back) to Glade's page and expects the same
// message: an input posted under its client ID still binds.
func TestVisualforceRenderMarkupNativePostback(t *testing.T) {
	const family = "testdata/vf_render_markup"
	f, err := os.Open(filepath.Join(family, "postback.tsv"))
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
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			keep := map[string]bool{}
			var rows [][]string
			for _, r := range records[1:] {
				if r[0] == api && !strings.HasPrefix(r[1], "PostRerender") && posPostbackKnownDifferences[r[1]] == "" {
					keep[r[1]] = true
					rows = append(rows, r)
				}
			}
			if len(rows) == 0 {
				t.Fatal("no postback rows")
			}
			machine, _, vfIndex := ispickvalMachine(t, posProject(t, family, "", "", api, keep))
			// The Save button's action: save, or save2 for the Contact rows.
			actions := map[string]string{}
			for _, c := range posCases(t, family) {
				actions[c.name] = "{!save}"
				if strings.Contains(c.markup, `action="{!save2}"`) {
					actions[c.name] = "{!save2}"
				}
			}
			for _, r := range rows {
				name, posted, want := r[1], r[2], r[3]
				t.Run(name, func(t *testing.T) {
					values := map[string]string{}
					var nativeNames []string
					for _, pair := range strings.Split(posted, ";") {
						k, v, _ := strings.Cut(pair, "=")
						values[k] = v
						nativeNames = append(nativeNames, k)
					}
					page, err := visualforce.RenderPage(visualforce.PageRenderRequest{VFIndex: vfIndex, Machine: machine, PageName: name})
					if err != nil {
						t.Fatal(err)
					}
					// The names Glade renders are the names the org posted.
					seen := map[string]bool{}
					var glade []string
					for _, m := range posViewControlName.FindAllStringSubmatch(page.HTML, -1) {
						attrs := map[string]string{}
						for _, a := range posViewAttrPattern.FindAllStringSubmatch(m[2], -1) {
							if _, ok := attrs[a[1]]; !ok {
								attrs[a[1]] = a[2]
							}
						}
						n := attrs["name"]
						if n == "" || strings.HasPrefix(n, "__") || strings.HasPrefix(n, "com.salesforce") || attrs["type"] == "submit" || (attrs["type"] == "hidden" && attrs["value"] == n) || seen[n] {
							continue
						}
						if _, ok := values[n]; !ok && attrs["type"] == "hidden" {
							continue // the checkbox companion and the sectionHeader fields
						}
						seen[n] = true
						glade = append(glade, n)
					}
					if name == "PstTextTwoForms" {
						// The org posts one form; Glade renders both, with the posted names among them.
						if !seen[nativeNames[0]] {
							t.Fatalf("rendered names %q lack %q", glade, nativeNames)
						}
					} else if strings.Join(glade, ";") != strings.Join(nativeNames, ";") {
						t.Fatalf("rendered names %q, native %q", glade, nativeNames)
					}
					result, err := visualforce.RenderPage(visualforce.PageRenderRequest{
						VFIndex: vfIndex, Machine: machine, PageName: name, Action: actions[name], FormValues: values,
					})
					if err != nil {
						t.Fatal(err)
					}
					if result.Error != nil {
						t.Fatal(result.Error.Message)
					}
					if !strings.Contains(result.HTML, want) {
						got := result.HTML
						if i := strings.Index(got, "hid="); i >= 0 {
							got = got[i:]
							if j := strings.Index(got, "<"); j >= 0 {
								got = got[:j]
							}
						}
						t.Fatalf("message %q, Glade %q", want, got)
					}
				})
			}
		})
	}
}

// posRawCaseHTML returns the raw inner HTML of the div whose data-case is name.
func posRawCaseHTML(page, name string) (string, bool) {
	z := html.NewTokenizer(strings.NewReader(page))
	offset, depth, start := 0, 0, -1
	for {
		kind := z.Next()
		raw := z.Raw()
		if kind == html.ErrorToken {
			return "", false
		}
		next := offset + len(raw)
		switch kind {
		case html.StartTagToken:
			tag, hasAttr := z.TagName()
			if string(tag) != "div" {
				break
			}
			if start < 0 {
				for hasAttr {
					var key, value []byte
					key, value, hasAttr = z.TagAttr()
					if string(key) == "data-case" && string(value) == name {
						start, depth = next, 1
					}
				}
			} else {
				depth++
			}
		case html.EndTagToken:
			tag, _ := z.TagName()
			if start >= 0 && string(tag) == "div" {
				depth--
				if depth == 0 {
					return page[start:offset], true
				}
			}
		}
		offset = next
	}
}

var (
	posViewTagPattern  = regexp.MustCompile(`<(input|textarea|select|label|a|img)\b([^>]*)>`)
	posViewAttrPattern = regexp.MustCompile(`\s([A-Za-z-]+)="([^"]*)"`)
	posViewHeader      = []*regexp.Regexp{regexp.MustCompile(`(?s)(<div class="bPageTitle">.*)$`)}
	posViewHeaderBlock = []*regexp.Regexp{regexp.MustCompile(`(?s)(<div class="bPageTitle">.*?<div class="blank"></div></div></div></div>)`)}
	posViewHeaderState = []*regexp.Regexp{regexp.MustCompile(`(?s)(<div class="bPageTitle">.*?name="[^"]*:rlx" />)`)}
	posViewApexp       = []*regexp.Regexp{regexp.MustCompile(`(?s)(<div class="apexp">.*)$`)}
	posViewMessage     = []*regexp.Regexp{regexp.MustCompile(`(?s)(<span id="[^"]*">\s*<div class="message.*?</div></span>)`)}
	// Glade writes the action status text in classed spans; the org writes ids.
	posViewRelativeHref = regexp.MustCompile(`(^|[|;])href=/([^/]|$)`)
	posViewRelativeSrc  = regexp.MustCompile(`(^|[|;])src=/([^/]|$)`)
	posViewSpanTag      = regexp.MustCompile(`<span\b([^>]*)>`)
	posViewStatus       = []*regexp.Regexp{
		regexp.MustCompile(`(?s)<span class="actionStatusStart[ "][^>]*>(.*?)</span>`),
		regexp.MustCompile(`(?s)<span class="actionStatusStop[ "][^>]*>(.*?)</span>`),
	}
)

type posViewTag struct {
	tag, typ string
	names    []string
}

// posRenderMarkupView mirrors kind_of and apply in
// testdata/vf_render_markup/convert.py.
func posRenderMarkupView(name, rendered string) string {
	var specs []posViewTag
	var texts []*regexp.Regexp
	switch {
	case strings.HasPrefix(name, "Unb"):
		specs = []posViewTag{{"controls", "", []string{"id", "name"}}}
	case name == "SHFFormInput":
		texts = posViewHeaderState
	case strings.HasPrefix(name, "SHF"):
		texts = posViewHeader
	case name == "SHCBlock" || name == "SHCFormBlock":
		texts = posViewApexp
	case strings.HasPrefix(name, "PostRerender"):
		texts = posViewHeaderState
	case strings.HasPrefix(name, "Hid"):
		specs = []posViewTag{{"named", "", []string{"type", "id", "name"}}}
	case strings.HasPrefix(name, "Rep") || strings.HasPrefix(name, "Pst"):
		specs = []posViewTag{{"controls", "", []string{"id", "name"}}}
	case strings.HasPrefix(name, "InputTextarea"):
		specs = []posViewTag{{"textarea", "", []string{"id", "name", "rows", "cols", "class", "style", "title"}}}
	case strings.HasPrefix(name, "InputText"):
		specs = []posViewTag{{"input", "text", []string{"id", "name", "maxlength", "size", "disabled", "class", "style", "title"}}}
	case strings.HasPrefix(name, "InputSecret"):
		specs = []posViewTag{{"input", "password", []string{"id", "name", "class", "style", "title"}}}
	case strings.HasPrefix(name, "InputCheckbox"):
		specs = []posViewTag{{"input", "checkbox", []string{"id", "name", "class", "style", "title"}}}
	case strings.HasPrefix(name, "SelectList"):
		specs = []posViewTag{{"select", "", []string{"id", "name", "size", "multiple", "class", "style", "title"}}}
	case strings.HasPrefix(name, "Link"):
		specs = []posViewTag{{"a", "", []string{"id", "name", "href", "style", "title", "target", "class"}}}
	case strings.HasPrefix(name, "Image"):
		specs = []posViewTag{{"img", "", []string{"id", "src", "alt", "height", "style", "title", "width", "class"}}}
	case strings.HasPrefix(name, "Seq"):
		specs = []posViewTag{{"controls", "", []string{"name"}}}
	case name == "OutLabelIdFormFor":
		specs = []posViewTag{{"label", "", []string{"id", "for"}}, {"input", "text", []string{"id"}}}
	case name == "OutLabelIdForm" || name == "OutLabelIdBlock" || name == "OutLabelForMissingForm" || name == "OutLabelForMissingFormNoId":
		specs = []posViewTag{{"label", "", []string{"id", "for"}}}
	case strings.HasPrefix(name, "StatusSpan"):
		return posStatusSpanView(rendered)
	case strings.HasPrefix(name, "SecHdrId"):
		texts = posViewHeaderBlock
	case name == "LabelForInput":
		specs = []posViewTag{{"label", "", []string{"for"}}, {"input", "text", []string{"id", "name"}}}
	case strings.HasPrefix(name, "SecHdr") || name == "PosSecHdrSecond":
		texts = posViewHeader
	case name == "PosPageMsgForm" || name == "PosPageMsgBlock":
		texts = posViewMessage
	case strings.HasPrefix(name, "ActStat") || strings.HasPrefix(name, "PosActStat"):
		texts = posViewStatus
	default:
		return rendered
	}
	if texts != nil {
		if strings.HasPrefix(name, "SHF") || strings.HasPrefix(name, "SHC") {
			// The form's own state fields and closing tag follow the content;
			// convert.py drops them from the native side.
			for _, marker := range []string{"\n<input type=\"hidden\" name=\"com.salesforce.visualforce.ViewState\"", "\n<div><span></span><span></span></div></form>", "\n</form>"} {
				if i := strings.Index(rendered, marker); i >= 0 {
					rendered = rendered[:i]
				}
			}
		}
		var found []string
		for _, pattern := range texts {
			for _, m := range pattern.FindAllStringSubmatch(rendered, -1) {
				found = append(found, m[1])
			}
		}
		return strings.Join(found, "|")
	}
	// convert.py reads the controls of the page's first form only.
	if m := posConvertFirstForm.FindStringSubmatch(rendered); m != nil {
		rendered = m[1]
	}
	var out []string
	for _, spec := range specs {
		for _, m := range posViewTagPattern.FindAllStringSubmatch(rendered, -1) {
			values := map[string]string{}
			for _, a := range posViewAttrPattern.FindAllStringSubmatch(m[2], -1) {
				if _, seen := values[a[1]]; !seen {
					values[a[1]] = a[2]
				}
			}
			if spec.tag == "named" {
				if m[1] != "input" && m[1] != "textarea" && m[1] != "select" {
					continue
				}
				if values["type"] == "submit" || values["type"] == "button" {
					continue
				}
				if values["type"] == "hidden" && (strings.HasPrefix(values["name"], "__") || strings.HasPrefix(values["name"], "com.salesforce") || values["value"] == values["name"]) {
					continue
				}
				if m[1] == "input" && values["type"] == "" {
					values["type"] = "text"
				}
			} else if spec.tag == "controls" {
				if m[1] != "input" && m[1] != "textarea" && m[1] != "select" {
					continue
				}
				if m[1] == "input" && (values["type"] == "hidden" || values["type"] == "submit") {
					continue
				}
			} else {
				if m[1] != spec.tag || (spec.typ != "" && values["type"] != spec.typ) {
					continue
				}
			}
			var pairs []string
			for _, n := range spec.names {
				if v, ok := values[n]; ok {
					pairs = append(pairs, n+"="+v)
				}
			}
			out = append(out, strings.Join(pairs, "|"))
		}
	}
	joined := strings.Join(out, ";")
	if strings.HasPrefix(name, "Link") {
		// The org writes its own host before a server-relative outputLink URL
		// (convert.py keeps {ORGHOST}); Glade has no org host in the rendered
		// page and leaves the URL relative, so the view adds the placeholder.
		joined = posViewRelativeHref.ReplaceAllString(joined, "${1}href={ORGHOST}/${2}")
	}
	if strings.HasPrefix(name, "ImageUniUrlfor") {
		// URLFOR of a server-relative literal target returns the org's own
		// absolute URL, which an image keeps (convert.py: {ORGHOST}); Glade
		// returns the path.
		joined = posViewRelativeSrc.ReplaceAllString(joined, "${1}src={ORGHOST}/${2}")
	}
	if strings.HasPrefix(name, "Link") || strings.HasPrefix(name, "Image") {
		// A static resource URL carries the org's cache timestamp
		// (convert.py: {TS}); Glade's has none.
		joined = strings.ReplaceAll(joined, "/resource/", "/resource/{TS}/")
	}
	return joined
}

var (
	posConvertFirstForm   = regexp.MustCompile(`(?s)<form [^>]*>(.*?)</form>`)
	posConvertStateInput  = regexp.MustCompile(`<input type="hidden" name="[^"]*" value="[^"]*" />\s*`)
	posConvertFormScript  = regexp.MustCompile(`(?s)<script type="text/javascript" language="Javascript">.*?</script>\s*`)
	posConvertTrailingDiv = regexp.MustCompile(`<div id="[^"]*"></div>\s*$`)
)

// posConvertFormBody mirrors the "form" outcome of convert.py: the first
// form's content without the state fields of the form (hidden inputs written
// as type, name, value), its submit script and a trailing empty div.
func posConvertFormBody(rendered string) string {
	m := posConvertFirstForm.FindStringSubmatch(rendered)
	if m == nil {
		return rendered
	}
	body := posConvertStateInput.ReplaceAllString(m[1], "")
	body = posConvertFormScript.ReplaceAllString(body, "")
	return strings.TrimSpace(posConvertTrailingDiv.ReplaceAllString(body, ""))
}

// posStatusSpanView mirrors the "spans" rule of convert.py: the start and stop
// spans with the style (the org's display: none removed) and class the page
// wrote. Glade marks them with the actionStatusStart and actionStatusStop
// classes; the marker is not part of the page's class.
func posStatusSpanView(rendered string) string {
	var out []string
	for _, m := range posViewSpanTag.FindAllStringSubmatch(rendered, -1) {
		values := map[string]string{}
		for _, a := range posViewAttrPattern.FindAllStringSubmatch(m[1], -1) {
			if _, seen := values[a[1]]; !seen {
				values[a[1]] = a[2]
			}
		}
		which := ""
		classes := strings.Fields(values["class"])
		var kept []string
		for _, c := range classes {
			switch c {
			case "actionStatusStart":
				which = "start"
			case "actionStatusStop":
				which = "stop"
			default:
				kept = append(kept, c)
			}
		}
		if which == "" {
			continue
		}
		parts := []string{which}
		if style := values["style"]; style != "" {
			parts = append(parts, "style="+style)
		}
		if len(kept) > 0 {
			parts = append(parts, "class="+strings.Join(kept, " "))
		}
		out = append(out, strings.Join(parts, "|"))
	}
	return strings.Join(out, ";")
}
