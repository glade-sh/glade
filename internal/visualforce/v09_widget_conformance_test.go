package visualforce_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/visualforce"
	"golang.org/x/net/html"
)

type v09WidgetTable struct {
	OracleCommit string                `json:"oracle_commit"`
	Fixtures     map[string]v09Fixture `json:"fixtures"`
	Cases        []struct {
		ID          string                     `json:"id"`
		SourceRow   string                     `json:"source_row"`
		Kind        string                     `json:"kind"`
		Markup      string                     `json:"markup"`
		Inputs      map[string]v04Input        `json:"inputs"`
		Expected    map[string]json.RawMessage `json:"expected"`
		Observation v09Observation             `json:"observation"`
	} `json:"cases"`
}

func v09LoadWidgetControls(t *testing.T) v09WidgetTable {
	t.Helper()
	data, err := os.ReadFile("testdata/v09_widget_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v09WidgetTable
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Cases) != 46 || table.OracleCommit == "" {
		t.Fatal("incomplete captured V09 widget table")
	}
	return table
}

var v09Shortcut = regexp.MustCompile(`DatePicker\.insertDate\('([^']*)',`)
var v09Shield = regexp.MustCompile(`^Sfdc\.onload\(function\(\) \{\s*blankshield\(document\.getElementById\('([^']+)'\)\.parentElement\);\s*\}\);$`)
var v09LinkShield = regexp.MustCompile(`Sfdc\.onload\(function\(\) \{\s*blankshield\(document\.getElementById\('([^']+)'\)\.parentElement\);\s*\}\);`)

// Preserve the original oracle bytes. Only controls with the identical captured
// source-row input qualify the clock argument/text and referenced shield ID.
// No other whitespace, case, field values, hrefs, or script source is removed.
func v09QualifyOriginal(t *testing.T, raw string) string {
	t.Helper()
	var row map[string]any
	if json.Unmarshal([]byte(strings.TrimPrefix(raw, "DOM|")), &row) != nil {
		return raw
	}
	links, _ := row["links"].([]any)
	for _, item := range links {
		link := item.(map[string]any)
		href, _ := link["href"].(string)
		text, _ := link["text"].(string)
		if date := v09Shortcut.FindStringSubmatch(href); len(date) == 2 && text == date[1] {
			link["href"] = strings.Replace(href, "'"+text+"'", "'CURRENT_DATE_SHORTCUT'", 1)
			link["text"] = "CURRENT_DATE_SHORTCUT"
			if content, ok := row["text"].(string); ok {
				row["text"] = strings.ReplaceAll(content, "[\u00a0"+text+"\u00a0]", "[\u00a0CURRENT_DATE_SHORTCUT\u00a0]")
			}
		}
		if shield := v09LinkShield.FindStringSubmatch(text); len(shield) == 2 {
			link["text"] = strings.ReplaceAll(text, "'"+shield[1]+"'", "'URL_SHIELD_ID'")
		}
	}
	return v09DOM(t, row)
}

func v09ClockFacts(text string, started, ended time.Time) (bool, bool) {
	zone, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return false, false
	}
	start, end := started.In(zone), ended.In(zone)
	return text == start.Format("1/2/2006") || text == end.Format("1/2/2006"),
		text == start.Format("1/2/2006, 3:04 PM") || text == end.Format("1/2/2006, 3:04 PM")
}

func v09EqualOutsideDestination(t *testing.T, actual, expected string) bool {
	var left, right map[string]json.RawMessage
	if json.Unmarshal([]byte(strings.TrimPrefix(actual, "DOM|")), &left) != nil || json.Unmarshal([]byte(strings.TrimPrefix(expected, "DOM|")), &right) != nil {
		return false
	}
	delete(left, "destination")
	delete(right, "destination")
	return v04Encode(t, left) == v04Encode(t, right)
}

func v09VerifyDynamicFacts(t *testing.T, raw string, doc *html.Node, c v09Case, started, ended time.Time) string {
	var row map[string]any
	if json.Unmarshal([]byte(strings.TrimPrefix(raw, "DOM|")), &row) != nil {
		return raw
	}
	marker := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-v09-case") == c.ID })
	v04Walk(marker, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		if date := v09Shortcut.FindStringSubmatch(v04Attr(n, "href")); len(date) == 2 {
			text := v09Text(n)
			day, minute := v09ClockFacts(text, started, ended)
			if text != date[1] || (!day && !minute) {
				row["invalidClockShortcut"] = true
			}
		}
		v04Walk(n, func(script *html.Node) {
			if script.Type != html.ElementNode || script.Data != "script" {
				return
			}
			match := v09Shield.FindStringSubmatch(v09Text(script))
			if len(match) != 2 || v04Find(n, func(target *html.Node) bool { return v04Attr(target, "id") == match[1] }) == nil {
				row["invalidShieldReference"] = true
			}
		})
	})
	return v09DOM(t, row)
}

func v09ObserveWidget(t *testing.T, doc *html.Node, handler http.Handler, request *http.Request, status int, c v09Case, ownerID string, started, ended time.Time) string {
	var base map[string]any
	raw := v09ObserveDOM(t, doc, handler, request, status, c, ownerID)
	if json.Unmarshal([]byte(strings.TrimPrefix(raw, "DOM|")), &base) != nil {
		return raw
	}
	marker := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-v09-case") == c.ID })
	if marker == nil {
		return raw
	}
	controls, hidden, links, scripts, shortcuts := []any{}, []any{}, []any{}, []any{}, []any{}
	shieldID := ""
	text := base["text"].(string)
	v04Walk(marker, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if n.Data == "input" || n.Data == "textarea" || n.Data == "select" {
			kind := v09ControlType(n)
			id := v04Attr(n, "id")
			if kind == "hidden" {
				if strings.Contains(id, ":owned_") {
					hidden = append(hidden, map[string]any{"nativeId": id, "name": v04Attr(n, "name"), "value": map[string]any{"stored": true, "text": v09Stable(v04Attr(n, "value"), ownerID)}})
				}
				return
			}
			value, options := v09ControlValue(n, kind)
			var tabindex any
			if v04HasAttr(n, "tabindex") {
				tabindex = v04Attr(n, "tabindex")
			}
			controls = append(controls, map[string]any{"nativeId": id, "name": v04Attr(n, "name"), "tag": n.Data, "type": kind, "value": v09Stable(value, ownerID), "options": options, "disabled": v04HasAttr(n, "disabled"), "required": v04HasAttr(n, "required"), "class": v04Attr(n, "class"), "style": v04Attr(n, "style"), "tabindex": tabindex})
		}
		if n.Data != "a" || !v04HasAttr(n, "href") {
			return
		}
		href := v04Attr(n, "href")
		// This original capture projects same-origin links to their path.
		// The review controls independently assert the full target query.
		if parsed, err := url.Parse(href); err == nil && parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") {
			href = parsed.EscapedPath()
		}
		var visible strings.Builder
		var walk func(*html.Node)
		walk = func(child *html.Node) {
			if child.Type == html.ElementNode && (child.Data == "script" || child.Data == "style") {
				return
			}
			if child.Type == html.TextNode {
				visible.WriteString(child.Data)
			}
			for sub := child.FirstChild; sub != nil; sub = sub.NextSibling {
				walk(sub)
			}
		}
		walk(n)
		linkText := visible.String()
		if date := v09Shortcut.FindStringSubmatch(href); len(date) == 2 && linkText == date[1] {
			day, minute := v09ClockFacts(linkText, started, ended)
			shortcuts = append(shortcuts, map[string]any{"parameterEqualsText": true, "matchesServerDate": day, "matchesServerMinute": minute})
			href = strings.Replace(href, "'"+linkText+"'", "'CURRENT_DATE_SHORTCUT'", 1)
			text = strings.ReplaceAll(text, "[\u00a0"+linkText+"\u00a0]", "[\u00a0CURRENT_DATE_SHORTCUT\u00a0]")
			linkText = "CURRENT_DATE_SHORTCUT"
		}
		v04Walk(n, func(script *html.Node) {
			if script.Type != html.ElementNode || script.Data != "script" {
				return
			}
			source := v09Text(script)
			match := v09Shield.FindStringSubmatch(source)
			owned := false
			var qualified, reason any
			if len(match) == 2 && v04Find(n, func(target *html.Node) bool { return v04Attr(target, "id") == match[1] }) != nil {
				owned = true
				shieldID = match[1]
				qualified = strings.ReplaceAll(source, match[1], "URL_SHIELD_ID")
			} else {
				reason = "Unrecognized or unowned script content excluded"
			}
			scripts = append(scripts, map[string]any{"recognizedBlankshield": len(match) == 2, "referenceInsideOwnedAnchor": owned, "source": qualified, "reason": reason})
		})
		links = append(links, map[string]any{"nativeId": v04Attr(n, "id"), "href": v09Stable(href, ownerID), "text": v09Stable(linkText, ownerID)})
	})
	context := map[string]any{}
	contextNode := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-v09-control-context") == "true" })
	if contextNode != nil {
		_ = json.Unmarshal([]byte(v09Text(contextNode)), &context)
	}
	for _, key := range []string{"serverStartUtc", "serverEndUtc", "serverStartDate", "serverEndDate", "serverStartMinute", "serverEndMinute"} {
		delete(context, key)
	}
	return v09DOM(t, map[string]any{"text": text, "rich": base["rich"], "labels": base["labels"], "controls": controls, "hidden": hidden, "links": links, "scripts": scripts, "shortcuts": shortcuts, "context": context, "privateShieldID": shieldID})
}

func v09RunWidgetControls(t *testing.T, table v09WidgetTable, capture bool, writeRow func(...string)) {
	t.Helper()
	total := 0
	for _, api := range []string{"59.0", "67.0"} {
		root := t.TempDir()
		fixture := table.Fixtures[api]
		v09WriteFiles(t, root, fixture.Files)
		renderer := v09NewRenderer(t, root)
		matches := 0
		for _, row := range table.Cases {
			func() {
				input := row.Inputs[api]
				v09WriteFiles(t, root, input.Files)
				defer func() {
					for path := range input.Files {
						if err := os.Remove(root + "/" + path); err != nil {
							t.Error(err)
						}
					}
				}()
				got, want := "", ""
				if row.Kind == "compile" {
					answer, diagnostics := v04Execute(t, root, input.Name, v04Case{Kind: "compile"})
					got = answer
					if len(diagnostics) != 0 {
						got += "|" + v04Encode(t, diagnostics)
					}
					if err := json.Unmarshal(row.Expected[api], &want); err != nil {
						t.Fatal(err)
					}
				} else {
					c := v09Case{ID: row.ID, Kind: row.Kind, Observation: row.Observation, Widget: true}
					var first, repeat map[string]any
					if err := json.Unmarshal([]byte(strings.TrimPrefix(renderer.render(t, input.Name, c), "DOM|")), &first); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal([]byte(strings.TrimPrefix(renderer.render(t, input.Name, c), "DOM|")), &repeat); err != nil {
						t.Fatal(err)
					}
					id1, id2 := first["privateShieldID"], repeat["privateShieldID"]
					delete(first, "privateShieldID")
					delete(repeat, "privateShieldID")
					first["identityFacts"] = map[string]any{"urlShieldPresent": id1 != "", "repeatShieldChanged": id1 != "" && id1 != id2, "repeatShieldSourceEqual": v04Encode(t, first["scripts"]) == v04Encode(t, repeat["scripts"])}
					got = v09DOM(t, first)
					var expected map[string]any
					if err := json.Unmarshal(row.Expected[api], &expected); err != nil {
						t.Fatal(err)
					}
					want = v09DOM(t, expected)
				}
				status := "MISMATCH"
				if got == want {
					status = "MATCH"
					matches++
				} else if !capture {
					t.Errorf("V09 widget API %s %s actual <%s> expected <%s>", api, row.ID, got, want)
				}
				writeRow(api, row.ID, status, got, want, "widget-controls")
			}()
			v09ReleaseRowState(t, root)
		}
		visualforce.V09ReleaseRenderEnvironmentsForTest(root)
		renderer = nil
		v09ReleaseRowState(t, root)
		t.Logf("V09 widget API %s exact %d/46", api, matches)
		total += matches
	}
	writeRow("WIDGET_TOTAL", fmt.Sprintf("%d/92", total))
	t.Logf("V09 widget controls exact %d/92", total)
}
