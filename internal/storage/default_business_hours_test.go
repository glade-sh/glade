package storage

import "testing"

func TestDefaultBusinessHoursPreservesConfiguredDefault(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "BusinessHours")
	configured := Record{ID: ID("01m000000000099"), Object: "BusinessHours", Fields: map[string]Value{"IsDefault": BooleanValue(true), "Name": StringValue("Configured hours"), "TimeZoneSidKey": StringValue("Europe/London")}}
	state := org.Objects["BusinessHours"]
	state.Records = map[ID]Record{configured.ID: configured}
	org.Objects["BusinessHours"] = state
	EnsureDeterministicPlatformData(&org)
	if len(org.Objects["BusinessHours"].Records) != 1 {
		t.Fatal("existing default duplicated")
	}
	got := org.Objects["BusinessHours"].Records[configured.ID]
	if got.Fields["Name"].String != configured.Fields["Name"].String || got.Fields["TimeZoneSidKey"].String != configured.Fields["TimeZoneSidKey"].String {
		t.Fatal("configured default altered")
	}
}

func TestDefaultBusinessHoursSeedIsDeterministicAndIdempotent(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	EnsureDeterministicPlatformData(&org)
	if len(org.Objects["BusinessHours"].Records) != 1 {
		t.Fatal("expected one local default seed")
	}
	for _, row := range org.Objects["BusinessHours"].Records {
		if row.Fields["Name"].String != "Default" {
			t.Fatalf("default name = %q", row.Fields["Name"].String)
		}
		if !row.Fields["IsDefault"].Boolean {
			t.Fatal("default marker missing")
		}
		if row.Fields["TimeZoneSidKey"].String != "America/Los_Angeles" {
			t.Fatalf("default timezone = %q", row.Fields["TimeZoneSidKey"].String)
		}
		for _, day := range []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"} {
			if row.Fields[day+"StartTime"].String != "00:00:00.000Z" || row.Fields[day+"EndTime"].String != "00:00:00.000Z" {
				t.Fatalf("default %s window is not 24x7 zero-to-zero", day)
			}
		}
	}
}
