package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v07Case struct {
	ID                 string                     `json:"id"`
	Group              string                     `json:"group"`
	Kind               string                     `json:"kind"`
	Postback           bool                       `json:"postback"`
	Submitted          []string                   `json:"submitted"`
	Repeat             bool                       `json:"repeat"`
	Inputs             map[string]v10Input        `json:"inputs"`
	Expected           map[string]string          `json:"expected"`
	Diagnostics        map[string][]v10Diagnostic `json:"diagnostics"`
	Owner              string                     `json:"owner,omitempty"`
	Reason             string                     `json:"reason,omitempty"`
	PendingObservation *v07PendingObservation     `json:"pending_observation,omitempty"`
}

type v07PendingObservation struct {
	Boundary string `json:"boundary"`
	Reason   string `json:"reason"`
}

// TestV07SalesforceConformance compares all 142 compile/metadata and 122 DOM
// answers at API 59/67 as exact text, including null and diagnostic messages.
// Compile uses the form-lifecycle runner; runtime uses the same product request, fresh VM,
// encoded view state and CSRF path. Only browser DOM observation and successful
// control serialization live in this adapter. CI needs no org, tools or browser.
// GLADE_V07_CAPTURE=1 reports mismatches without failing; GLADE_V07_REPORT sets
// the per-row TSV path. Structural fixture errors still fail in capture mode.
func TestV07SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V07_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v07_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source string    `json:"source"`
		Cases  []v07Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned select-control observations at API 59.0 and 67.0." || len(table.Cases) != 264 {
		t.Fatalf("Select controls requires all 264 finalized native cases, got %d", len(table.Cases))
	}
	versions := []string{"59.0", "67.0"}
	seen, counts := map[string]bool{}, map[string]int{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate Select controls case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		if (c.Owner == "") != (c.Reason == "") || strings.Contains(c.Owner, "Select controls") {
			t.Fatalf("%s: invalid carry owner/reason", c.ID)
		}
		if pending := c.PendingObservation; pending != nil {
			if c.Kind != "runtime" || c.Owner != "" || pending.Boundary != "Select option rendering" || pending.Reason == "" {
				t.Fatalf("%s: invalid pending native observation", c.ID)
			}
		}
		counts[c.Kind]++
		for _, api := range versions {
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) != 5 || input.Files["sfdx-project.json"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete captured input at API %s", c.ID, api)
			}
			answer, ok := c.Expected[api]
			if !ok {
				t.Fatalf("%s: missing native answer at API %s", c.ID, api)
			}
			diagnostics, ok := c.Diagnostics[api]
			if !ok || diagnostics == nil {
				t.Fatalf("%s: missing native diagnostic list at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				if (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") ||
					(answer == "COMPILE_OK" && len(diagnostics) != 0) ||
					(answer == "COMPILE_ERROR" && len(diagnostics) == 0) {
					t.Fatalf("%s: inconsistent native compile answer at API %s", c.ID, api)
				}
			} else if !strings.HasPrefix(answer, "DOM|") || !json.Valid([]byte(strings.TrimPrefix(answer, "DOM|"))) || len(diagnostics) != 0 {
				t.Fatalf("%s: invalid native DOM answer at API %s", c.ID, api)
			}
			if c.PendingObservation != nil && answer != `DOM|{"nativeRenderObservation":["BOUNDED_NO_DATA_20_SECONDS"],"stage":"render"}` {
				t.Fatalf("%s: pending observation requires the captured bounded no-data answer at API %s", c.ID, api)
			}
		}
	}
	if counts["compile"] != 142 || counts["runtime"] != 122 {
		t.Fatalf("Select controls requires 142 compile and 122 DOM cases, got %v", counts)
	}

	var report strings.Builder
	tsv := csv.NewWriter(&report)
	tsv.Comma = '\t'
	write := func(fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
	}
	write("api", "id", "status", "actual", "expected", "group", "resource", "diagnostic", "owner", "reason", "acceptance_status", "diagnostic_status", "expected_diagnostic")
	matches, compileMatches, acceptanceMatches, diagnosticMatches, domMatches := 0, 0, 0, 0, 0
	for _, api := range versions {
		apiMatches, apiCompile, apiAcceptance, apiDiagnostic, apiDOM := 0, 0, 0, 0, 0
		for _, c := range table.Cases {
			// Release each row's fixture promptly instead of retaining 528 roots.
			t.Run(api+"/"+c.ID, func(t *testing.T) {
				root := t.TempDir()
				input := c.Inputs[api]
				for path, source := range input.Files {
					if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
						t.Fatalf("invalid fixture path %q", path)
					}
					fullPath := filepath.Join(root, path)
					if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				got, diagnostics := v07Execute(t, root, input.Name, c)
				want := c.Expected[api]
				actualDiagnostic := v10Encode(t, diagnostics)
				expectedDiagnostic := v10Encode(t, c.Diagnostics[api])
				acceptanceMatch := got == want
				diagnosticMatch := c.Kind != "compile" || actualDiagnostic == expectedDiagnostic
				status, acceptanceStatus, diagnosticStatus := "MISMATCH", "MISMATCH", "NOT_APPLICABLE"
				if acceptanceMatch {
					acceptanceStatus = "MATCH"
					if c.Kind == "compile" {
						apiAcceptance++
					}
				}
				if c.Kind == "compile" {
					diagnosticStatus = "MISMATCH"
					if diagnosticMatch {
						diagnosticStatus = "MATCH"
						apiDiagnostic++
					}
				}
				if acceptanceMatch && diagnosticMatch && c.PendingObservation == nil {
					status = "MATCH"
					apiMatches++
					if c.Kind == "compile" {
						apiCompile++
					} else {
						apiDOM++
					}
				}
				owner, reason := "", ""
				if pending := c.PendingObservation; pending != nil {
					status = "UNOBSERVABLE"
					owner, reason = pending.Boundary, pending.Reason
					t.Logf("pending native observation owned by %s: %s", owner, reason)
				}
				if (!acceptanceMatch || !diagnosticMatch) && c.Owner != "" {
					owner, reason = c.Owner, c.Reason
					t.Logf("carry %s: %s", owner, reason)
				}
				write(api, c.ID, status, got, want, c.Group, "ApexPage", strings.ReplaceAll(actualDiagnostic, root, "<fixture>"), owner, reason, acceptanceStatus, diagnosticStatus, expectedDiagnostic)
				if !capture && owner == "" && (!acceptanceMatch || !diagnosticMatch) {
					t.Errorf("actual <%s> expected <%s>; diagnostics <%s> expected <%s>", got, want, actualDiagnostic, expectedDiagnostic)
				}
			})
		}
		write("API_TOTAL", api, fmt.Sprintf("%d/264", apiMatches))
		write("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/142", apiCompile))
		write("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/142", apiAcceptance))
		write("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/142", apiDiagnostic))
		write("DOM_API_TOTAL", api, fmt.Sprintf("%d/122", apiDOM))
		t.Logf("Select controls API %s exact %d/264; compile %d/142, acceptance %d/142, diagnostics %d/142; DOM %d/122", api, apiMatches, apiCompile, apiAcceptance, apiDiagnostic, apiDOM)
		matches += apiMatches
		compileMatches += apiCompile
		acceptanceMatches += apiAcceptance
		diagnosticMatches += apiDiagnostic
		domMatches += apiDOM
	}
	write("COMPILE_TOTAL", fmt.Sprintf("%d/284", compileMatches))
	write("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/284", acceptanceMatches))
	write("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/284", diagnosticMatches))
	write("DOM_TOTAL", fmt.Sprintf("%d/244", domMatches))
	write("TOTAL", fmt.Sprintf("%d/528", matches))
	t.Logf("Select controls exact matches %d/528; compile %d/284; DOM %d/244", matches, compileMatches, domMatches)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V07_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("required_ID_controls", v07RequiredIDSalesforceConformance)
}

func v07Execute(t *testing.T, root, name string, c v07Case) (string, []v10Diagnostic) {
	t.Helper()
	if c.Kind == "compile" {
		return v10Execute(t, root, name, v10Case{Kind: "compile"})
	}
	diagnostics := []v10Diagnostic{}
	fail := func(err error) (string, []v10Diagnostic) {
		return v07DOM(t, map[string]any{"localError": err.Error(), "stage": "render"}), diagnostics
	}
	p, err := project.Load(root)
	if err != nil {
		return fail(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		return fail(err)
	}
	index := typesys.Build(p, sch)
	vf, err := visualforce.LoadProjectForRender(p)
	if err != nil {
		return fail(err)
	}
	org := storage.NewOrgState()
	request := visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, PageName: name, PageURL: "/apex/" + name, ViewStateSecret: []byte("Select controls owned conformance view-state secret")}
	render := func() (*html.Node, error) {
		machine := vm.New(nil)
		machine.Org = &org
		if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
			return nil, err
		}
		request.Machine = machine
		result, err := visualforce.RenderPage(request)
		if err == nil && result.Error != nil {
			err = result.Error
		}
		if err != nil {
			return nil, err
		}
		if result.RedirectURL != "" {
			return nil, fmt.Errorf("unexpected local redirect: %s", result.RedirectURL)
		}
		// Current native browsers retain phrasing children inside options. Parse
		// select in ordinary element mode so x/net/html's legacy select insertion
		// mode does not discard the observed unescaped <b> label markup.
		doc, err := html.Parse(strings.NewReader(v07SelectTag.ReplaceAllString(result.HTML, "${1}v07-select")))
		if err == nil {
			v10Walk(doc, func(n *html.Node) {
				if n.Data == "v07-select" {
					n.Data = "select"
				}
			})
			v07InitializeControls(doc)
		}
		return doc, err
	}
	doc, err := render()
	if err != nil {
		return fail(err)
	}
	answer := map[string]any{"before": v07Observe(t, doc, c.ID)}
	if !c.Postback {
		return v07DOM(t, answer), diagnostics
	}
	row := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "data-v07-case") == c.ID })
	answer["transition"] = v07SetSelection(row, c.Submitted)
	after := []any{}
	submissions := 1
	if c.Repeat {
		submissions = 2
	}
	for submission := 1; submission <= submissions; submission++ {
		button := v10Find(row, func(n *html.Node) bool { return n.Data == "input" && v10Attr(n, "value") == "V07_SUBMIT" })
		form := v10Ancestor(button, func(n *html.Node) bool { return n.Data == "form" })
		if form == nil {
			after = append(after, map[string]any{"localError": "rendered submit form missing"})
			break
		}
		values := v07FormValues(form, button)
		action := v10Attr(button, "data-action")
		if action == "" {
			if match := v10ActionHook.FindStringSubmatch(v10Attr(button, "onclick")); len(match) == 2 {
				action = match[1]
			}
		}
		values[visualforce.ViewStateActionFieldName()] = action
		payload, err := visualforce.DecodeViewState(values[visualforce.ViewStateFormFieldName()], request.ViewStateSecret)
		if err == nil {
			err = visualforce.VerifyViewStateCSRF(payload, values["__vf_csrf"])
		}
		if err == nil {
			parsed := visualforce.ParseAjaxPayload(values)
			request.ViewState, request.FormValues, request.Action = &payload, parsed.SubmittedFields, parsed.Action
			request.PageURL = v10Attr(form, "action")
			doc, err = render()
		}
		if err != nil {
			after = append(after, map[string]any{"localError": err.Error()})
			break
		}
		observation := v07Observe(t, doc, c.ID)
		if result, ok := observation["result"].(map[string]any); ok {
			if posts, ok := result["posts"].(float64); ok && posts < float64(submission) {
				// The native postback acknowledgement waits for the action counter
				// and returns sorted unique visible error lines when validation
				// blocks it. Observe the actual rendered text, not model assertions.
				seen := map[string]bool{}
				v10Walk(doc, func(n *html.Node) {
					if n.Type == html.TextNode && strings.Contains(n.Data, "Validation Error") {
						seen[strings.TrimSpace(n.Data)] = true
					}
				})
				lines := []string{}
				for line := range seen {
					lines = append(lines, line)
				}
				sort.Strings(lines)
				if len(lines) == 0 {
					lines = append(lines, "BOUNDED_NO_DATA_20_SECONDS")
				}
				observation["postbackAcknowledgement"] = lines
			}
		}
		after = append(after, observation)
		row = v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "data-v07-case") == c.ID })
	}
	answer["after"] = after
	return v07DOM(t, answer), diagnostics
}

var v07SelectTag = regexp.MustCompile(`(?i)(</?)select\b`)
var v07FixtureName = regexp.MustCompile(`FamilyV07[CP]\d+`)
var v07StackLocation = regexp.MustCompile(`(line )\d+(, column )\d+`)

func v07DOM(t *testing.T, value any) string {
	encoded := v10Encode(t, value)
	encoded = v07FixtureName.ReplaceAllString(encoded, "<owned>")
	encoded = v07StackLocation.ReplaceAllString(encoded, "${1}<line>${2}<column>")
	var out strings.Builder
	out.WriteString("DOM|")
	for _, r := range encoded {
		if r <= 127 {
			out.WriteRune(r)
		} else {
			for _, unit := range utf16.Encode([]rune{r}) {
				fmt.Fprintf(&out, "\\u%04x", unit)
			}
		}
	}
	return out.String()
}

func v07Toggle(n *html.Node, key string, enabled bool) {
	attrs := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Key != key {
			attrs = append(attrs, a)
		}
	}
	n.Attr = attrs
	if enabled {
		v10SetAttr(n, key, "")
	}
}

func v07Options(n *html.Node) []*html.Node {
	options := []*html.Node{}
	v10Walk(n, func(child *html.Node) {
		if child.Type == html.ElementNode && child.Data == "option" {
			options = append(options, child)
		}
	})
	return options
}

func v07OptionText(n *html.Node) string {
	// HTMLOptionElement.text collapses ASCII whitespace; textContent does not.
	return strings.Join(strings.FieldsFunc(v10Text(n), func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	}), " ")
}

func v07OptionValue(n *html.Node) string {
	if v10HasAttr(n, "value") {
		return v10Attr(n, "value")
	}
	return v07OptionText(n)
}

func v07Value(n *html.Node) string {
	if n.Data == "select" {
		for _, option := range v07Options(n) {
			if v10HasAttr(option, "selected") {
				return v07OptionValue(option)
			}
		}
		return ""
	}
	return v10ControlValue(n)
}

func v07InitializeControls(doc *html.Node) {
	radios := map[string]*html.Node{}
	v10Walk(doc, func(n *html.Node) {
		if n.Data == "select" && !v10HasAttr(n, "multiple") {
			options := v07Options(n)
			var selected *html.Node
			for _, option := range options {
				if v10HasAttr(option, "selected") {
					if selected != nil {
						v07Toggle(selected, "selected", false)
					}
					selected = option
				}
			}
			size, _ := strconv.Atoi(v10Attr(n, "size"))
			if selected == nil && size <= 1 {
				for _, option := range options {
					if !v10HasAttr(option, "disabled") {
						v07Toggle(option, "selected", true)
						break
					}
				}
			}
		}
		if n.Data == "input" && v10Attr(n, "type") == "radio" && v10HasAttr(n, "checked") {
			name := v10Attr(n, "name")
			if previous := radios[name]; previous != nil && name != "" {
				v07Toggle(previous, "checked", false)
			}
			radios[name] = n
		}
	})
}

func v07InnerHTML(t *testing.T, n *html.Node, stable bool) string {
	t.Helper()
	var out strings.Builder
	textEscape := strings.NewReplacer("&", "&amp;", "\u00a0", "&nbsp;", "<", "&lt;", ">", "&gt;")
	attrEscape := strings.NewReplacer("&", "&amp;", "\u00a0", "&nbsp;", "\"", "&quot;")
	var write func(*html.Node)
	write = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			out.WriteString(textEscape.Replace(n.Data))
		case html.CommentNode:
			out.WriteString("<!--" + n.Data + "-->")
		case html.ElementNode:
			if stable && (n.Data == "script" || n.Data == "style") {
				return
			}
			out.WriteString("<" + n.Data)
			for _, a := range n.Attr {
				if !stable || a.Key == "class" || a.Key == "style" || a.Key == "type" || a.Key == "value" || a.Key == "disabled" || a.Key == "selected" || a.Key == "checked" {
					out.WriteString(" " + a.Key + "=\"" + attrEscape.Replace(a.Val) + "\"")
				}
			}
			out.WriteString(">")
			switch n.Data {
			case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
				return
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				write(child)
			}
			out.WriteString("</" + n.Data + ">")
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		write(child)
	}
	return out.String()
}

func v07Observe(t *testing.T, doc *html.Node, id string) map[string]any {
	row := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "data-v07-case") == id })
	if row == nil {
		return map[string]any{"localError": "rendered row marker missing"}
	}
	parse := func(key string) any {
		n := v10Find(row, func(n *html.Node) bool { return v10HasAttr(n, key) })
		if n == nil {
			return map[string]any{"unparsed": nil}
		}
		var value any
		if err := json.Unmarshal([]byte(v10Text(n)), &value); err != nil {
			return map[string]any{"unparsed": v10Text(n)}
		}
		return value
	}
	controls, labels, messages := []any{}, []any{}, []string{}
	v10Walk(row, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		kind := v10ControlType(n)
		if n.Data == "select" || (n.Data == "input" && (kind == "radio" || kind == "checkbox")) {
			var checked any
			size := 20
			if n.Data == "select" {
				size, _ = strconv.Atoi(v10Attr(n, "size"))
			} else {
				checked = v10HasAttr(n, "checked")
				if value, err := strconv.Atoi(v10Attr(n, "size")); err == nil && value > 0 {
					size = value
				}
			}
			options := []any{}
			if n.Data == "select" {
				for _, option := range v07Options(n) {
					label := v07OptionText(option)
					if v10HasAttr(option, "label") {
						label = v10Attr(option, "label")
					}
					options = append(options, map[string]any{"value": v07OptionValue(option), "text": v07OptionText(option), "label": label, "html": v07InnerHTML(t, option, false), "selected": v10HasAttr(option, "selected"), "disabled": v10HasAttr(option, "disabled")})
				}
			}
			controls = append(controls, map[string]any{"tag": n.Data, "type": kind, "value": v07Value(n), "disabled": v10HasAttr(n, "disabled"), "checked": checked, "multiple": v10HasAttr(n, "multiple"), "size": size, "class": v10Attr(n, "class"), "style": v10Attr(n, "style"), "options": options})
		}
		if n.Data == "label" {
			labels = append(labels, map[string]any{"text": v10Text(n), "html": v07InnerHTML(t, n, true)})
		}
		for _, class := range strings.Fields(v10Attr(n, "class")) {
			if class == "messageText" || class == "errorMsg" || class == "errorMessage" {
				messages = append(messages, strings.TrimSpace(v10Text(n)))
				break
			}
		}
	})
	return map[string]any{"state": parse("data-v07-state"), "result": parse("data-v07-result"), "controls": controls, "labels": labels, "messages": messages}
}

func v07SetSelection(row *html.Node, values []string) map[string]any {
	selects, inputs := []*html.Node{}, []*html.Node{}
	v10Walk(row, func(n *html.Node) {
		if n.Data == "select" {
			selects = append(selects, n)
		} else if n.Data == "input" && (v10ControlType(n) == "radio" || v10ControlType(n) == "checkbox") {
			inputs = append(inputs, n)
		}
	})
	wanted := func(value string) bool {
		for _, v := range values {
			if value == v {
				return true
			}
		}
		return false
	}
	events := []any{}
	dispatch := func(n *html.Node) {
		for _, kind := range []string{"input", "change"} {
			events = append(events, map[string]any{"type": kind, "value": v07Value(n)})
		}
	}
	for _, n := range selects {
		var selected *html.Node
		for _, option := range v07Options(n) {
			on := wanted(v07OptionValue(option))
			if on && !v10HasAttr(n, "multiple") && selected != nil {
				v07Toggle(selected, "selected", false)
			}
			v07Toggle(option, "selected", on)
			if on {
				selected = option
			}
		}
		dispatch(n)
	}
	radios := map[string]*html.Node{}
	for _, n := range inputs {
		on := wanted(v07Value(n))
		if on && v10ControlType(n) == "radio" {
			name := v10Attr(n, "name")
			if previous := radios[name]; previous != nil && name != "" {
				v07Toggle(previous, "checked", false)
			}
			radios[name] = n
		}
		v07Toggle(n, "checked", on)
		dispatch(n)
	}
	injected := 0
	if len(selects) != 0 {
		n := selects[0]
		for _, value := range values {
			found := false
			for _, option := range v07Options(n) {
				found = found || v07OptionValue(option) == value
			}
			if !found {
				if !v10HasAttr(n, "multiple") {
					for _, option := range v07Options(n) {
						v07Toggle(option, "selected", false)
					}
				}
				option := &html.Node{Type: html.ElementNode, Data: "option"}
				v10SetAttr(option, "value", value)
				v10SetAttr(option, "selected", "")
				option.AppendChild(&html.Node{Type: html.TextNode, Data: "OWNED_INJECTED"})
				n.AppendChild(option)
				injected++
			}
		}
	}
	return map[string]any{"method": "DOM property assignment + input/change dispatch; absent select values injected as owned options", "requested": values, "events": events, "injected": injected, "controlCount": len(selects) + len(inputs)}
}

func v07FormValues(form, button *html.Node) map[string]string {
	// The server joins repeated successful form values with semicolons before
	// passing them to the product binder. Do not submit disabled/unchecked items.
	values := map[string][]string{}
	v10Walk(form, func(n *html.Node) {
		if n.Type != html.ElementNode || (n.Data != "input" && n.Data != "select" && n.Data != "textarea" && n.Data != "button") {
			return
		}
		name, kind := v10Attr(n, "name"), v10ControlType(n)
		if name == "" || v10HasAttr(n, "disabled") || ((kind == "checkbox" || kind == "radio") && !v10HasAttr(n, "checked")) || ((kind == "submit" || kind == "button" || kind == "reset") && n != button) {
			return
		}
		if n.Data == "select" {
			for _, option := range v07Options(n) {
				if v10HasAttr(option, "selected") && !v10HasAttr(option, "disabled") {
					values[name] = append(values[name], v07OptionValue(option))
				}
			}
		} else {
			values[name] = append(values[name], v07Value(n))
		}
	})
	out := map[string]string{}
	for name, items := range values {
		out[name] = strings.Join(items, ";")
	}
	return out
}
