package visualforce

import (
	"strings"
	"testing"
)

func TestNewPartialResponseResolvesShortTargetsToFormClientIDs(t *testing.T) {
	tests := []struct {
		name      string
		formID    string
		shortID   string
		clientID  string
		valueText string
	}{
		{
			name:      "actionSupport rerender",
			formID:    "rerenderForm",
			shortID:   "targetPanel",
			clientID:  "j_id0:rerenderForm:targetPanel",
			valueText: "After rerender",
		},
		{
			name:      "actionFunction rerender",
			formID:    "actionFunctionForm",
			shortID:   "resultPanel",
			clientID:  "j_id0:actionFunctionForm:resultPanel",
			valueText: "After action function",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := `<form id="j_id0:` + tt.formID + `"><div id="` + tt.clientID + `" data-rerender="` + tt.clientID + `"><span>` + tt.valueText + `</span></div></form>`
			response := NewPartialResponse(rendered, "next-view-state", []string{tt.shortID})
			if len(response.Messages) != 0 {
				t.Fatalf("Messages = %#v, want none", response.Messages)
			}
			if len(response.Targets) != 1 {
				t.Fatalf("Targets = %#v, want one form-qualified response ID", response.Targets)
			}
			fragment, ok := response.Targets[tt.clientID]
			if !ok {
				t.Fatalf("Targets missing DOM client ID %q: %#v", tt.clientID, response.Targets)
			}
			if !strings.Contains(fragment, `id="`+tt.clientID+`"`) || !strings.Contains(fragment, tt.valueText) {
				t.Fatalf("target fragment = %q, want %q containing %q", fragment, tt.clientID, tt.valueText)
			}
		})
	}
}
