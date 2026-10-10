package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// SOURCE ONLY: five public API 67 programs, uncompiled and unexecuted.
// the native compatibility contract owns review; Root alone owns installation and execution admission.
// Public source: apex_methods_system_url.md, lines 345-366, SHA256
// 4603656c0a4296f10e64939ee225286de77635bf06d8692348e40568e7b43b6b.
// The approved offline policy supplies a saved canonical org origin, independent
// of request origins, with https://local.glade.example as the unset default.
// JSON deliberately binds the proposed domainUrl without requiring a future Go
// field. Today's decoder ignores that unknown key; successful decoding does not
// establish that the canonical origin was installed. Freeze these bytes for red.
const urlOrgOriginFixtureJSON = `{"orgId":"00D000000000001","apiVersion":"65.0","domainUrl":"https://canonical-org.example.test","objects":{}}`

func TestExecURLCanonicalOrgOriginAPI67(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configured bool
		requestURL string
		source     string
	}{
		{
			name:       "case01ConfiguredOrgRequestA",
			configured: true,
			requestURL: "https://request-a.example.test:8443/",
			source: `URL value = URL.getOrgDomainUrl();
System.assertEquals('https://canonical-org.example.test', value.toExternalForm());
`,
		},
		{
			name:       "case02ConfiguredOrgRequestB",
			configured: true,
			requestURL: "http://request-b.example.test:8080/",
			source: `URL value = URL.getOrgDomainUrl();
System.assertEquals('https://canonical-org.example.test', value.toExternalForm());
`,
		},
		{
			name:       "case03UnconfiguredLocalRequestA",
			requestURL: "https://request-a.example.test:8443/",
			source: `URL value = URL.getOrgDomainUrl();
System.assertEquals('https://local.glade.example', value.toExternalForm());
`,
		},
		{
			name: "case04DefaultNoOrgNoRequestControl",
			source: `URL value = URL.getOrgDomainUrl();
System.assertEquals('https://local.glade.example', value.toExternalForm());
`,
		},
		{
			name:       "case05CurrentRequestAControl",
			requestURL: "https://request-a.example.test:8443/",
			source: `URL value = URL.getCurrentRequestUrl();
System.assertEquals('https://request-a.example.test:8443/apex/current', value.toExternalForm());
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, compileErr := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if compileErr != nil {
				t.Fatalf("step=compile case=%s api=67.0 source=%q compileErr=%v", tc.name, tc.source, compileErr)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("step=program-api-guard case=%s got=%q want=67.0", tc.name, program.APIVersion)
			}
			machine := New(nil)
			if tc.configured {
				org := storage.NewOrgState()
				if err := json.Unmarshal([]byte(urlOrgOriginFixtureJSON), &org); err != nil {
					t.Fatalf("step=org-json case=%s decodeErr=%v", tc.name, err)
				}
				if org.APIVersion != "65.0" {
					t.Fatalf("step=org-api-guard case=%s got=%q want=65.0", tc.name, org.APIVersion)
				}
				machine.SetOrg(&org)
			}
			if tc.requestURL != "" {
				machine.SetServerBaseURL(tc.requestURL)
			}
			if _, executeErr := machine.Execute(program); executeErr != nil {
				t.Fatalf("step=execute case=%s api=67.0 source=%q executeErr=%v", tc.name, tc.source, executeErr)
			}
		})
	}
}
