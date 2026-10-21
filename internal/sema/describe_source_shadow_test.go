package sema

import (
	"fmt"
	"testing"
)

// Owned API62/67/68 anonymous and named observations accept this source class,
// including its constructor, fields, method, instanceof and class literal.
func TestAnalyzeDescribeSourceShadowConstructor(t *testing.T) {
	const declaration = `public class Schema {
 public class DescribeSObjectResult {
  public String name;
  public String sObjectDescribeOption;
  public DescribeSObjectResult(String objectName){name=objectName;sObjectDescribeOption='DEFERRED';}
  public String sourceMarker(){return 'owned-source';}
 }
}`
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, object := range []string{"User", "Account"} {
			t.Run(api+"/"+object, func(t *testing.T) {
				consumer := fmt.Sprintf(`public class DescribeSourceConsumer {
 public static void observe() {
  Schema.DescribeSObjectResult descriptor=new Schema.DescribeSObjectResult('%s');
  String marker=descriptor.sourceMarker();
  String name=descriptor.name;
  String option=descriptor.sObjectDescribeOption;
  Object erased=descriptor;
  Boolean instance=erased instanceof Schema.DescribeSObjectResult;
  String literal=Schema.DescribeSObjectResult.class.getName();
 }
}`, object)
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"Schema.cls": declaration, "DescribeSourceConsumer.cls": consumer,
				}, api)
				if result.HasErrors() {
					t.Fatalf("native source constructor accepted; diagnostics=%v", result.Diagnostics)
				}
			})
		}
	}
}

func TestAnalyzeDescribePlatformConstructorControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, body string
			wantErrors bool
		}{
			{"platform constructor", `Schema.DescribeSObjectResult descriptor=new Schema.DescribeSObjectResult('User');`, true},
			{"platform describe", `Schema.DescribeSObjectResult descriptor=Account.SObjectType.getDescribe();String name=descriptor.getName();`, false},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"DescribePlatformConsumer.cls": "public class DescribePlatformConsumer { public static void observe(){" + tc.body + "} }",
				}, api)
				if result.HasErrors() != tc.wantErrors {
					t.Fatalf("errors=%v, want %v: %v", result.HasErrors(), tc.wantErrors, result.Diagnostics)
				}
			})
		}
	}
}
