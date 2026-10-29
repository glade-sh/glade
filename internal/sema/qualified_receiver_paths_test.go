package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestQualifiedReceiverNativePathClassification(t *testing.T) {
	index := qualifiedReceiverWideningIndex(t, "testdata/qualified_receiver_widening")
	data, err := os.ReadFile("testdata/qualified_receiver_shapes/force-app/main/default/classes/KnownType.cls")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "KnownType.cls")
	writeSemaFile(t, path, string(data))
	known := typesys.Build(project.Project{Root: filepath.Dir(path), SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{})
	index.Types = append(index.Types, known.Types...)
	a := NewAnalyzer()
	a.Analyze(index)
	model, scope := buildSemaTypeMemberView(index), newIRSemaScope(nil)
	// Each case names its unchanged API 62/67 native row in the receiver
	// fixtures, including ../apextest/testdata/conformance/qualified_receivers.
	// These classify the receiver before lookup.
	for _, tc := range []struct {
		row, receiver string
		known         bool
	}{
		// Static call receivers with an unknown root keep the native and
		// release rejection; offline execution has no method to run.
		{"deep_missing_static_receiver", "a.b.MissingUtility", false},
		{"missingns_known_type_static", "missingns.KnownType", false},
		{"missingns_known_inner_static", "missingns.KnownType.Inner", false},
		{"known_type_bogus_static_member", "KnownType.Bogus", false},
		{"system_bogus_static_member", "System.Bogus", false},
		{"system_database_bogus_member", "System.Database.Bogus", false},
		{"account_fieldsets_getmap_reject", "Account.SObjectType.fieldSets", false},
		{"schema_unknown_object_fields_getmap", "Schema.Thing__c.fields", false},
		{"static_instance_property", "GatewayService.Instance", true},
		{"schema_account_fieldsets_getmap", "Schema.SObjectType.Account.fieldSets", true},
		{"account_field_token", "Account.Name", true},
		{"standard_schema_token_field", "Schema.Account.SObjectType.fields.Name", true},
		{"standard_schema_fields", "Schema.Account.fields.Name", true},
		{"standard_sobjecttype_field", "SObjectType.Account.fields.Name", true},
		{"standard_fieldset", "SObjectType.Account.fieldSets.Summary", true},
		{"custom_settings_field_token_path", "Settings__c.fields.Enabled__c", true},
		{"rollup_enum_nested_receiver", "Rollup.InvocationPoint.FROM_FULL_RECALC_FLOW", true},
		{"schema_account_sobject_type", "Schema.Account", true},
		{"boolean_class_literal", "Boolean.class", true},
	} {
		t.Run("call-receiver/"+tc.row, func(t *testing.T) {
			if got := semaQualifiedCallReceiverKnown(a, "", tc.receiver, scope, model); got != tc.known {
				t.Fatalf("receiver %q known=%v, want %v", tc.receiver, got, tc.known)
			}
		})
	}
	// Native receiver_member_routes/org.tsv:86-89,94-97: a local suffix does not establish a namespace.
	t.Run("undeclared-package-static-property-rejection", func(t *testing.T) {
		if semaQualifiedCallReceiverKnown(a, "", "pkgx.GatewayService.Instance", scope, model) {
			t.Fatal("unindexed namespace receiver was accepted")
		}
	})
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			reference string
			known     bool
		}{
			{"KnownType.Nested", true}, {"Schema.Address", true}, {"Cache.Org.OrgCacheException", true},
			{"Cache.Session.SessionCacheException", true}, {"Auth.JWTValidationException", true},
			{"a.b.KnownType", true}, {"KnownType.Bogus", false}, {"System.NotAType", false},
			// Undeclared qualifiers cannot prove absence offline; they defer
			// (qualified_receiver_shapes/deferred.tsv, undeclared_namespace_types).
			// Local declarations and constructors share the deferral boundary.
			{"missingns.KnownType.Nested", true}, {"bar.Wrapper", true}, {"fooNs.Unknown", true},
			// A platform class root's nested inventory is not closed.
			{"String.Bogus", true},
		} {
			t.Run(api+"/local-and-constructor-type/"+tc.reference, func(t *testing.T) {
				if got := semaTypeReferenceKnownAtVersion(a, model, "", tc.reference, api); got != tc.known {
					t.Fatalf("type %q known=%v, want %v", tc.reference, got, tc.known)
				}
			})
		}
	}
}

// Partial metadata is still declared metadata. A namespace alias must not
// make it indistinguishable from a provisional object inferred from source.
func TestQualifiedReceiverDeclaredPartialAndInferredSchemaRoutes(t *testing.T) {
	for _, namespace := range []string{"", "gardenpkg"} {
		t.Run("namespace="+namespace, func(t *testing.T) {
			objectName := "Settings__c"
			if namespace != "" {
				objectName = namespace + "__" + objectName
			}
			declared := schema.Object{Name: objectName, Partial: true, Fields: []schema.Field{{Name: "Enabled__c", Type: "Checkbox"}}}
			index := typesys.Index{Project: typesys.ProjectInfo{Namespace: namespace}, Objects: []schema.Object{declared, {Name: "Missing__c", Partial: true}}}
			model := buildSemaTypeMemberView(index)
			a := NewAnalyzer()
			a.queryDeclaredObjects = []schema.Object{declared}
			scope := newIRSemaScope(nil)
			for _, name := range []string{"Settings__c", objectName} {
				if _, missing := semaMissingSObjectFieldsObject(a, "Schema.SObjectType."+name+".fields.Enabled__c", scope, model); missing {
					t.Fatalf("declared partial metadata rejected through %s", name)
				}
			}
			if _, missing := semaMissingSObjectFieldsObject(a, "Schema.SObjectType.Missing__c.fields.X", scope, model); !missing {
				t.Fatal("source inference established the existence of an absent object")
			}
		})
	}
}
