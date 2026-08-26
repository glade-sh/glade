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
    ConnectApi.Photo uploadedPhoto = ConnectApi.UserProfiles.setPhoto(
      'i', '005-user', new ConnectApi.BinaryInput(Blob.valueOf('bytes'), 'image/png', 'avatar.png'));
    System.assertNotEquals(null, uploadedPhoto);
    ConnectApi.Photo versionedPhoto = ConnectApi.UserProfiles.setPhoto('i', '005-user', 'fileId', 1);
    System.assertNotEquals(null, versionedPhoto);
    ConnectApi.UserProfiles.deletePhoto('i', '005-user');
  }
}
`)

	run := project.run()
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunProjectConnectApiManagedContentUsesBuiltinDispatch(t *testing.T) {
	project := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	project.writeClass("ConnectApiManagedContentTest", `
@isTest private class ConnectApiManagedContentTest {
  @isTest static void testGetAllManagedContent() {
    ConnectApi.ManagedContentVersionCollection result =
      ConnectApi.ManagedContent.getAllManagedContent(null, 0, 1, 'en_US', 'News');
    System.assertEquals(1, result.items.size());
  }

  @isTest static void testGetManagedContentByContentKeys() {
    ConnectApi.ManagedContentVersionCollection result =
      ConnectApi.ManagedContent.getManagedContentByContentKeys(
        null, new List<String>{ 'home-hero' }, 0, 1, 'en_US', 'News', false);
    System.assertEquals('home-hero', result.items[0].contentKey);
  }
}
`)

	run := project.run()
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v problem=%q", got, run.Suites[0].Cases, firstRunProblem(run))
	}
}
