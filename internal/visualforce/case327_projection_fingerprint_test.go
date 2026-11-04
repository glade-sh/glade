package visualforce

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
)

func TestCase327ExpressionProjectionFingerprint(t *testing.T) {
	markup := `<apex:page><apex:dataTable value="{!accounts}" var="a"><apex:column value="{!a.Name}"/><apex:repeat value="{!headers}" var="h"><apex:column headerValue="{!h}" value="{!h}"/></apex:repeat><apex:repeat rendered="false" value="{!(}" var="x"><apex:column value="{!x}"/></apex:repeat></apex:dataTable></apex:page>`
	var first string
	for i := 0; i < 40; i++ {
		got := renderDataTableRepeatColumnsFixture(t, markup)
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatal("page projection changed between renders")
		}
	}
	t.Logf("projection sha256 %x", sha256.Sum256([]byte(first)))
}

func TestCase327PartialProjectionFingerprint(t *testing.T) {
	html := strings.Repeat(`<div id="f:item"><span>value</span></div>`, 1000) + `<div id="poll" data-vf-poller="true" data-enabled="true" data-interval="5"></div>`
	var first string
	for i := 0; i < 40; i++ {
		data, err := json.Marshal(NewPartialResponse(html, "state", []string{"item", "missing"}))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = string(data)
		} else if string(data) != first {
			t.Fatal("partial projection changed between renders")
		}
	}
	t.Logf("projection sha256 %x", sha256.Sum256([]byte(first)))
}
