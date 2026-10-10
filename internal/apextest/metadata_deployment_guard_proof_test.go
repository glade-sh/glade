package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact three API66 test deployment restrictions admitted by Salesforce SF157.
func TestMetadataDeploymentTestRestrictionAPI66Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeMetadataGuard66Proof.cls"), "@IsTest private class GladeMetadataGuard66Proof {\n    private static Integer callbackCount = 0;\n    private class Callback implements Metadata.DeployCallback {\n        public void handleResult(Metadata.DeployResult result, Metadata.DeployCallbackContext context) {\n            callbackCount++;\n        }\n    }\n    private static Metadata.DeployContainer container() {\n        Metadata.CustomMetadata item = new Metadata.CustomMetadata();\n        item.fullName = 'GladeDeployGuard66__mdt.Probe';\n        item.label = 'Probe';\n        Metadata.CustomMetadataValue value = new Metadata.CustomMetadataValue();\n        value.field = 'Active__c';\n        value.value = true;\n        item.values.add(value);\n        Metadata.DeployContainer result = new Metadata.DeployContainer();\n        result.addMetadata(item);\n        return result;\n    }\n    private static Id initiateMetadataSave() {\n        return Metadata.Operations.enqueueDeployment(container(), new Callback());\n    }\n    @IsTest static void rejectsDeploymentWithCallback() {\n        try {\n            Metadata.Operations.enqueueDeployment(container(), new Callback());\n            Assert.fail('Expected to fail starting the deployment');\n        } catch (System.AsyncException e) {\n            Assert.areEqual('Metadata cannot be deployed from within a test', e.getMessage());\n        }\n        Assert.areEqual(0, callbackCount);\n    }\n    @IsTest static void rejectsDeploymentWithNullCallback() {\n        try {\n            Metadata.Operations.enqueueDeployment(container(), null);\n            Assert.fail('Expected to fail starting the deployment');\n        } catch (System.AsyncException e) {\n            Assert.areEqual('Metadata cannot be deployed from within a test', e.getMessage());\n        }\n    }\n    @IsTest static void rejectsDeploymentThroughServiceWrapper() {\n        try {\n            Id deploymentId = initiateMetadataSave();\n            Assert.fail('Should throw an exception');\n        } catch (System.AsyncException e) {\n            Assert.areEqual('Metadata cannot be deployed from within a test', e.getMessage());\n        }\n        Assert.areEqual(0, callbackCount);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeMetadataGuard66Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDeployGuard66__mdt/GladeDeployGuard66__mdt.object-meta.xml"), "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><label>Glade Deploy Guard</label><pluralLabel>Glade Deploy Guards</pluralLabel><visibility>Public</visibility></CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDeployGuard66__mdt/fields/Active__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Active__c</fullName><defaultValue>false</defaultValue><fieldManageability>DeveloperControlled</fieldManageability><label>Active</label><type>Checkbox</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"66.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, _ := json.Marshal(run)
	t.Logf("report: %s", data)
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("metadata proof: %s", data)
	}
}
