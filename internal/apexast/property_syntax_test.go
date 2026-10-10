package apexast

import (
	"strings"
	"testing"
)

func TestNativePropertySyntaxTransientGetterDiagnosticLocations(t *testing.T) {
	// Native transient-getter diagnostics were captured at API 59 and 67.
	// Keep the captured controller source intact: native recovery reports the
	// setter's semicolon at column 85, not the getter's at column 80.
	const capturedSource = `public with sharing class FamilyV1167_226C {public String value { transient get; set; }public List<String> aliasValue;public String observation {get;set;} public Integer posts {get;set;} public class OwnedValue {public Integer n;public String secret;public OwnedValue(Integer x){n=x;secret='INNER';}} public FamilyV1167_226C(){posts=0;observation='INITIAL';} public String getReadOnly(){return 'OWNED';} public String getBinding(){return String.valueOf(value);} public PageReference prepare(){value='PREPARED';posts++;observation='PREPARED';return null;} public PageReference observe(){posts++;try{observation=JSON.serialize(new Map<String,Object>{'value'=>value,'posts'=>posts});}catch(Exception e){observation=JSON.serialize(new Map<String,Object>{'errorType'=>e.getTypeName(),'error'=>e.getMessage(),'posts'=>posts});}return null;} public List<String> sortedSet(Set<String> s){if(s==null)return null;List<String> r=new List<String>(s);r.sort();return r;} public List<String> payload(Integer bytes){List<String> r=new List<String>();for(Integer i=0;i<(bytes+63)/64;i++){String h=EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256',Blob.valueOf('V11_FIXED_'+i)));Integer remain=bytes-i*64;r.add(h.substring(0,Math.min(64,remain)));}return r;} }
`
	want := map[string]int{
		"Invalid type: transient":       67,
		"Missing '<EOF>' at 'public'":   88,
		"Unexpected token ';'.":         85,
		"Unexpected token 'transient'.": 67,
	}
	for _, api := range []string{"59", "67"} {
		t.Run(api+".0", func(t *testing.T) {
			name := "FamilyV11" + api + "_226C"
			source := strings.ReplaceAll(capturedSource, "FamilyV1167_226C", name)
			file := NewParser().ParseSource(name+".cls", source)
			if len(file.Diagnostics) != len(want) {
				t.Fatalf("diagnostics = %#v, want four captured parser diagnostics", file.Diagnostics)
			}
			seen := make(map[string]bool)
			for _, got := range file.Diagnostics {
				column, ok := want[got.Message]
				if !ok || seen[got.Message] {
					t.Errorf("unexpected or duplicate diagnostic: %#v", got)
					continue
				}
				seen[got.Message] = true
				if got.NativeMessage != got.Message {
					t.Errorf("native diagnostic = %q, want %q", got.NativeMessage, got.Message)
				}
				if got.Range == nil || got.Range.Start.Line != 1 || got.Range.Start.Column != column {
					t.Errorf("%q range = %#v, want line 1 column %d", got.Message, got.Range, column)
				}
			}
		})
	}
}
