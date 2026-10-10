package lwc

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

type conditionalCase struct {
	ID              string         `json:"id"`
	Template        string         `json:"template"`
	Properties      map[string]any `json:"properties"`
	Initial         map[string]any `json:"initial"`
	Expected        string         `json:"expected"`
	InitialExpected string         `json:"initialExpected"`
	ExpectedTags    []string       `json:"expectedTags"`
	ErrorCode       string         `json:"errorCode"`
}

func TestConditionalSalesforceConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/lwc_conditionals.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []conditionalCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("empty Salesforce family")
	}
	matches := 0
	var report strings.Builder
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			tree, err := ParseTemplate(c.Template)
			if c.ErrorCode != "" {
				if err == nil || !strings.Contains(err.Error(), c.ErrorCode) {
					fmt.Fprintf(&report, "%s\tMISMATCH\texpected %s; got %v\n", c.ID, c.ErrorCode, err)
					t.Errorf("expected %s, got %v", c.ErrorCode, err)
					return
				}
				matches++
				fmt.Fprintf(&report, "%s\tMATCH\n", c.ID)
				return
			}
			if err != nil {
				t.Error(err)
				fmt.Fprintf(&report, "%s\tMISMATCH\t%v\n", c.ID, err)
				return
			}
			ctx := &RenderContext{Properties: conditionalProperties(c.Properties)}
			if c.Initial != nil {
				ctx.Properties = conditionalProperties(c.Initial)
				out, err := RenderTemplate(tree, ctx)
				text, _, parseErr := conditionalDOM(out)
				if err != nil || parseErr != nil || text != c.InitialExpected {
					t.Errorf("initial text=%q want=%q render=%v parse=%v", text, c.InitialExpected, err, parseErr)
					fmt.Fprintf(&report, "%s\tMISMATCH\tinitial state\n", c.ID)
					return
				}
				// Reuse both the parsed template and the context; values change between renders.
				ctx.Properties = conditionalProperties(c.Properties)
			}
			out, err := RenderTemplate(tree, ctx)
			text, tags, parseErr := conditionalDOM(out)
			if err != nil || parseErr != nil || text != c.Expected || !reflect.DeepEqual(tags, c.ExpectedTags) {
				t.Errorf("text=%q want=%q tags=%v want=%v render=%v parse=%v", text, c.Expected, tags, c.ExpectedTags, err, parseErr)
				fmt.Fprintf(&report, "%s\tMISMATCH\ttext=%q tags=%v\n", c.ID, text, tags)
				return
			}
			matches++
			fmt.Fprintf(&report, "%s\tMATCH\n", c.ID)
		})
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\n", matches, len(cases))
	t.Logf("Salesforce matches: %d/%d", matches, len(cases))
	if path := os.Getenv("GLADE_LWC_PROBE_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func conditionalProperties(properties map[string]any) PropertyBag {
	bag := PropertyBag{}
	for key, value := range properties {
		bag[key] = conditionalValue(value)
	}
	return bag
}

func conditionalValue(value any) Value {
	switch v := value.(type) {
	case nil:
		return NullValue()
	case bool:
		return BoolValue(v)
	case string:
		return StringValue(v)
	case float64:
		return IntValue(int64(v))
	case []any:
		items := make([]Value, len(v))
		for i, item := range v {
			items[i] = conditionalValue(item)
		}
		return ArrayValue(items)
	case map[string]any:
		fields := map[string]Value{}
		for key, item := range v {
			fields[key] = conditionalValue(item)
		}
		return ObjectValue(fields)
	default:
		panic(fmt.Sprintf("unexpected fixture value %T", value))
	}
}

func conditionalDOM(source string) (string, []string, error) {
	root, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", nil, err
	}
	var text strings.Builder
	tags := []string{}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		if n.Type == html.ElementNode && (n.Data == "span" || n.Data == "template") {
			tags = append(tags, n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return text.String(), tags, nil
}
