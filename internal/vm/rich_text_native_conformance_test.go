package vm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type richTextNativeFixture struct {
	HelperSource, HelperSourceSha256, HelperMetadata, HelperMetadataSha256 string
	Captures                                                               []struct {
		Context, SourceAPI, TransportAPI, OrgIdSha256, DeploymentID, DeploymentStatus         string
		HelperSourceSha256, ObserverSource, ObserverSourceSha256                              string
		ObserverMetadata, ObserverMetadataSha256, RawResponseSha256, AnswerJSON, AnswerSha256 string
	}
	Rows []struct {
		SourceAPI, Context, Path string
		Expected                 struct {
			Available     bool
			Type          string
			HTMLFormatted bool
			Length        int
		}
	}
	Carries []struct {
		SourceAPI, Context, Path, Reason string
		NativeAvailable                  bool
	}
}

func loadRichTextNativeFixture(t *testing.T) richTextNativeFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/rich_text_display_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture richTextNativeFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestRichTextDisplayNativeFixtureIntegrity(t *testing.T) {
	fixture := loadRichTextNativeFixture(t)
	digest := func(source string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(source))) }
	if digest(fixture.HelperSource) != fixture.HelperSourceSha256 || digest(fixture.HelperMetadata) != fixture.HelperMetadataSha256 {
		t.Fatal("native helper source or source metadata changed")
	}
	if !strings.Contains(fixture.HelperMetadata, "<apiVersion>61.0</apiVersion>") || len(fixture.Captures) != 2 || len(fixture.Rows) != 4 || len(fixture.Carries) != 2 {
		t.Fatal("native fixture scope changed")
	}
	answers := make(map[string]map[string]json.RawMessage)
	for _, capture := range fixture.Captures {
		if capture.SourceAPI != "61.0" || capture.DeploymentID == "" || capture.DeploymentStatus != "Succeeded" || len(capture.OrgIdSha256) != 64 || len(capture.RawResponseSha256) != 64 {
			t.Fatal("incomplete native provenance")
		}
		if capture.HelperSourceSha256 != fixture.HelperSourceSha256 || digest(capture.ObserverSource) != capture.ObserverSourceSha256 || digest(capture.ObserverMetadata) != capture.ObserverMetadataSha256 || digest(capture.AnswerJSON) != capture.AnswerSha256 {
			t.Fatal("native source or answer hash changed")
		}
		if _, duplicate := answers[capture.Context]; duplicate {
			t.Fatal("duplicate native context")
		}
		switch capture.Context {
		case "istest":
			if capture.TransportAPI != "67.0" || !strings.Contains(capture.ObserverMetadata, "<apiVersion>61.0</apiVersion>") {
				t.Fatal("named source/transport mismatch")
			}
		case "anonymous":
			if capture.TransportAPI != "61.0" || capture.ObserverMetadata != "" {
				t.Fatal("anonymous source/transport mismatch")
			}
		default:
			t.Fatal("unknown native context")
		}
		var extracted []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(capture.AnswerJSON), &extracted); err != nil {
			t.Fatal(err)
		}
		if len(extracted) != 1 {
			t.Fatal("native answer extraction changed")
		}
		answers[capture.Context] = extracted[0]
	}
	seen := make(map[string]bool)
	for _, row := range fixture.Rows {
		key := row.Context + "/" + row.Path
		if seen[key] || row.SourceAPI != "61.0" || !row.Expected.Available {
			t.Fatal("unobserved or duplicate native row")
		}
		seen[key] = true
		encoded, ok := answers[row.Context][row.Path]
		if !ok {
			t.Fatal("native row absent from raw answer")
		}
		var observed struct {
			Available     bool
			Type          string
			HTMLFormatted bool
			Length        int
		}
		if err := json.Unmarshal(encoded, &observed); err != nil {
			t.Fatal(err)
		}
		if observed != row.Expected {
			t.Fatalf("expected row differs from captured answer: %s", key)
		}
	}
	for _, carry := range fixture.Carries {
		if carry.SourceAPI != "61.0" || carry.Path != "CaseComment.CommentBodyRichtext" || carry.NativeAvailable || carry.Reason == "" {
			t.Fatal("availability carry changed")
		}
		var observed struct{ Available bool }
		if err := json.Unmarshal(answers[carry.Context][carry.Path], &observed); err != nil {
			t.Fatal(err)
		}
		if observed.Available {
			t.Fatal("availability carry contradicts native answer")
		}
	}
}

// Replay the four available standard-field observations only. The unavailable
// field remains a recorded provisioning carry, with no local absence assertion.
// The custom Html import regression is a separate synthetic test.
func TestRichTextDisplayNativeConformance(t *testing.T) {
	fixture := loadRichTextNativeFixture(t)
	for _, row := range fixture.Rows {
		t.Run(row.Context+"/"+row.Path, func(t *testing.T) {
			if row.SourceAPI != "61.0" || !row.Expected.Available {
				t.Fatal("unobserved native row")
			}
			parts := strings.Split(row.Path, ".")
			if len(parts) != 2 {
				t.Fatal("invalid fixture field path")
			}
			source := fmt.Sprintf(`
Schema.SObjectType objectToken=Schema.getGlobalDescribe().get('%s');
System.assertNotEquals(null,objectToken);
Schema.SObjectField fieldToken=objectToken.getDescribe().fields.getMap().get('%s');
System.assertNotEquals(null,fieldToken);
Schema.DescribeFieldResult detail=fieldToken.getDescribe();
System.assertEquals('%s',String.valueOf(detail.getType()));
System.assertEquals(%t,detail.isHtmlFormatted());
System.assertEquals(%d,detail.getLength());
`, parts[0], parts[1], row.Expected.Type, row.Expected.HTMLFormatted, row.Expected.Length)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row.SourceAPI})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, parts[0])
			machine := New(nil)
			machine.SetOrg(&org)
			switch row.Context {
			case "istest":
				_, err = machine.ExecuteTestInClass(program, "RichFacetReplay")
			case "anonymous":
				_, err = machine.Execute(program)
			default:
				t.Fatal("unobserved native context")
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
