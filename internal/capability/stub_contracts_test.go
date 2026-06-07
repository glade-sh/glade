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
	if report.Totals.WithOrgEvidence == 0 {
		t.Fatalf("expected at least one org-evidence contract")
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

func TestStubContractEvidenceIDStable(t *testing.T) {
	entry := StubBehaviorEntry{
		Type:   "Schema.DescribeSObjectResult",
		Member: "getLocalName",
	}
	if got := stubContractEvidenceID(entry); got != "stub.schema-describesobjectresult.getlocalname" {
		t.Fatalf("evidence id = %q", got)
	}
}

func TestStubContractEvidenceIDIncludesSignature(t *testing.T) {
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
	idA := stubContractEvidenceID(entryA)
	idB := stubContractEvidenceID(entryB)
	if idA == idB {
		t.Fatalf("expected unique evidence IDs for overloads: %q", idA)
	}
	if idA != "stub.date.adderror.sig-string" {
		t.Fatalf("idA = %q", idA)
	}
	if idB != "stub.date.adderror.sig-string-boolean" {
		t.Fatalf("idB = %q", idB)
	}
}
