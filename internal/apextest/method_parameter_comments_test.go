package apextest

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestParseParamsIgnoresMethodHeaderComments(t *testing.T) {
	cases := []struct {
		name, source string
		want         []vm.Param
	}{
		{"doc_inline_link", `@InvocableMethod(label='Apply input' category='Owned' description='Apply input values')
/** Maps {@link Input} values (including empty values). */
public static List<Output> apply(List<Input> inputs) { return null; }`, []vm.Param{{Name: "inputs", Type: "List<Input>"}}},
		{"doc_parentheses_before_brace", `/** Calls (apply) with {@link Input}. */
public static List<Output> apply(Map<String, List<Input>> inputs) { return null; }`, []vm.Param{{Name: "inputs", Type: "Map<String, List<Input>>"}}},
		{"line_comment", `// Ignore { marker (not parameters).
public static List<Output> apply(List<Input> inputs) { return null; }`, []vm.Param{{Name: "inputs", Type: "List<Input>"}}},
		{"zero_parameters", `@AuraEnabled(cacheable=true)
/** Returns {@link Output} (without inputs). */
public static List<Output> apply() { return null; }`, nil},
		{"annotation_control", `@AuraEnabled(cacheable=true)
public static String render(final Id templateId, Map<String, Object> values) { return ''; }`, []vm.Param{{Name: "templateId", Type: "Id"}, {Name: "values", Type: "Map<String, Object>"}}},
		{"parameter_comment_control", `public static String render(String value, /* ignored parameter note */ Set<Id> ids) { return ''; }`, []vm.Param{{Name: "value", Type: "String"}, {Name: "ids", Type: "Set<Id>"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseParams(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("params = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestAnnotatedMethodCommentDirectCalls(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	classes := map[string]string{"AtlasAction": annotatedMethodAtlasSource, "GlyphAction": annotatedMethodGlyphSource, "OwnedActionTest": annotatedMethodCallerSource}
	for name, source := range classes {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 5 || got.Passed != 5 || got.Failed != 0 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("direct method contracts: %s", data)
	}
}

const annotatedMethodAtlasSource = `public inherited sharing class AtlasAction {
 public class Input { @InvocableVariable public String value; }
 public class Output { @InvocableVariable public String value; }
 @InvocableMethod(
  label='Apply owned values'
  category='Owned inputs'
  description='Project input values into output values'
 )
 /** Projects {@link Input} values (including empty values). */
 public static List<Output> apply(List<Input> inputs) {
  List<Output> outputs = new List<Output>();
  if (inputs == null) { return outputs; }
  for (Input input : inputs) { Output output = new Output(); output.value = input.value; outputs.add(output); }
  return outputs;
 }
 public static String scalar(String value) { return value; }
}`
const annotatedMethodGlyphSource = `public inherited sharing class GlyphAction {
 public class Input { @InvocableVariable public String value; }
 public class Output { @InvocableVariable public String value; }
 @InvocableMethod(label='Apply owned values' category='Owned inputs' description='Project input values into output values')
 public static List<Output> apply(List<Input> inputs) {
  List<Output> outputs = new List<Output>();
  if (inputs == null) { return outputs; }
  for (Input input : inputs) { Output output = new Output(); output.value = input.value; outputs.add(output); }
  return outputs;
 }
}`
const annotatedMethodCallerSource = `@IsTest private class OwnedActionTest {
 @IsTest static void commentedNull() { System.assertEquals(0, AtlasAction.apply(null).size()); }
 @IsTest static void commentedEmpty() { System.assertEquals(0, AtlasAction.apply(new List<AtlasAction.Input>()).size()); }
 @IsTest static void commentedValues() {
  AtlasAction.Input a = new AtlasAction.Input(); a.value = 'alpha';
  AtlasAction.Input b = new AtlasAction.Input(); b.value = 'beta';
  List<AtlasAction.Output> results = AtlasAction.apply(new List<AtlasAction.Input>{a,b});
  System.assertEquals(2, results.size()); System.assertEquals('alpha', results[0].value); System.assertEquals('beta', results[1].value);
 }
 @IsTest static void plainControl() {
  GlyphAction.Input a = new GlyphAction.Input(); a.value = 'gamma';
  List<GlyphAction.Output> results = GlyphAction.apply(new List<GlyphAction.Input>{a});
  System.assertEquals(1, results.size()); System.assertEquals('gamma', results[0].value);
 }
 @IsTest static void scalarControl() { System.assertEquals('delta', AtlasAction.scalar('delta')); }
}`
