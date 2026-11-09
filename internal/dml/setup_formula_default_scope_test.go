package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured text default is a single setup field reference. Other shapes
// retain the existing literal-default path until separately captured.
func TestSetupFormulaDefaultCaptureScope(t *testing.T) {
	tests := []struct {
		name, display, expression string
		auto, want                bool
	}{
		{"text", "STRING", "$Setup.Prefs__c.Tone__c", false, true},
		{"phone", "PHONE", "$Setup.Prefs__c.Tone__c", false, false},
		{"url", "URL", "$Setup.Prefs__c.Tone__c", false, false},
		{"compound", "STRING", "$Setup.Prefs__c.Tone__c & 'suffix'", false, false},
		{"autonumber", "STRING", "$Setup.Prefs__c.Tone__c", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := storage.Field{Type: storage.FieldString, DisplayType: tt.display, AutoNumber: tt.auto}
			if got := FormulaDefaultShouldEvaluate(field, tt.expression); got != tt.want {
				t.Fatalf("evaluate=%v, want %v", got, tt.want)
			}
		})
	}
}
