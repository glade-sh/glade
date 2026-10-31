package storage

import "testing"

// org.tsv records two AutomatedProcess users at API 62 and 67 on both routes.
func TestAutomatedUserTimezoneNativeSeed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial string
		want    string
	}{
		{"new", "", "America/Los_Angeles"},
		{"existing-missing-zone", "missing", "America/Los_Angeles"},
		{"existing-explicit-zone", "Asia/Tokyo", "Asia/Tokyo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := OrgState{}
			if tc.initial != "" {
				fields := map[string]Value{"UserType": StringValue("AutomatedProcess")}
				if tc.initial != "missing" {
					fields["TimeZoneSidKey"] = StringValue(tc.initial)
				}
				org.Objects = map[string]ObjectState{"User": {Records: map[ID]Record{defaultAutomatedProcessUserID: {ID: defaultAutomatedProcessUserID, Object: "User", Fields: fields}}}}
			}
			EnsureDeterministicPlatformData(&org)
			EnsureDeterministicPlatformData(&org)
			got := org.Objects["User"].Records[defaultAutomatedProcessUserID].Fields["TimeZoneSidKey"].String
			if got != tc.want {
				t.Fatalf("automated user zone = %q, want %q", got, tc.want)
			}
			if got := org.Objects["User"].Records[ID(DefaultUserID)].Fields["TimeZoneSidKey"].String; got != "America/Los_Angeles" {
				t.Fatalf("running user zone = %q", got)
			}
		})
	}
}
