package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These synthetic tests restore error-field/describe agreement. Namespaced native
// parity remains uncaptured; stored LongTextArea trimming has separate native proof.
func TestValidationErrorFieldsRestorationProject(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, namespace := range []string{"", "parcel"} {
			label := namespace
			if label == "" {
				label = "plain"
			}
			t.Run(api+"/"+label, func(t *testing.T) {
				root := validationFieldsRestorationProject(t, api, namespace)
				result := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"ValidationFieldsRestoration"}, NoDiskCache: true})
				if summary := result.Summary(); summary.Total != 5 || summary.Passed != 5 {
					data, _ := json.Marshal(result)
					t.Fatalf("restored validation fields: %s", data)
				}
			})
		}
	}
}

func TestValidationErrorFieldsDependencyControl(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			owner := validationFieldsRestorationProject(t, api, "parcel")
			consumer := t.TempDir()
			writeFile(t, filepath.Join(consumer, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"namespace":"consumer","packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  managedPackageDependencies: [\"parcel:"+filepath.ToSlash(owner)+":1.0\"]\n")
			writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/ValidationFieldsConsumer.cls"), `@IsTest private class ValidationFieldsConsumer {
 @IsTest static void insertFieldIdentity() { check(false); }
 @IsTest static void updateFieldIdentity() { check(true); }
 private static void check(Boolean updating) {
  Database.Error held=parcel.ValidationFieldsObserver.reject(updating,'field');
  System.assertEquals(new List<String>{parcel__ValidationScoreLedger__c.Score__c.getDescribe().getName()},held.getFields());
  System.assertEquals(new List<String>{'parcel__Score__c'},held.getFields());
 }
}`)
			writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/ValidationFieldsConsumer.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			index := loadTestIndex(t, consumer)
			loaded := false
			for _, dependency := range index.Dependencies {
				loaded = loaded || dependency.Namespace == "parcel" && dependency.Status == "loaded"
			}
			if !loaded {
				t.Fatal("source dependency control was not loaded", index.Dependencies)
			}
			result := Run(index, Options{SelectedClasses: []string{"ValidationFieldsConsumer"}, NoDiskCache: true})
			if summary := result.Summary(); summary.Total != 2 || summary.Passed != 2 {
				data, _ := json.Marshal(result)
				t.Fatalf("dependency field control: %s", data)
			}
		})
	}
}

func validationFieldsRestorationProject(t *testing.T, api, namespace string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"namespace":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api, namespace))
	base := filepath.Join(root, "force-app/main/default")
	source := filepath.Join("testdata", "validation_error_fields_restoration")
	for _, name := range []string{"ValidationFieldsObserver", "ValidationFieldsRestoration"} {
		body, err := os.ReadFile(filepath.Join(source, name+".cls"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(base, "classes", name+".cls"), string(body))
		writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
	}
	for _, name := range []string{"ValidationScoreLedger__c", "ValidationErrorNotebook__c"} {
		body, err := os.ReadFile(filepath.Join(source, name+".object"))
		if err != nil {
			t.Fatal(err)
		}
		field := "QualifiedScore__c"
		if namespace != "" {
			field = namespace + "__" + field
		}
		writeFile(t, filepath.Join(base, "objects", name+".object"), strings.ReplaceAll(string(body), "QUALIFIED_FIELD", field))
	}
	return root
}
