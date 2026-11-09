package resource

import (
	"os"
	"path/filepath"
	"testing"
)

// Native source-format deploys at API 62.0 and 67.0 accept any root element
// name for a message channel file (see message_channel_root/native/ORG-RUN.txt).
func TestLoadMessageChannelAcceptsAnyRootElement(t *testing.T) {
	for _, root := range []string{"LightningMessageChannel", "MessageChannel", "TotallyDifferent"} {
		t.Run(root, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample.messageChannel-meta.xml")
			body := `<?xml version="1.0" encoding="UTF-8"?><` + root + ` xmlns="http://soap.sforce.com/2006/04/metadata"><masterLabel>Sample</masterLabel><isExposed>true</isExposed><description>d</description></` + root + `>`
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			channel, err := loadMessageChannel(path, "")
			if err != nil {
				t.Fatalf("root %s rejected: %v", root, err)
			}
			if channel.Name != "sample" || channel.MasterLabel != "Sample" {
				t.Fatalf("channel = %+v", channel)
			}
		})
	}
}
