package gladecli

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
)

func TestEventSubscriberDBTriggerTransitions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/SyntheticEvent__e/SyntheticEvent__e.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><eventType>HighVolume</eventType><label>Synthetic Event</label><pluralLabel>Synthetic Events</pluralLabel><publishBehavior>PublishAfterCommit</publishBehavior></CustomObject>`)
			trigger := filepath.Join(root, "force-app/main/default/triggers/SyntheticEventTrigger.trigger")
			writeTestFile(t, trigger, `trigger SyntheticEventTrigger on SyntheticEvent__e (after insert) { System.debug(Trigger.new); }`)
			db := filepath.Join(root, "local.sqlite")
			for _, step := range []struct {
				status string
				count  int
			}{{"Active", 1}, {"Inactive", 0}, {"Active", 1}} {
				writeTestFile(t, trigger+"-meta.xml", `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>`+step.status+`</status></ApexTrigger>`)
				store, org, err := openDBStore(db, root)
				if err != nil {
					t.Fatal(err)
				}
				got := len(org.Objects["EventBusSubscriber"].Records)
				if err := store.Save(org); err != nil {
					t.Fatal(err)
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
				if got != step.count {
					t.Fatalf("status %s: got %d rows, want %d", step.status, got, step.count)
				}
			}
		})
	}
}

func TestEventSubscriberToolingURLIdentity(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/SyntheticEvent__e/SyntheticEvent__e.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><eventType>HighVolume</eventType><label>Synthetic Event</label><pluralLabel>Synthetic Events</pluralLabel><publishBehavior>PublishAfterCommit</publishBehavior></CustomObject>`)
	for name, body := range map[string]string{
		"SyntheticEventTrigger": `trigger SyntheticEventTrigger on SyntheticEvent__e (after insert) { System.debug(Trigger.new); }`,
		"AnotherTrigger":        `trigger AnotherTrigger on Account (before insert) { System.debug(Trigger.new); }`,
	} {
		path := filepath.Join(root, "force-app/main/default/triggers", name+".trigger")
		writeTestFile(t, path, body)
		writeTestFile(t, path+"-meta.xml", `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexTrigger>`)
	}
	// Cross the decimal/base-36 boundary used by local trigger identities.
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("Prior%02d", i)
		path := filepath.Join(root, "force-app/main/default/triggers", name+".trigger")
		writeTestFile(t, path, fmt.Sprintf(`trigger %s on Account (before insert) { System.debug(Trigger.new); }`, name))
		writeTestFile(t, path+"-meta.xml", `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexTrigger>`)
	}
	org, err := orgForProject(root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	var triggerID, subscriberSource string
	for _, row := range source.ToolingOrg.Objects["ApexTrigger"].Records {
		if row.Fields["Name"].String == "SyntheticEventTrigger" {
			triggerID = string(row.ID)
		}
	}
	for _, row := range org.Objects["EventBusSubscriber"].Records {
		if row.Fields["Name"].String == "SyntheticEventTrigger" {
			subscriberSource = row.MetadataSourceUnit
		}
	}
	if triggerID == "" || !strings.EqualFold(subscriberSource, "glade:event-subscriber:"+triggerID) {
		t.Fatal("subscriber URL identity did not match tooling trigger identity")
	}
}
