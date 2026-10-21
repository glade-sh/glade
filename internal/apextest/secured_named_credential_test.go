package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, API 62.0. A SecuredEndpoint named credential answers NamedCredential queries with its label and a null Endpoint.
func TestSecuredEndpointNamedCredentialProjectsRowWithNullEndpoint(t *testing.T) {
	root := t.TempDir()
	const base = "force-app/main/default/"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, base, "namedCredentials/GISSOpenId.namedCredential-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<NamedCredential xmlns="http://soap.sforce.com/2006/04/metadata">
    <allowMergeFieldsInBody>false</allowMergeFieldsInBody>
    <allowMergeFieldsInHeader>false</allowMergeFieldsInHeader>
    <calloutStatus>Enabled</calloutStatus>
    <generateAuthorizationHeader>true</generateAuthorizationHeader>
    <label>GISSOpenId</label>
    <namedCredentialParameters>
        <parameterName>Url</parameterName>
        <parameterType>Url</parameterType>
        <parameterValue>https://w9-int.seg-social.es/</parameterValue>
    </namedCredentialParameters>
    <namedCredentialParameters>
        <externalCredential>GISSOpenId</externalCredential>
        <parameterName>ExternalCredential</parameterName>
        <parameterType>Authentication</parameterType>
    </namedCredentialParameters>
    <namedCredentialType>SecuredEndpoint</namedCredentialType>
</NamedCredential>
`)
	writeFile(t, filepath.Join(root, base, "externalCredentials/GISSOpenId.externalCredential-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ExternalCredential xmlns="http://soap.sforce.com/2006/04/metadata">
    <authenticationProtocol>Oauth</authenticationProtocol>
    <externalCredentialParameters>
        <parameterGroup>DefaultGroup</parameterGroup>
        <parameterName>Oauth</parameterName>
        <parameterType>AuthProtocolVariant</parameterType>
        <parameterValue>ClientCredentialsClientSecret</parameterValue>
    </externalCredentialParameters>
    <externalCredentialParameters>
        <parameterGroup>DefaultGroup</parameterGroup>
        <parameterName>AuthProviderUrl</parameterName>
        <parameterType>AuthProviderUrl</parameterType>
        <parameterValue>https://w9-int.seg-social.es/auth/realms/connect-realm/protocol/openid-connect/token</parameterValue>
    </externalCredentialParameters>
    <externalCredentialParameters>
        <parameterGroup>Credenciales API</parameterGroup>
        <parameterName>Credenciales API</parameterName>
        <parameterType>NamedPrincipal</parameterType>
        <sequenceNumber>1</sequenceNumber>
    </externalCredentialParameters>
    <externalCredentialParameters>
        <parameterGroup>DefaultGroup</parameterGroup>
        <parameterName>Scope</parameterName>
        <parameterType>AuthParameter</parameterType>
        <parameterValue>openid offline_access in-isac-mulesoft-access cs-pase-silcon-read tg-psaf-read tg-mrlbafil-read od-a360-read tg-psre-read tg-ceus-read</parameterValue>
    </externalCredentialParameters>
    <label>GISSOpenId</label>
</ExternalCredential>
`)
	writeFile(t, filepath.Join(root, base, "classes/NcProbe.cls"), `@isTest
public class NcProbe {
  @isTest static void run() {
    String o = '';
    for (NamedCredential n : [SELECT DeveloperName, MasterLabel, Endpoint, PrincipalType, NamespacePrefix FROM NamedCredential WHERE DeveloperName = 'GISSOpenId']) {
      o += '\nNC|' + n.DeveloperName + '|label=' + n.MasterLabel + '|endpoint=' + n.Endpoint + '|principal=' + n.PrincipalType + '|ns=' + n.NamespacePrefix;
    }
    NamedCredential x = [SELECT Endpoint FROM NamedCredential WHERE DeveloperName = 'GISSOpenId' LIMIT 1];
    o += '\nGET|' + x.Endpoint;
    System.assert(false, o);
  }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/NcProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{})
	var got string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")
			}
		}
	}
	want := `NC|GISSOpenId|label=GISSOpenId|endpoint=null|principal=null|ns=null
GET|null`
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("native rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}
