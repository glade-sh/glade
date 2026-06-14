package lwc

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseComponentMetaRecordPageTargetConfig(t *testing.T) {
	path := writeComponentMeta(t, `<?xml version="1.0" encoding="UTF-8"?>
<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>61.0</apiVersion>
    <isExposed>true</isExposed>
    <masterLabel>Record Inspector</masterLabel>
    <targets>
        <target>lightning__RecordPage</target>
    </targets>
    <targetConfigs>
        <targetConfig targets="lightning__RecordPage">
            <property name="recordId" type="String" label="Record ID" description="Current record" default="{!recordId}" required="true" datasource="Account,Contact" min="1" max="18" role="inputOnly"/>
            <objects>
                <object>Account</object>
                <object>Contact</object>
            </objects>
            <supportedFormFactors>
                <supportedFormFactor type="Large"/>
                <supportedFormFactor type="Small"/>
            </supportedFormFactors>
        </targetConfig>
    </targetConfigs>
</LightningComponentBundle>`)

	meta, err := ParseComponentMeta(path)
	if err != nil {
		t.Fatalf("ParseComponentMeta() error = %v", err)
	}
	if meta.APIVersion != "61.0" {
		t.Fatalf("APIVersion = %q, want 61.0", meta.APIVersion)
	}
	if meta.MasterLabel != "Record Inspector" {
		t.Fatalf("MasterLabel = %q, want Record Inspector", meta.MasterLabel)
	}
	if len(meta.TargetConfigs) != 1 {
		t.Fatalf("TargetConfigs length = %d, want 1", len(meta.TargetConfigs))
	}

	config := meta.TargetConfigs[0]
	if !reflect.DeepEqual(config.Targets, []string{"lightning__RecordPage"}) {
		t.Fatalf("TargetConfig targets = %#v", config.Targets)
	}
	if !reflect.DeepEqual(config.SupportedObjects, []string{"Account", "Contact"}) {
		t.Fatalf("SupportedObjects = %#v", config.SupportedObjects)
	}
	if !reflect.DeepEqual(config.SupportedFormFactors, []string{"Large", "Small"}) {
		t.Fatalf("SupportedFormFactors = %#v", config.SupportedFormFactors)
	}
	if len(config.Properties) != 1 {
		t.Fatalf("Properties length = %d, want 1", len(config.Properties))
	}

	prop := config.Properties[0]
	if prop.Name != "recordId" ||
		prop.Type != "String" ||
		prop.Label != "Record ID" ||
		prop.Description != "Current record" ||
		prop.Default != "{!recordId}" ||
		!prop.Required ||
		prop.DataSource != "Account,Contact" ||
		prop.Min != "1" ||
		prop.Max != "18" ||
		prop.Role != "inputOnly" {
		t.Fatalf("property parsed as %#v", prop)
	}
}

func TestParseComponentMetaSplitsCommaSeparatedTargetConfigTargets(t *testing.T) {
	path := writeComponentMeta(t, `<?xml version="1.0" encoding="UTF-8"?>
<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
    <isExposed>true</isExposed>
    <targets>
        <target>lightning__RecordPage</target>
        <target>lightning__AppPage</target>
    </targets>
    <targetConfigs>
        <targetConfig targets="lightning__RecordPage, lightning__AppPage">
            <property name="heading" type="String"/>
        </targetConfig>
    </targetConfigs>
</LightningComponentBundle>`)

	meta, err := ParseComponentMeta(path)
	if err != nil {
		t.Fatalf("ParseComponentMeta() error = %v", err)
	}
	if len(meta.TargetConfigs) != 1 {
		t.Fatalf("TargetConfigs length = %d, want 1", len(meta.TargetConfigs))
	}
	want := []string{"lightning__RecordPage", "lightning__AppPage"}
	if !reflect.DeepEqual(meta.TargetConfigs[0].Targets, want) {
		t.Fatalf("TargetConfig targets = %#v, want %#v", meta.TargetConfigs[0].Targets, want)
	}
}

func writeComponentMeta(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recordInspector.js-meta.xml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write component meta: %v", err)
	}
	return path
}
