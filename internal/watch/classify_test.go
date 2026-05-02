package watch

import (
	"path/filepath"
	"testing"
)

func TestClassifyPath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		kind       FileKind
		symbol     string
		objectName string
		watchable  bool
	}{
		{
			name:      "class",
			path:      filepath.Join("force-app", "main", "default", "classes", "InvoiceService.cls"),
			kind:      FileKindApexClass,
			symbol:    "InvoiceService",
			watchable: true,
		},
		{
			name:       "trigger",
			path:       filepath.Join("force-app", "main", "default", "triggers", "InvoiceTrigger.trigger"),
			kind:       FileKindApexTrigger,
			symbol:     "InvoiceTrigger",
			objectName: "",
			watchable:  true,
		},
		{
			name:       "object metadata",
			path:       filepath.Join("force-app", "main", "default", "objects", "Invoice__c", "Invoice__c.object-meta.xml"),
			kind:       FileKindObjectMeta,
			symbol:     "Invoice__c",
			objectName: "Invoice__c",
			watchable:  true,
		},
		{
			name:       "field metadata",
			path:       filepath.Join("force-app", "main", "default", "objects", "Invoice__c", "fields", "Amount__c.field-meta.xml"),
			kind:       FileKindFieldMeta,
			symbol:     "Amount__c",
			objectName: "Invoice__c",
			watchable:  true,
		},
		{
			name:      "ignored",
			path:      filepath.Join("force-app", "README.md"),
			kind:      FileKindIgnored,
			watchable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyPath(tt.path)
			if got.Kind != tt.kind || got.Name != tt.symbol || got.ObjectName != tt.objectName || got.Watchable != tt.watchable {
				t.Fatalf("ClassifyPath() = %#v", got)
			}
		})
	}
}
