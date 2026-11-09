package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Native member and constructor observations are retained in the Person
// Account conformance fixtures; this pins the local check command's shape.
func TestPersonAccountReadCheckRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, enabled := range []bool{false, true} {
			for _, form := range []string{"read", "constructor"} {
				for _, field := range []string{"FirstName", "LastName"} {
					t.Run(fmt.Sprintf("%s/enabled=%t/%s/%s", api, enabled, form, field), func(t *testing.T) {
						root := t.TempDir()
						writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
						feature := `{"features":[]}`
						if enabled {
							feature = `{"features":["PersonAccounts"]}`
						}
						writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), feature)
						writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`)
						body := `Account a = new Account(); String value = a.` + field + `;`
						if form == "constructor" {
							body = `Account a = new Account(` + field + `='A');`
						}
						path := filepath.Join(root, "force-app/main/default/classes/ReadProbe.cls")
						writeTestFile(t, path, `public class ReadProbe { public static void sample() { `+body+` } }`)
						writeTestFile(t, path+"-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
						var stdout, stderr bytes.Buffer
						code := Run(context.Background(), []string{"check", "--project", root, "--no-progress"}, &stdout, &stderr)
						if !enabled && form == "read" {
							if code != 1 || !strings.Contains(stdout.String(), "Variable does not exist: "+field) {
								t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
							}
						} else if !enabled && form == "constructor" {
							if code != 1 || !strings.Contains(stdout.String(), "GLADESEMA019") || !strings.Contains(stdout.String(), "Field does not exist: "+field+" on Account") {
								t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
							}
						} else if code != 0 || strings.Contains(stdout.String(), "GLADESEMA019") {
							t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
						}
					})
				}
			}
		}
	}
}
