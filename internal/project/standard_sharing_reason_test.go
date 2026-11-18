package project

import (
	"os"
	"path/filepath"
	"testing"
)

// apextest api68_native sharing-off SHARE-REASON-DEPLOY rejects an inline
// Account sharing reason named Capture. Suffixed names and other objects
// were not captured and are not rejected.
func TestStandardSharingReasonRejections(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"Account.object-meta.xml": `<CustomObject><sharingReasons><fullName>Capture</fullName><label>Capture</label></sharingReasons><sharingReasons><fullName>Other__c</fullName></sharingReasons></CustomObject>`,
		"Case.object-meta.xml":    `<CustomObject><sharingReasons><fullName>Capture</fullName></sharingReasons></CustomObject>`,
	}
	p := Project{Root: root}
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		p.ObjectFiles = append(p.ObjectFiles, path)
	}
	rejections := StandardSharingReasonRejections(p, nil)
	want := "'Capture' does not resolve to a valid sObject type, fullName must end with: __as, __b, __c, __e, __kav, __mdt, __x"
	if len(rejections) != 1 || filepath.Base(rejections[0].File) != "Account.object-meta.xml" || rejections[0].Message != want {
		t.Fatalf("rejections = %+v", rejections)
	}
}
