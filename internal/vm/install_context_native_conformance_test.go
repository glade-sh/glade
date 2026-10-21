package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"testing"
)

type installContextNativeFixture struct {
	Captures []struct {
		Context, SourceAPI, TransportAPI                                                       string
		OrgIDHash                                                                              string `json:"orgIdSHA256"`
		DeploymentID, Status                                                                   string
		HelperSource, HelperMetadata, ObserverSource, ObserverMetadata                         string
		HelperSourceSha256, HelperMetadataSha256, ObserverSourceSha256, ObserverMetadataSha256 string
		RawResponseSha256, AnswerJSON, AnswerSha256                                            string
	}
}

type installContextNativeObservation struct {
	OrganizationPresent, OrganizationMatches, OrganizationCaseMatches bool
	PreviousVersion                                                   *string
	Upgrade, Push, InstallerMatches                                   bool
}

func loadInstallContextNativeFixture(t *testing.T) installContextNativeFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/install_context_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture installContextNativeFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestInstallContextNativeFixtureIntegrity(t *testing.T) {
	fixture := loadInstallContextNativeFixture(t)
	if len(fixture.Captures) != 6 {
		t.Fatal("native capture scope changed")
	}
	digest := func(source string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(source))) }
	validHash := func(hash string) bool {
		decoded, err := hex.DecodeString(hash)
		return err == nil && len(decoded) == 32
	}
	seen := make(map[string]bool)
	for _, capture := range fixture.Captures {
		key := capture.SourceAPI + "/" + capture.Context
		if seen[key] {
			t.Fatal("duplicate native context")
		}
		seen[key] = true
		if capture.SourceAPI != "48.0" && capture.SourceAPI != "62.0" && capture.SourceAPI != "67.0" {
			t.Fatal("unobserved source API")
		}
		if capture.DeploymentID == "" || capture.Status != "Succeeded" || !validHash(capture.OrgIDHash) || !validHash(capture.RawResponseSha256) {
			t.Fatalf("incomplete native provenance: %s", key)
		}
		for _, pair := range [][2]string{
			{capture.HelperSource, capture.HelperSourceSha256},
			{capture.HelperMetadata, capture.HelperMetadataSha256},
			{capture.ObserverSource, capture.ObserverSourceSha256},
			{capture.ObserverMetadata, capture.ObserverMetadataSha256},
			{capture.AnswerJSON, capture.AnswerSha256},
		} {
			if digest(pair[0]) != pair[1] {
				t.Fatalf("native source or answer digest changed: %s", key)
			}
		}
		verifyPin := func(metadata string) {
			t.Helper()
			var pin struct {
				APIVersion string `xml:"apiVersion"`
			}
			if err := xml.Unmarshal([]byte(metadata), &pin); err != nil {
				t.Fatal(err)
			}
			if pin.APIVersion != capture.SourceAPI {
				t.Fatalf("source API metadata mismatch: %s", key)
			}
		}
		verifyPin(capture.HelperMetadata)
		switch capture.Context {
		case "istest":
			if capture.TransportAPI != "67.0" {
				t.Fatal("named transport API mismatch")
			}
			verifyPin(capture.ObserverMetadata)
			var rows []installContextNativeObservation
			if err := json.Unmarshal([]byte(capture.AnswerJSON), &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 {
				t.Fatal("native installation shapes changed")
			}
		case "anonymous":
			if capture.TransportAPI != capture.SourceAPI || capture.ObserverMetadata != "" || capture.ObserverSource == "" {
				t.Fatal("anonymous source/transport mismatch")
			}
			var row struct{ ExceptionMessage string }
			if err := json.Unmarshal([]byte(capture.AnswerJSON), &row); err != nil {
				t.Fatal(err)
			}
			if row.ExceptionMessage == "" {
				t.Fatal("native anonymous boundary lacks an answer")
			}
		default:
			t.Fatal("unobserved context")
		}
	}
}

func TestInstallContextNativeConformance(t *testing.T) {
	fixture := loadInstallContextNativeFixture(t)
	for _, capture := range fixture.Captures {
		t.Run(capture.SourceAPI+"/"+capture.Context, func(t *testing.T) {
			switch capture.Context {
			case "istest":
				var rows []installContextNativeObservation
				if err := json.Unmarshal([]byte(capture.AnswerJSON), &rows); err != nil {
					t.Fatal(err)
				}
				if len(rows) != 2 {
					t.Fatal("native installation shapes changed")
				}
				for index, row := range rows {
					t.Run(fmt.Sprint(index), func(t *testing.T) {
						// These two calls are the ordered inputs in the retained helper source.
						arguments := "null"
						if index == 1 {
							arguments = "new Version(1, 2, 3), true"
						}
						previous := "null"
						if row.PreviousVersion != nil {
							previous = "'" + strings.ReplaceAll(*row.PreviousVersion, "'", "\\'") + "'"
						}
						body := fmt.Sprintf(`
Id installedOrganization=context.organizationId();
System.assertEquals(%t,installedOrganization!=null);
System.assertEquals(%t,installedOrganization==UserInfo.getOrganizationId());
System.assertEquals(%t,installedOrganization==context.OrganizationId());
Version previous=context.previousVersion();
System.assertEquals(%s,previous==null ? null : previous.toString());
System.assertEquals(%t,context.isUpgrade());
System.assertEquals(%t,context.isPush());
System.assertEquals(%t,context.installerId()==UserInfo.getUserId());
`, row.OrganizationPresent, row.OrganizationMatches, row.OrganizationCaseMatches, previous, row.Upgrade, row.Push, row.InstallerMatches)
						executeInstallOrganizationProbe(t, capture.SourceAPI, arguments, body)
					})
				}
			case "anonymous":
				var observed struct{ ExceptionMessage string }
				if err := json.Unmarshal([]byte(capture.AnswerJSON), &observed); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions("Test.testInstall(null,null);", CompileOptions{APIVersion: capture.SourceAPI})
				if err != nil {
					t.Fatal(err)
				}
				// This boundary precedes handler validation and does not enable test mode.
				machine := New(nil)
				_, err = machine.Execute(program)
				if err == nil || err.Error() != observed.ExceptionMessage {
					t.Fatalf("anonymous boundary=%v, want %q", err, observed.ExceptionMessage)
				}
			default:
				t.Fatal("unobserved native context")
			}
		})
	}
}
