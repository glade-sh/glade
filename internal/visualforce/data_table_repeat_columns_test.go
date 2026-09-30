package visualforce

import (
	"reflect"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"

	"github.com/glade-sh/glade/internal/vm"
)

// VF row 40, AP-REPEAT-20: API 67 apex:dataTable permits apex:repeat to
// generate columns. Retained pages_compref_dataTable.md lines 1-6,
// SHA256 01135562fbe1418d5031d0aba0009570c7cd619b639c048572027161ee1a0da9.
func TestRenderDataTableRepeatColumnsAPI67(t *testing.T) {
	t.Run("direct column control", func(t *testing.T) {
		rendered := renderDataTableRepeatColumnsFixture(t, `<apex:page><apex:dataTable value="{!accounts}" var="a">
			<apex:column value="{!a.Name}"/>
		</apex:dataTable></apex:page>`)
		headers, cells := dataTableRepeatColumnTexts(t, rendered)
		if len(headers) != 1 || !reflect.DeepEqual(cells, []string{"Acme & <Probe>"}) {
			t.Fatalf("direct column headers=%d cells=%q; want one header and one row cell", len(headers), cells)
		}
		if !strings.Contains(rendered, "Acme &amp; &lt;Probe&gt;") {
			t.Fatal("direct column cell was not HTML-escaped")
		}
	})

	t.Run("three repeat-generated columns follow direct column", func(t *testing.T) {
		rendered := renderDataTableRepeatColumnsFixture(t, `<apex:page><apex:dataTable value="{!accounts}" var="a">
			<apex:column value="{!a.Name}"/>
			<apex:repeat value="{!headers}" var="h"><apex:column headerValue="{!h}" value="{!h}"/></apex:repeat>
		</apex:dataTable></apex:page>`)
		headers, cells := dataTableRepeatColumnTexts(t, rendered)
		wantHeaders := []string{"", "First", "Second & <Three>", "Fourth"}
		wantCells := []string{"Acme & <Probe>", "First", "Second & <Three>", "Fourth"}
		if !reflect.DeepEqual(headers, wantHeaders) || !reflect.DeepEqual(cells, wantCells) {
			t.Fatalf("repeat columns headers=%q cells=%q; want four ordered columns in one row", headers, cells)
		}
		if !strings.Contains(rendered, "<th>Second &amp; &lt;Three&gt;</th>") || !strings.Contains(rendered, "<td>Second &amp; &lt;Three&gt;</td>") || !strings.Contains(rendered, "Acme &amp; &lt;Probe&gt;") {
			t.Fatal("repeat-generated header/cell or direct column cell was not HTML-escaped")
		}
	})

	t.Run("rendered false skips invalid repeat collection", func(t *testing.T) {
		rendered := renderDataTableRepeatColumnsFixture(t, `<apex:page><apex:dataTable value="{!accounts}" var="a">
			<apex:column value="{!a.Name}"/>
			<apex:repeat rendered="false" value="{!(}" var="h"><apex:column value="{!h}"/></apex:repeat>
		</apex:dataTable></apex:page>`)
		headers, cells := dataTableRepeatColumnTexts(t, rendered)
		if len(headers) != 1 || !reflect.DeepEqual(cells, []string{"Acme & <Probe>"}) {
			t.Fatalf("hidden repeat headers=%d cells=%q; want only direct column", len(headers), cells)
		}
	})

	t.Run("nested rendered uses outer repeat binding", func(t *testing.T) {
		rendered := renderDataTableRepeatColumnsFixture(t, `<apex:page><apex:dataTable value="{!accounts}" var="a">
			<apex:column value="{!a.Name}"/>
			<apex:repeat value="{!outerFlags}" var="flag">
				<apex:repeat rendered="{!flag}" value="{!headers}" var="h"><apex:column headerValue="{!h}" value="{!h}"/></apex:repeat>
			</apex:repeat>
		</apex:dataTable></apex:page>`)
		headers, cells := dataTableRepeatColumnTexts(t, rendered)
		wantHeaders := []string{"", "First", "Second & <Three>", "Fourth"}
		wantCells := []string{"Acme & <Probe>", "First", "Second & <Three>", "Fourth"}
		if !reflect.DeepEqual(headers, wantHeaders) || !reflect.DeepEqual(cells, wantCells) {
			t.Fatalf("nested repeat headers=%q cells=%q; want one visible three-column expansion", headers, cells)
		}
	})
}

func renderDataTableRepeatColumnsFixture(t *testing.T, markup string) string {
	t.Helper()
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	controller := vm.Object("RepeatColumnsController")
	controller.Fields["headers"] = vm.List(vm.String("First"), vm.String("Second & <Three>"), vm.String("Fourth"))
	controller.Fields["outerFlags"] = vm.List(vm.Bool(false), vm.Bool(true))
	account := vm.Object("Account")
	account.Fields["Name"] = vm.String("Acme & <Probe>")
	controller.Fields["accounts"] = vm.List(account)
	scope := NewScopeStack()
	rendered, err := RenderMarkupTree(tree, &RenderContext{
		PageName:   "RepeatColumns67",
		PageMeta:   Page{APIVersion: "67.0"},
		Expression: &ExpressionContext{Controller: controller, Scope: scope},
		Scope:      scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	return rendered
}

func dataTableRepeatColumnTexts(t *testing.T, rendered string) ([]string, []string) {
	t.Helper()
	doc, err := nethtml.Parse(strings.NewReader(rendered))
	if err != nil {
		t.Fatal(err)
	}
	tables := dataTableRepeatNodes(doc, "table")
	if len(tables) != 1 {
		t.Fatalf("rendered tables=%d, want one", len(tables))
	}
	heads := dataTableRepeatNodes(tables[0], "thead")
	bodies := dataTableRepeatNodes(tables[0], "tbody")
	if len(heads) != 1 || len(bodies) != 1 {
		t.Fatalf("table heads=%d bodies=%d, want one each", len(heads), len(bodies))
	}
	rows := dataTableRepeatNodes(bodies[0], "tr")
	if len(rows) != 1 {
		t.Fatalf("table body rows=%d, want one", len(rows))
	}
	headerNodes := dataTableRepeatNodes(heads[0], "th")
	cellNodes := dataTableRepeatNodes(rows[0], "td")
	headers := make([]string, 0, len(headerNodes))
	for _, node := range headerNodes {
		headers = append(headers, dataTableRepeatText(node))
	}
	cells := make([]string, 0, len(cellNodes))
	for _, node := range cellNodes {
		cells = append(cells, dataTableRepeatText(node))
	}
	return headers, cells
}

func dataTableRepeatNodes(root *nethtml.Node, tag string) []*nethtml.Node {
	var nodes []*nethtml.Node
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && node.Data == tag {
			nodes = append(nodes, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return nodes
}

func dataTableRepeatText(root *nethtml.Node) string {
	var text strings.Builder
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			text.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return text.String()
}
