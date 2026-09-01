package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPublicCorpusStandardSymbolAssignments(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PublicCorpusSymbols.cls"), `
public class PublicCorpusSymbols {
  public void namedCredential(ConnectApi.ExternalCredentialInput input) {
    ConnectApi.ExternalCredential externalCredential = new ConnectApi.ExternalCredential();
    externalCredential.developerName = input.developerName;
    externalCredential.masterLabel = input.masterLabel;
    externalCredential.authenticationProtocol = input.authenticationProtocol;
    externalCredential.principals = new List<ConnectApi.ExternalCredentialPrincipal>();
    for (ConnectApi.ExternalCredentialPrincipalInput principalInput : input.principals) {
      ConnectApi.ExternalCredentialPrincipal principal = new ConnectApi.ExternalCredentialPrincipal();
      principal.principalName = principalInput.principalName;
      principal.principalType = principalInput.principalType;
      principal.sequenceNumber = principalInput.sequenceNumber;
      externalCredential.principals.add(principal);
    }
  }

  public void orchestration() {
    ConnectApi.OrchestrationStepInstance step = new ConnectApi.OrchestrationStepInstance();
    step.status = ConnectApi.OrchestrationInstanceStatus.InProgress;
    ConnectApi.OrchestrationStageInstance stage = new ConnectApi.OrchestrationStageInstance();
    stage.status = ConnectApi.OrchestrationInstanceStatus.NotStarted;
  }

  public void deploy() {
    Metadata.DeployResult result = new Metadata.DeployResult();
    result.errorStatusCode = 'INTERNAL_ERROR';
    String errorStatusCode = result.errorStatusCode;
  }
}
`)
	result := analyzeStandardSymbolProject(root, filepath.Join(root, "PublicCorpusSymbols.cls"))
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "developerName")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "masterLabel")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "authenticationProtocol")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "principalName")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "sequenceNumber")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "OrchestrationInstanceStatus")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "errorStatusCode")
}

func TestPublicCorpusMetadataDeployProblemTypeAssignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MetadataDeployProblemType.cls"), `
public class MetadataDeployProblemType {
  public void deployMessage() {
    Metadata.DeployMessage message = new Metadata.DeployMessage();
    message.problemType = Metadata.DeployProblemType.Info;
    System.assertEquals(Metadata.DeployProblemType.Info, message.problemType);
  }
}
`)
	result := analyzeStandardSymbolProject(root, filepath.Join(root, "MetadataDeployProblemType.cls"))
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "problemType")
}

func TestPublicCorpusSObjectFieldDirectBooleanAccessors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "FieldAccessors.cls"), `
public class FieldAccessors {
  public Boolean run() {
    return Schema.SObjectType.Preference__c.fields.User__c.isCreateable();
  }
}
`)
	result := analyzePublicCorpusWithSchema(t, root, schema.Schema{Objects: []schema.Object{{
		Name: "Preference__c",
		Fields: []schema.Field{{
			Name:        "User__c",
			Type:        "Lookup",
			ReferenceTo: []string{"User"},
		}},
	}}}, "FieldAccessors.cls")

	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "isCreateable")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "Boolean")
}

func TestPublicCorpusPlatformEventIdentityFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PlatformEventIdentity.cls"), `
public class PlatformEventIdentity {
  public void run(Demo_Event__e eventRecord, Widget__c widget) {
    String eventUuid = eventRecord.EventUuid;
    String replayId = eventRecord.ReplayId;
    List<Demo_Event__e> events = [SELECT Payload__c, EventUuid, ReplayId FROM Demo_Event__e];

    String missingCustom = eventRecord.Invented__c;
    String missingPlain = eventRecord.Invented;
    String wrongObject = widget.EventUuid;
  }
}
`)
	result := analyzePublicCorpusWithSchema(t, root, schema.Schema{Objects: []schema.Object{
		{Name: "Demo_Event__e", Fields: []schema.Field{{Name: "Payload__c", Type: "Text"}}},
		{Name: "Widget__c", Fields: []schema.Field{{Name: "Name", Type: "Text"}}},
	}}, "PlatformEventIdentity.cls")

	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA021" && strings.Contains(diag.Message, "Demo_Event__e") &&
			(strings.Contains(diag.Message, "EventUuid") || strings.Contains(diag.Message, "ReplayId")) {
			t.Fatalf("platform event identity field was unresolved: %#v", result.Diagnostics)
		}
		if diag.Code == "GLADESEMA_QUERY_FIELD" &&
			(strings.Contains(diag.Message, "EventUuid") || strings.Contains(diag.Message, "ReplayId")) {
			t.Fatalf("platform event identity query field was unresolved: %#v", result.Diagnostics)
		}
		if diag.Code == "GLADESEMA018" {
			t.Fatalf("platform event identity field was not String: %#v", result.Diagnostics)
		}
	}
	for _, missing := range []string{
		`unknown field "Invented__c" on Demo_Event__e`,
		`unknown field "Invented" on Demo_Event__e`,
		`unknown field "EventUuid" on Widget__c`,
	} {
		found := false
		for _, diag := range result.Diagnostics {
			if diag.Code == "GLADESEMA021" && strings.Contains(diag.Message, missing) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing closed-schema diagnostic for %s: %#v", missing, result.Diagnostics)
		}
	}
}

func analyzeStandardSymbolProject(root string, files ...string) Result {
	return Analyze(typesys.Build(project.Project{Root: root, ApexFiles: files}, schema.Schema{}))
}
