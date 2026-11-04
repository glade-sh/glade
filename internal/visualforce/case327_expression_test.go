package visualforce

import (
	"reflect"
	"testing"
)

func TestCase327ExpressionProjectionWorkload(t *testing.T) {
	markup := `<apex:page><apex:dataTable value="{!accounts}" var="a"><apex:column value="{!a.Name}"/><apex:repeat value="{!headers}" var="h"><apex:column headerValue="{!h}" value="{!h}"/></apex:repeat><apex:repeat rendered="false" value="{!(}" var="x"><apex:column value="{!x}"/></apex:repeat></apex:dataTable></apex:page>`
	for i := 0; i < 40; i++ {
		rendered := renderDataTableRepeatColumnsFixture(t, markup)
		headers, cells := dataTableRepeatColumnTexts(t, rendered)
		if !reflect.DeepEqual(headers, []string{"", "First", "Second & <Three>", "Fourth"}) || !reflect.DeepEqual(cells, []string{"Acme & <Probe>", "First", "Second & <Three>", "Fourth"}) {
			t.Fatalf("headers=%q cells=%q", headers, cells)
		}
	}
}
