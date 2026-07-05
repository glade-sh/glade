package sema

import (
	"path/filepath"
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
    result.errorStatusCode = Metadata.StatusCode.INTERNAL_ERROR;
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

func analyzeStandardSymbolProject(root string, files ...string) Result {
	return Analyze(typesys.Build(project.Project{Root: root, ApexFiles: files}, schema.Schema{}))
}
