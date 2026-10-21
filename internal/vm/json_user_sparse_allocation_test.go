package vm

import (
	"bytes"
	"encoding/json"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestJSONUserAllocationStaysSparse(t *testing.T) {
	program, err := CompileAnonymous(`
User before=new User(Id=UserInfo.getUserId(),Username=UserInfo.getUsername());
SObject after=(SObject)JSON.deserialize(JSON.serialize(before),SObject.class);

User typed=(User)JSON.deserialize(JSON.serialize(before),User.class);

User explicitProfile=(User)JSON.deserialize('{"attributes":{"type":"User"},"Id":"005000000000001","ProfileId":"00e000000000099"}',User.class);
// Explicitly supplied references survive JSON transport.
System.assertEquals(Id.valueOf('00e000000000099'),explicitProfile.ProfileId);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	saved, err := json.Marshal(org.Objects["User"].Records)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	observed, err := json.Marshal(org.Objects["User"].Records)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, observed) {
		t.Fatal("JSON allocation changed persisted User storage")
	}
}
