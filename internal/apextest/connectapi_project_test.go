package apextest

import "testing"

func TestRunProjectConnectApiIdentityUsesBuiltinDispatch(t *testing.T) {
	project := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	project.writeClass("ConnectApiIdentityTest", `
@isTest private class ConnectApiIdentityTest {
  @isTest static void testNamedCredentials() {
    ConnectApi.ExternalCredentialInput eci = new ConnectApi.ExternalCredentialInput();
    ConnectApi.ExternalCredential ec = ConnectApi.NamedCredentials.createExternalCredential(eci);
    System.assertNotEquals(null, ec);
    ConnectApi.NamedCredentialInput nci = new ConnectApi.NamedCredentialInput();
    ConnectApi.NamedCredential nc = ConnectApi.NamedCredentials.createNamedCredential(nci);
    System.assertNotEquals(null, nc);
    System.assertNotEquals(null, ConnectApi.NamedCredentials.getNamedCredentials());
    System.assertNotEquals(null, ConnectApi.NamedCredentials.getExternalCredential('e1'));
  }

  @isTest static void testUserProfiles() {
    ConnectApi.UserProfile p = ConnectApi.UserProfiles.getUserProfile('i', '005-user');
    System.assertNotEquals(null, p);
    System.assertNotEquals(null, ConnectApi.UserProfiles.getPhoto('i', '005-user'));
    ConnectApi.UserProfiles.setPhoto('i', '005-user', 'fileId', 1);
    ConnectApi.UserProfiles.deletePhoto('i', '005-user');
  }
}
`)

	run := project.run()
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}
