package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Present/absent controls for the corpus expected-failure reason external_config_missing.
// The corpus tooling matches this failure signature; if it changes, the tooling reverts the test to a
// normal failure, so the shape is pinned here. The excluded_surface_D45 reason covers Slack and
// DataWeave only since 2026-11-07: the d45_connectapi_* rules were dropped from the corpus tooling
// after v0.3.4, so ConnectApi failures are ordinary failures and need no expected-failure shape.
// The ConnectApi control below stays as a plain regression pin of the captured native rows.
// Metadata (repo_metadata_missing) is covered by TestRoad2701MetadataFixtureTransitions, missing
// dependencies by TestDependencyGateMissingDependencyPolicy, and uninstalled namespaces behind
// Type.forName by TestMissingTypeReflectionNullNativeConformance.

func writeExternalConfigProject(t *testing.T, withRecord bool) salesforceSurfaceProject {
	t.Helper()
	project := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	meta := filepath.Join(project.root, "force-app/main/default")
	writeFile(t, filepath.Join(meta, "objects/ExtCfg__mdt/ExtCfg__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Ext Cfg</label><pluralLabel>Ext Cfgs</pluralLabel><visibility>Public</visibility></CustomObject>`)
	writeFile(t, filepath.Join(meta, "objects/ExtCfg__mdt/fields/Key__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Key__c</fullName><label>Key</label><type>Text</type><length>40</length><required>false</required></CustomField>`)
	if withRecord {
		writeFile(t, filepath.Join(meta, "customMetadata/ExtCfg.svc.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>svc</label><protected>false</protected><values><field>Key__c</field><value xsi:type="xsd:string">k-123</value></values></CustomMetadata>`)
	}
	project.writeClass("ExtSvc", `public class ExtSvc {
  public class ConfigException extends Exception {}
  public static String key() {
    List<ExtCfg__mdt> rows = [SELECT Key__c FROM ExtCfg__mdt WHERE DeveloperName = 'svc'];
    if (rows.isEmpty() || String.isBlank(rows[0].Key__c)) { throw new ConfigException('You should set your API key'); }
    return rows[0].Key__c;
  }
}`)
	project.writeClass("ExtSvcTest", `@isTest private class ExtSvcTest {
  @isTest static void usesConfiguredKey() { System.assertEquals('k-123', ExtSvc.key()); }
}`)
	return project
}

func TestExpectedFailureControlExternalConfigAbsentPresent(t *testing.T) {
	absent := writeExternalConfigProject(t, false).run()
	if got := absent.Summary(); got.Total != 1 || got.Failed != 1 {
		t.Fatalf("absent config: summary=%#v problem=%q", got, firstRunProblem(absent))
	}
	if msg := firstRunProblem(absent); !strings.Contains(msg, "You should set your API key") {
		t.Fatalf("absent config: signature changed: %q", msg)
	}
	present := writeExternalConfigProject(t, true).run()
	if got := present.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("present config: summary=%#v problem=%q", got, firstRunProblem(present))
	}
}

func TestConnectApiNativeRowControl(t *testing.T) {
	supported := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	// Native connectapi.json rows: a data-siloed test gets UnsupportedOperationException
	// from ConnectApi.NamedCredentials.getNamedCredentials(); SeeAllData keeps local dispatch.
	supported.writeClass("SupportedConnectApiTest", `@isTest(SeeAllData=true) private class SupportedConnectApiTest {
  @isTest static void listsNamedCredentials() { System.assertNotEquals(null, ConnectApi.NamedCredentials.getNamedCredentials()); }
}`)
	if got := supported.run().Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("supported ConnectApi call: summary=%#v", got)
	}
	excluded := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	excluded.writeClass("ExcludedConnectApiTest", `@isTest private class ExcludedConnectApiTest {
  @isTest static void picklists() { ConnectApi.RecordUi.getPicklistValuesByRecordType('Account', '012000000000000AAA'); }
}`)
	run := excluded.run()
	// At the default source API 65.0 the captured connectapi.json compile row
	// rejects this method ("Method not added until version 66.0"), so Glade
	// rejects the class at compile time with its unsupported-feature code.
	if got := run.Summary(); got.Total != 1 || got.CompileErrors != 1 {
		t.Fatalf("excluded ConnectApi call: summary=%#v problem=%q", got, firstRunProblem(run))
	}
	if msg := firstRunProblem(run); !strings.Contains(msg, `GLADESEMA028`) || !strings.Contains(msg, `unsupported local feature "ConnectApi.RecordUi.getPicklistValuesByRecordType"`) {
		t.Fatalf("excluded ConnectApi signature changed: %q", msg)
	}
}
