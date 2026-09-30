package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// SF185 admitted these API52 caller and API53 helper assertions.
func TestWizardStringEscapesAndLocaleAPI52Project(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": "{\"sourceApiVersion\":\"52.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}",
		"force-app/main/default/classes/GladeWizardStrings53Helper.cls":          "public class GladeWizardStrings53Helper {\n public static Integer at(String s,Integer i){return s.codePointAt(i);}\n public static Integer before(String s,Integer i){return s.codePointBefore(i);}\n public static Integer offset(String s,Integer i,Integer n){return s.offsetByCodePoints(i,n);}\n public static Integer last(String s,Integer c){return s.lastIndexOfChar(c);}\n public static Integer lastBefore(String s,Integer c,Integer i){return s.lastIndexOfChar(c,i);}\n public static String lower(String s,String locale){return s.toLowerCase(locale);}\n public static String upper(String s,String locale){return s.toUpperCase(locale);}\n}\n",
		"force-app/main/default/classes/GladeWizardStrings53Helper.cls-meta.xml": "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n",
		"force-app/main/default/classes/GladeWizardStrings52Proof.cls":           "@IsTest private class GladeWizardStrings52Proof {\n @IsTest static void unicodeEscapesAndCodePoints(){\n  String s='\\u03A9 is Ω (Omega), and \\uD835\\uDD0A ' + ' is Fraktur Capital G.';\n  System.assertEquals(937,GladeWizardStrings53Helper.at(s,0));\n  System.assertEquals(937,GladeWizardStrings53Helper.before(s,1));\n  System.assertEquals(4,GladeWizardStrings53Helper.offset('A \\uD835\\uDD0A BC',0,3));\n  System.assertEquals(5,GladeWizardStrings53Helper.last('\\u03A9 is Ω (Omega)',937));\n  System.assertEquals(6,GladeWizardStrings53Helper.lastBefore('Ω and \\u03A9 and Ω',937,11));\n }\n @IsTest static void localeCaseConversion(){\n  System.assertEquals('kıymetli',GladeWizardStrings53Helper.lower('KIYMETLİ','tr'));\n  System.assertEquals('İMKANSIZ',GladeWizardStrings53Helper.upper('imkansız','tr'));\n  System.assertEquals('this is hard to read','ThIs iS hArD tO rEaD'.toLowerCase());\n  System.assertEquals('ABCD','abcd'.toUpperCase());\n }\n}\n",
		"force-app/main/default/classes/GladeWizardStrings52Proof.cls-meta.xml":  "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>52.0</apiVersion><status>Active</status></ApexClass>\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	r := apextest.Run(typesys.Build(p, s), apextest.Options{NoDiskCache: true, Parallelism: 1})
	if got := r.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
		t.Fatalf("wizard strings: %+v; %+v", got, r)
	}
}
