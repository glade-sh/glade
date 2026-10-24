package storage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestEmailServicesSeedResourceRouteActiveInactiveAndBoundary(t *testing.T) {
	root, err := filepath.Abs("../apextest/testdata/conformance/email_services/project")
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "ApexClass")
	state := org.Objects["ApexClass"]
	state.Records["01p000000000001"] = storage.Record{ID: "01p000000000001", Object: "ApexClass", Fields: map[string]storage.Value{"Name": storage.StringValue("EmailServiceHandler")}}
	org.Objects["ApexClass"] = state
	if err := resource.ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for object, want := range map[string]int{"EmailServicesFunction": 2, "EmailServicesAddress": 4} {
		if got := len(org.Objects[object].Records); got != want {
			t.Errorf("%s seeded rows = %d, want %d", object, got, want)
		}
	}
	if t.Failed() {
		return
	}
	ids := map[string]storage.ID{}
	for id, r := range org.Objects["EmailServicesAddress"].Records {
		name := r.Fields["DeveloperName"].String
		ids[name] = id
		if (name == "Disabled") == r.Fields["IsActive"].Boolean {
			t.Errorf("%s active flag = %v", name, r.Fields["IsActive"])
		}
		if name == "Boundary" && len(r.Fields["LocalPart"].String) != 64 {
			t.Error("local-part boundary changed")
		}
		if r.Fields["FunctionId"].ID == "" || r.Fields["RunAsUserId"].ID == "" {
			t.Errorf("%s missing relationship fields", name)
		}
		function, exists := org.Objects["EmailServicesFunction"].Records[r.Fields["FunctionId"].ID]
		if !exists || !storage.IDsEqual(function.Fields["ApexClassId"].ID, "01p000000000001") {
			t.Errorf("%s function does not reference the deployed handler", name)
		}
		if !storage.IDsEqual(r.Fields["RunAsUserId"].ID, storage.DefaultUserID) {
			t.Errorf("%s does not reference the resolved running user", name)
		}
	}
	if err := resource.ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for id, r := range org.Objects["EmailServicesAddress"].Records {
		if ids[r.Fields["DeveloperName"].String] != id {
			t.Error("reapplying project changed address identity")
		}
	}
	if len(org.Objects["EmailServicesAddress"].Records) != 4 {
		t.Error("reapplying project duplicated addresses")
	}
}

func TestEmailServicesSeedResourceRouteRejectsInvalidMetadataAtomically(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"UnknownUser", "UnknownClass", "TooLong"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				fixture := "../apextest/testdata/conformance/email_services"
				native, err := os.ReadFile(filepath.Join(fixture, "native/api"+strings.TrimSuffix(api, ".0"), "metadata_rejection/org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(native), "\n"+name+"\t") {
					t.Fatal("missing native rejection")
				}
				root := t.TempDir()
				dir := filepath.Join(root, "force-app/main/default/emailservices")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for filename, source := range map[string]string{"Alpha.xml": "project/force-app/main/default/emailservices/MailReceiver.xml", "ZRejected.xml": "rejected/" + name + ".xml"} {
					data, err := os.ReadFile(filepath.Join(fixture, source))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, filename), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"`+api+`"}`), 0600); err != nil {
					t.Fatal(err)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				storage.EnsureDeterministicPlatformData(&org)
				storage.EnsureStandardObject(&org, "ApexClass")
				org.Objects["ApexClass"].Records["01p000000000001"] = storage.Record{ID: "01p000000000001", Object: "ApexClass", Fields: map[string]storage.Value{"Name": storage.StringValue("EmailServiceHandler")}}
				if err := resource.ApplyProject(&org, p); err == nil {
					t.Error("native rejected metadata was accepted")
				}
				if len(org.Objects["EmailServicesFunction"].Records) != 0 || len(org.Objects["EmailServicesAddress"].Records) != 0 {
					t.Error("rejected metadata left partial rows")
				}
			})
		}
	}
}

func TestEmailServicesSeedResourceRouteFreshOrgAndCapturedReader(t *testing.T) {
	root, err := filepath.Abs("../apextest/testdata/conformance/email_services/project")
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	reader := func(path string) ([]byte, error) {
		reads++
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return []byte(strings.ReplaceAll(string(data), "<localPart>mail_receiver</localPart>", "<localPart>captured_receiver</localPart>")), nil
	}
	org := storage.NewOrgState()
	if err := resource.ApplyProject(&org, p, reader); err != nil {
		t.Fatal(err)
	}
	if reads == 0 {
		t.Fatal("email-service metadata reader was bypassed")
	}
	found := false
	for _, r := range org.Objects["EmailServicesAddress"].Records {
		if r.Fields["LocalPart"].String == "captured_receiver" {
			found = true
		}
	}
	if !found {
		t.Error("captured metadata body was not used")
	}
	if len(org.Objects["EmailServicesFunction"].Records) != 2 {
		t.Error("fresh resource org lacks email services")
	}
}

func TestEmailServicesDomainDynamicPutReadOnly(t *testing.T) {
	for _, names := range [][2]string{{"EmailServicesAddress", "EmailDomainName"}, {"emailservicesaddress", "emaildomainname"}} {
		if !storage.StandardFieldDynamicPutReadOnly(names[0], names[1]) {
			t.Errorf("native anonymous and named E005: %s.%s must reject dynamic put", names[0], names[1])
		}
	}
}
