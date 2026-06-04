package capability

import "testing"

func TestBuildStubContractReport(t *testing.T) {
	report, err := BuildStubContractReport("")
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if report.SchemaVersion != StubContractsSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", report.SchemaVersion, StubContractsSchemaVersion)
	}
	if report.Totals.Entries == 0 || len(report.Entries) == 0 {
		t.Fatalf("expected non-empty entries")
	}
	if report.Totals.WithProbe == 0 {
		t.Fatalf("expected at least one probe-backed contract")
	}
	if report.Totals.ByMode[string(StubContractPassiveDTO)] == 0 {
		t.Fatalf("expected passive-dto contracts")
	}
	foundString := false
	for _, entry := range report.Entries {
		if entry.Type == "String" && entry.Member != "" {
			foundString = true
			if entry.OddityRisk == "" || len(entry.EdgeTags) == 0 {
				t.Fatalf("expected oddity metadata for String member: %#v", entry)
			}
			break
		}
	}
	if !foundString {
		t.Fatalf("expected String member contract entry")
	}
}

func TestClassifyStubContractMode(t *testing.T) {
	tests := []struct {
		name  string
		entry StubBehaviorEntry
		want  StubContractMode
	}{
		{
			name: "unsupported maps to local-contract",
			entry: StubBehaviorEntry{
				Kind:   "method",
				Status: StubBehaviorUnsupported,
			},
			want: StubContractLocalOnly,
		},
		{
			name: "property maps to passive-dto",
			entry: StubBehaviorEntry{
				Kind:   "property",
				Status: StubBehaviorPassiveDefault,
			},
			want: StubContractPassiveDTO,
		},
		{
			name: "unknown method maps to compile-shape",
			entry: StubBehaviorEntry{
				Kind:   "method",
				Status: StubBehaviorUnknown,
			},
			want: StubContractCompileShape,
		},
		{
			name: "schema describe method maps to compile-shape",
			entry: StubBehaviorEntry{
				Type:   "Schema.DescribeFieldResult",
				Member: "getName",
				Kind:   "method",
				Status: StubBehaviorImplemented,
			},
			want: StubContractCompileShape,
		},
		{
			name: "json create parser maps to compile-shape",
			entry: StubBehaviorEntry{
				Type:       "JSON",
				Member:     "createParser",
				Kind:       "method",
				Status:     StubBehaviorImplemented,
				Parameters: []string{"String"},
			},
			want: StubContractCompileShape,
		},
		{
			name: "adderror maps to compile-shape",
			entry: StubBehaviorEntry{
				Type:   "Date",
				Member: "addError",
				Kind:   "method",
				Status: StubBehaviorImplemented,
			},
			want: StubContractCompileShape,
		},
		{
			name: "tail-constructor-like member maps to compile-shape",
			entry: StubBehaviorEntry{
				Type:   "Datetime",
				Member: "datetime",
				Kind:   "method",
				Status: StubBehaviorImplemented,
			},
			want: StubContractCompileShape,
		},
	}
	for _, tc := range tests {
		if got := classifyStubContractMode(tc.entry); got != tc.want {
			t.Fatalf("%s: mode = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestStubContractProbeIDStable(t *testing.T) {
	entry := StubBehaviorEntry{
		Type:   "Schema.DescribeSObjectResult",
		Member: "getLocalName",
	}
	if got := stubContractProbeID(entry); got != "stub.schema-describesobjectresult.getlocalname" {
		t.Fatalf("probe id = %q", got)
	}
}

func TestStubContractProbeIDIncludesSignature(t *testing.T) {
	entryA := StubBehaviorEntry{
		Type:       "Date",
		Member:     "addError",
		Parameters: []string{"String"},
	}
	entryB := StubBehaviorEntry{
		Type:       "Date",
		Member:     "addError",
		Parameters: []string{"String", "Boolean"},
	}
	idA := stubContractProbeID(entryA)
	idB := stubContractProbeID(entryB)
	if idA == idB {
		t.Fatalf("expected unique probe IDs for overloads: %q", idA)
	}
	if idA != "stub.date.adderror.sig-string" {
		t.Fatalf("idA = %q", idA)
	}
	if idB != "stub.date.adderror.sig-string-boolean" {
		t.Fatalf("idB = %q", idB)
	}
}

func TestBuildStubContractProbeManifest(t *testing.T) {
	report := StubContractReport{
		Entries: []StubContractEntry{
			{ID: "String.trim()", Type: "String", Member: "trim", Mode: StubContractOrgDiff, ProbeID: "stub.string.trim"},
			{ID: "CustomDto.value()", Type: "CustomDto", Member: "value", Mode: StubContractPassiveDTO},
			{ID: "Network.call()", Type: "Network", Member: "call", Mode: StubContractLocalOnly},
		},
	}
	core := BuildStubContractProbeManifest(report, "core")
	if len(core) != 1 || core[0].ID != "stub.string.trim" {
		t.Fatalf("core manifest = %#v", core)
	}
	full := BuildStubContractProbeManifest(report, "full")
	if len(full) != 3 {
		t.Fatalf("full manifest len = %d, want 3", len(full))
	}
}
