package sema

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardRelationshipDoesNotOverwriteProjectInheritedProperty(t *testing.T) {
	for _, test := range []struct {
		className, childName, fieldName, relationship string
	}{
		{"Coupon", "CouponCodeRedemption", "CouponId", "CouponCodeRedemptions"},
		{"Account", "Contact", "AccountId", "Contacts"},
	} {
		t.Run(test.className, func(t *testing.T) {
			root := t.TempDir()
			basePath := filepath.Join(root, "ProbeHistoryBase.cls")
			classPath := filepath.Join(root, test.className+".cls")
			consumerPath := filepath.Join(root, "ProbeConsumer.cls")
			writeSemaFile(t, basePath, `public virtual class ProbeHistoryBase {
    public Schema.SObjectType HistoryType { get; set; }
}`)
			writeSemaFile(t, classPath, fmt.Sprintf(`public class %s extends ProbeHistoryBase {
    public %s() { this.HistoryType = Contact.SObjectType; }
}`, test.className, test.className))
			writeSemaFile(t, consumerPath, fmt.Sprintf(`public class ProbeConsumer {
    public static Schema.SObjectType readToken() {
        %s wrapper = new %s();
        Schema.%s platformRecord = new Schema.%s();
        List<%s> children = platformRecord.%s;
        return wrapper.HistoryType;
    }
}`, test.className, test.className, test.className, test.className, test.childName, test.relationship))
			for _, file := range []string{basePath, classPath, consumerPath} {
				version := "41.0"
				if file == basePath {
					version = "36.0"
				}
				writeSemaFile(t, file+"-meta.xml", fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, version))
			}
			index, artifacts := typesys.BuildWithArtifacts(project.Project{
				Root: root, Namespace: "PKG", SourceAPIVersion: "61.0",
				ApexFiles: []string{basePath, classPath, consumerPath},
			}, schema.Schema{Objects: []schema.Object{
				{Name: test.childName, Partial: true, Fields: []schema.Field{{
					Name: test.fieldName, Type: "Lookup", ReferenceTo: []string{test.className}, ChildRelationshipName: test.relationship,
				}}},
				{Name: test.className, Partial: true},
			}})
			result := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts})
			if result.HasErrors() {
				t.Fatalf("standard relationship corrupted project %s inherited property: %#v", test.className, result.Diagnostics)
			}
		})
	}
}

func TestStandardRelationshipParentWithoutObjectDeclarationKeepsStandardFields(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Account.cls")
	consumerPath := filepath.Join(root, "ProbeConsumer.cls")
	writeSemaFile(t, classPath, `public class Account {}`)
	writeSemaFile(t, consumerPath, `public class ProbeConsumer {
    public static void inspect(Schema.Account row) {
        String name = row.Name;
        List<Contact> children = row.Contacts;
    }
}`)
	index, artifacts := typesys.BuildWithArtifacts(project.Project{
		Root: root, SourceAPIVersion: "41.0", ApexFiles: []string{classPath, consumerPath},
	}, schema.Schema{Objects: []schema.Object{{Name: "Contact", Partial: true, Fields: []schema.Field{{
		Name: "AccountId", Type: "Lookup", ReferenceTo: []string{"Account"}, ChildRelationshipName: "Contacts",
	}}}}})
	result := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts})
	if result.HasErrors() {
		t.Fatalf("relationship-only schema parent lost standard fields: %#v", result.Diagnostics)
	}
}

func TestCouponInheritedSObjectTypePropertyPreservesProjectClass(t *testing.T) {
	for _, test := range []struct {
		name          string
		namespace     string
		projectAPI    string
		hydrateSchema bool
	}{
		{name: "inherited property"},
		{name: "explicit standard object before inherited property", hydrateSchema: true},
		{name: "original namespace and project API", namespace: "PKG", projectAPI: "61.0"},
		{name: "original namespace and project API with standard object", namespace: "PKG", projectAPI: "61.0", hydrateSchema: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			basePath := filepath.Join(root, "HistoryData.cls")
			couponPath := filepath.Join(root, "Coupon.cls")
			writeSemaFile(t, basePath, `global virtual class HistoryData {
    global Schema.SObjectType HistoryType { get; set; }
}`)
			prefix := ""
			if test.hydrateSchema {
				prefix = "Schema.Coupon platformRecord = new Schema.Coupon();\n"
			}
			writeSemaFile(t, couponPath, `public class Coupon extends HistoryData {
    public Coupon() {
        `+prefix+`this.HistoryType = Coupon__c.SObjectType;
        Schema.SObjectType inheritedToken = this.HistoryType;
    }
}`)
			writeSemaFile(t, basePath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>36.0</apiVersion><status>Active</status></ApexClass>`)
			writeSemaFile(t, couponPath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>41.0</apiVersion><status>Active</status></ApexClass>`)
			index, artifacts := typesys.BuildWithArtifacts(project.Project{
				Root:             root,
				Namespace:        test.namespace,
				SourceAPIVersion: test.projectAPI,
				ApexFiles:        []string{basePath, couponPath},
			}, schema.Schema{Objects: []schema.Object{{Name: "Coupon__c"}}})
			versions := map[string]string{}
			for _, typ := range index.Types {
				if typ.Name == "HistoryData" || typ.Name == "Coupon" {
					versions[typ.Name] = typ.EffectiveAPIVersion
				}
			}
			if versions["HistoryData"] != "36.0" || versions["Coupon"] != "41.0" {
				t.Fatalf("source API versions were not preserved: %#v", versions)
			}
			result := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts})
			if result.HasErrors() {
				t.Fatalf("project Coupon inherited property rejected: %#v", result.Diagnostics)
			}
		})
	}
}
