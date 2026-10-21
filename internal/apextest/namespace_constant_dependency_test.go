package apextest

import (
	"path/filepath"
	"testing"
)

func TestNamespaceRemapConstantPrefixDependency(t *testing.T) {
	root := t.TempDir()
	provider := filepath.Join(root, "provider")
	consumer := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(provider, "sfdx-project.json"), `{"namespace":"BasePkg","sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(provider, "force-app/main/default/objects/PrefixSetting__c/PrefixSetting__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Prefix Setting</label><pluralLabel>Prefix Settings</pluralLabel><customSettingsType>Hierarchy</customSettingsType><visibility>Public</visibility></CustomObject>`)
	writeFile(t, filepath.Join(provider, "force-app/main/default/classes/PrefixCatalog.cls"), `global class PrefixCatalog {
 private static final String namespaceName = 'BasePkg';
 private static final String objectPrefix = namespaceName + '__';
 private static final String classPrefix = namespaceName + '.';
 global static Boolean enabled(Map<String,Boolean> flags) {
  String prefix = PrefixSetting__c.SObjectType.getDescribe().getName().startsWith(objectPrefix) ? objectPrefix : '';
  return flags.get(prefix + 'Example');
 }
 global static String qualifiedClass() { return classPrefix + 'PrefixCatalog'; }
}`)
	writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  namespaceRemaps: [\"BasePkg:stagepkg\"]\n  managedPackageDependencies: [\"stagepkg:../provider\"]\n")
	writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/PrefixConsumerTest.cls"), `@IsTest private class PrefixConsumerTest {
 @IsTest static void computedPrefixUsesDependencyAlias() {
  Map<String,Boolean> flags = new Map<String,Boolean>{'stagepkg__Example' => true};
  System.assertEquals(true, stagepkg.PrefixCatalog.enabled(flags));
  System.assertEquals('stagepkg.PrefixCatalog', stagepkg.PrefixCatalog.qualifiedClass());
 }
}`)
	run := Run(loadTestIndex(t, consumer), Options{NoDiskCache: true, Parallelism: 1})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
	}
}
